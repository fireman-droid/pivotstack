package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ErrorType 错误类型枚举
type ErrorType string

const (
	ErrorTypeTokenExpired        ErrorType = "token_expired"
	ErrorTypeAccountSuspended    ErrorType = "account_suspended"
	ErrorTypeRateLimit           ErrorType = "rate_limit"
	ErrorTypeUpstreamError       ErrorType = "upstream_error"
	ErrorTypeInvalidRequest      ErrorType = "invalid_request"
	ErrorTypeAuthenticationError ErrorType = "authentication_error"
	ErrorTypeServerError         ErrorType = "server_error"
)

// AppError 统一错误结构
type AppError struct {
	Type      ErrorType  `json:"type"`
	Message   string     `json:"message"`
	Retryable bool       `json:"retryable"`
	Debug     *DebugInfo `json:"debug,omitempty"`
}

// KiroExecError 携带 Kiro 执行错误上下文，供渠道层 / legacy 层决定重试、退款、日志。
//   - Retryable=true && !ResponseStarted → 调用方可换账号重试
//   - !ResponseStarted && !Retryable → caller 需写错误响应 + 退款 + 失败日志
//   - ResponseStarted → 已经流式发送给客户端，**不能**再写新错误且**不应**退款（上游成本已发生）
type KiroExecError struct {
	Err              error
	Retryable        bool
	ResponseStarted  bool
	PayloadKB        int
	UpstreamAppError *AppError
}

func (e *KiroExecError) Error() string { return e.Err.Error() }
func (e *KiroExecError) Unwrap() error { return e.Err }

// DebugInfo 调试信息
type DebugInfo struct {
	AccountID           string `json:"account_id,omitempty"`
	UpstreamStatusCode  int    `json:"upstream_status_code,omitempty"`
	UpstreamEndpoint    string `json:"upstream_endpoint,omitempty"`
	RequestID           string `json:"request_id,omitempty"`
	UpstreamBodySnippet string `json:"upstream_body_snippet,omitempty"`
}

// UpstreamError 上游错误（CallKiroAPI 返回）
type UpstreamError struct {
	StatusCode int
	Endpoint   string
	Body       string
	AccountID  string
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("upstream error: HTTP %d from %s: %s", e.StatusCode, e.Endpoint, truncateStr(e.Body, 100))
}

// ToAppError 将 UpstreamError 转换为 AppError
func (e *UpstreamError) ToAppError(requestID string) *AppError {
	appErr := &AppError{
		Debug: &DebugInfo{
			AccountID:           e.AccountID,
			UpstreamStatusCode:  e.StatusCode,
			UpstreamEndpoint:    e.Endpoint,
			RequestID:           requestID,
			UpstreamBodySnippet: truncateStr(e.Body, 200),
		},
	}

	bodyLower := strings.ToLower(e.Body)

	// 根据状态码和 body 内容映射错误类型
	switch {
	case e.StatusCode == 401 || e.StatusCode == 403:
		if strings.Contains(bodyLower, "expired") || strings.Contains(bodyLower, "invalid") {
			appErr.Type = ErrorTypeTokenExpired
			appErr.Message = "Token expired or invalid. Please re-authenticate."
			appErr.Retryable = false
		} else if strings.Contains(bodyLower, "temporarily_suspended") || strings.Contains(bodyLower, "account suspended") {
			appErr.Type = ErrorTypeAccountSuspended
			appErr.Message = "Account has been suspended."
			appErr.Retryable = false
		} else {
			appErr.Type = ErrorTypeAuthenticationError
			appErr.Message = "Authentication failed."
			appErr.Retryable = false
		}
	case e.StatusCode == 429:
		appErr.Type = ErrorTypeRateLimit
		appErr.Message = "Upstream rate limit exceeded. Please retry later."
		appErr.Retryable = true
	case e.StatusCode >= 500:
		appErr.Type = ErrorTypeUpstreamError
		appErr.Message = "Upstream service error. Please retry later."
		appErr.Retryable = true
	case e.StatusCode == 400:
		appErr.Type = ErrorTypeInvalidRequest
		appErr.Message = "Invalid request format."
		appErr.Retryable = false
	default:
		appErr.Type = ErrorTypeUpstreamError
		appErr.Message = fmt.Sprintf("Upstream returned status %d", e.StatusCode)
		appErr.Retryable = false
	}

	return appErr
}

// NewAppError 创建简单的 AppError
func NewAppError(errType ErrorType, message string, retryable bool, requestID string) *AppError {
	return &AppError{
		Type:      errType,
		Message:   message,
		Retryable: retryable,
		Debug: &DebugInfo{
			RequestID: requestID,
		},
	}
}

// WriteErrorResponse 统一错误响应写入（Admin API 格式）
func WriteErrorResponse(w http.ResponseWriter, appErr *AppError, httpStatus int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if appErr.Debug != nil && appErr.Debug.RequestID != "" {
		w.Header().Set("X-Request-ID", appErr.Debug.RequestID)
	}
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": appErr,
	})
}

// WriteOpenAIError OpenAI 格式错误响应
func WriteOpenAIError(w http.ResponseWriter, appErr *AppError, httpStatus int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if appErr.Debug != nil && appErr.Debug.RequestID != "" {
		w.Header().Set("X-Request-ID", appErr.Debug.RequestID)
	}
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"type":      appErr.Type,
			"message":   appErr.Message,
			"retryable": appErr.Retryable,
		},
		"debug": appErr.Debug,
	})
}

// WriteClaudeError Claude 格式错误响应
func WriteClaudeError(w http.ResponseWriter, appErr *AppError, httpStatus int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if appErr.Debug != nil && appErr.Debug.RequestID != "" {
		w.Header().Set("X-Request-ID", appErr.Debug.RequestID)
	}
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":      appErr.Type,
			"message":   appErr.Message,
			"retryable": appErr.Retryable,
		},
		"debug": appErr.Debug,
	})
}

// WriteClaudeStreamError Claude 流式错误（SSE 格式）
func WriteClaudeStreamError(w http.ResponseWriter, appErr *AppError) {
	errorEvent := map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":      appErr.Type,
			"message":   appErr.Message,
			"retryable": appErr.Retryable,
		},
		"debug": appErr.Debug,
	}
	data, _ := json.Marshal(errorEvent)
	fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// WriteOpenAIStreamError OpenAI 流式错误
func WriteOpenAIStreamError(w http.ResponseWriter, appErr *AppError) {
	errorPayload := map[string]interface{}{
		"error": map[string]interface{}{
			"type":      appErr.Type,
			"message":   appErr.Message,
			"retryable": appErr.Retryable,
		},
		"debug": appErr.Debug,
	}
	data, _ := json.Marshal(errorPayload)
	fmt.Fprintf(w, "data: %s\n\n", data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// truncateStr 截断字符串
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
