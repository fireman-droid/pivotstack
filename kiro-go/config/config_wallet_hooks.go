package config

// WalletDelta is a wallet mutation request — used to credit balance / gifts when a code is redeemed
// or admin adjusts a bound user's wallet.
type WalletDelta struct {
	Balance        float64
	GiftBalance    float64
	TotalRecharged float64
	TotalGifted    float64
}

// WalletSnapshot is a read-only view of a wallet's current totals.
type WalletSnapshot struct {
	Balance        float64
	GiftBalance    float64
	TotalRecharged float64
	TotalGifted    float64
}

// BoundUserRechargeHook lets the users package intercept config-level wallet writes so that
// bound users have the credit applied to User.Balance instead of ApiKeyInfo.Balance.
//
// Contract:
//   - Return handled=true  → config skips writing to the key's legacy wallet fields.
//   - Return handled=false → config falls back to the legacy key-level mutation (orphan keys / reseller children).
//   - The hook MUST NOT call any config write helpers (re-entrant cfgLock = deadlock).
type BoundUserRechargeHook func(key ApiKeyInfo, delta WalletDelta) (handled bool, after WalletSnapshot, err error)

var boundUserRechargeHook BoundUserRechargeHook

// RegisterBoundUserRechargeHook is called from users package init() to wire the wallet routing.
// Subsequent registrations replace the previous hook (last writer wins).
func RegisterBoundUserRechargeHook(h BoundUserRechargeHook) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	boundUserRechargeHook = h
}
