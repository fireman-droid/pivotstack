package proxy

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type adminCtxKey string

// adminSessionHashCtxKey 用于在 SSE handler 中拿到 session hash，
// 长连接 loop 定时校验 session 是否仍有效，无效则主动断开。
const adminSessionHashCtxKey adminCtxKey = "adminSessionHash"

const (
	adminSessionCookieName = "admin_session"
	adminSessionTokenBytes = 32
	adminSessionTTL        = 72 * time.Hour
	adminSessionIdleTTL    = 12 * time.Hour
	adminSSETokenTTL       = 5 * time.Minute
)

type adminSession struct {
	TokenHash string
	CSRFToken string
	CreatedAt time.Time
	LastSeen  time.Time
	ExpiresAt time.Time
	IP        string
	UserAgent string
}

type sseToken struct {
	TokenHash   string
	SessionHash string
	Stream      string
	ExpiresAt   time.Time
}

type adminSessionStore struct {
	mu        sync.RWMutex
	sessions  map[string]*adminSession
	sseTokens map[string]*sseToken
	limiter   *loginLimiter
}

func newAdminSessionStore() *adminSessionStore {
	return &adminSessionStore{
		sessions:  make(map[string]*adminSession),
		sseTokens: make(map[string]*sseToken),
		limiter:   newLoginLimiter(),
	}
}

func (s *adminSessionStore) Create(w http.ResponseWriter, r *http.Request) (*adminSession, error) {
	rawToken, err := randomAdminToken()
	if err != nil {
		return nil, err
	}
	csrfToken, err := randomAdminToken()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	sess := &adminSession{
		TokenHash: hashAdminToken(rawToken),
		CSRFToken: csrfToken,
		CreatedAt: now,
		LastSeen:  now,
		ExpiresAt: now.Add(adminSessionTTL),
		IP:        clientIP(r),
		UserAgent: r.UserAgent(),
	}

	s.mu.Lock()
	s.sessions[sess.TokenHash] = sess
	s.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    rawToken,
		Path:     "/",
		MaxAge:   int(adminSessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteStrictMode,
	})

	return sess, nil
}

// isSecureRequest 判断请求是否走 HTTPS。直连 TLS → r.TLS != nil；
// 经 nginx 反代 → X-Forwarded-Proto: https。用于动态决定 cookie 的 Secure 标志，
// 避免 HTTP 部署下浏览器丢弃带 Secure 的 cookie。
func isSecureRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (s *adminSessionStore) Get(r *http.Request) (*adminSession, bool) {
	cookie, err := r.Cookie(adminSessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil, false
	}

	tokenHash := hashAdminToken(cookie.Value)
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[tokenHash]
	if !ok {
		return nil, false
	}
	if now.After(sess.ExpiresAt) || now.Sub(sess.LastSeen) > adminSessionIdleTTL {
		delete(s.sessions, tokenHash)
		return nil, false
	}

	sess.LastSeen = now
	return sess, true
}

// IsValid 检查 session token 是否仍有效（未过期、未被踢出）。
// SSE 长连接 loop 定时调用，配合 InvalidateAll() 在改密后真正踢出所有设备。
func (s *adminSessionStore) IsValid(tokenHash string) bool {
	if tokenHash == "" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[tokenHash]
	if !ok {
		return false
	}
	now := time.Now()
	return !now.After(sess.ExpiresAt) && now.Sub(sess.LastSeen) <= adminSessionIdleTTL
}

func (s *adminSessionStore) Invalidate(tokenHash string) {
	if tokenHash == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, tokenHash)
	for token, t := range s.sseTokens {
		if t.SessionHash == tokenHash {
			delete(s.sseTokens, token)
		}
	}
}

func (s *adminSessionStore) InvalidateAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions = make(map[string]*adminSession)
	s.sseTokens = make(map[string]*sseToken)
}

func (s *adminSessionStore) ClearCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteStrictMode,
	})
}

