//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func achievementAccessResult(t testing.TB, db achievementRechargeTestDB, id int64) map[string]any {
	t.Helper()
	var raw []byte
	require.NoError(t, db.QueryRowContext(context.Background(), `SELECT achievement_access_snapshot($1)`, id).Scan(&raw))
	var result map[string]any
	require.NoError(t, json.Unmarshal(raw, &result))
	return result
}

func achievementAccessUser(t testing.TB, db achievementRechargeTestDB) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.QueryRowContext(context.Background(), `INSERT INTO users(email,password_hash,balance,created_at) VALUES($1,'fixture',0,now()-interval '10 days') RETURNING id`, uuid.NewString()+"@example.com").Scan(&id))
	t.Cleanup(func() { _, _ = db.ExecContext(context.Background(), `DELETE FROM users WHERE id=$1`, id) })
	return id
}

func achievementExpectAccessError(t *testing.T, tx *sql.Tx, status int, action func() error) {
	t.Helper()
	_, err := tx.ExecContext(context.Background(), `SAVEPOINT access_attempt`)
	require.NoError(t, err)
	err = action()
	require.Equal(t, status, infraerrors.Code(err), "%v", err)
	_, rollbackErr := tx.ExecContext(context.Background(), `ROLLBACK TO SAVEPOINT access_attempt`)
	require.NoError(t, rollbackErr)
}

