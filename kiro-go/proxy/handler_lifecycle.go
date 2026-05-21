package proxy

import (
	"context"
	"fmt"
	"kiro-api-proxy/auth"
	"kiro-api-proxy/config"
	"kiro-api-proxy/pool"
	"mime"
	"time"
)

func init() {
	mime.AddExtensionType(".css", "text/css")
	mime.AddExtensionType(".js", "application/javascript")
	mime.AddExtensionType(".json", "application/json")
	mime.AddExtensionType(".svg", "image/svg+xml")
	mime.AddExtensionType(".woff", "font/woff")
	mime.AddExtensionType(".woff2", "font/woff2")
	mime.AddExtensionType(".ttf", "font/ttf")
	mime.AddExtensionType(".png", "image/png")
	mime.AddExtensionType(".jpg", "image/jpeg")
	mime.AddExtensionType(".ico", "image/x-icon")
}

func NewHandler() *Handler {
	totalReq, successReq, failedReq, totalTokens, totalCredits := config.GetStats()
	h := &Handler{
		pool:                pool.GetPool(),
		totalRequests:       int64(totalReq),
		successRequests:     int64(successReq),
		failedRequests:      int64(failedReq),
		totalTokens:         int64(totalTokens),
		totalCredits:        totalCredits,
		startTime:           time.Now().Unix(),
		stopRefresh:         make(chan struct{}),
		stopStatsSaver:      make(chan struct{}),
		apiKeyStats:         make(map[string]*ApiKeyStats),
		logSubscribers:      make(map[chan CallLog]bool),
		logPersistCh:        make(chan CallLog, callLogPersistQueueSize),
		logPersistStop:      make(chan struct{}),
		logPersistDone:      make(chan struct{}),
		creditPredictor:     newCreditPredictor(200, 0.3),
		proCreditPredictor:  newCreditPredictor(200, 0.3),
		freeCreditPredictor: newCreditPredictor(200, 0.3),
		adminSessions:       newAdminSessionStore(),
	}
	h.newapiManager = NewNewAPIManager(h)
	h.newapiReconciler = NewNewAPIReconciler(h.newapiManager)
	h.newapiReconciler.Start(context.Background())
	h.startLogPersistWorker()
	h.reloadChannelRouter()
	// 从磁盘恢复历史日志和 CreditPredictor
	h.loadLogsFromDisk()
	// 启动日志自动清理（每6小时清理超过7天的）
	h.startLogCleanupTicker()
	// 启动后台刷新
	go h.backgroundRefresh()
	// 启动后台统计保存 (每5分钟批量写入)
	go h.backgroundStatsSaver()
	// 启动 admin session / SSE token / login limiter 后台清理
	go h.adminSessions.StartCleanup(context.Background())
	startUserLoginLimiterCleanup(context.Background())
	h.newapiManager.StartAllSchedulers()
	return h
}

// Stop 优雅停止后台 worker。main.go 在 SIGTERM 时调用。
// log persist worker drain：把内存队列里的 entry 强制落盘后再退出。
func (h *Handler) Stop() {
	if h == nil {
		return
	}
	if h.newapiReconciler != nil {
		h.newapiReconciler.Stop()
	}
	h.stopLogPersistWorker()
}

// reloadChannelRouter 从最新的 config.Series + config.Channels 重建渠道路由器并原子替换。
// admin API 改 channels 或 series 后必须调用此方法让新配置生效。
// v4：原子读 series+channels（GetRoutingConfig）避免读取过程中 admin 半路改 config。
func (h *Handler) reloadChannelRouter() {
	series, channels := config.GetRoutingConfig()
	h.channelRouter.Store(NewChannelRouter(series, channels, h,
		WithNewAPIChannels(config.GetNewAPIChannels()),
		WithDirectChannels(config.GetDirectChannels()),
		WithNewAPIProviders(config.GetNewAPIProviders()),
		WithChannelGroups(config.GetActiveChannelGroups()),
	))
}

// currentChannelRouter 返回当前渠道路由器（线程安全）。
func (h *Handler) currentChannelRouter() *ChannelRouter {
	return h.channelRouter.Load()
}

// backgroundRefresh 后台定时刷新账户信息
func (h *Handler) backgroundRefresh() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	time.Sleep(5 * time.Second)
	h.refreshAllAccounts()
	h.refreshModelsCache()

	for {
		select {
		case <-ticker.C:
			h.refreshAllAccounts()
			h.refreshModelsCache()
		case <-h.stopRefresh:
			return
		}
	}
}

// refreshAllAccounts 刷新所有账户信息
func (h *Handler) refreshAllAccounts() {
	accounts := config.GetAccounts()
	refreshed := 0
	failed := 0
	skipped := 0

	for i := range accounts {
		account := &accounts[i]
		if !account.Enabled || (account.AccessToken == "" && account.RefreshToken == "") {
			fmt.Printf("[BackgroundRefresh] SKIP %s: enabled=%v, atLen=%d, rtLen=%d\n",
				account.Email, account.Enabled, len(account.AccessToken), len(account.RefreshToken))
			skipped++
			continue
		}

		// 主动刷新 token：过期前 30 分钟或已过期
		needsRefresh := false
		if account.ExpiresAt == 0 {
			needsRefresh = true // ExpiresAt 未设置，强制刷新
		} else if time.Now().Unix() > account.ExpiresAt-1800 {
			needsRefresh = true // 30 分钟内过期或已过期
		}

		if needsRefresh {
			fmt.Printf("[BackgroundRefresh] Refreshing token for %s (expiresAt=%d, now=%d)...\n",
				account.Email, account.ExpiresAt, time.Now().Unix())

			// 最多重试 2 次
			var lastErr error
			for retry := 0; retry < 2; retry++ {
				newAccessToken, newRefreshToken, newExpiresAt, err := auth.RefreshToken(account)
				if err != nil {
					lastErr = err
					fmt.Printf("[BackgroundRefresh] Retry %d failed for %s: %v\n", retry+1, account.Email, err)
					time.Sleep(3 * time.Second)
					continue
				}
				account.AccessToken = newAccessToken
				if newRefreshToken != "" {
					account.RefreshToken = newRefreshToken
				}
				account.ExpiresAt = newExpiresAt
				config.UpdateAccountToken(account.ID, newAccessToken, newRefreshToken, newExpiresAt)
				h.pool.UpdateToken(account.ID, newAccessToken, newRefreshToken, newExpiresAt)
				fmt.Printf("[BackgroundRefresh] Token refreshed OK for %s, new expiresAt=%d (in %dm)\n",
					account.Email, newExpiresAt, (newExpiresAt-time.Now().Unix())/60)
				lastErr = nil
				refreshed++
				break
			}
			if lastErr != nil {
				fmt.Printf("[BackgroundRefresh] Token refresh FAILED for %s after retries: %v\n", account.Email, lastErr)
				failed++
				continue
			}
		}

		info, err := RefreshAccountInfo(account)
		if err != nil {
			fmt.Printf("[BackgroundRefresh] Failed to refresh info for %s: %v\n", account.Email, err)
			continue
		}

		config.UpdateAccountInfo(account.ID, *info)
	}
	h.pool.Reload()
	fmt.Printf("[BackgroundRefresh] Done: %d refreshed, %d failed, %d skipped, %d total\n",
		refreshed, failed, skipped, len(accounts))
}
