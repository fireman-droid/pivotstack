package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"kiro-api-proxy/config"
)

type NewAPIClient struct {
	httpClient *http.Client
	userAgent  string
}

func NewNewAPIClient() *NewAPIClient {
	return &NewAPIClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		userAgent:  "PivotStack-NewAPI/5.0",
	}
}

type LoginResult struct {
	AccessToken string
	UserID      int
	ExpiresAt   int64
}

type NewAPILogEntry struct {
	CreatedAt        int64  `json:"created_at"`
	TokenID          int    `json:"token_id"`            // v5 Phase 4b: primary fingerprint key (reservation 与 upstream 配对)
	ModelName        string `json:"model_name"`
	Model            string `json:"model,omitempty"`     // 部分 new-api 实例用 "model" 而非 "model_name"，logModelName() 兜底
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	Quota            int64  `json:"quota"`
	UseTime          int    `json:"use_time"`
	Group            string `json:"group"`
	TokenName        string `json:"token_name"`
	ID               int64  `json:"id"`
}

func (c *NewAPIClient) Login(ctx context.Context, baseURL, username, password string) (*LoginResult, error) {
	endpoint, err := newAPIEndpoint(baseURL, "/api/user/login")
	if err != nil {
		return nil, fmt.Errorf("newapi login: %w", err)
	}
	body, err := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	if err != nil {
		return nil, fmt.Errorf("newapi login: %w", err)
	}
	req, err := c.newRequest(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("newapi login: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// new-api 用 cookie session 鉴权：login 响应 body 不返 access_token，
	// 而是通过 Set-Cookie: session=... 设服务端 session，后续所有 admin endpoint
	// 用 Cookie + New-Api-User header 鉴权。LoginResult.AccessToken 实际存这个 session 值。
	var out struct {
		Success bool            `json:"success"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	resp, err := c.doJSONWithResponse(req, "newapi login", &out)
	if err != nil {
		return nil, err
	}
	if !out.Success {
		return nil, fmt.Errorf("newapi login: %s", newAPIMessage(out.Message))
	}
	// 抓 data.id（user_id），不同实例字段名兼容。
	var parsed struct {
		ID     int `json:"id"`
		UserID int `json:"user_id"`
	}
	_ = json.Unmarshal(out.Data, &parsed)
	userID := parsed.ID
	if userID == 0 {
		userID = parsed.UserID
	}
	if userID == 0 {
		return nil, fmt.Errorf("newapi login: missing user id (raw data: %s)", strings.TrimSpace(string(out.Data)))
	}

	// 从 Set-Cookie 抽 session 值（new-api gorilla/securecookie，opaque base64）
	var sessionCookie string
	if resp != nil {
		for _, ck := range resp.Cookies() {
			if ck.Name == "session" && ck.Value != "" {
				sessionCookie = ck.Value
				break
			}
		}
	}
	if sessionCookie == "" {
		return nil, fmt.Errorf("newapi login: server did not set 'session' cookie (response headers missing Set-Cookie); admin may need to enable cookie auth on upstream")
	}

	// 默认 cookie 有效期 24h（new-api 服务端常用 maxAge=86400s，调用方会在 ExpiresAt-30min 主动 re-login）
	expiresAt := time.Now().Add(24 * time.Hour).Unix()
	return &LoginResult{
		AccessToken: sessionCookie, // 字段名延用，存的是 cookie session 值
		UserID:      userID,
		ExpiresAt:   expiresAt,
	}, nil
}

func (c *NewAPIClient) FetchPricing(ctx context.Context, baseURL string) ([]config.NewAPIModel, error) {
	endpoint, err := newAPIEndpoint(baseURL, "/api/pricing")
	if err != nil {
		return nil, fmt.Errorf("newapi pricing: %w", err)
	}
	req, err := c.newRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("newapi pricing: %w", err)
	}
	var out struct {
		Success bool                 `json:"success"`
		Message string               `json:"message"`
		Data    []config.NewAPIModel `json:"data"`
	}
	if err := c.doJSON(req, "newapi pricing", &out); err != nil {
		return nil, err
	}
	if !out.Success {
		return nil, fmt.Errorf("newapi pricing: %s", newAPIMessage(out.Message))
	}
	return out.Data, nil
}

func (c *NewAPIClient) FetchGroups(ctx context.Context, baseURL, accessToken string, userID int) ([]config.NewAPIGroup, error) {
	endpoint, err := newAPIEndpoint(baseURL, "/api/user/groups")
	if err != nil {
		return nil, fmt.Errorf("newapi groups: %w", err)
	}
	req, err := c.newRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("newapi groups: %w", err)
	}
	setNewAPIAuthHeaders(req, accessToken, userID)

	var out struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    map[string]struct {
			Desc  string  `json:"desc"`
			Ratio float64 `json:"ratio"`
		} `json:"data"`
	}
	if err := c.doJSON(req, "newapi groups", &out); err != nil {
		return nil, err
	}
	if !out.Success {
		return nil, fmt.Errorf("newapi groups: %s", newAPIMessage(out.Message))
	}
	groups := make([]config.NewAPIGroup, 0, len(out.Data))
	for name, item := range out.Data {
		groups = append(groups, config.NewAPIGroup{
			Name:  name,
			Desc:  item.Desc,
			Ratio: item.Ratio,
		})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups, nil
}

func (c *NewAPIClient) FetchAllTokens(ctx context.Context, baseURL, accessToken string, userID int) ([]config.NewAPIToken, error) {
	const pageSize = 100
	var all []config.NewAPIToken
	for page := 1; ; page++ {
		endpoint, err := newAPIEndpoint(baseURL, "/api/token/")
		if err != nil {
			return nil, fmt.Errorf("newapi tokens: %w", err)
		}
		u, err := url.Parse(endpoint)
		if err != nil {
			return nil, fmt.Errorf("newapi tokens: %w", err)
		}
		q := u.Query()
		q.Set("p", strconv.Itoa(page))
		q.Set("size", strconv.Itoa(pageSize))
		u.RawQuery = q.Encode()

		req, err := c.newRequest(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("newapi tokens: %w", err)
		}
		setNewAPIAuthHeaders(req, accessToken, userID)

		var out struct {
			Success bool            `json:"success"`
			Message string          `json:"message"`
			Data    json.RawMessage `json:"data"`
		}
		if err := c.doJSON(req, "newapi tokens", &out); err != nil {
			return nil, err
		}
		if !out.Success {
			return nil, fmt.Errorf("newapi tokens: %s", newAPIMessage(out.Message))
		}
		items, err := decodeNewAPITokens(out.Data)
		if err != nil {
			return nil, fmt.Errorf("newapi tokens: %w", err)
		}
		if len(items) == 0 {
			break
		}
		all = append(all, items...)
		if len(items) < pageSize {
			break
		}
	}
	return all, nil
}

func (c *NewAPIClient) FetchRecentLogs(ctx context.Context, baseURL, accessToken string, userID int, query map[string]string) ([]NewAPILogEntry, error) {
	endpoint, err := newAPIEndpoint(baseURL, "/api/log/self")
	if err != nil {
		return nil, fmt.Errorf("newapi logs: %w", err)
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("newapi logs: %w", err)
	}
	q := u.Query()
	q.Set("p", "1")
	q.Set("page_size", "20")
	for k, v := range query {
		if strings.TrimSpace(k) != "" && strings.TrimSpace(v) != "" {
			q.Set(k, v)
		}
	}
	u.RawQuery = q.Encode()

	req, err := c.newRequest(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("newapi logs: %w", err)
	}
	setNewAPIAuthHeaders(req, accessToken, userID)

	var out struct {
		Success bool            `json:"success"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := c.doJSON(req, "newapi logs", &out); err != nil {
		return nil, err
	}
	if !out.Success {
		return nil, fmt.Errorf("newapi logs: %s", newAPIMessage(out.Message))
	}
	items, err := decodeNewAPILogEntries(out.Data)
	if err != nil {
		return nil, fmt.Errorf("newapi logs: %w", err)
	}
	return items, nil
}

func (c *NewAPIClient) newRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	return req, nil
}

func (c *NewAPIClient) doJSON(req *http.Request, op string, out any) error {
	_, err := c.doJSONWithResponse(req, op, out)
	return err
}

// doJSONWithResponse 同 doJSON，但返回原 http.Response 让调用方读 Set-Cookie 等 header。
// Login 需要从 Set-Cookie 抽 session cookie（new-api 是 cookie session 鉴权，不是 access_token）。
func (c *NewAPIClient) doJSONWithResponse(req *http.Request, op string, out any) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return resp, fmt.Errorf("%s: upstream HTTP %d: %s", op, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, out); err != nil {
		fmt.Printf("[NewAPIClient] %s raw response: %s\n", op, strings.TrimSpace(string(body)))
		return resp, fmt.Errorf("%s: %w", op, err)
	}
	return resp, nil
}

func newAPIEndpoint(baseURL, path string) (string, error) {
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("base URL must use http or https")
	}
	if u.Host == "" {
		return "", fmt.Errorf("base URL missing host")
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

// setNewAPIAuthHeaders 用 cookie session 鉴权 admin endpoints。
// accessToken 参数实际是 LoginResult.AccessToken 里存的 session cookie 值
// （new-api 是 cookie session 鉴权而不是 Authorization Bearer）。
func setNewAPIAuthHeaders(req *http.Request, sessionCookie string, userID int) {
	req.Header.Set("New-Api-User", strconv.Itoa(userID))
	if sessionCookie != "" {
		req.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	}
}

func newAPIMessage(message string) string {
	if strings.TrimSpace(message) == "" {
		return "upstream returned success=false"
	}
	return message
}

func decodeNewAPITokens(raw json.RawMessage) ([]config.NewAPIToken, error) {
	var direct []config.NewAPIToken
	if err := json.Unmarshal(raw, &direct); err == nil {
		return direct, nil
	}
	var wrapped struct {
		Items  []config.NewAPIToken `json:"items"`
		Tokens []config.NewAPIToken `json:"tokens"`
		Data   []config.NewAPIToken `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, err
	}
	switch {
	case wrapped.Items != nil:
		return wrapped.Items, nil
	case wrapped.Tokens != nil:
		return wrapped.Tokens, nil
	default:
		return wrapped.Data, nil
	}
}

func decodeNewAPILogEntries(raw json.RawMessage) ([]NewAPILogEntry, error) {
	var direct []NewAPILogEntry
	if err := json.Unmarshal(raw, &direct); err == nil {
		return direct, nil
	}
	var wrapped struct {
		Items []NewAPILogEntry `json:"items"`
		Logs  []NewAPILogEntry `json:"logs"`
		Data  []NewAPILogEntry `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, err
	}
	switch {
	case wrapped.Items != nil:
		return wrapped.Items, nil
	case wrapped.Logs != nil:
		return wrapped.Logs, nil
	default:
		return wrapped.Data, nil
	}
}