type achievementRechargeTestDB interface {
	sqlExecutor
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func TestAchievementRechargeRefundFactsAndLifetimeCardClaim(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	id := achievementAccessUser(t, tx)
	order := seedAchievementRecharge(t, tx, id, 10)
	r := &userRepository{sql: tx}
	_, err = tx.ExecContext(ctx, `INSERT INTO achievement_activity_passes(user_id,kind,topic) VALUES($1,'knowledge','intro')`, id)
	require.NoError(t, err)
	raw, err := r.AchievementMutation(ctx, id, "claim", "A-K01", "", "")
	require.NoError(t, err)
	require.Contains(t, string(raw), `"cards_awarded": 1`)
	_, err = tx.ExecContext(ctx, `UPDATE users SET balance=0 WHERE id=$1`, id)
	require.NoError(t, err)
	for _, status := range []string{"REFUND_REQUESTED", "REFUND_PENDING", "REFUND_FAILED"} {
		_, err = tx.ExecContext(ctx, `UPDATE payment_orders SET status=$2,refund_amount=10,refund_at=NULL WHERE id=$1`, order, status)
		require.NoError(t, err)
		require.Equal(t, 10.0, achievementAccessResult(t, tx, id)["qualified_recharge_cny"])
	}
	_, err = tx.ExecContext(ctx, `UPDATE payment_orders SET status='PARTIALLY_REFUNDED',refund_amount=1,refund_at=now() WHERE id=$1`, order)
	require.NoError(t, err)
	require.Equal(t, 9.0, achievementAccessResult(t, tx, id)["qualified_recharge_cny"])
	require.Equal(t, "recharge_refunded", achievementAccessResult(t, tx, id)["access_reason"])
	achievementExpectAccessError(t, tx, 403, func() error { _, e := r.AchievementMutation(ctx, id, "claim", "A-K01", "", ""); return e })
	// Pending fields cannot erase or overwrite an already confirmed refund.
	_, err = tx.ExecContext(ctx, `UPDATE payment_orders SET status='REFUND_PENDING',refund_amount=9,refund_at=NULL WHERE id=$1`, order)
	require.NoError(t, err)
	require.Equal(t, 9.0, achievementAccessResult(t, tx, id)["qualified_recharge_cny"])
	seedAchievementRecharge(t, tx, id, 1)
	raw, err = r.AchievementMutation(ctx, id, "claim", "A-K01", "", "")
	require.NoError(t, err)
	require.Contains(t, string(raw), `"replayed": true`)
	var cards int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT achievement_card_balance($1)`, id).Scan(&cards))
	require.Equal(t, 1, cards)
	_, err = tx.ExecContext(ctx, `UPDATE users SET status='disabled' WHERE id=$1`, id)
	require.NoError(t, err)
	require.Equal(t, "account_unavailable", achievementAccessResult(t, tx, id)["access_reason"])
	achievementExpectAccessError(t, tx, 403, func() error { return r.EnsureAchievementAccess(ctx, id) })
}

func TestAchievementRechargePrincipalCurrencyAndFees(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	for _, test := range []struct {
		name, snapshot    string
		amount, paid, fee float64
		qualified         any
	}{
		{"legacy_cny", `null`, 200, 10, 0, 10.0},
		{"legacy_fee", `null`, 200, 10, .1, 9.99},
		{"confirmation_cent_tolerance", `{"currency":"CNY","recharge_principal":10}`, 10, 9.99, 0, 9.99},
		{"bonus", `{"currency":"CNY","recharge_principal":6}`, 10, 6, 0, 6.0},
		{"usd_without_conversion", `{"currency":"USD","recharge_principal":100}`, 100, 100, 0, 0.0},
		{"trusted_cny_equivalent", `{"currency":"USD","recharge_principal_cny":10}`, 2, 2, 0, 10.0},
		{"invalid_principal", `{"currency":"CNY","recharge_principal":"invalid"}`, 10, 10, 0, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			tx, err := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback() }()
			id := achievementAccessUser(t, tx)
			order := seedAchievementRecharge(t, tx, id, 10)
			_, err = tx.ExecContext(ctx, `UPDATE payment_orders SET amount=$2,pay_amount=$3,fee_rate=$4,provider_snapshot=$5::jsonb WHERE id=$1`, order, test.amount, test.paid, test.fee, test.snapshot)
			require.NoError(t, err)
			result := achievementAccessResult(t, tx, id)
			require.Equal(t, test.qualified, result["qualified_recharge_cny"])
			if test.qualified == nil {
				require.Equal(t, "verification_unavailable", result["access_reason"])
				r := &userRepository{sql: tx}
				achievementExpectAccessError(t, tx, 503, func() error { return r.EnsureAchievementAccess(ctx, id) })
			}
		})
	}
}

func TestAchievementRechargeHistoricalConfirmedRefundBackfill(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	id := achievementAccessUser(t, tx)
	order := seedAchievementRecharge(t, tx, id, 10)
	_, err = tx.ExecContext(ctx, `UPDATE payment_orders SET status='REFUND_PENDING',refund_amount=8,refund_at=NULL WHERE id=$1`, order)
	require.NoError(t, err)
	otherID := achievementAccessUser(t, tx)
	otherOrder := seedAchievementRecharge(t, tx, otherID, 10)
	_, err = tx.ExecContext(ctx, `INSERT INTO payment_audit_logs(order_id,action,operator,detail) VALUES($1,'REFUND_SUCCESS','fixture','{"refundAmount":1}'),($2,'REFUND_SUCCESS','fixture','legacy unstructured note')`, fmt.Sprint(order), fmt.Sprint(otherOrder))
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `DELETE FROM achievement_access_refunds`)
	require.NoError(t, err)
	content, err := migrations.FS.ReadFile("259_achievement_recharge_access.sql")
	require.NoError(t, err)
	bootstrap := strings.SplitN(strings.SplitN(string(content), "INSERT INTO achievement_access_refunds(order_id,confirmed_amount)", 2)[1], "-- Order writers", 2)[0]
	_, err = tx.ExecContext(ctx, "INSERT INTO achievement_access_refunds(order_id,confirmed_amount)"+bootstrap)
	require.NoError(t, err)
	require.Equal(t, 9.0, achievementAccessResult(t, tx, id)["qualified_recharge_cny"])
}

func TestAchievementRechargeConcurrentConfirmationAndCheckin(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	var original string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT payload::text FROM achievement_config WHERE id=true`).Scan(&original))
	_, err := integrationDB.ExecContext(ctx, `UPDATE achievement_config SET payload=payload||'{"cash_enabled":true,"cash_scope":"all","budget_enabled":false}'::jsonb`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `UPDATE achievement_config SET payload=$1::jsonb`, original)
	})
	for _, sequence := range []string{"refund_first", "checkin_first", "payment_first"} {
		t.Run(sequence, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			id := achievementAccessUser(t, integrationDB)
			principal := 10.0
			if sequence == "payment_first" {
				seedAchievementRecharge(t, integrationDB, id, 6)
				principal = 4
			}
			order := seedAchievementRecharge(t, integrationDB, id, principal)
			if sequence == "payment_first" {
				_, err := integrationDB.ExecContext(ctx, `UPDATE payment_orders SET status='PENDING',paid_at=NULL,completed_at=NULL WHERE id=$1`, order)
				require.NoError(t, err)
			}
			holding, err := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			waiter, err := integrationDB.Conn(ctx)
			require.NoError(t, err)
			defer func() { cancel(); _ = holding.Rollback(); _ = waiter.Close() }()
			var pid int
			require.NoError(t, waiter.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
			checkin := func(db achievementRechargeTestDB) error {
				var result []byte
				err := db.QueryRowContext(ctx, `SELECT achievement_checkin($1,$2,$3)`, id, service.CheckinDate(time.Now()), uuid.NewString()).Scan(&result)
				return achievementError(err)
			}
			refund := func(db achievementRechargeTestDB) error {
				_, err := db.ExecContext(ctx, `UPDATE payment_orders SET status='PARTIALLY_REFUNDED',refund_amount=1,refund_at=now() WHERE id=$1`, order)
				return err
			}
			switch sequence {
			case "refund_first":
				require.NoError(t, refund(holding))
			case "checkin_first":
				require.NoError(t, checkin(holding))
			case "payment_first":
				_, err = holding.ExecContext(ctx, `UPDATE payment_orders SET status='COMPLETED',paid_at=now(),completed_at=now() WHERE id=$1`, order)
				require.NoError(t, err)
			}
			done := make(chan error, 1)
			go func() {
				if sequence == "checkin_first" {
					done <- refund(waiter)
				} else {
					done <- checkin(waiter)
				}
			}()
			require.Eventually(t, func() bool {
				var waiting bool
				err := integrationDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock')`, pid).Scan(&waiting)
				return err == nil && waiting
			}, 5*time.Second, 10*time.Millisecond, "confirmation and checkin must serialize, including legacy orders without VIP snapshots")
			require.NoError(t, holding.Commit())
			result := <-done
			if sequence == "refund_first" {
				require.Equal(t, 403, infraerrors.Code(result), "%v", result)
			} else {
				require.NoError(t, result)
			}
			var count int
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM achievement_checkins WHERE user_id=$1`, id).Scan(&count))
			if sequence == "refund_first" {
				require.Zero(t, count)
			} else {
				require.Equal(t, 1, count)
			}
			access := achievementAccessResult(t, integrationDB, id)
			require.Equal(t, sequence == "payment_first", access["access_unlocked"])
		})
	}
}

func TestAchievementRechargeAllUserWriteEntrypointsRejectWithoutSideEffects(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	id := achievementAccessUser(t, tx)
	r := &userRepository{sql: tx}
	for _, action := range []string{"checkin", "claim", "claim_series"} {
		achievementExpectAccessError(t, tx, 403, func() error {
			_, e := r.AchievementMutation(ctx, id, action, "A-K01", service.CheckinDate(time.Now()), uuid.NewString())
			return e
		})
	}
	achievementExpectAccessError(t, tx, 403, func() error {
		_, e := r.AchievementCardPreview(ctx, id, service.CheckinDate(time.Now().Add(-24*time.Hour)))
		return e
	})
	achievementExpectAccessError(t, tx, 403, func() error {
		_, e := r.UseAchievementCard(ctx, id, service.AchievementCardCommand{Date: service.CheckinDate(time.Now().Add(-24 * time.Hour)), RequestKey: uuid.NewString()})
		return e
	})
	achievementExpectAccessError(t, tx, 403, func() error {
		_, e := r.SaveActivityAttempt(ctx, id, "knowledge", "intro", uuid.NewString(), uuid.NewString(), json.RawMessage(`[0]`), service.ActivityGrade{Score: 10, Passed: true})
		return e
	})
	achievementExpectAccessError(t, tx, 403, func() error { return r.EnsureAchievementAccess(ctx, id) })
	for _, table := range []string{"achievement_checkins", "achievement_attempts", "achievement_activity_passes", "achievement_claims", "achievement_series_claims", "achievement_card_claims", "achievement_card_ledger"} {
		var count int
		require.NoError(t, tx.QueryRowContext(ctx, fmt.Sprintf(`SELECT count(*) FROM %s WHERE user_id=$1`, table), id).Scan(&count))
		require.Zero(t, count, table)
	}
	raw, err := r.AchievementSnapshot(ctx, id)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"access_unlocked": false`)
}

func TestAchievementRechargeLegacyConfigurationPreservesThreshold(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	id := achievementAccessUser(t, tx)
	r := &userRepository{sql: tx}
	minimum := 12.35
	require.NoError(t, r.SaveAchievementConfig(ctx, id, service.AchievementConfig{MinimumRechargeCNY: &minimum, CashScope: "all"}))
	require.NoError(t, r.SaveAchievementConfig(ctx, id, service.AchievementConfig{CashScope: "all"}))
	cfg, err := r.AchievementConfig(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg.MinimumRechargeCNY)
	require.Equal(t, minimum, *cfg.MinimumRechargeCNY)
	_, err = tx.ExecContext(ctx, `UPDATE achievement_config SET payload=payload-'minimum_recharge_cny'`)
	require.NoError(t, err)
	require.Equal(t, 10.0, achievementAccessResult(t, tx, id)["minimum_recharge_cny"])
	cfg, err = r.AchievementConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, 10.0, *cfg.MinimumRechargeCNY)
}

