package config

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strings"
	"sync"
	"time"
)

// 全局支持模型表（main.go 启动时通过 SetSupportedModels 注入，避免 config import proxy 循环）
var (
	supportedModels   = map[string][]string{}
	supportedModelsMu sync.RWMutex
)

// SetSupportedModels 由 main.go 在启动时调用一次，注入 proxy.SupportedModels() 的结果。
// 迁移函数 / 默认价兜底等逻辑会读这个表。
func SetSupportedModels(m map[string][]string) {
	supportedModelsMu.Lock()
	defer supportedModelsMu.Unlock()
	supportedModels = make(map[string][]string, len(m))
	for k, v := range m {
		cp := make([]string, len(v))
		copy(cp, v)
		supportedModels[k] = cp
	}
}

// GetSupportedModels 返回模型→pool 表副本。
func GetSupportedModels() map[string][]string {
	supportedModelsMu.RLock()
	defer supportedModelsMu.RUnlock()
	out := make(map[string][]string, len(supportedModels))
	for k, v := range supportedModels {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// normalizeModelKey 把 model 名标准化用于 map key 匹配（小写 + '-/.' 互换）。
func normalizeModelKey(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), "-", ".")
}

// ===========================================================================
// 备份：迁移触发时把当前 config.json 复制一份带时间戳的副本
// ===========================================================================

var (
	backupOnce       sync.Once
	backupOnceErr    error
	backupExecutedAt time.Time
)

// backupConfigBeforeMigrate 在迁移触发的当前进程内只执行一次（sync.Once 保护）。
// 备份文件名：<configPath>.before-pricing-refactor-YYYYMMDD-HHMMSS
// 失败时打 stderr 但不阻塞迁移（备份是保险措施，不能因为它失败而错过迁移）。
func backupConfigBeforeMigrate() error {
	backupOnce.Do(func() {
		if cfgPath == "" {
			backupOnceErr = fmt.Errorf("cfgPath empty, skip backup")
			return
		}
		backup := cfgPath + ".before-pricing-refactor-" + time.Now().Format("20060102-150405")
		src, err := os.Open(cfgPath)
		if err != nil {
			backupOnceErr = fmt.Errorf("open src: %w", err)
			return
		}
		defer src.Close()
		dst, err := os.OpenFile(backup, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			backupOnceErr = fmt.Errorf("create backup %s: %w", backup, err)
			return
		}
		defer dst.Close()
		if _, err := io.Copy(dst, src); err != nil {
			backupOnceErr = fmt.Errorf("copy: %w", err)
			return
		}
		backupExecutedAt = time.Now()
		log.Printf("[Migrate] config backup written: %s", backup)
	})
	return backupOnceErr
}

// ===========================================================================
// PricingConfig 迁移：v1 (PoolPriceUSD × ModelMultiplier) → v2 (ModelPrices)
// ===========================================================================

// MigratePricingToModelLevel 把旧字段 ProPoolPriceUSD/FreePoolPriceUSD/ModelMultipliers
// 转成 ModelPrices map。幂等：ModelPrices 已存在 → 跳过；全空 → 跳过（新装走默认）。
//
// 守护：迁移完成后对每个 ModelPrices[m] 反算旧值，差距 > 0.001 → 中止 + 日志。
//
// 返回 true 表示发生了迁移（调用方应触发持久化）。
func MigratePricingToModelLevel(p *PricingConfig) bool {
	if p == nil {
		return false
	}
	if len(p.ModelPrices) > 0 {
		return false // 已迁移
	}
	hasOld := p.ProPoolPriceUSD > 0 || p.FreePoolPriceUSD > 0 || len(p.ModelMultipliers) > 0
	if !hasOld {
		return false // 新装，无东西可迁
	}

	// 备份原 config（保险措施）
	if err := backupConfigBeforeMigrate(); err != nil {
		log.Printf("[Migrate WARNING] backup failed: %v", err)
	}

	models := GetSupportedModels()
	if len(models) == 0 {
		log.Printf("[Migrate WARNING] supportedModels empty, skip pricing migration")
		return false
	}

	tmp := make(map[string]float64)
	for pool, ms := range models {
		var poolPrice float64
		if pool == "pro" {
			poolPrice = p.ProPoolPriceUSD
		} else {
			poolPrice = p.FreePoolPriceUSD
		}
		for _, m := range ms {
			key := strings.ToLower(m)
			mult := lookupMultiplierByName(p.ModelMultipliers, key)
			if mult <= 0 {
				mult = 1.0
			}
			tmp[key] = poolPrice * mult
		}
	}

	// 守护：每个 ModelPrices[m] 必须等于"按 ResolveModelPool(m) 取的 pool_price × multiplier(m)"
	// 这个守护跟 tmp 的构造逻辑相同，主要是冗余校验（防代码改坏）+ 日志可见
	for model, newPrice := range tmp {
		expected := computeLegacyPriceUSD(p, model)
		if math.Abs(newPrice-expected) > 0.001 {
			log.Printf("[Migrate ERROR] price mismatch %s: new=%.4f expected=%.4f, ABORT migration",
				model, newPrice, expected)
			return false
		}
	}

	p.ModelPrices = tmp
	if p.DefaultProPriceUSD == 0 {
		p.DefaultProPriceUSD = p.ProPoolPriceUSD
	}
	if p.DefaultFreePriceUSD == 0 {
		p.DefaultFreePriceUSD = p.FreePoolPriceUSD
	}
	logPricingMigration(p)
	return true
}

