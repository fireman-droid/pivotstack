package users

import (
	"errors"
	"fmt"

	"kiro-api-proxy/config"
)

// WalletSnapshot mirrors config.WalletSnapshot — kept local so callers stay in the users package.
type WalletSnapshot struct {
	Balance        float64
	GiftBalance    float64
	TotalRecharged float64
	TotalGifted    float64
}

// init wires the config→users recharge hook (resolves config↔users import cycle).
// Bound user keys credit go into User.Balance; orphan/reseller-child returns handled=false.
func init() {
	config.RegisterBoundUserRechargeHook(func(key config.ApiKeyInfo, delta config.WalletDelta) (bool, config.WalletSnapshot, error) {
		if key.ParentKeyID != "" {
			return false, config.WalletSnapshot{}, nil
		}
		after, handled, err := addWalletDeltaToBoundUser(key.ID, WalletSnapshot{
			Balance:        delta.Balance,
			GiftBalance:    delta.GiftBalance,
			TotalRecharged: delta.TotalRecharged,
			TotalGifted:    delta.TotalGifted,
		})
		return handled, config.WalletSnapshot{
			Balance:        after.Balance,
			GiftBalance:    after.GiftBalance,
			TotalRecharged: after.TotalRecharged,
			TotalGifted:    after.TotalGifted,
		}, err
	})
}

// resolveWallet routes a keyID to the wallet that should be read/written.
//
//	notfound       → key missing in config
//	"key", "", k   → reseller child (ParentKeyID set) OR orphan key (no user owns it)
//	"user", uid, k → bound user wallet
func resolveWallet(keyID string) (kind string, userID string, key *config.ApiKeyInfo) {
	key = config.FindApiKeyByID(keyID)
	if key == nil {
		return "notfound", "", nil
	}
	if key.ParentKeyID != "" {
		return "key", "", key
	}
	if u, ok := Default().FindByApiKeyID(keyID); ok {
		return "user", u.ID, key
	}
	return "key", "", key
}

// GetWalletBalance returns the spendable wallet split (paid, gift, total).
func GetWalletBalance(keyID string) (paid, gift, total float64, err error) {
	totals, err := GetWalletTotals(keyID)
	if err != nil {
		return 0, 0, 0, err
	}
	return totals.Balance, totals.GiftBalance, totals.Balance + totals.GiftBalance, nil
}

// GetWalletTotals returns full wallet snapshot (paid/gift/lifetime recharged/lifetime gifted).
func GetWalletTotals(keyID string) (WalletSnapshot, error) {
	kind, userID, key := resolveWallet(keyID)
	switch kind {
	case "notfound":
		return WalletSnapshot{}, fmt.Errorf("api key not found: %s", keyID)
	case "key":
		return WalletSnapshot{
			Balance:        key.Balance,
			GiftBalance:    key.GiftBalance,
			TotalRecharged: key.TotalRecharged,
			TotalGifted:    key.TotalGifted,
		}, nil
	case "user":
		u, ok := Default().FindByID(userID)
		if !ok {
			return WalletSnapshot{}, fmt.Errorf("user not found: %s", userID)
		}
		return WalletSnapshot{
			Balance:        u.Balance,
			GiftBalance:    u.GiftBalance,
			TotalRecharged: u.TotalRecharged,
			TotalGifted:    u.TotalGifted,
		}, nil
	default:
		return WalletSnapshot{}, fmt.Errorf("unknown wallet kind: %s", kind)
	}
}

// DeductWalletBalance spends from the routed wallet. Paid balance is consumed first
// (matches the original config.DeductKeyBalance semantics — admin sees paid drain before gift).
//
// Returns ok=false if total balance < amount (no mutation performed).
func DeductWalletBalance(keyID string, amount float64) (ok bool, remaining, paidDeducted, giftDeducted float64) {
	if amount <= 0 {
		_, _, total, err := GetWalletBalance(keyID)
		if err != nil {
			return false, 0, 0, 0
		}
		return true, total, 0, 0
	}
	kind, userID, _ := resolveWallet(keyID)
	switch kind {
	case "notfound":
		return false, 0, 0, 0
	case "key":
		// 孤儿 / 子卡 → 沿用 config.DeductKeyBalance 原子语义（paid-first）
		return config.DeductKeyBalance(keyID, amount)
	case "user":
		err := Default().UpdateUser(userID, func(u *User) {
			total := u.Balance + u.GiftBalance
			if total < amount {
				remaining = total
				return
			}
			if u.Balance >= amount {
				u.Balance -= amount
				paidDeducted = amount
			} else {
				paidDeducted = u.Balance
				rest := amount - paidDeducted
				u.Balance = 0
				u.GiftBalance -= rest
				giftDeducted = rest
			}
			remaining = u.Balance + u.GiftBalance
			ok = true
		})
		if err != nil {
			return false, 0, 0, 0
		}
		return ok, remaining, paidDeducted, giftDeducted
	default:
		return false, 0, 0, 0
	}
}