func TestAchievementRechargeManualOrdersAndPendingPayment(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	id := achievementAccessUser(t, tx)
	order := seedAchievementRecharge(t, tx, id, 10)
	for _, method := range []string{"admin_balance", "manual", "admin_recharge", ""} {
		_, err = tx.ExecContext(ctx, `UPDATE payment_orders SET payment_type=$2 WHERE id=$1`, order, method)
		require.NoError(t, err)
		require.Equal(t, 0.0, achievementAccessResult(t, tx, id)["qualified_recharge_cny"])
	}
	_, err = tx.ExecContext(ctx, `UPDATE payment_orders SET payment_type='alipay',status='PAID',completed_at=NULL WHERE id=$1`, order)
	require.NoError(t, err)
	result := achievementAccessResult(t, tx, id)
	require.Equal(t, "payment_pending", result["access_reason"])
	require.Equal(t, false, result["access_unlocked"])
}

func TestAchievementRechargeStagingSmokeRollsBack(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	conn, err := integrationDB.Conn(ctx)
	require.NoError(t, err)
	defer func() { _, _ = conn.ExecContext(ctx, `ROLLBACK`); _ = conn.Close() }()
	var before, after int
	query := `SELECT count(*) FROM users WHERE email LIKE 'achievement-access-verify-%@example.invalid'`
	require.NoError(t, conn.QueryRowContext(ctx, query).Scan(&before))
	content, err := os.ReadFile("../../../deploy/sql/verify-achievement-recharge-access.sql")
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, string(content))
	require.NoError(t, err)
	require.NoError(t, conn.QueryRowContext(ctx, query).Scan(&after))
	require.Equal(t, before, after)
}

