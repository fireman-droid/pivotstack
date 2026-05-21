package users

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"kiro-api-proxy/config"
)

func setupWalletTest(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := config.Init(filepath.Join(dir, "config.json")); err != nil {
		t.Fatalf("config.Init() error = %v", err)
	}
	for _, k := range config.GetAllApiKeys() {
		_ = config.DeleteApiKey(k.ID)
	}
	once = sync.Once{}
	defaultStore = nil
	t.Cleanup(func() {
		once = sync.Once{}
		defaultStore = nil
	})
}

func mustAddKey(t *testing.T, k config.ApiKeyInfo) {
	t.Helper()
	if err := config.AddApiKey(k); err != nil {
		t.Fatalf("AddApiKey() error = %v", err)
	}
}

func TestWalletOrphanKeyRoutesToKey(t *testing.T) {
	setupWalletTest(t)
	mustAddKey(t, config.ApiKeyInfo{
		ID: "orphan-1", Key: "sk-orphan", Plan: "credit", Enabled: true,
		Balance: 10, GiftBalance: 5,
		CreatedAt: time.Now().Unix(),
	})
	totals, err := GetWalletTotals("orphan-1")
	if err != nil {
		t.Fatalf("GetWalletTotals() error = %v", err)
	}
	if totals.Balance != 10 || totals.GiftBalance != 5 {
		t.Fatalf("orphan wallet = %+v, want balance=10 gift=5", totals)
	}
}

func TestWalletBoundUserRoutesToUser(t *testing.T) {
	setupWalletTest(t)
	mustAddKey(t, config.ApiKeyInfo{
		ID: "k-bound", Key: "sk-bound", Plan: "credit", Enabled: true,
		CreatedAt: time.Now().Unix(),
	})
	now := time.Now().Unix()
	if err := Default().AddUser(User{
		ID: "u1", Email: "u1@test.example", Username: "u1",
		PasswordHash: "x", ApiKeyIDs: []string{"k-bound"}, DefaultKeyID: "k-bound",
		Balance: 100, GiftBalance: 20, CreatedAt: now,
	}); err != nil {
		t.Fatalf("AddUser() error = %v", err)
	}
	totals, err := GetWalletTotals("k-bound")
	if err != nil {
		t.Fatalf("GetWalletTotals() error = %v", err)
	}
	if totals.Balance != 100 || totals.GiftBalance != 20 {
		t.Fatalf("bound wallet = %+v, want balance=100 gift=20", totals)
	}
}

func TestWalletChildKeyRoutesToKey(t *testing.T) {
	setupWalletTest(t)
	mustAddKey(t, config.ApiKeyInfo{
		ID: "parent", Key: "sk-parent", Plan: "credit", Enabled: true,
		Balance: 100, CreatedAt: time.Now().Unix(),
		IsReseller: true,
	})
	mustAddKey(t, config.ApiKeyInfo{
		ID: "child", Key: "sk-child", Plan: "credit", Enabled: true,
		Balance: 50, GiftBalance: 10, ParentKeyID: "parent",
		CreatedAt: time.Now().Unix(),
	})
	// 给父 key 绑 user，但子卡仍应走 key-level
	_ = Default().AddUser(User{
		ID: "u-reseller", Email: "r@test.example", Username: "reseller",
		PasswordHash: "x", ApiKeyIDs: []string{"parent"},
		Balance: 999, CreatedAt: time.Now().Unix(),
	})
	totals, err := GetWalletTotals("child")
	if err != nil {
		t.Fatalf("GetWalletTotals(child) error = %v", err)
	}
	if totals.Balance != 50 || totals.GiftBalance != 10 {
		t.Fatalf("child wallet = %+v, want balance=50 gift=10 (key-level)", totals)
	}
}

func TestDeductWalletPaidFirst(t *testing.T) {
	setupWalletTest(t)
	mustAddKey(t, config.ApiKeyInfo{
		ID: "k", Key: "sk-x", Plan: "credit", Enabled: true,
		CreatedAt: time.Now().Unix(),
	})
	_ = Default().AddUser(User{
		ID: "u", Email: "u@x", Username: "u", PasswordHash: "x",
		ApiKeyIDs: []string{"k"}, DefaultKeyID: "k",
		Balance: 100, GiftBalance: 30, CreatedAt: time.Now().Unix(),
	})
	// 扣 20 → 全走 paid（paid-first）
	ok, _, paid, gift := DeductWalletBalance("k", 20)
	if !ok || paid != 20 || gift != 0 {
		t.Fatalf("deduct 20: ok=%v paid=%v gift=%v want paid-first", ok, paid, gift)
	}
	// 再扣 90 → paid 剩 80，扣完 paid(80) + gift(10)
	ok, _, paid, gift = DeductWalletBalance("k", 90)
	if !ok || paid != 80 || gift != 10 {
		t.Fatalf("deduct 90: ok=%v paid=%v gift=%v want paid=80 gift=10", ok, paid, gift)
	}
	// 剩 20 gift，扣 100 → 不足
	ok, remaining, _, _ := DeductWalletBalance("k", 100)
	if ok {
		t.Fatalf("deduct 100: ok=true, want insufficient (remaining=%v)", remaining)
	}
}

