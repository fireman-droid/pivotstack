package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

type WalletRoute struct {
	OwnerType string
	OwnerID   string
}

type WalletTotals struct {
	Balance        decimal.Decimal
	GiftBalance    decimal.Decimal
	TotalRecharged decimal.Decimal
	TotalGifted    decimal.Decimal
}

type WalletMeta struct {
	Operation     string
	ReservationID string
	RequestID     string
	Note          string
	Operator      string
	Extra         map[string]any
}

type DeductResult struct {
	OK        bool
	Reason    string
	PaidDelta decimal.Decimal
	GiftDelta decimal.Decimal
	PaidAfter decimal.Decimal
	GiftAfter decimal.Decimal
}

type walletState struct {
	route  WalletRoute
	totals WalletTotals
}

func resolveWalletRouteForUpdate(ctx context.Context, tx pgx.Tx, keyID string) (WalletRoute, error) {
	st, err := lockWalletState(ctx, tx, keyID)
	if err != nil {
		return WalletRoute{}, err
	}
	return st.route, nil
}

func GetWalletTotals(ctx context.Context, keyID string) (WalletTotals, error) {
	p, err := requirePool()
	if err != nil {
		return WalletTotals{}, err
	}
	var balance, gift, totalRecharged, totalGifted pgtype.Numeric
	err = p.QueryRow(ctx, `
		SELECT
			CASE WHEN uak.user_id IS NOT NULL AND k.parent_key_id IS NULL
				THEN COALESCE(w.balance, 0) ELSE k.balance END,
			CASE WHEN uak.user_id IS NOT NULL AND k.parent_key_id IS NULL
				THEN COALESCE(w.gift_balance, 0) ELSE k.gift_balance END,
			CASE WHEN uak.user_id IS NOT NULL AND k.parent_key_id IS NULL
				THEN COALESCE(w.total_recharged, 0) ELSE k.total_recharged END,
			CASE WHEN uak.user_id IS NOT NULL AND k.parent_key_id IS NULL
				THEN COALESCE(w.total_gifted, 0) ELSE k.total_gifted END
		FROM api_keys k
		LEFT JOIN user_api_keys uak ON uak.api_key_id = k.id
		LEFT JOIN user_wallets w ON w.user_id = uak.user_id
		WHERE k.id=$1 AND k.deleted_at IS NULL
	`, keyID).Scan(&balance, &gift, &totalRecharged, &totalGifted)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return WalletTotals{}, ErrNotFound
		}
		return WalletTotals{}, fmt.Errorf("get wallet totals: %w", err)
	}
	return totalsFromNumerics(balance, gift, totalRecharged, totalGifted)
}