func seedAchievementRecharge(t testing.TB, db achievementRechargeTestDB, userID int64, principal float64) int64 {
	t.Helper()
	var orderID int64
	snapshot, err := json.Marshal(map[string]any{"currency": "CNY", "recharge_principal": principal})
	require.NoError(t, err)
	require.NoError(t, db.QueryRowContext(context.Background(), `INSERT INTO payment_orders(user_id,amount,pay_amount,expires_at,status,payment_type,provider_key,provider_snapshot,out_trade_no,paid_at,completed_at)
 VALUES($1,$2,$2,now(),'COMPLETED','alipay','easypay',$3::jsonb,$4,now(),now()) RETURNING id`, userID, principal, string(snapshot), uuid.NewString()).Scan(&orderID))
	t.Cleanup(func() { _, _ = db.ExecContext(context.Background(), `DELETE FROM payment_orders WHERE id=$1`, orderID) })
	return orderID
}

func TestAchievementRechargeAccessBoundaries(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	for _, test := range []struct {
		name      string
		payments  []float64
		qualified float64
		unlocked  bool
	}{
		{"zero", nil, 0, false}, {"below", []float64{9.99}, 9.99, false},
		{"exact", []float64{10}, 10, true}, {"split", []float64{6, 4}, 10, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			tx, err := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback() }()
			var id int64
			require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,balance) VALUES($1,'fixture',1000) RETURNING id`, uuid.NewString()+"@example.com").Scan(&id))
			for _, amount := range test.payments {
				seedAchievementRecharge(t, tx, id, amount)
			}
			// A large balance and manual VIP growth are never qualifying online principal.
			_, err = tx.ExecContext(ctx, `INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) VALUES($1,'admin_balance',$2,1000)`, id, uuid.NewString())
			require.NoError(t, err)
			var raw []byte
			require.NoError(t, tx.QueryRowContext(ctx, `SELECT achievement_access_snapshot($1)`, id).Scan(&raw))
			var result map[string]any
			require.NoError(t, json.Unmarshal(raw, &result))
			require.Equal(t, test.qualified, result["qualified_recharge_cny"])
			require.Equal(t, test.unlocked, result["access_unlocked"])
		})
	}
}