// AddWalletBalance credits paid + gift (used by refunds / admin add).
// Lifetime totals (TotalRecharged/TotalGifted) are NOT bumped; use AddWalletRecharge / AddWalletGifted.
func AddWalletBalance(keyID string, paid, gift float64) error {
	kind, userID, _ := resolveWallet(keyID)
	switch kind {
	case "notfound":
		return fmt.Errorf("api key not found: %s", keyID)
	case "key":
		return config.AddKeyBalance(keyID, paid, gift)
	case "user":
		return Default().UpdateUser(userID, func(u *User) {
			u.Balance += paid
			u.GiftBalance += gift
		})
	default:
		return fmt.Errorf("unknown wallet kind: %s", kind)
	}
}

// AddWalletRecharge credits Balance and bumps TotalRecharged.
func AddWalletRecharge(keyID string, amountUSD float64) (WalletSnapshot, error) {
	return addWalletDelta(keyID, WalletSnapshot{Balance: amountUSD, TotalRecharged: amountUSD})
}

// AddWalletGifted credits GiftBalance and bumps TotalGifted.
func AddWalletGifted(keyID string, amountUSD float64) (WalletSnapshot, error) {
	return addWalletDelta(keyID, WalletSnapshot{GiftBalance: amountUSD, TotalGifted: amountUSD})
}

// SetWalletBalances directly SETs paid + gift (admin adjustment).
func SetWalletBalances(keyID string, paid, gift float64) (WalletSnapshot, error) {
	kind, userID, _ := resolveWallet(keyID)
	switch kind {
	case "notfound":
		return WalletSnapshot{}, fmt.Errorf("api key not found: %s", keyID)
	case "key":
		if err := config.SetKeyBalances(keyID, paid, gift); err != nil {
			return WalletSnapshot{}, err
		}
		updated := config.FindApiKeyByID(keyID)
		if updated == nil {
			return WalletSnapshot{}, fmt.Errorf("api key not found after update: %s", keyID)
		}
		return WalletSnapshot{
			Balance:        updated.Balance,
			GiftBalance:    updated.GiftBalance,
			TotalRecharged: updated.TotalRecharged,
			TotalGifted:    updated.TotalGifted,
		}, nil
	case "user":
		var out WalletSnapshot
		err := Default().UpdateUser(userID, func(u *User) {
			if gift > u.GiftBalance {
				u.TotalGifted += gift - u.GiftBalance
			}
			u.Balance = paid
			u.GiftBalance = gift
			out = WalletSnapshot{
				Balance:        u.Balance,
				GiftBalance:    u.GiftBalance,
				TotalRecharged: u.TotalRecharged,
				TotalGifted:    u.TotalGifted,
			}
		})
		return out, err
	default:
		return WalletSnapshot{}, fmt.Errorf("unknown wallet kind: %s", kind)
	}
}

// OverlayWalletOnKey returns a shallow copy of info with the 4 wallet fields overlaid from
// the routed wallet. billing/handler code can keep reading info.Balance as before.
func OverlayWalletOnKey(info *config.ApiKeyInfo) *config.ApiKeyInfo {
	if info == nil {
		return nil
	}
	cp := *info
	totals, err := GetWalletTotals(info.ID)
	if err != nil {
		return &cp
	}
	cp.Balance = totals.Balance
	cp.GiftBalance = totals.GiftBalance
	cp.TotalRecharged = totals.TotalRecharged
	cp.TotalGifted = totals.TotalGifted
	return &cp
}