func DeductWalletBalance(ctx context.Context, keyID string, amount decimal.Decimal, meta WalletMeta) (DeductResult, error) {
	if amount.IsNegative() {
		return DeductResult{}, errors.New("deduct amount must be non-negative")
	}
	p, err := requirePool()
	if err != nil {
		return DeductResult{}, err
	}
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return DeductResult{}, fmt.Errorf("begin wallet deduct: %w", err)
	}
	defer tx.Rollback(ctx)

	st, err := lockWalletState(ctx, tx, keyID)
	if err != nil {
		return DeductResult{}, err
	}
	total := st.totals.Balance.Add(st.totals.GiftBalance)
	if total.LessThan(amount) {
		return DeductResult{
			OK:        false,
			Reason:    "insufficient",
			PaidAfter: st.totals.Balance,
			GiftAfter: st.totals.GiftBalance,
		}, nil
	}

	paidDeducted, giftDeducted := paidFirstSplit(st.totals.Balance, amount)
	after := st.totals
	after.Balance = after.Balance.Sub(paidDeducted)
	after.GiftBalance = after.GiftBalance.Sub(giftDeducted)
	if err := updateLockedWallet(ctx, tx, st.route, after); err != nil {
		return DeductResult{}, err
	}
	op := walletOperation(meta, "deduct")
	if err := insertWalletLedger(ctx, tx, keyID, st.route, meta, op, paidDeducted.Neg(), giftDeducted.Neg(), after); err != nil {
		return DeductResult{}, err
	}
	if meta.ReservationID != "" {
		if err := insertBillingReservation(ctx, tx, keyID, st.route, meta, amount, paidDeducted, giftDeducted); err != nil {
			return DeductResult{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return DeductResult{}, fmt.Errorf("commit wallet deduct: %w", err)
	}
	return DeductResult{
		OK:        true,
		PaidDelta: paidDeducted,
		GiftDelta: giftDeducted,
		PaidAfter: after.Balance,
		GiftAfter: after.GiftBalance,
	}, nil
}

func AddWalletBalance(ctx context.Context, keyID string, amount decimal.Decimal, meta WalletMeta) error {
	return addWallet(ctx, keyID, amount, decimal.Zero, decimal.Zero, decimal.Zero, walletOperation(meta, "recharge"), meta)
}

func AddWalletGift(ctx context.Context, keyID string, amount decimal.Decimal, meta WalletMeta) error {
	return addWallet(ctx, keyID, decimal.Zero, amount, decimal.Zero, amount, walletOperation(meta, "gift"), meta)
}

func AddWalletRecharge(ctx context.Context, keyID string, amount decimal.Decimal, meta WalletMeta) error {
	return addWallet(ctx, keyID, amount, decimal.Zero, amount, decimal.Zero, walletOperation(meta, "recharge"), meta)
}

func RefundWalletByReservation(ctx context.Context, reservationID string, meta WalletMeta) error {
	if reservationID == "" {
		return errors.New("reservationID is required")
	}
	p, err := requirePool()
	if err != nil {
		return err
	}
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin wallet refund: %w", err)
	}
	defer tx.Rollback(ctx)

	// 关键：从 reservation 读取**原始** owner（预扣时定下的钱包），
	// 不能用当前 keyID 重新路由 — 解绑后 lockWalletState 会得到不同的 owner，导致退款打错账户。
	var keyID, ownerType, ownerID string
	if err := tx.QueryRow(ctx, `
		SELECT api_key_id, owner_type, owner_id
		FROM billing_reservations
		WHERE id=$1
	`, reservationID).Scan(&keyID, &ownerType, &ownerID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("load reservation route: %w", err)
	}
	// 按计划锁顺序：先锁钱包行，再锁 reservation 行。
	route := WalletRoute{OwnerType: ownerType, OwnerID: ownerID}
	st, err := lockWalletOwnerState(ctx, tx, route)
	if err != nil {
		return err
	}
	var status string
	if err := tx.QueryRow(ctx, `
		SELECT status
		FROM billing_reservations
		WHERE id=$1
		FOR UPDATE
	`, reservationID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("lock reservation: %w", err)
	}
	if status == "refunded" {
		return nil
	}

	var paidN, giftN pgtype.Numeric
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(sum(-paid_delta), 0), COALESCE(sum(-gift_delta), 0)
		FROM wallet_ledger
		WHERE reservation_id=$1 AND owner_type=$2 AND owner_id=$3
			AND (paid_delta < 0 OR gift_delta < 0)
	`, reservationID, route.OwnerType, route.OwnerID).Scan(&paidN, &giftN); err != nil {
		return fmt.Errorf("sum reservation refund: %w", err)
	}
	paid, err := decimalFromNumeric(paidN)
	if err != nil {
		return err
	}
	gift, err := decimalFromNumeric(giftN)
	if err != nil {
		return err
	}
	after := st.totals
	after.Balance = after.Balance.Add(paid)
	after.GiftBalance = after.GiftBalance.Add(gift)
	if err := updateLockedWallet(ctx, tx, st.route, after); err != nil {
		return err
	}
	meta.ReservationID = reservationID
	if err := insertWalletLedger(ctx, tx, keyID, st.route, meta, walletOperation(meta, "refund"), paid, gift, after); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE billing_reservations SET status='refunded', settled_at=now(), actual_cost_usd=0 WHERE id=$1`,
		reservationID,
	); err != nil {
		return fmt.Errorf("mark reservation refunded: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit wallet refund: %w", err)
	}
	return nil
}

func SetWalletBalances(ctx context.Context, keyID string, newPaid, newGift decimal.Decimal, meta WalletMeta) (WalletTotals, error) {
	if newPaid.IsNegative() || newGift.IsNegative() {
		return WalletTotals{}, errors.New("wallet balances must be non-negative")
	}
	p, err := requirePool()
	if err != nil {
		return WalletTotals{}, err
	}
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return WalletTotals{}, fmt.Errorf("begin set wallet: %w", err)
	}
	defer tx.Rollback(ctx)

	st, err := lockWalletState(ctx, tx, keyID)
	if err != nil {
		return WalletTotals{}, err
	}
	paidDelta := newPaid.Sub(st.totals.Balance)
	giftDelta := newGift.Sub(st.totals.GiftBalance)
	after := st.totals
	after.Balance = newPaid
	after.GiftBalance = newGift
	if giftDelta.IsPositive() {
		after.TotalGifted = after.TotalGifted.Add(giftDelta)
	}
	if err := updateLockedWallet(ctx, tx, st.route, after); err != nil {
		return WalletTotals{}, err
	}
	if err := insertWalletLedger(ctx, tx, keyID, st.route, meta, walletOperation(meta, "admin_adjust"), paidDelta, giftDelta, after); err != nil {
		return WalletTotals{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return WalletTotals{}, fmt.Errorf("commit set wallet: %w", err)
	}
	return after, nil
}

