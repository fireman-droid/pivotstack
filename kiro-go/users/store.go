package users

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"kiro-api-proxy/config"
)

// Store 进程级单例。
type Store struct {
	mu   sync.RWMutex
	file UsersFile
	path string
}

var (
	defaultStore *Store
	once         sync.Once
)

// Default 取单例（首次调用时 lazy load）。
func Default() *Store {
	once.Do(func() {
		defaultStore = newStore(filepath.Join(config.GetDataDir(), "users.json"))
	})
	return defaultStore
}

func newStore(path string) *Store {
	s := &Store{path: path}
	if err := s.load(); err != nil {
		fmt.Printf("[users] load %s failed: %v (starting empty)\n", path, err)
		s.file = UsersFile{SchemaVersion: CurrentSchemaVersion}
	}
	return s
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.file = UsersFile{SchemaVersion: CurrentSchemaVersion}
			return nil
		}
		return err
	}
	if len(data) == 0 {
		s.file = UsersFile{SchemaVersion: CurrentSchemaVersion}
		return nil
	}
	var f UsersFile
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	if f.SchemaVersion == 0 {
		f.SchemaVersion = 1
	}
	if f.Users == nil {
		f.Users = []User{}
	}
	if f.InviteCodes == nil {
		f.InviteCodes = []InviteCode{}
	}
	s.file = f
	if s.file.SchemaVersion < CurrentSchemaVersion {
		if err := s.migrateLocked(s.file.SchemaVersion); err != nil {
			return fmt.Errorf("schema migration: %w", err)
		}
	}
	return nil
}

// backupFile copies `path` to `path<suffix>.<unix>` (no-op if file missing).
func backupFile(path, suffix string) error {
	if path == "" {
		return errors.New("backup path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	backupPath := fmt.Sprintf("%s%s.%d", path, suffix, time.Now().Unix())
	return os.WriteFile(backupPath, data, 0o600)
}

// appendWalletMigrationAudit writes one JSON line to data/audit.log describing a v3 migration event.
func appendWalletMigrationAudit(action string, fields map[string]any) {
	if fields == nil {
		fields = map[string]any{}
	}
	fields["ts"] = time.Now().Unix()
	fields["action"] = action
	line, err := json.Marshal(fields)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[users.wallet_migration] marshal audit failed: %v\n", err)
		return
	}
	path := filepath.Join(config.GetDataDir(), "audit.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[users.wallet_migration] mkdir audit failed: %v\n", err)
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[users.wallet_migration] open audit failed: %v\n", err)
		return
	}
	defer f.Close()
	_, _ = f.Write(append(line, '\n'))
}

// migrateLocked v1→v2: 填充缺失 username（从 email 前缀派生，冲突 suffix）+ 修复重复 username。
// v2→v3: 把 bound non-child key 的 4 个钱字段累加到 User，并清零 key（孤儿/子卡保持不动）。
// 调用方持有写锁（load 时单线程，安全）。
func (s *Store) migrateLocked(fromVersion int) error {
	if fromVersion >= CurrentSchemaVersion {
		return nil
	}
	// v1 → v2: username 升级为唯一登录键
	if fromVersion < 2 {
		used := make(map[string]int, len(s.file.Users))
		for i := range s.file.Users {
			candidate := NormalizeUsername(s.file.Users[i].Username)
			if candidate == "user" || candidate == "" {
				// username 缺失 / 全非法 → 从 email 前缀派生
				email := s.file.Users[i].Email
				if at := strings.IndexByte(email, '@'); at > 0 {
					candidate = NormalizeUsername(email[:at])
				} else {
					candidate = "user"
				}
			}
			final := candidate
			for {
				if _, dup := used[final]; !dup {
					break
				}
				used[candidate]++
				final = fmt.Sprintf("%s-%d", candidate, used[candidate]+1)
			}
			used[final] = 1
			s.file.Users[i].Username = final
		}
	}

	// v2 → v3: 把 bound non-child key 的 4 个钱字段累加到 User，再清零 key。
	// 孤儿 key（未在任何 user.ApiKeyIDs 中）和 reseller 子卡（ParentKeyID != ""）保持 key-level。
	// 幂等：user 已有非零钱字段时跳过累加。清零失败不回滚（user wallet 已是事实源），写审计。
	if fromVersion < 3 {
		if err := backupFile(s.path, ".wallet-v3.bak"); err != nil {
			appendWalletMigrationAudit("wallet_v3_backup_users_failed", map[string]any{
				"path": s.path,
				"err":  err.Error(),
			})
		}
		if err := config.BackupCurrentConfig("wallet-v3"); err != nil {
			appendWalletMigrationAudit("wallet_v3_backup_config_failed", map[string]any{
				"err": err.Error(),
			})
		}
		for i := range s.file.Users {
			u := &s.file.Users[i]
			if u.Balance != 0 || u.GiftBalance != 0 || u.TotalRecharged != 0 || u.TotalGifted != 0 {
				appendWalletMigrationAudit("wallet_v3_skip_existing_user_wallet", map[string]any{
					"userID":         u.ID,
					"email":          u.Email,
					"balance":        u.Balance,
					"giftBalance":    u.GiftBalance,
					"totalRecharged": u.TotalRecharged,
					"totalGifted":    u.TotalGifted,
				})
				continue
			}

			var ownedKeyIDs []string
			for _, keyID := range u.ApiKeyIDs {
				k := config.FindApiKeyByID(keyID)
				if k == nil || k.ParentKeyID != "" {
					continue
				}
				u.Balance += k.Balance
				u.GiftBalance += k.GiftBalance
				u.TotalRecharged += k.TotalRecharged
				u.TotalGifted += k.TotalGifted
				ownedKeyIDs = append(ownedKeyIDs, k.ID)
			}

			appendWalletMigrationAudit("wallet_v3_moved_to_user", map[string]any{
				"userID":         u.ID,
				"email":          u.Email,
				"keyCount":       len(ownedKeyIDs),
				"balance":        u.Balance,
				"giftBalance":    u.GiftBalance,
				"totalRecharged": u.TotalRecharged,
				"totalGifted":    u.TotalGifted,
			})

			for _, kid := range ownedKeyIDs {
				if err := config.ClearKeyWalletFields(kid); err != nil {
					appendWalletMigrationAudit("wallet_v3_clear_key_failed", map[string]any{
						"userID": u.ID,
						"keyID":  kid,
						"err":    err.Error(),
					})
				}
			}
		}
	}

	s.file.SchemaVersion = CurrentSchemaVersion
	// 写盘（调用方持锁；flushLocked 会更新 UpdatedAt）
	return s.flushLocked()
}

