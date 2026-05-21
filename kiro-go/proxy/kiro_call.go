package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"kiro-api-proxy/config"
	"net/http"
	"strings"
)

// CallKiroAPI 调用 Kiro API（流式），双端点自动 fallback
func CallKiroAPI(account *config.Account, payload *KiroPayload, callback *KiroStreamCallback) (*UpstreamError, error) {

	if _, err := json.Marshal(payload); err != nil {
		apiDebugLog("[API Error] payload marshal failed: %v", err)
		return nil, err
	}

	// User-Agent（Kiro CLI 格式）
	machineId := account.MachineId
	var userAgent, amzUserAgent string
	if machineId != "" {
		userAgent = fmt.Sprintf("Kiro-Cli/%s ua/2.1 os/linux lang/rust api/codewhispererstreaming cfg/retry-mode/standard m/E %s", KiroVersion, machineId)
		amzUserAgent = fmt.Sprintf("Kiro-Cli/%s os/linux lang/rust %s", KiroVersion, machineId)
	} else {
		userAgent = fmt.Sprintf("Kiro-Cli/%s ua/2.1 os/linux lang/rust api/codewhispererstreaming cfg/retry-mode/standard", KiroVersion)
		amzUserAgent = fmt.Sprintf("Kiro-Cli/%s os/linux lang/rust", KiroVersion)
	}

	// 计算 payload 大小用于日志
	payloadBytes, _ := json.Marshal(payload)
	payloadKB := len(payloadBytes) / 1024
	toolCount := 0
	if payload.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext != nil {
		toolCount = len(payload.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.Tools)
	}
	histLen := len(payload.ConversationState.History)
	apiDebugLog("[API Request] account=%s | payload=%dKB | tools=%d | history=%d msgs",
		account.Email, payloadKB, toolCount, histLen)

	// 大上下文软提醒：仅提示，不拦截
	// 阈值 350KB，避免每轮注入重复提醒
	if payloadKB >= 350 {
		warn := "\n\n[System Note] 当前对话上下文 is large and may cause stream interruptions. Please keep only relevant code snippets and shorten long logs/history."
		content := payload.ConversationState.CurrentMessage.UserInputMessage.Content
		if !strings.Contains(content, "当前对话上下文较大") {
			payload.ConversationState.CurrentMessage.UserInputMessage.Content = content + warn
			payloadBytes, _ = json.Marshal(payload)
			payloadKB = len(payloadBytes) / 1024
			apiDebugLog("[SoftWarn] injected large-context reminder, payload=%dKB", payloadKB)
		}
	}

	// 根据配置排序端点
	endpoints := getSortedEndpoints(config.GetPreferredEndpoint())

	var lastErr error
	var lastUpstreamErr *UpstreamError
	for _, ep := range endpoints {
		// 更新 payload 中的 origin
		payload.ConversationState.CurrentMessage.UserInputMessage.Origin = ep.Origin

		reqBody, _ := json.Marshal(payload)
		req, err := http.NewRequest("POST", ep.URL, bytes.NewReader(reqBody))
		if err != nil {
			lastErr = err
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "*/*")
		req.Header.Set("X-Amz-Target", ep.AmzTarget)
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("X-Amz-User-Agent", amzUserAgent)
		req.Header.Set("x-amzn-kiro-agent-mode", "vibe")
		req.Header.Set("x-amzn-codewhisperer-optout", "true")
		req.Header.Set("Amz-Sdk-Request", "attempt=1; max=3")
		req.Header.Set("Amz-Sdk-Invocation-Id", uuid.New().String())
		req.Header.Set("Authorization", "Bearer "+account.AccessToken)

		resp, err := kiroHttpClient.Do(req)
		if err != nil {
			lastErr = err
			fmt.Printf("[KiroAPI] Endpoint %s failed: %v\n", ep.Name, err)
			apiDebugLog("[API Error] Endpoint %s HTTP request failed: %v", ep.Name, err)
			continue
		}

		if resp.StatusCode == 429 {
			resp.Body.Close()
			fmt.Printf("[KiroAPI] Endpoint %s quota exhausted (429), trying fallback...\n", ep.Name)
			apiDebugLog("[API Error] Endpoint %s quota exhausted (429), trying fallback", ep.Name)
			lastUpstreamErr = &UpstreamError{
				StatusCode: 429,
				Endpoint:   ep.Name,
				Body:       "quota exhausted",
				AccountID:  account.ID,
			}
			lastErr = fmt.Errorf("quota exhausted on %s", ep.Name)
			continue // 试第二端点（独立限流，相当于双倍额度）
		}

		if resp.StatusCode != 200 {
			errBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			upstreamErr := &UpstreamError{
				StatusCode: resp.StatusCode,
				Endpoint:   ep.Name,
				Body:       string(errBody),
				AccountID:  account.ID,
			}
			lastUpstreamErr = upstreamErr
			lastErr = upstreamErr

			// 记录到 debug.log
			errBodyStr := string(errBody)
			if len(errBodyStr) > 500 {
				errBodyStr = errBodyStr[:500] + "...(truncated)"
			}
			apiDebugLog("[API Error] Endpoint %s status=%d | payload=%dKB | response: %s",
				ep.Name, resp.StatusCode, len(reqBody)/1024, errBodyStr)

			// Token 失效自动检测：月额度耗尽或被暂停时标记账号
			errStr := string(errBody)
			if strings.Contains(errStr, "MONTHLY_REQUEST_COUNT") {
				fmt.Printf("[KiroAPI] Account %s MONTHLY_REQUEST_COUNT exhausted, disabling\n", account.ID[:8])
				apiDebugLog("[API Error] Account %s MONTHLY_REQUEST_COUNT exhausted, disabling", account.ID[:8])
				account.Enabled = false
			} else if strings.Contains(errStr, "TEMPORARILY_SUSPENDED") {
				fmt.Printf("[KiroAPI] Account %s TEMPORARILY_SUSPENDED, disabling\n", account.ID[:8])
				apiDebugLog("[API Error] Account %s TEMPORARILY_SUSPENDED, disabling", account.ID[:8])
				account.Enabled = false
			}

			// 认证错误不继续尝试
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				apiDebugLog("[API Error] Auth error %d, not retrying", resp.StatusCode)
				return upstreamErr, upstreamErr
			}
			fmt.Printf("[KiroAPI] Endpoint %s error %d | payload: %dKB | response: %s\n",
				ep.Name, resp.StatusCode, len(reqBody)/1024, string(errBody))
			// Debug: print payload details on 400 errors
			if resp.StatusCode == 400 {
				toolCount := 0
				if payload.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext != nil {
					toolCount = len(payload.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.Tools)
				}
				histLen := len(payload.ConversationState.History)
				contentLen := len(payload.ConversationState.CurrentMessage.UserInputMessage.Content)
				preview := string(reqBody)
				if len(preview) > 500 {
					preview = preview[:500]
				}
				fmt.Printf("[KiroAPI-DEBUG] 400 details | tools=%d | history=%d msgs | content_len=%d | payload_preview: %s\n",
					toolCount, histLen, contentLen, preview)
			}
			continue
		}

		apiDebugLog("[API OK] Endpoint %s connected, starting stream parse", ep.Name)
		err = parseEventStream(resp.Body, callback)
		resp.Body.Close()
		if err != nil {
			apiDebugLog("[API Error] Stream parse error on %s: %v", ep.Name, err)
		}
		return nil, err
	}

	apiDebugLog("[API Error] All endpoints failed, last error: %v", lastErr)
	if lastErr != nil {
		return lastUpstreamErr, lastErr
	}
	return nil, fmt.Errorf("all endpoints failed")
}
