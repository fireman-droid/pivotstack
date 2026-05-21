package users

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"kiro-api-proxy/config"

	"golang.org/x/crypto/bcrypt"
)

const serviceTestPassword = "password-123"

func setupServiceTest(t *testing.T) *Store {
	t.Helper()

	dir := t.TempDir()
	if err := config.Init(filepath.Join(dir, "config.json")); err != nil {
		t.Fatalf("config.Init() error = %v", err)
	}

	preexistingKeys := config.GetAllApiKeys()
	for _, k := range preexistingKeys {
		if err := config.DeleteApiKey(k.ID); err != nil {
			t.Fatalf("DeleteApiKey(%q) error = %v", k.ID, err)
		}
	}
	t.Cleanup(func() {
		for _, k := range config.GetAllApiKeys() {
			_ = config.DeleteApiKey(k.ID)
		}
		for _, k := range preexistingKeys {
			_ = config.AddApiKey(k)
		}
	})

	oldAllowSelfRegister := AllowSelfRegister
	oldRequireActivationCode := RequireActivationCode
	AllowSelfRegister = true
	RequireActivationCode = false
	t.Cleanup(func() {
		AllowSelfRegister = oldAllowSelfRegister
		RequireActivationCode = oldRequireActivationCode
	})

	once = sync.Once{}
	defaultStore = nil
	t.Cleanup(func() {
		once = sync.Once{}
		defaultStore = nil
	})

	return Default()
}

func newTempStore(t *testing.T) *Store {
	t.Helper()
	return newStore(filepath.Join(t.TempDir(), "data", "users.json"))
}

func sanitizeTestName(s string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", " ", "_", ":", "_")
	return strings.ToLower(r.Replace(s))
}
func testID(t *testing.T, label string) string {
	t.Helper()
	return "test_" + sanitizeTestName(t.Name()) + "_" + sanitizeTestName(label)
}
func testEmail(t *testing.T, label string) string {
	t.Helper()
	return fmt.Sprintf("%s_%s@test.example", sanitizeTestName(t.Name()), sanitizeTestName(label))
}

func requireErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want containing %q", err.Error(), want)
	}
}

func mustHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}
	return string(hash)
}

func mustAddAPIKey(t *testing.T, id string) config.ApiKeyInfo {
	t.Helper()
	key := config.ApiKeyInfo{
		ID:        id,
		Key:       "sk-" + id,
		Plan:      "credit",
		Enabled:   true,
		CreatedAt: time.Now().Unix(),
	}
	if err := config.AddApiKey(key); err != nil {
		t.Fatalf("AddApiKey(%q) error = %v", id, err)
	}
	return key
}

func mustAddStoreUser(t *testing.T, s *Store, id, email, password string, keyIDs []string, defaultKeyID string, disabled bool) User {
	t.Helper()
	u := User{
		ID:           id,
		Email:        NormalizeEmail(email),
		Username:     NormalizeUsername(id),
		PasswordHash: mustHash(t, password),
		ApiKeyIDs:    append([]string(nil), keyIDs...),
		DefaultKeyID: defaultKeyID,
		CreatedAt:    time.Now().Unix(),
		Disabled:     disabled,
	}
	if err := s.AddUser(u); err != nil {
		t.Fatalf("AddUser(%q) error = %v", email, err)
	}
	return u
}

// ───────── Pure helpers ─────────

func TestNormalizeEmail(t *testing.T) {
	got := NormalizeEmail("  USER+Tag@Example.COM  ")
	if want := "user+tag@example.com"; got != want {
		t.Fatalf("NormalizeEmail() = %q, want %q", got, want)
	}
}

func TestValidateEmail(t *testing.T) {
	maxLen := strings.Repeat("a", 250) + "@x.com"
	tooLong := strings.Repeat("a", 251) + "@x.com"
	tests := []struct {
		name, email, wantErr string
	}{
		{"empty", "   ", "email is required"},
		{"missing at", "user.example.com", "invalid email"},
		{"missing dot", "user@example", "invalid email"},
		{"too long", tooLong, "email too long"},
		{"max length", maxLen, ""},
		{"normal", "user@example.com", ""},
		{"uppercase and spaces", "  USER@Example.COM  ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateEmail(%q) error = %v", tt.email, err)
				}
				return
			}
			requireErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct{ name, password, wantErr string }{
		{"too short", "1234567", "password too short"},
		{"minimum length", "12345678", ""},
		{"normal", "password-123", ""},
		{"maximum length", strings.Repeat("a", 256), ""},
		{"too long", strings.Repeat("a", 257), "password too long"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidatePassword() error = %v", err)
				}
				return
			}
			requireErrorContains(t, err, tt.wantErr)
		})
	}
}