func RebalanceUserWallets(ctx context.Context, factor decimal.Decimal) (affected int64, paidDiff, giftDiff decimal.Decimal, err error) {
	if !factor.IsPositive() {
		return 0, decimal.Zero, decimal.Zero, errors.New("rebalance factor must be positive")
	}
	p, err := requirePool()
	if err != nil {
		return 0, decimal.Zero, decimal.Zero, err
	}
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return 0, decimal.Zero, decimal.Zero, fmt.Errorf("begin rebalance wallets: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT user_id, balance, gift_balance, total_recharged, total_gifted
		FROM user_wallets
		WHERE balance <> 0 OR gift_balance <> 0
		ORDER BY user_id
		FOR UPDATE
	`)
	if err != nil {
		return 0, decimal.Zero, decimal.Zero, fmt.Errorf("lock user wallets: %w", err)
	}
	defer rows.Close()

	type rowState struct {
		userID string
		totals WalletTotals
	}
	var locked []rowState
	for rows.Next() {
		var userID string
		var b, g, tr, tg pgtype.Numeric
		if err := rows.Scan(&userID, &b, &g, &tr, &tg); err != nil {
			return 0, decimal.Zero, decimal.Zero, fmt.Errorf("scan rebalance wallet: %w", err)
		}
		totals, err := totalsFromNumerics(b, g, tr, tg)
		if err != nil {
			return 0, decimal.Zero, decimal.Zero, err
		}
		locked = append(locked, rowState{userID: userID, totals: totals})
	}
	if err := rows.Err(); err != nil {
		return 0, decimal.Zero, decimal.Zero, fmt.Errorf("iterate rebalance wallets: %w", err)
	}

	meta := WalletMeta{Operation: "rebalance", Operator: "system"}
	for _, row := range locked {
		after := row.totals
		after.Balance = after.Balance.Mul(factor)
		after.GiftBalance = after.GiftBalance.Mul(factor)
		pd := after.Balance.Sub(row.totals.Balance)
		gd := after.GiftBalance.Sub(row.totals.GiftBalance)
		route := WalletRoute{OwnerType: "user", OwnerID: row.userID}
		if err := updateLockedWallet(ctx, tx, route, after); err != nil {
			return 0, decimal.Zero, decimal.Zero, err
		}
		if err := insertWalletLedger(ctx, tx, "", route, meta, "rebalance", pd, gd, after); err != nil {
			return 0, decimal.Zero, decimal.Zero, err
		}
		affected++
		paidDiff = paidDiff.Add(pd)
		giftDiff = giftDiff.Add(gd)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, decimal.Zero, decimal.Zero, fmt.Errorf("commit rebalance wallets: %w", err)
	}
	return affected, paidDiff, giftDiff, nil
}

func addWallet(ctx context.Context, keyID string, paidDelta, giftDelta, totalRechargeDelta, totalGiftDelta decimal.Decimal, op string, meta WalletMeta) error {
	if paidDelta.IsNegative() || giftDelta.IsNegative() || totalRechargeDelta.IsNegative() || totalGiftDelta.IsNegative() {
		return errors.New("wallet add deltas must be non-negative")
	}
	p, err := requirePool()
	if err != nil {
		return err
	}
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin wallet add: %w", err)
	}
	defer tx.Rollback(ctx)
	st, err := lockWalletState(ctx, tx, keyID)
	if err != nil {
		return err
	}
	after := st.totals
	after.Balance = after.Balance.Add(paidDelta)
	after.GiftBalance = after.GiftBalance.Add(giftDelta)
	after.TotalRecharged = after.TotalRecharged.Add(totalRechargeDelta)
	after.TotalGifted = after.TotalGifted.Add(totalGiftDelta)
	if err := updateLockedWallet(ctx, tx, st.route, after); err != nil {
		return err
	}
	if err := insertWalletLedger(ctx, tx, keyID, st.route, meta, op, paidDelta, giftDelta, after); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit wallet add: %w", err)
	}
	return nil
}

// lockWalletState 解析 key 的钱包路由并把对应行锁住。锁顺序：
//   1. api_keys row (FOR UPDATE) — 防止解绑/重绑期间路由改变
//   2. user_api_keys row (FOR UPDATE) — 防止扣费提交时绑定突然变更
//   3. user_wallets / api_keys 钱包行 (FOR UPDATE)
// 所有 bind/unbind 必须先经 lockApiKeyForBinding 走同一把 api_keys 锁。
func lockWalletState(ctx context.Context, tx pgx.Tx, keyID string) (walletState, error) {
	if tx == nil {
		return walletState{}, errors.New("wallet route requires transaction")
	}
	var ownerType, ownerID string
	var b, g, tr, tg pgtype.Numeric
	err := tx.QueryRow(ctx, `
		WITH locked_key AS MATERIALIZED (
			SELECT k.id, k.parent_key_id
			FROM api_keys k
			WHERE k.id=$1 AND k.deleted_at IS NULL
			FOR UPDATE OF k
		),
		locked_binding AS MATERIALIZED (
			SELECT uak.user_id
			FROM user_api_keys uak
			JOIN locked_key k ON k.id=uak.api_key_id
			FOR UPDATE OF uak
		),
		route AS (
			SELECT
				CASE WHEN uak.user_id IS NOT NULL AND k.parent_key_id IS NULL
					THEN 'user' ELSE 'api_key' END AS owner_type,
				CASE WHEN uak.user_id IS NOT NULL AND k.parent_key_id IS NULL
					THEN uak.user_id ELSE k.id END AS owner_id
			FROM locked_key k
			LEFT JOIN locked_binding uak ON true
		),
		branch_user AS (
			SELECT r.owner_type, r.owner_id, w.balance, w.gift_balance,
				w.total_recharged, w.total_gifted
			FROM route r
			JOIN user_wallets w ON r.owner_type='user' AND w.user_id=r.owner_id
			FOR UPDATE OF w
		),
		branch_key AS (
			SELECT r.owner_type, r.owner_id, k.balance, k.gift_balance,
				k.total_recharged, k.total_gifted
			FROM route r
			JOIN api_keys k ON r.owner_type='api_key' AND k.id=r.owner_id
			FOR UPDATE OF k
		)
		SELECT owner_type, owner_id, balance, gift_balance, total_recharged, total_gifted
		FROM branch_user
		UNION ALL
		SELECT owner_type, owner_id, balance, gift_balance, total_recharged, total_gifted
		FROM branch_key
	`, keyID).Scan(&ownerType, &ownerID, &b, &g, &tr, &tg)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return walletState{}, ErrNotFound
		}
		return walletState{}, fmt.Errorf("lock wallet route: %w", err)
	}
	totals, err := totalsFromNumerics(b, g, tr, tg)
	if err != nil {
		return walletState{}, err
	}
	return walletState{route: WalletRoute{OwnerType: ownerType, OwnerID: ownerID}, totals: totals}, nil
}

// lockWalletOwnerState 锁定一个已知 owner 的钱包行（不重新走路由解析）。
// 用于 RefundWalletByReservation：必须用 reservation 里记录的原始 owner
// 而不是当前 keyID 的路由（解绑后路由变了）。
func lockWalletOwnerState(ctx context.Context, tx pgx.Tx, route WalletRoute) (walletState, error) {
	if tx == nil {
		return walletState{}, errors.New("wallet owner lock requires transaction")
	}
	var b, g, tr, tg pgtype.Numeric
	var err error
	switch route.OwnerType {
	case "user":
		err = tx.QueryRow(ctx, `
			SELECT balance, gift_balance, total_recharged, total_gifted
			FROM user_wallets
			WHERE user_id=$1
			FOR UPDATE
		`, route.OwnerID).Scan(&b, &g, &tr, &tg)
	case "api_key":
		err = tx.QueryRow(ctx, `
			SELECT balance, gift_balance, total_recharged, total_gifted
			FROM api_keys
			WHERE id=$1 AND deleted_at IS NULL
			FOR UPDATE
		`, route.OwnerID).Scan(&b, &g, &tr, &tg)
	default:
		return walletState{}, fmt.Errorf("unknown wallet owner type: %s", route.OwnerType)
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return walletState{}, ErrNotFound
		}
		return walletState{}, fmt.Errorf("lock wallet owner: %w", err)
	}
	totals, err := totalsFromNumerics(b, g, tr, tg)
	if err != nil {
		return walletState{}, err
	}
	return walletState{route: route, totals: totals}, nil
}

func updateLockedWallet(ctx context.Context, tx pgx.Tx, route WalletRoute, totals WalletTotals) error {
	switch route.OwnerType {
	case "user":
		tag, err := tx.Exec(ctx, `
			UPDATE user_wallets
			SET balance=$2, gift_balance=$3, total_recharged=$4,
				total_gifted=$5, version=version+1, updated_at=now()
			WHERE user_id=$1
		`, route.OwnerID, numericFromDecimal(totals.Balance), numericFromDecimal(totals.GiftBalance),
			numericFromDecimal(totals.TotalRecharged), numericFromDecimal(totals.TotalGifted))
		if err != nil {
			return fmt.Errorf("update user wallet: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
	case "api_key":
		tag, err := tx.Exec(ctx, `
			UPDATE api_keys
			SET balance=$2, gift_balance=$3, total_recharged=$4, total_gifted=$5
			WHERE id=$1
		`, route.OwnerID, numericFromDecimal(totals.Balance), numericFromDecimal(totals.GiftBalance),
			numericFromDecimal(totals.TotalRecharged), numericFromDecimal(totals.TotalGifted))
		if err != nil {
			return fmt.Errorf("update api key wallet: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
	default:
		return fmt.Errorf("unknown wallet owner type: %s", route.OwnerType)
	}
	return nil
}

func insertWalletLedger(ctx context.Context, tx pgx.Tx, keyID string, route WalletRoute, meta WalletMeta, op string, paidDelta, giftDelta decimal.Decimal, after WalletTotals) error {
	raw, err := walletMetadata(meta)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO wallet_ledger (
			id, api_key_id, owner_type, owner_id, operation, reservation_id,
			request_id, paid_delta, gift_delta, paid_after, gift_after, metadata
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`, uuid.NewString(), keyID, route.OwnerType, route.OwnerID, op,
		textFromString(meta.ReservationID), textFromString(meta.RequestID),
		numericFromDecimal(paidDelta), numericFromDecimal(giftDelta),
		numericFromDecimal(after.Balance), numericFromDecimal(after.GiftBalance), raw)
	if err != nil {
		return fmt.Errorf("insert wallet ledger: %w", err)
	}
	return nil
}

