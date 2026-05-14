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
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	return sess, nil
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

func (s *adminSessionStore) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
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

func clientIP(r *http.Request) string {
	if cf := r.Header.Get("CF-Connecting-IP"); cf != "" {
		return strings.TrimSpace(cf)
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func (h *Handler) requireAdminSession(w http.ResponseWriter, r *http.Request) (*adminSession, bool) {
	sess, ok := h.adminSessions.Get(r)
	if !ok {
		h.adminSessions.ClearCookie(w)
		writeJSONStatus(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return nil, false
	}
	return sess, true
}

func (h *Handler) requireSSEToken(w http.ResponseWriter, r *http.Request, path string) bool {
	stream := strings.TrimPrefix(path, "/sse/")
	raw := r.URL.Query().Get("sse_token")
	if _, ok := h.adminSessions.ConsumeSSEToken(raw, stream); !ok {
		writeJSONStatus(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired SSE token"})
		return false
	}
	return true
}

// writeJSONStatus writes a JSON body with a status code. Project handlers use
// inline json encoding elsewhere; we localize this helper to keep auth code clean.
func writeJSONStatus(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