func (s *Store) flushLocked() error {
	s.file.SchemaVersion = CurrentSchemaVersion
	s.file.UpdatedAt = time.Now().Unix()
	data, err := json.MarshalIndent(s.file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// ───────── Users ─────────

// FindByEmail 返回邮箱匹配的 User 副本。
func (s *Store) FindByEmail(email string) (User, bool) {
	email = NormalizeEmail(email)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.file.Users {
		if u.Email == email {
			return u, true
		}
	}
	return User{}, false
}

// FindByUsername 返回用户名匹配的 User 副本（v2 新增：username 作为主登录键）。
func (s *Store) FindByUsername(username string) (User, bool) {
	username = NormalizeUsername(username)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.file.Users {
		if u.Username == username {
			return u, true
		}
	}
	return User{}, false
}

// FindByApiKeyID 返回拥有指定 ApiKey 的 User（用于 user-keys CRUD 的 ownership 校验）。
func (s *Store) FindByApiKeyID(keyID string) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.file.Users {
		for _, id := range u.ApiKeyIDs {
			if id == keyID {
				return u, true
			}
		}
	}
	return User{}, false
}

// NextAvailableUsername 返回 base 或 base-N 的下一个未占用 username（在读锁内扫描）。
// 调用方应在 AddUser 的写锁内**重新**校验，以应对并发注册场景。
func (s *Store) NextAvailableUsername(base string) string {
	base = NormalizeUsername(base)
	s.mu.RLock()
	defer s.mu.RUnlock()
	taken := map[string]bool{}
	for _, u := range s.file.Users {
		taken[u.Username] = true
	}
	if !taken[base] {
		return base
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s-%d", base, i)
		if !taken[cand] {
			return cand
		}
	}
}

// FindByID 通过 ID 查 User。
func (s *Store) FindByID(id string) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.file.Users {
		if u.ID == id {
			return u, true
		}
	}
	return User{}, false
}

// AddUser 写入新 User。Email + Username 都强制唯一（v2 起 username 是主登录键）。
func (s *Store) AddUser(u User) error {
	if u.Username == "" {
		return errors.New("username is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.file.Users {
		if existing.Email == u.Email {
			return errors.New("email already registered")
		}
		if existing.Username == u.Username {
			return errors.New("username already taken")
		}
	}
	s.file.Users = append(s.file.Users, u)
	return s.flushLocked()
}

// UpdateUser 全字段更新。
func (s *Store) UpdateUser(id string, mutator func(*User)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.file.Users {
		if s.file.Users[i].ID == id {
			mutator(&s.file.Users[i])
			return s.flushLocked()
		}
	}
	return errors.New("user not found")
}

// DeleteUser 删除指定 User。供 Register 后置步骤失败时回滚。
func (s *Store) DeleteUser(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.file.Users {
		if s.file.Users[i].ID == id {
			s.file.Users = append(s.file.Users[:i], s.file.Users[i+1:]...)
			return s.flushLocked()
		}
	}
	return errors.New("user not found")
}

// ListUsers 返回所有 user 副本。
func (s *Store) ListUsers() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]User, len(s.file.Users))
	copy(out, s.file.Users)
	return out
}

