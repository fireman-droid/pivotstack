package proxy

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"kiro-api-proxy/config"
)

const (
	newAPILoginRefreshSkew = 5 * time.Minute
	newAPISyncTimeout      = 30 * time.Second
	defaultNewAPIMarkup    = 2.0
	defaultNewAPISyncSec   = 3600
)

type providerCache struct {
	Groups    []config.NewAPIGroup
	Models    []config.NewAPIModel
	Tokens    []config.NewAPIToken
	UpdatedAt int64
	mu        sync.RWMutex
}

type NewAPIManager struct {
	handler           *Handler
	client            *NewAPIClient
	caches            sync.Map
	schedulers        sync.Map
	loginSingleflight loginFlightGroup
}

func NewNewAPIManager(handler *Handler) *NewAPIManager {
	return &NewAPIManager{
		handler: handler,
		client:  NewNewAPIClient(),
	}
}

func (m *NewAPIManager) EnsureLogin(ctx context.Context, providerID string, force bool) error {
	p, ok := config.GetNewAPIProvider(providerID)
	if !ok {
		return fmt.Errorf("newapi ensure login: provider %q not found", providerID)
	}
	if !force && p.AccessTokenEnc != "" && p.AccessTokenExpiresAt > time.Now().Add(newAPILoginRefreshSkew).Unix() {
		return nil
	}

	return m.loginSingleflight.Do(providerID, func() error {
		p, ok := config.GetNewAPIProvider(providerID)
		if !ok {
			return fmt.Errorf("newapi ensure login: provider %q not found", providerID)
		}
		if !force && p.AccessTokenEnc != "" && p.AccessTokenExpiresAt > time.Now().Add(newAPILoginRefreshSkew).Unix() {
			return nil
		}
		password, err := config.DecryptSecret(p.PasswordEnc)
		if err != nil {
			return fmt.Errorf("newapi ensure login: decrypt password: %w", err)
		}
		result, err := m.client.Login(ctx, p.BaseURL, p.Username, password)
		if err != nil {
			return fmt.Errorf("newapi ensure login: %w", err)
		}
		tokenEnc, err := config.EncryptSecret(result.AccessToken)
		if err != nil {
			return fmt.Errorf("newapi ensure login: encrypt token: %w", err)
		}
		return updateNewAPIProvider(providerID, func(item *config.NewAPIProvider) {
			item.AccessTokenEnc = tokenEnc
			item.AccessTokenExpiresAt = result.ExpiresAt
			item.UserID = result.UserID
			item.LastSyncError = ""
		})
	})
}

func (m *NewAPIManager) SyncProviderMetadata(ctx context.Context, providerID string) error {
	if err := m.EnsureLogin(ctx, providerID, false); err != nil {
		m.recordProviderSyncError(providerID, err)
		return err
	}
	p, ok := config.GetNewAPIProvider(providerID)
	if !ok {
		return fmt.Errorf("newapi sync: provider %q not found", providerID)
	}
	accessToken, err := config.DecryptSecret(p.AccessTokenEnc)
	if err != nil {
		err = fmt.Errorf("newapi sync: decrypt access token: %w", err)
		m.recordProviderSyncError(providerID, err)
		return err
	}

	syncCtx, cancel := context.WithTimeout(ctx, newAPISyncTimeout)
	defer cancel()

	var (
		models []config.NewAPIModel
		groups []config.NewAPIGroup
		tokens []config.NewAPIToken
	)
	if err := m.fetchMetadataParallel(syncCtx, p, accessToken, &models, &groups, &tokens); err != nil {
		m.recordProviderSyncError(providerID, err)
		return err
	}

	now := time.Now().Unix()
	cache := &providerCache{
		Groups:    copyNewAPIGroups(groups),
		Models:    copyNewAPIModels(models),
		Tokens:    copyNewAPITokens(tokens),
		UpdatedAt: now,
	}
	m.caches.Store(providerID, cache)

	existing := config.GetNewAPIChannels()
	providerChannels := m.materializeNewAPIChannels(existing, p, models, groups, tokens)
	next := make([]config.NewAPIChannel, 0, len(existing)+len(providerChannels))
	for _, ch := range existing {
		if ch.ProviderID != providerID {
			next = append(next, ch)
		}
	}
	next = append(next, providerChannels...)
	if err := config.UpdateNewAPIChannels(next); err != nil {
		err = fmt.Errorf("newapi sync: update channels: %w", err)
		m.recordProviderSyncError(providerID, err)
		return err
	}
	if err := updateNewAPIProvider(providerID, func(item *config.NewAPIProvider) {
		item.LastSyncAt = now
		item.LastSyncError = ""
	}); err != nil {
		return err
	}
	if m.handler != nil {
		m.handler.reloadChannelRouter()
	}
	AuditLog("provider_sync", "system", fmt.Sprintf("id=%s groups=%d models=%d tokens=%d channels=%d", providerID, len(groups), len(models), len(tokens), len(providerChannels)))
	return nil
}

