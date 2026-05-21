package proxy

// Stage 3 — PivotStack 调上游 POST /api/token/ 主动创建 token / DELETE /api/token/{id} 删除。
// 跟 newapi_client.go 拆开避免文件超过 500 行硬约束（codex 审计意见）。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// NewAPICreateTokenRequest 是 PivotStack 调上游 POST /api/token/ 创建 token 的入参。
// Models 在 marshal 时转为逗号分隔 string；ExpiredTime=-1 表示永不过期。
type NewAPICreateTokenRequest struct {
	Name               string
	Group              string
	Models             []string
	UnlimitedQuota     bool
	RemainQuota        int64
	ExpiredTime        int64
	ModelLimitsEnabled bool
	ModelLimits        string
	CrossGroupRetry    bool
	AllowIPs           string
}

// NewAPICreatedToken 是上游 POST /api/token/ 响应 data 部分。
// Key 是完整 token 字符串（不带 sk- 前缀），caller 自行加 sk- 后落库。
type NewAPICreatedToken struct {
	ID  int    `json:"id"`
	Key string `json:"key"`
}

// CreateToken 调上游 POST /api/token/ 创建新的 upstream token。
// 必须传 sessionCookie + userID（cookie session + new-api-user header 鉴权）。
// 返回完整 key（不带 sk- 前缀），caller 负责加 sk- 后加密落库。
// 上游若返回 masked key（含 ****）、空 key、或 id<=0，返回 error 防漏判 / 不可用 token。
func (c *NewAPIClient) CreateToken(ctx context.Context, baseURL, sessionCookie string, userID int, createReq NewAPICreateTokenRequest) (*NewAPICreatedToken, error) {
	endpoint, err := newAPIEndpoint(baseURL, "/api/token/")
	if err != nil {
		return nil, fmt.Errorf("newapi create token: %w", err)
	}
	body, err := buildCreateTokenBody(createReq)
	if err != nil {
		return nil, fmt.Errorf("newapi create token: %w", err)
	}
	req, err := c.newRequest(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("newapi create token: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	setNewAPIAuthHeaders(req, sessionCookie, userID)

	var out struct {
		Success bool               `json:"success"`
		Message string             `json:"message"`
		Data    NewAPICreatedToken `json:"data"`
	}
	if err := c.doJSON(req, "newapi create token", &out); err != nil {
		return nil, err
	}
	if !out.Success {
		return nil, fmt.Errorf("newapi create token: %s", newAPIMessage(out.Message))
	}
	out.Data.Key = strings.TrimSpace(out.Data.Key)
	if out.Data.Key == "" {
		return nil, fmt.Errorf("newapi create token: upstream returned empty key")
	}
	if newAPITokenKeyMasked(out.Data.Key) {
		return nil, fmt.Errorf("newapi create token: upstream returned masked key")
	}
	if out.Data.ID <= 0 {
		return nil, fmt.Errorf("newapi create token: upstream returned invalid id %d", out.Data.ID)
	}
	return &out.Data, nil
}

// buildCreateTokenBody marshal create-token 请求体 — 拆出来避免 CreateToken 超 80 行。
func buildCreateTokenBody(req NewAPICreateTokenRequest) ([]byte, error) {
	return json.Marshal(struct {
		Name               string `json:"name"`
		Group              string `json:"group"`
		Models             string `json:"models"`
		UnlimitedQuota     bool   `json:"unlimited_quota"`
		RemainQuota        int64  `json:"remain_quota"`
		ExpiredTime        int64  `json:"expired_time"`
		ModelLimitsEnabled bool   `json:"model_limits_enabled"`
		ModelLimits        string `json:"model_limits"`
		CrossGroupRetry    bool   `json:"cross_group_retry"`
		AllowIPs           string `json:"allow_ips"`
	}{
		Name:               req.Name,
		Group:              req.Group,
		Models:             strings.Join(req.Models, ","),
		UnlimitedQuota:     req.UnlimitedQuota,
		RemainQuota:        req.RemainQuota,
		ExpiredTime:        req.ExpiredTime,
		ModelLimitsEnabled: req.ModelLimitsEnabled,
		ModelLimits:        req.ModelLimits,
		CrossGroupRetry:    req.CrossGroupRetry,
		AllowIPs:           req.AllowIPs,
	})
}

// DeleteToken 调上游 DELETE /api/token/{id} 删除 upstream token。
// 用于 admin 主动删 PivotStack-created channel 时同步上游清理。
func (c *NewAPIClient) DeleteToken(ctx context.Context, baseURL, sessionCookie string, userID int, tokenID int) error {
	endpoint, err := newAPIEndpoint(baseURL, fmt.Sprintf("/api/token/%d", tokenID))
	if err != nil {
		return fmt.Errorf("newapi delete token: %w", err)
	}
	req, err := c.newRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("newapi delete token: %w", err)
	}
	setNewAPIAuthHeaders(req, sessionCookie, userID)

	var out struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := c.doJSON(req, "newapi delete token", &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("newapi delete token: %s", newAPIMessage(out.Message))
	}
	return nil
}

// newAPITokenKeyMasked 检测上游 token key 是否是 masked 形式（含 4+ 连续 *）。
// new-api list-tokens API 永远返回 masked，但 POST create 应该返回完整 — 用来兜底防漏判。
func newAPITokenKeyMasked(key string) bool {
	return strings.Contains(strings.TrimSpace(key), "****")
}