func (s *adminSessionStore) NewSSEToken(sessionHash, stream string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = adminSSETokenTTL
	}

	rawToken, err := randomAdminToken()
	if err != nil {
		return "", err
	}
	tokenHash := hashAdminToken(rawToken)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[sessionHash]; !ok {
		return "", http.ErrNoCookie
	}
	s.sseTokens[tokenHash] = &sseToken{
		TokenHash:   tokenHash,
		SessionHash: sessionHash,
		Stream:      stream,
		ExpiresAt:   time.Now().Add(ttl),
	}
	return rawToken, nil
}

func (s *adminSessionStore) ConsumeSSEToken(raw, stream string) (string, bool) {
	if raw == "" {
		return "", false
	}

	tokenHash := hashAdminToken(raw)
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	tok, ok := s.sseTokens[tokenHash]
	if !ok {
		return "", false
	}
	delete(s.sseTokens, tokenHash)

	if tok.Stream != stream || now.After(tok.ExpiresAt) {
		return "", false
	}
	sess, ok := s.sessions[tok.SessionHash]
	if !ok || now.After(sess.ExpiresAt) || now.Sub(sess.LastSeen) > adminSessionIdleTTL {
		delete(s.sessions, tok.SessionHash)
		return "", false
	}
	return tok.SessionHash, true
}

func (s *adminSessionStore) StartCleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cleanupExpired()
			s.limiter.cleanupExpired()
		}
	}
}

func (s *adminSessionStore) cleanupExpired() {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	for tokenHash, sess := range s.sessions {
		if now.After(sess.ExpiresAt) || now.Sub(sess.LastSeen) > adminSessionIdleTTL {
			delete(s.sessions, tokenHash)
		}
	}
	for tokenHash, tok := range s.sseTokens {
		if now.After(tok.ExpiresAt) {
			delete(s.sseTokens, tokenHash)
		}
	}
}

func randomAdminToken() (string, error) {
	buf := make([]byte, adminSessionTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashAdminToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// clientIP 用于 admin login 的 IP 速率限制。
// 故意只信任 r.RemoteAddr —— 不读 X-Forwarded-For / CF-Connecting-IP，
// 因为攻击者可以伪造这俩 header 绕过 "同 IP 5 次失败锁 10 分钟" 限速。
// 如果将来部署在可信反代（nginx/cloudflare）后面，需要走显式配置 + 校验
// 代理来源 IP（trusted_proxies）才能启用，否则永远只看 socket 层 IP。
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func (h *Handler) requireAdminSession(w http.ResponseWriter, r *http.Request) (*adminSession, bool) {
	sess, ok := h.adminSessions.Get(r)
	if !ok {
		h.adminSessions.ClearCookie(w, r)
		writeJSONStatus(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return nil, false
	}
	return sess, true
}

// requireSSEToken 验证一次性 SSE token，成功时把 session hash 写入 request.Context()
// 供下游 handleSSEStats / handleSSELogs 长连接 loop 定时校验 session 有效性。
// 返回 (新 request, ok)；ok=false 时调用方直接 return（错误响应已写）。
func (h *Handler) requireSSEToken(w http.ResponseWriter, r *http.Request, path string) (*http.Request, bool) {
	stream := strings.TrimPrefix(path, "/sse/")
	raw := r.URL.Query().Get("sse_token")
	sessionHash, ok := h.adminSessions.ConsumeSSEToken(raw, stream)
	if !ok {
		writeJSONStatus(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired SSE token"})
		return r, false
	}
	return r.WithContext(context.WithValue(r.Context(), adminSessionHashCtxKey, sessionHash)), true
}

// adminSessionHashFromCtx 从 SSE handler 的 request context 里读 session hash。
// 配合 adminSessionStore.IsValid 实现长连接侧的 session invalidation。
func adminSessionHashFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(adminSessionHashCtxKey).(string); ok {
		return v
	}
	return ""
}

// writeJSONStatus writes a JSON body with a status code. Project handlers use
// inline json encoding elsewhere; we localize this helper to keep auth code clean.
func writeJSONStatus(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