func (m *NewAPIManager) fetchMetadataParallel(ctx context.Context, p config.NewAPIProvider, accessToken string, models *[]config.NewAPIModel, groups *[]config.NewAPIGroup, tokens *[]config.NewAPIToken) error {
	var (
		wg       sync.WaitGroup
		errMu    sync.Mutex
		firstErr error
	)
	setErr := func(err error) {
		if err == nil {
			return
		}
		errMu.Lock()
		defer errMu.Unlock()
		if firstErr == nil {
			firstErr = err
		}
	}

	wg.Add(3)
	go func() {
		defer wg.Done()
		out, err := m.client.FetchPricing(ctx, p.BaseURL)
		if err != nil {
			setErr(err)
			return
		}
		*models = out
	}()
	go func() {
		defer wg.Done()
		out, err := m.client.FetchGroups(ctx, p.BaseURL, accessToken, p.UserID)
		if err != nil {
			setErr(err)
			return
		}
		*groups = out
	}()
	go func() {
		defer wg.Done()
		out, err := m.client.FetchAllTokens(ctx, p.BaseURL, accessToken, p.UserID)
		if err != nil {
			setErr(err)
			return
		}
		*tokens = out
	}()
	wg.Wait()
	return firstErr
}

func (m *NewAPIManager) materializeNewAPIChannels(existing []config.NewAPIChannel, p config.NewAPIProvider, models []config.NewAPIModel, groups []config.NewAPIGroup, tokens []config.NewAPIToken) []config.NewAPIChannel {
	now := time.Now().Unix()
	existingByID := make(map[string]config.NewAPIChannel)
	for _, ch := range existing {
		if ch.ProviderID == p.ID {
			existingByID[ch.ID] = ch
		}
	}

	seen := make(map[string]bool, len(tokens))
	out := make([]config.NewAPIChannel, 0, len(tokens)+len(existingByID))
	for _, token := range tokens {
		id := p.ID + ":tok-" + strconv.Itoa(token.ID)
		seen[id] = true
		ch, exists := existingByID[id]
		if !exists {
			ch = config.NewAPIChannel{
				ID:         id,
				ProviderID: p.ID,
				Alias:      firstNonEmpty(token.Name, token.Group, id),
				Markup:     defaultNewAPIMarkup,
				Enabled:    token.Status == 1,
			}
		}
		ch.ID = id
		ch.ProviderID = p.ID
		ch.UpstreamTokenID = token.ID
		ch.UpstreamTokenName = token.Name
		// 安全：sync 时只在拿到非 masked 的 raw key 时才覆盖加密存储。
		// 上游 /api/token/ list endpoint 永远返回 mask 版（"sk-xxxx••••yyyy"），
		// 如果直接当 raw 加密存了，转发时 decrypt 出 mask 字符串给上游 → 401 "无效的令牌"。
		// 真实 raw key 只能从 CreateToken 响应拿到一次，之后必须 preserve。
		if token.Key != "" && !newAPITokenKeyMasked(token.Key) {
			if enc, err := config.EncryptSecret(token.Key); err == nil {
				ch.UpstreamKeyEnc = enc
			}
		}
		ch.GroupName = token.Group
		ch.Models = modelsForNewAPIGroup(models, token.Group)
		ch.RemainQuota = token.RemainQuota
		ch.UnlimitedQuota = token.UnlimitedQuota
		ch.Status = token.Status
		ch.LastSeenAt = now
		// v6 修复（codex stage 4 audit Critical）：sync 时若 admin 已软删该 channel，
		// 保留 tombstone（DeletedAt + Enabled=false）— 不要因为上游 token 仍存在就复活。
		// 否则 admin 软删后下次 sync 又把它放回路由，违反删除语义。
		if ch.DeletedAt > 0 {
			ch.Enabled = false
		}
		out = append(out, ch)
	}

	for id, ch := range existingByID {
		if seen[id] {
			continue
		}
		ch.Enabled = false
		if ch.DeletedAt == 0 {
			ch.DeletedAt = now
		}
		out = append(out, ch)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (m *NewAPIManager) StartScheduler(providerID string) {
	m.StopScheduler(providerID)
	ctx, cancel := context.WithCancel(context.Background())
	m.schedulers.Store(providerID, cancel)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[newapi] scheduler panic provider=%s panic=%v\n", providerID, r)
			}
		}()
		for {
			p, ok := config.GetNewAPIProvider(providerID)
			if !ok || !p.Enabled {
				return
			}
			// 先 sync 再等：容器启动后立刻预热 cache，
			// 否则首次请求要等 syncIntervalSec+jitter（默认 3600s）才有 pricing 数据，
			// 导致 channel-options / billing reads cache miss → 价格全空。
			if err := m.SyncProviderMetadata(ctx, providerID); err != nil {
				fmt.Printf("[newapi] sync failed provider=%s error=%v\n", providerID, err)
			}
			delay := jitteredSyncDelay(p.SyncIntervalSec)
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return
			case <-timer.C:
			}
		}
	}()
}