// DetachKeyFromUsers removes a keyID from any user's ApiKeyIDs (called before config.DeleteApiKey).
func DetachKeyFromUsers(keyID string) error {
	for _, u := range Default().ListUsers() {
		owned := false
		for _, id := range u.ApiKeyIDs {
			if id == keyID {
				owned = true
				break
			}
		}
		if !owned {
			continue
		}
		return Default().UpdateUser(u.ID, func(uu *User) {
			out := uu.ApiKeyIDs[:0]
			for _, id := range uu.ApiKeyIDs {
				if id != keyID {
					out = append(out, id)
				}
			}
			uu.ApiKeyIDs = out
			if uu.DefaultKeyID == keyID {
				uu.DefaultKeyID = ""
				if len(uu.ApiKeyIDs) > 0 {
					uu.DefaultKeyID = uu.ApiKeyIDs[0]
				}
			}
		})
	}
	return errors.New("key not bound to user")
}

// RebalanceWallets multiplies all user wallets by `factor` (used when PivotStackDollarsPerYuan changes).
func RebalanceWallets(factor float64) (usersAffected int, paidDiff, giftDiff float64, err error) {
	if factor <= 0 {
		return 0, 0, 0, fmt.Errorf("rebalance factor must be positive")
	}
	for _, u := range Default().ListUsers() {
		if u.Balance == 0 && u.GiftBalance == 0 {
			continue
		}
		uerr := Default().UpdateUser(u.ID, func(uu *User) {
			oldPaid := uu.Balance
			oldGift := uu.GiftBalance
			uu.Balance = oldPaid * factor
			uu.GiftBalance = oldGift * factor
			paidDiff += uu.Balance - oldPaid
			giftDiff += uu.GiftBalance - oldGift
			usersAffected++
		})
		if uerr != nil {
			return usersAffected, paidDiff, giftDiff, uerr
		}
	}
	return usersAffected, paidDiff, giftDiff, nil
}

// addWalletDelta is the shared "+= delta" implementation for AddWalletRecharge/Gifted.
func addWalletDelta(keyID string, delta WalletSnapshot) (WalletSnapshot, error) {
	kind, userID, _ := resolveWallet(keyID)
	switch kind {
	case "notfound":
		return WalletSnapshot{}, fmt.Errorf("api key not found: %s", keyID)
	case "key":
		if err := config.AddKeyBalance(keyID, delta.Balance, delta.GiftBalance); err != nil {
			return WalletSnapshot{}, err
		}
		// Note: legacy key TotalRecharged/TotalGifted are not bumped here — orphan-key recharge
		// goes through config.RedeemActivationCode which writes TotalRecharged directly.
		return GetWalletTotals(keyID)
	case "user":
		var out WalletSnapshot
		err := Default().UpdateUser(userID, func(u *User) {
			u.Balance += delta.Balance
			u.GiftBalance += delta.GiftBalance
			u.TotalRecharged += delta.TotalRecharged
			u.TotalGifted += delta.TotalGifted
			out = WalletSnapshot{
				Balance:        u.Balance,
				GiftBalance:    u.GiftBalance,
				TotalRecharged: u.TotalRecharged,
				TotalGifted:    u.TotalGifted,
			}
		})
		return out, err
	default:
		return WalletSnapshot{}, fmt.Errorf("unknown wallet kind: %s", kind)
	}
}

// addWalletDeltaToBoundUser is invoked by the recharge hook (skips orphan/child).
func addWalletDeltaToBoundUser(keyID string, delta WalletSnapshot) (WalletSnapshot, bool, error) {
	u, ok := Default().FindByApiKeyID(keyID)
	if !ok {
		return WalletSnapshot{}, false, nil
	}
	var out WalletSnapshot
	err := Default().UpdateUser(u.ID, func(uu *User) {
		uu.Balance += delta.Balance
		uu.GiftBalance += delta.GiftBalance
		uu.TotalRecharged += delta.TotalRecharged
		uu.TotalGifted += delta.TotalGifted
		out = WalletSnapshot{
			Balance:        uu.Balance,
			GiftBalance:    uu.GiftBalance,
			TotalRecharged: uu.TotalRecharged,
			TotalGifted:    uu.TotalGifted,
		}
	})
	return out, true, err
}