// ───────── Store CRUD ─────────

func TestStoreUserCRUD(t *testing.T) {
	s := newTempStore(t)

	if _, ok := s.FindByID("missing"); ok {
		t.Fatal("FindByID(missing) ok = true")
	}
	if _, ok := s.FindByEmail("missing@test.example"); ok {
		t.Fatal("FindByEmail(missing) ok = true")
	}

	u := User{ID: "u1", Email: "user@test.example", Username: "before", PasswordHash: "hash", CreatedAt: 1}
	if err := s.AddUser(u); err != nil {
		t.Fatalf("AddUser() error = %v", err)
	}
	if err := s.AddUser(User{ID: "u2", Email: u.Email}); err == nil {
		t.Fatal("AddUser duplicate error = nil")
	}

	got, ok := s.FindByEmail("  USER@Test.Example  ")
	if !ok || got.ID != u.ID {
		t.Fatalf("FindByEmail() ok=%v id=%q", ok, got.ID)
	}
	if err := s.UpdateUser("missing", func(*User) {}); err == nil {
		t.Fatal("UpdateUser missing error = nil")
	}
	if err := s.UpdateUser(u.ID, func(uu *User) {
		uu.Username = "after"; uu.Disabled = true; uu.DefaultKeyID = "k1"
	}); err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	got, _ = s.FindByID(u.ID)
	if got.Username != "after" || !got.Disabled || got.DefaultKeyID != "k1" {
		t.Fatalf("updated user = %+v", got)
	}

	users := s.ListUsers()
	if len(users) != 1 {
		t.Fatalf("ListUsers len = %d", len(users))
	}
	reloaded := newStore(s.path)
	got, ok = reloaded.FindByID(u.ID)
	if !ok || got.Username != "after" {
		t.Fatalf("reloaded user = %+v ok=%v", got, ok)
	}
}

func TestStoreConcurrentAddUsers(t *testing.T) {
	s := newTempStore(t)
	const N = 50
	errCh := make(chan error, N)
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- s.AddUser(User{
				ID: fmt.Sprintf("u-%02d", i), Email: fmt.Sprintf("user-%02d@test.example", i),
				Username:     fmt.Sprintf("user-%02d", i),
				PasswordHash: "hash", CreatedAt: int64(i),
			})
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent AddUser error = %v", err)
		}
	}
	if len(s.ListUsers()) != N {
		t.Fatalf("ListUsers len = %d, want %d", len(s.ListUsers()), N)
	}
}

func TestStoreDeleteUser(t *testing.T) {
	s := newTempStore(t)
	u := User{
		ID: "usr-delete-1", Email: "delete@test.example", Username: "todel",
		PasswordHash: "hash", CreatedAt: time.Now().Unix(),
	}
	if err := s.AddUser(u); err != nil {
		t.Fatalf("AddUser() error = %v", err)
	}
	if err := s.DeleteUser("missing"); err == nil {
		t.Fatal("DeleteUser(missing) error = nil")
	}
	if err := s.DeleteUser(u.ID); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	if list := s.ListUsers(); len(list) != 0 {
		t.Fatalf("ListUsers after delete = %d, want 0", len(list))
	}
}

// ───────── Register ─────────