func TestOverlayWalletDoesNotMutateOriginal(t *testing.T) {
	setupWalletTest(t)
	mustAddKey(t, config.ApiKeyInfo{
		ID: "k", Key: "sk-x", Plan: "credit", Enabled: true,
		Balance: 0, GiftBalance: 0, CreatedAt: time.Now().Unix(),
	})
	_ = Default().AddUser(User{
		ID: "u", Email: "u@x", Username: "u", PasswordHash: "x",
		ApiKeyIDs: []string{"k"}, Balance: 77, GiftBalance: 33,
		CreatedAt: time.Now().Unix(),
	})
	orig := config.FindApiKeyByID("k")
	overlayed := OverlayWalletOnKey(orig)
	if orig.Balance != 0 || orig.GiftBalance != 0 {
		t.Fatalf("original key mutated: balance=%v gift=%v", orig.Balance, orig.GiftBalance)
	}
	if overlayed.Balance != 77 || overlayed.GiftBalance != 33 {
		t.Fatalf("overlay = %v/%v, want 77/33", overlayed.Balance, overlayed.GiftBalance)
	}
}

func TestDetachKeyFromUsers(t *testing.T) {
	setupWalletTest(t)
	mustAddKey(t, config.ApiKeyInfo{ID: "k1", Key: "sk1", Plan: "credit", Enabled: true, CreatedAt: time.Now().Unix()})
	mustAddKey(t, config.ApiKeyInfo{ID: "k2", Key: "sk2", Plan: "credit", Enabled: true, CreatedAt: time.Now().Unix()})
	_ = Default().AddUser(User{
		ID: "u", Email: "u@x", Username: "u", PasswordHash: "x",
		ApiKeyIDs: []string{"k1", "k2"}, DefaultKeyID: "k1", Balance: 50,
		CreatedAt: time.Now().Unix(),
	})
	if err := DetachKeyFromUsers("k1"); err != nil {
		t.Fatalf("DetachKeyFromUsers() error = %v", err)
	}
	u, ok := Default().FindByID("u")
	if !ok {
		t.Fatal("user disappeared")
	}
	if len(u.ApiKeyIDs) != 1 || u.ApiKeyIDs[0] != "k2" {
		t.Fatalf("ApiKeyIDs = %v, want [k2]", u.ApiKeyIDs)
	}
	if u.DefaultKeyID != "k2" {
		t.Fatalf("DefaultKeyID = %v, want k2 (re-routed)", u.DefaultKeyID)
	}
	// user wallet 不动
	if u.Balance != 50 {
		t.Fatalf("Balance changed = %v, want 50", u.Balance)
	}
}

func TestRebalanceWallets(t *testing.T) {
	setupWalletTest(t)
	mustAddKey(t, config.ApiKeyInfo{ID: "k", Key: "sk", Plan: "credit", Enabled: true, CreatedAt: time.Now().Unix()})
	_ = Default().AddUser(User{
		ID: "u", Email: "u@x", Username: "u", PasswordHash: "x",
		ApiKeyIDs: []string{"k"}, Balance: 200, GiftBalance: 50,
		CreatedAt: time.Now().Unix(),
	})
	affected, paidDiff, giftDiff, err := RebalanceWallets(0.5)
	if err != nil {
		t.Fatalf("RebalanceWallets() error = %v", err)
	}
	if affected != 1 {
		t.Fatalf("affected = %d, want 1", affected)
	}
	if paidDiff != -100 || giftDiff != -25 {
		t.Fatalf("diff = paid=%v gift=%v, want -100/-25", paidDiff, giftDiff)
	}
	u, _ := Default().FindByID("u")
	if u.Balance != 100 || u.GiftBalance != 25 {
		t.Fatalf("after rebalance: balance=%v gift=%v, want 100/25", u.Balance, u.GiftBalance)
	}
}
