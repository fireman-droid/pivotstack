package proxy

import (
	"bufio"
	"encoding/json"
	"kiro-api-proxy/config"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// GET /admin/api/insights/freeloaders?since=ts&min_calls=5
// since: 活动开始 timestamp（unix sec），不传则用 promotion.StartTs
// 算法：since 之后的"活动期"调用 vs since 之前 7 天"平时"调用
// 对每个 key 输出 score（白嫖度评分 0-10）
func (h *Handler) apiInsightsFreeloaders(w http.ResponseWriter, r *http.Request) {
	since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	if since == 0 {
		if promo := config.GetPromotion(); promo != nil && promo.StartTs > 0 {
			since = promo.StartTs
		}
	}
	if since == 0 {
		// 默认用 24 小时前
		since = time.Now().Unix() - 86400
	}
	minCalls := 5
	if v, _ := strconv.Atoi(r.URL.Query().Get("min_calls")); v > 0 {
		minCalls = v
	}
	normalStart := since - 7*86400 // 平时窗口：since 之前 7 天

	type stat struct {
		Normal  int     // 平时调用次数
		Active  int     // 活动期调用次数
		Credits float64 // 活动期消耗 credits
		CostUSD float64 // 活动期实际付费 USD（享受了活动价的）
	}
	keyStats := map[string]*stat{}

	logPath := filepath.Join(config.GetDataDir(), "call_logs.jsonl")
	f, err := os.Open(logPath)
	if err != nil {
		writeJSON(w, 200, map[string]interface{}{"rows": []interface{}{}})
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var entry CallLog
		if json.Unmarshal(scanner.Bytes(), &entry) != nil {
			continue
		}
		if entry.ApiKeyID == "" {
			continue
		}
		if _, ok := keyStats[entry.ApiKeyID]; !ok {
			keyStats[entry.ApiKeyID] = &stat{}
		}
		s := keyStats[entry.ApiKeyID]
		if entry.Timestamp >= since {
			s.Active++
			s.Credits += entry.Credits
			s.CostUSD += entry.CostUSD
		} else if entry.Timestamp >= normalStart {
			s.Normal++
		}
	}

	apiKeys := config.GetAllApiKeys()
	keyInfoMap := map[string]*config.ApiKeyInfo{}
	for i := range apiKeys {
		keyInfoMap[apiKeys[i].ID] = &apiKeys[i]
	}

	const proPriceOriginal = 1.4
	// 整个请求生命周期内 PivotStackDollarsPerYuan 视为不变，取一次 snapshot 避免多次加锁。
	cnyPerUSDSnap := func() float64 {
		rate := config.GetPivotStackDollarsPerYuan()
		if rate <= 0 {
			return 0
		}
		return 1.0 / rate
	}()
	type row struct {
		KeyID         string  `json:"keyId"`
		Note          string  `json:"note"`
		Score         int     `json:"score"`
		RechargeCNY   float64 `json:"rechargeCNY"`
		Normal        int     `json:"normal"`
		Active        int     `json:"active"`
		Surge         float64 `json:"surge"`
		ActivePaidCNY float64 `json:"activePaidCNY"`
		SavedCNY      float64 `json:"savedCNY"`
	}
	rows := []row{}
	for kid, s := range keyStats {
		if s.Active < minCalls {
			continue
		}
		info := keyInfoMap[kid]
		var rch float64
		var note string
		if info != nil {
			rch = info.TotalRecharged * cnyPerUSDSnap
			note = info.Note
		}
		surge := float64(s.Active)
		if s.Normal > 0 {
			surge = float64(s.Active) / float64(s.Normal)
		}
		// 评分（参考 chat 里 Python 脚本逻辑）
		score := 0
		if rch < 30 {
			score += 3
		} else if rch < 50 {
			score += 2
		} else if rch < 100 {
			score += 1
		}
		if s.Normal == 0 {
			score += 3
		} else if s.Normal < 10 {
			score += 2
		} else if s.Normal < 30 {
			score += 1
		}
		if s.Active > 50 {
			score += 2
		} else if s.Active > 20 {
			score += 1
		}
		if s.Normal > 0 {
			if surge > 10 {
				score += 2
			} else if surge > 3 {
				score += 1
			}
		}
		origUSD := s.Credits * proPriceOriginal
		savedCNY := (origUSD - s.CostUSD) * cnyPerUSDSnap
		rows = append(rows, row{
			KeyID:         kid,
			Note:          note,
			Score:         score,
			RechargeCNY:   rch,
			Normal:        s.Normal,
			Active:        s.Active,
			Surge:         surge,
			ActivePaidCNY: s.CostUSD * cnyPerUSDSnap,
			SavedCNY:      savedCNY,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Score != rows[j].Score {
			return rows[i].Score > rows[j].Score
		}
		return rows[i].SavedCNY > rows[j].SavedCNY
	})

	writeJSON(w, 200, map[string]interface{}{
		"since":       since,
		"normalStart": normalStart,
		"rows":        rows,
		"updated":     time.Now().Unix(),
	})
}