func TestRegister(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		s := setupServiceTest(t)
		email := "  " + strings.ToUpper(testEmail(t, "normal")) + "  "
		u, key, err := Register(RegisterInput{Email: email, Username: "alice", Password: serviceTestPassword})
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		if u.Email != NormalizeEmail(email) {
			t.Fatalf("Email = %q, want %q", u.Email, NormalizeEmail(email))
		}
		if len(u.ApiKeyIDs) != 1 || u.ApiKeyIDs[0] != key.ID {
			t.Fatalf("ApiKeyIDs = %v, key = %+v", u.ApiKeyIDs, key)
		}
		if stored, ok := s.FindByEmail(email); !ok || stored.ID != u.ID {
			t.Fatalf("stored user ok=%v %+v", ok, stored)
		}
	})

	t.Run("email duplicate", func(t *testing.T) {
		setupServiceTest(t)
		email := testEmail(t, "duplicate")
		if _, _, err := Register(RegisterInput{Email: email, Password: serviceTestPassword}); err != nil {
			t.Fatalf("first Register() error = %v", err)
		}
		_, _, err := Register(RegisterInput{Email: strings.ToUpper(email), Password: serviceTestPassword})
		if !errors.Is(err, ErrEmailAlreadyRegistered) {
			t.Fatalf("Register duplicate error = %v", err)
		}
	})

	t.Run("self register disabled", func(t *testing.T) {
		setupServiceTest(t)
		AllowSelfRegister = false
		_, _, err := Register(RegisterInput{Email: testEmail(t, "x"), Password: serviceTestPassword})
		requireErrorContains(t, err, "self-register disabled")
	})

	t.Run("activation required but missing", func(t *testing.T) {
		setupServiceTest(t)
		RequireActivationCode = true
		_, _, err := Register(RegisterInput{Email: testEmail(t, "x"), Password: serviceTestPassword})
		requireErrorContains(t, err, "activation code required")
	})

	t.Run("activation invalid", func(t *testing.T) {
		setupServiceTest(t)
		RequireActivationCode = true
		_, _, err := Register(RegisterInput{
			Email: testEmail(t, "x"), Password: serviceTestPassword, ActivationCode: "ACT-MISSING",
		})
		requireErrorContains(t, err, "activation code not found")
	})

	t.Run("activation consumed and credited", func(t *testing.T) {
		setupServiceTest(t)
		RequireActivationCode = true
		code := "ACT-" + testID(t, "consume")
		if err := config.AddActivationCode(config.ActivationCode{
			Code:      code,
			Type:      "balance",
			Amount:    config.CNYFromVirtualUSD(2.0), // 2 virtual$ face value
			CreatedAt: time.Now().Unix(),
		}); err != nil {
			t.Fatalf("AddActivationCode() error = %v", err)
		}
		u, _, err := Register(RegisterInput{Email: testEmail(t, "ok"), Password: serviceTestPassword, ActivationCode: code})
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		if u.InvitedBy != code || u.InviterUserID != "" {
			t.Fatalf("activation audit fields = %+v", u)
		}
		// v8: redeem 钱进 user wallet（hook 路径）；key.Balance 保持 0。
		reloaded, ok := Default().FindByID(u.ID)
		if !ok {
			t.Fatalf("user disappeared after register")
		}
		if reloaded.Balance < 1.99 || reloaded.Balance > 2.01 {
			t.Fatalf("redeemed user wallet balance = %.6f, want about 2", reloaded.Balance)
		}
		// 同一个码再注册 → 找不到（redeem 时已删除）
		_, _, err = Register(RegisterInput{Email: testEmail(t, "ok2"), Password: serviceTestPassword, ActivationCode: code})
		requireErrorContains(t, err, "activation code not found")
	})
}

// ───────── Login ─────────

func TestLoginByPassword(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		setupServiceTest(t)
		email := testEmail(t, "normal")
		registered, registeredKey, err := Register(RegisterInput{Email: email, Password: serviceTestPassword})
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		u, key, err := LoginByPassword("  "+strings.ToUpper(email)+"  ", serviceTestPassword)
		if err != nil {
			t.Fatalf("LoginByPassword() error = %v", err)
		}
		if u.ID != registered.ID || key.ID != registeredKey.ID {
			t.Fatalf("login mismatch: u=%+v key=%+v", u, key)
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		setupServiceTest(t)
		email := testEmail(t, "wrong")
		_, _, _ = Register(RegisterInput{Email: email, Password: serviceTestPassword})
		_, _, err := LoginByPassword(email, "wrong-password")
		requireErrorContains(t, err, "invalid credentials")
	})

	t.Run("email not found", func(t *testing.T) {
		setupServiceTest(t)
		_, _, err := LoginByPassword(testEmail(t, "missing"), serviceTestPassword)
		requireErrorContains(t, err, "invalid credentials")
	})

	t.Run("user disabled", func(t *testing.T) {
		s := setupServiceTest(t)
		email := testEmail(t, "disabled")
		u, _, _ := Register(RegisterInput{Email: email, Password: serviceTestPassword})
		_ = s.UpdateUser(u.ID, func(uu *User) { uu.Disabled = true })
		_, _, err := LoginByPassword(email, serviceTestPassword)
		requireErrorContains(t, err, "account disabled")
	})
}

// ───────── BindKeyToNewUser ─────────

