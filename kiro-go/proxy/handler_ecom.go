package proxy

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"kiro-api-proxy/config"
	"kiro-api-proxy/pool"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const ecomUpstream = "https://api.ecomagent.in"

// Shared HTTP client for EcomAgent proxy (connection pooling, TLS reuse)
var ecomClient *http.Client
var ecomClientOnce sync.Once

func getEcomClient() *http.Client {
	ecomClientOnce.Do(func() {
		transport := &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 50,
			IdleConnTimeout:     90 * time.Second,
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: false},
		}
		// Support VPN proxy for reaching ecomagent.in dashboard API
		if proxyURL := os.Getenv("VPN_PROXY_URL"); proxyURL != "" {
			if u, err := url.Parse(proxyURL); err == nil {
				transport.Proxy = http.ProxyURL(u)
				fmt.Printf("[EcomClient] Using VPN proxy: %s\n", proxyURL)
			}
		}
		ecomClient = &http.Client{
			Timeout:   5 * time.Minute,
			Transport: transport,
		}
	})
	return ecomClient
}

// handleEcomProxy proxies a request to api.ecomagent.in via the EcomAgent pool.
// Supports both Claude (/v1/messages) and OpenAI (/v1/chat/completions) format requests —
// the upstream EcomAgent API accepts OpenAI format, so Claude-format requests are
// forwarded as-is (the client is expected to send the correct format for the upstream).
func (h *Handler) handleEcomProxy(w http.ResponseWriter, r *http.Request, uc *UserContext) {
	ecomPool := pool.GetEcomPool()

	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		account := ecomPool.GetNext()
		if account == nil {
			http.Error(w, `{"error":{"type":"api_error","message":"No available EcomAgent accounts"}}`, 503)
			return
		}

		fmt.Printf("[EcomAgent] → %s | account: %s | attempt: %d/%d\n",
			r.URL.Path, account.Email, attempt+1, maxRetries)

		shouldRetry := h.proxyToEcom(w, r, account, uc)

		if shouldRetry {
			ecomPool.ReleaseAccount(account.ID)
			lastErr = fmt.Errorf("429/quota error, retrying with next account")
			fmt.Printf("[EcomAgent-Retry] Account %s got error, trying next (attempt %d/%d)\n",
				account.Email, attempt+1, maxRetries)
			continue
		}
		return // success or non-retryable error, done
	}

	// All retries failed
	errMsg := "All EcomAgent accounts failed"
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	http.Error(w, fmt.Sprintf(`{"error":{"type":"api_error","message":"%s"}}`, errMsg), 503)
}

// proxyToEcom forwards a single request to api.ecomagent.in.
// Returns true if the request should be retried with a different account.
func (h *Handler) proxyToEcom(w http.ResponseWriter, r *http.Request, account *config.EcomAccount, uc *UserContext) (shouldRetry bool) {
	ecomPool := pool.GetEcomPool()
	defer ecomPool.ReleaseAccount(account.ID)
	startTime := time.Now()

	// Read original request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error":{"type":"invalid_request","message":"Failed to read request body"}}`, 400)
		return false
	}

	// Extract model name from request body for logging
	var reqBody struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	json.Unmarshal(body, &reqBody)
	modelName := reqBody.Model
	isStream := reqBody.Stream

	// Determine upstream path
	upstreamPath := r.URL.Path
	upstreamURL := ecomUpstream + upstreamPath
	if r.URL.RawQuery != "" {
		upstreamURL += "?" + r.URL.RawQuery
	}

	// Build upstream request
	upstreamReq, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, strings.NewReader(string(body)))
	if err != nil {
		http.Error(w, `{"error":{"type":"api_error","message":"Failed to create upstream request"}}`, 500)
		return false
	}

	// Set headers — use the account's API key for authorization
	upstreamReq.Header.Set("Authorization", "Bearer "+account.ApiKey)
	upstreamReq.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	if ct := r.Header.Get("Accept"); ct != "" {
		upstreamReq.Header.Set("Accept", ct)
	}
	// Forward anthropic-specific headers
	if v := r.Header.Get("Anthropic-Version"); v != "" {
		upstreamReq.Header.Set("Anthropic-Version", v)
	}

	// Make the request
	client := getEcomClient()
	resp, err := client.Do(upstreamReq)
	if err != nil {
		ecomPool.RecordError(account.ID, false)
		http.Error(w, fmt.Sprintf(`{"error":{"type":"api_error","message":"Upstream error: %s"}}`, err.Error()), 502)
		return false
	}
	defer resp.Body.Close()

	// Check for retryable errors (429 / quota)
	if resp.StatusCode == 429 {
		ecomPool.RecordError(account.ID, true)
		io.Copy(io.Discard, resp.Body) // drain body
		return true                    // retry with next account
	}

	// Record success if 2xx
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		ecomPool.RecordSuccess(account.ID)
	} else {
		ecomPool.RecordError(account.ID, false)
	}

	// Check if streaming response
	respIsStream := strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream")

	durationMs := time.Since(startTime).Milliseconds()
	requestID := r.Header.Get("X-Request-ID")

	if respIsStream {
		h.proxyEcomStream(w, resp, account, uc, modelName, isStream, requestID, durationMs)
	} else {
		h.proxyEcomNonStream(w, resp, account, uc, modelName, isStream, requestID, durationMs)
	}
	return false
}