func (m *NewAPIManager) StopScheduler(providerID string) {
	if value, ok := m.schedulers.LoadAndDelete(providerID); ok {
		if cancel, ok := value.(context.CancelFunc); ok {
			cancel()
		}
	}
}

func (m *NewAPIManager) StartAllSchedulers() {
	for _, p := range config.GetNewAPIProviders() {
		if p.Enabled {
			m.StartScheduler(p.ID)
		}
	}
}

func (m *NewAPIManager) Cache(providerID string) (*providerCache, bool) {
	value, ok := m.caches.Load(providerID)
	if !ok {
		return nil, false
	}
	cache, ok := value.(*providerCache)
	return cache, ok
}

func (m *NewAPIManager) recordProviderSyncError(providerID string, err error) {
	if err == nil {
		return
	}
	_ = updateNewAPIProvider(providerID, func(item *config.NewAPIProvider) {
		item.LastSyncError = err.Error()
	})
	AuditLog("provider_sync_fail", "system", fmt.Sprintf("id=%s error=%q", providerID, err.Error()))
}

func updateNewAPIProvider(providerID string, mutate func(*config.NewAPIProvider)) error {
	providers := config.GetNewAPIProviders()
	for i := range providers {
		if providers[i].ID == providerID {
			mutate(&providers[i])
			return config.UpdateNewAPIProviders(providers)
		}
	}
	return fmt.Errorf("newapi provider %q not found", providerID)
}

type loginFlightGroup struct {
	mu    sync.Mutex
	calls map[string]*loginFlightCall
}

type loginFlightCall struct {
	done chan struct{}
	err  error
}

func (g *loginFlightGroup) Do(key string, fn func() error) error {
	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[string]*loginFlightCall)
	}
	if call := g.calls[key]; call != nil {
		g.mu.Unlock()
		<-call.done
		return call.err
	}
	call := &loginFlightCall{done: make(chan struct{})}
	g.calls[key] = call
	g.mu.Unlock()

	call.err = fn()
	close(call.done)

	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()
	return call.err
}

func modelsForNewAPIGroup(models []config.NewAPIModel, groupName string) []string {
	if groupName == "" {
		return nil
	}
	var out []string
	for _, model := range models {
		for _, group := range model.EnableGroups {
			if strings.TrimSpace(group) == groupName {
				out = append(out, model.ModelName)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

func jitteredSyncDelay(syncIntervalSec int) time.Duration {
	if syncIntervalSec <= 0 {
		syncIntervalSec = defaultNewAPISyncSec
	}
	base := time.Duration(syncIntervalSec) * time.Second
	jitter := time.Duration(time.Now().UnixNano() % int64(30*time.Second))
	return base + jitter
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func copyNewAPIGroups(in []config.NewAPIGroup) []config.NewAPIGroup {
	if in == nil {
		return nil
	}
	out := make([]config.NewAPIGroup, len(in))
	copy(out, in)
	return out
}

func copyNewAPIModels(in []config.NewAPIModel) []config.NewAPIModel {
	if in == nil {
		return nil
	}
	out := make([]config.NewAPIModel, len(in))
	for i := range in {
		out[i] = in[i]
		if in[i].EnableGroups != nil {
			out[i].EnableGroups = append([]string(nil), in[i].EnableGroups...)
		}
	}
	return out
}

func copyNewAPITokens(in []config.NewAPIToken) []config.NewAPIToken {
	if in == nil {
		return nil
	}
	out := make([]config.NewAPIToken, len(in))
	copy(out, in)
	return out
}