func TestBindKeyToNewUser(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		setupServiceTest(t)
		keyID := testID(t, "key")
		mustAddAPIKey(t, keyID)
		email := "  " + strings.ToUpper(testEmail(t, "normal")) + "  "
		u, err := BindKeyToNewUser(keyID, email, serviceTestPassword, "")
		if err != nil {
			t.Fatalf("BindKeyToNewUser() error = %v", err)
		}
		if u.DefaultKeyID != keyID || len(u.ApiKeyIDs) != 1 {
			t.Fatalf("bound user = %+v", u)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		setupServiceTest(t)
		_, err := BindKeyToNewUser(testID(t, "missing"), testEmail(t, "x"), serviceTestPassword, "")
		requireErrorContains(t, err, "api key not found")
	})

	t.Run("key already bound", func(t *testing.T) {
		setupServiceTest(t)
		keyID := testID(t, "key")
		mustAddAPIKey(t, keyID)
		_, _ = BindKeyToNewUser(keyID, testEmail(t, "first"), serviceTestPassword, "")
		_, err := BindKeyToNewUser(keyID, testEmail(t, "second"), serviceTestPassword, "")
		if !errors.Is(err, ErrKeyAlreadyBound) {
			t.Fatalf("error = %v, want ErrKeyAlreadyBound", err)
		}
	})

	t.Run("email duplicate", func(t *testing.T) {
		setupServiceTest(t)
		k1, k2 := testID(t, "k1"), testID(t, "k2")
		mustAddAPIKey(t, k1)
		mustAddAPIKey(t, k2)
		email := testEmail(t, "x")
		_, _ = BindKeyToNewUser(k1, email, serviceTestPassword, "")
		_, err := BindKeyToNewUser(k2, strings.ToUpper(email), serviceTestPassword, "")
		if !errors.Is(err, ErrEmailAlreadyRegistered) {
			t.Fatalf("error = %v, want ErrEmailAlreadyRegistered", err)
		}
	})

	t.Run("email validation fail", func(t *testing.T) {
		setupServiceTest(t)
		k := testID(t, "k")
		mustAddAPIKey(t, k)
		_, err := BindKeyToNewUser(k, "invalid.example", serviceTestPassword, "")
		requireErrorContains(t, err, "invalid email")
	})

	t.Run("password validation fail", func(t *testing.T) {
		setupServiceTest(t)
		k := testID(t, "k")
		mustAddAPIKey(t, k)
		_, err := BindKeyToNewUser(k, testEmail(t, "x"), "short", "")
		requireErrorContains(t, err, "password too short")
	})

	t.Run("key present in another user's list", func(t *testing.T) {
		s := setupServiceTest(t)
		target := testID(t, "shared")
		other := testID(t, "other")
		mustAddAPIKey(t, target)
		mustAddAPIKey(t, other)
		mustAddStoreUser(t, s, testID(t, "h1"), testEmail(t, "h1"), serviceTestPassword,
			[]string{other, target}, other, false)
		_, err := BindKeyToNewUser(target, testEmail(t, "cand"), serviceTestPassword, "")
		if !errors.Is(err, ErrKeyAlreadyBound) {
			t.Fatalf("error = %v, want ErrKeyAlreadyBound", err)
		}
	})
}

// ───────── ChangePassword ─────────

func TestChangePassword(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		setupServiceTest(t)
		email := testEmail(t, "x")
		u, _, _ := Register(RegisterInput{Email: email, Password: serviceTestPassword})
		newPwd := "new-password-123"
		if err := ChangePassword(u.ID, serviceTestPassword, newPwd); err != nil {
			t.Fatalf("ChangePassword() error = %v", err)
		}
		if _, _, err := LoginByPassword(email, newPwd); err != nil {
			t.Fatalf("LoginByPassword(new) error = %v", err)
		}
	})

	t.Run("old wrong", func(t *testing.T) {
		setupServiceTest(t)
		u, _, _ := Register(RegisterInput{Email: testEmail(t, "x"), Password: serviceTestPassword})
		err := ChangePassword(u.ID, "wrong", "new-password-123")
		requireErrorContains(t, err, "old password incorrect")
	})

	t.Run("new too short", func(t *testing.T) {
		setupServiceTest(t)
		u, _, _ := Register(RegisterInput{Email: testEmail(t, "x"), Password: serviceTestPassword})
		err := ChangePassword(u.ID, serviceTestPassword, "short")
		requireErrorContains(t, err, "password too short")
	})

	t.Run("user not found", func(t *testing.T) {
		setupServiceTest(t)
		err := ChangePassword(testID(t, "missing"), serviceTestPassword, "new-password-123")
		requireErrorContains(t, err, "user not found")
	})
}