func insertBillingReservation(ctx context.Context, tx pgx.Tx, keyID string, route WalletRoute, meta WalletMeta, estCost, paid, gift decimal.Decimal) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO billing_reservations (
			id, request_id, api_key_id, owner_type, owner_id, status, action,
			est_cost_usd, pre_paid_usd, pre_gift_usd, price_snapshot
		) VALUES ($1,$2,$3,$4,$5,'pending',$6,$7,$8,$9,$10)
	`, meta.ReservationID, textFromString(meta.RequestID), keyID, route.OwnerType, route.OwnerID,
		walletOperation(meta, "deduct"), numericFromDecimal(estCost), numericFromDecimal(paid),
		numericFromDecimal(gift), "{}")
	if err != nil {
		return fmt.Errorf("insert billing reservation: %w", err)
	}
	return nil
}

func walletOperation(meta WalletMeta, fallback string) string {
	if meta.Operation != "" {
		return meta.Operation
	}
	return fallback
}

func walletMetadata(meta WalletMeta) (string, error) {
	m := map[string]any{}
	if meta.Note != "" {
		m["note"] = meta.Note
	}
	if meta.Operator != "" {
		m["operator"] = meta.Operator
	}
	for k, v := range meta.Extra {
		m[k] = v
	}
	return jsonObjectParam(m)
}

func paidFirstSplit(paid, amount decimal.Decimal) (decimal.Decimal, decimal.Decimal) {
	if paid.GreaterThanOrEqual(amount) {
		return amount, decimal.Zero
	}
	return paid, amount.Sub(paid)
}

func totalsFromNumerics(balance, gift, totalRecharged, totalGifted pgtype.Numeric) (WalletTotals, error) {
	b, err := decimalFromNumeric(balance)
	if err != nil {
		return WalletTotals{}, err
	}
	g, err := decimalFromNumeric(gift)
	if err != nil {
		return WalletTotals{}, err
	}
	tr, err := decimalFromNumeric(totalRecharged)
	if err != nil {
		return WalletTotals{}, err
	}
	tg, err := decimalFromNumeric(totalGifted)
	if err != nil {
		return WalletTotals{}, err
	}
	return WalletTotals{Balance: b, GiftBalance: g, TotalRecharged: tr, TotalGifted: tg}, nil
}