// proxyEcomStream transparently forwards a streaming SSE response.
func (h *Handler) proxyEcomStream(w http.ResponseWriter, resp *http.Response, account *config.EcomAccount, uc *UserContext, modelName string, isStream bool, requestID string, durationMs int64) {
	ecomPool := pool.GetEcomPool()

	// Copy response headers
	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	flusher, ok := w.(http.Flusher)
	if !ok {
		io.Copy(w, resp.Body)
		return
	}

	// Stream the response while counting tokens
	var totalTokens int
	var inputTokens, outputTokens int
	buf := make([]byte, 32*1024)

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			w.Write(chunk)
			flusher.Flush()

			// Try to extract token counts from SSE data
			totalTokens += extractTokensFromSSE(string(chunk))
		}
		if err != nil {
			break
		}
	}

	// Update stats
	ecomPool.UpdateStats(account.ID, totalTokens)
	h.recordSuccess(0, 0, 0) // global stat

	// Record call log for ecom requests
	h.addCallLogWithKey("ecom", modelName, modelName, account.Email, "ecom",
		inputTokens, outputTokens, isStream, 0,
		"", "", "", requestID, durationMs, uc)
}

// proxyEcomNonStream transparently forwards a non-streaming response.
func (h *Handler) proxyEcomNonStream(w http.ResponseWriter, resp *http.Response, account *config.EcomAccount, uc *UserContext, modelName string, isStream bool, requestID string, durationMs int64) {
	ecomPool := pool.GetEcomPool()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, `{"error":{"type":"api_error","message":"Failed to read upstream response"}}`, 502)
		return
	}

	// Copy response headers
	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)

	// Extract token usage from response
	totalTokens := extractTokensFromResponse(respBody)
	ecomPool.UpdateStats(account.ID, totalTokens)
	h.recordSuccess(0, 0, 0)

	// Extract input/output tokens for logging
	var parsed struct {
		Usage struct {
			InputTokens      int `json:"input_tokens"`
			OutputTokens     int `json:"output_tokens"`
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	var inputTokens, outputTokens int
	if json.Unmarshal(respBody, &parsed) == nil {
		inputTokens = parsed.Usage.InputTokens + parsed.Usage.PromptTokens
		outputTokens = parsed.Usage.OutputTokens + parsed.Usage.CompletionTokens
	}

	// Record call log for ecom requests
	h.addCallLogWithKey("ecom", modelName, modelName, account.Email, "ecom",
		inputTokens, outputTokens, isStream, 0,
		"", "", "", requestID, durationMs, uc)
}

// extractTokensFromResponse extracts total tokens from an OpenAI-format JSON response.
func extractTokensFromResponse(body []byte) int {
	var resp struct {
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
			InputTokens      int `json:"input_tokens"`
			OutputTokens     int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0
	}
	if resp.Usage.TotalTokens > 0 {
		return resp.Usage.TotalTokens
	}
	// Claude format
	return resp.Usage.InputTokens + resp.Usage.OutputTokens
}

// extractTokensFromSSE attempts to parse token usage from SSE event data.
// This is a best-effort extraction from streaming responses.
func extractTokensFromSSE(chunk string) int {
	total := 0
	for _, line := range strings.Split(chunk, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			continue
		}
		var evt struct {
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
				InputTokens      int `json:"input_tokens"`
				OutputTokens     int `json:"output_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal([]byte(data), &evt) == nil {
			if evt.Usage.TotalTokens > 0 {
				total = evt.Usage.TotalTokens // last one wins
			} else if evt.Usage.InputTokens+evt.Usage.OutputTokens > 0 {
				total = evt.Usage.InputTokens + evt.Usage.OutputTokens
			}
		}
	}
	return total
}

// RefreshEcomAccountUsage queries EcomAgent dashboard API for an account's current usage and subscription.
// Requires access_token (JWT) — queries ecomagent.in (dashboard site), not api.ecomagent.in.
func RefreshEcomAccountUsage(acc *config.EcomAccount) error {
	client := getEcomClient()
	token := acc.AccessToken
	if token == "" {
		fmt.Printf("[EcomRefresh] %s → skipped (no access_token)\n", acc.Email)
		return fmt.Errorf("no access_token for %s", acc.Email)
	}

	const dashboardBase = "https://ecomagent.in"
	var usedRequests, usedTokens int
	var requestLimit, tokenLimit, plan string

	// 1. Query account usage: GET /api/account-usage/{account_id}
	usageURL := dashboardBase + "/api/account-usage/" + acc.AccountId
	req, _ := http.NewRequest("GET", usageURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Origin", "https://ecomagent.in")
	req.Header.Set("Referer", "https://ecomagent.in/dashboard")
	resp, err := client.Do(req)
	if err == nil && resp.StatusCode == 200 {
		var data struct {
			Success bool `json:"success"`
			Usage   struct {
				Requests int `json:"requests"`
				Tokens   int `json:"tokens"`
			} `json:"usage"`
		}
		if json.NewDecoder(resp.Body).Decode(&data) == nil && data.Success {
			usedRequests = data.Usage.Requests
			usedTokens = data.Usage.Tokens
		}
		resp.Body.Close()
	} else {
		if resp != nil {
			resp.Body.Close()
		}
		fmt.Printf("[EcomRefresh] %s usage query failed: err=%v\n", acc.Email, err)
	}

	// 2. Query subscription: GET /api/subscription/{account_id}
	subURL := dashboardBase + "/api/subscription/" + acc.AccountId
	req2, _ := http.NewRequest("GET", subURL, nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Origin", "https://ecomagent.in")
	req2.Header.Set("Referer", "https://ecomagent.in/dashboard")
	resp2, err2 := client.Do(req2)
	if err2 == nil && resp2.StatusCode == 200 {
		var data struct {
			Success      bool `json:"success"`
			Subscription struct {
				Plan         string `json:"plan"`
				RequestLimit string `json:"requestLimit"`
				TokenLimit   string `json:"tokenLimit"`
			} `json:"subscription"`
		}
		if json.NewDecoder(resp2.Body).Decode(&data) == nil && data.Success {
			plan = data.Subscription.Plan
			requestLimit = data.Subscription.RequestLimit
			tokenLimit = data.Subscription.TokenLimit
		}
		resp2.Body.Close()
	} else {
		if resp2 != nil {
			resp2.Body.Close()
		}
		fmt.Printf("[EcomRefresh] %s subscription query failed: err=%v\n", acc.Email, err2)
	}

	config.UpdateEcomAccountUpstream(acc.ID, usedRequests, usedTokens, requestLimit, tokenLimit, plan)
	fmt.Printf("[EcomRefresh] %s → requests=%d/%s tokens=%d/%s plan=%s\n",
		acc.Email, usedRequests, requestLimit, usedTokens, tokenLimit, plan)
	return nil
}

// apiRefreshEcomAccounts refreshes upstream usage for all EcomAgent accounts.
func (h *Handler) apiRefreshEcomAccounts(w http.ResponseWriter, _ *http.Request) {
	accounts := config.GetEcomAccounts()
	var successCount, failCount int64
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10) // max 10 concurrent refreshes

	for i := range accounts {
		wg.Add(1)
		go func(acc *config.EcomAccount) {
			defer wg.Done()
			sem <- struct{}{}        // acquire
			defer func() { <-sem }() // release
			if err := RefreshEcomAccountUsage(acc); err != nil {
				atomic.AddInt64(&failCount, 1)
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}(&accounts[i])
	}
	wg.Wait()

	pool.GetEcomPool().Reload()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"refreshed": atomic.LoadInt64(&successCount),
		"failed":    atomic.LoadInt64(&failCount),
	})
}