// lookupMultiplierByName 在 ModelMultipliers map 里找 model 的倍率（'-/.' 互换、大小写不敏感）。
// 找不到返回 0.
func lookupMultiplierByName(m map[string]float64, model string) float64 {
	if len(m) == 0 || model == "" {
		return 0
	}
	target := normalizeModelKey(model)
	if v, ok := m[strings.ToLower(strings.TrimSpace(model))]; ok && v > 0 {
		return v
	}
	for k, v := range m {
		if v <= 0 {
			continue
		}
		if normalizeModelKey(k) == target {
			return v
		}
	}
	return 0
}

// computeLegacyPriceUSD 按旧公式（PoolPriceUSD × ModelMultiplier）算某 model 的价格。
// shadow 校验和迁移守护用。
func computeLegacyPriceUSD(p *PricingConfig, model string) float64 {
	pool := resolveModelPoolFromMap(model)
	var pool_price float64
	if pool == "pro" {
		pool_price = p.ProPoolPriceUSD
	} else {
		pool_price = p.FreePoolPriceUSD
	}
	mult := lookupMultiplierByName(p.ModelMultipliers, model)
	if mult <= 0 {
		mult = 1.0
	}
	return pool_price * mult
}

// resolveModelPoolFromMap 在 config 包里独立判断 pool（避免 import proxy）。
// 用 supportedModels 表（main.go 注入）反查，找不到就关键字 fallback。
func resolveModelPoolFromMap(model string) string {
	target := normalizeModelKey(model)
	models := GetSupportedModels()
	for pool, ms := range models {
		for _, m := range ms {
			if normalizeModelKey(m) == target {
				return pool
			}
		}
	}
	// fallback 关键字（跟 proxy.ResolveModelPool 同款）
	low := strings.ToLower(model)
	if strings.Contains(low, "4.6") || strings.Contains(low, "4-6") ||
		strings.Contains(low, "4.7") || strings.Contains(low, "4-7") ||
		strings.Contains(low, "opus") {
		return "pro"
	}
	return "free"
}

func logPricingMigration(p *PricingConfig) {
	keys := make([]string, 0, len(p.ModelPrices))
	for k := range p.ModelPrices {
		keys = append(keys, k)
	}
	log.Printf("[Migrate] pricing v1→v2: from ProPool=$%.4f FreePool=$%.4f Multipliers=%d entries → ModelPrices=%d entries (DefaultPro=$%.4f, DefaultFree=$%.4f)",
		p.ProPoolPriceUSD, p.FreePoolPriceUSD, len(p.ModelMultipliers),
		len(p.ModelPrices), p.DefaultProPriceUSD, p.DefaultFreePriceUSD)
	if len(keys) > 0 {
		preview, _ := json.Marshal(p.ModelPrices)
		log.Printf("[Migrate]   ModelPrices = %s", string(preview))
	}
}

// ===========================================================================
// PromotionConfig 迁移：v1 (ProPoolPriceUSD/FreePoolPriceUSD) → v2 (ModelPrices)
// ===========================================================================

// MigratePromotionToModelLevel 同样迁移活动配置。
// 旧逻辑：活动期所有 PRO 模型一刀切 ProPoolPriceUSD（× model 倍率仍生效）
// 新逻辑：活动期 ModelPrices[m] 优先；未列出则按 pool 用 DefaultProPriceUSD/DefaultFreePriceUSD
//
// 迁移策略：v1 没有 per-model 活动价的概念，只有"池一刀切"——直接把 ProPoolPriceUSD/FreePoolPriceUSD
// 搬到 DefaultProPriceUSD/DefaultFreePriceUSD（不填 ModelPrices）。这等价于"所有 PRO 模型用兜底"。
//
// 返回 true 表示发生了迁移。
func MigratePromotionToModelLevel(p *PromotionConfig) bool {
	if p == nil {
		return false
	}
	if len(p.ModelPrices) > 0 || p.DefaultProPriceUSD > 0 || p.DefaultFreePriceUSD > 0 {
		return false // 已迁移或新装
	}
	hasOld := p.ProPoolPriceUSD > 0 || p.FreePoolPriceUSD > 0
	if !hasOld {
		return false
	}

	if err := backupConfigBeforeMigrate(); err != nil {
		log.Printf("[Migrate WARNING] backup failed: %v", err)
	}

	p.DefaultProPriceUSD = p.ProPoolPriceUSD
	p.DefaultFreePriceUSD = p.FreePoolPriceUSD
	log.Printf("[Migrate] promotion v1→v2: DefaultPro=$%.4f DefaultFree=$%.4f (ModelPrices=empty, all models fallback to default)",
		p.DefaultProPriceUSD, p.DefaultFreePriceUSD)
	return true
}
