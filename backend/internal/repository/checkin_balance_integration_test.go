//go:build integration

package repository

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func addCheckinBalanceFixture(t *testing.T, db achievementRechargeTestDB, id int64, day string, amount float64, granted time.Time) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `UPDATE users SET balance=balance+$2 WHERE id=$1`, id, amount)
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), `INSERT INTO achievement_checkins
        (user_id,day,request_key,tier,revision,gross,net,reason,streak,created_at)
        VALUES ($1,$2,$3,0,1,$4,$4,'eligible',1,$5)`, id, day, uuid.NewString(), amount, granted)
	require.NoError(t, err)
}

func TestCheckinCreditFIFOAndExpiry(t *testing.T) {
	ctx := context.Background()
	id := achievementAccessUser(t, integrationDB)
	key := mustCreateApiKey(t, testEntClient(t), &service.APIKey{UserID: id, Key: "sk-test-" + uuid.NewString(), Name: "checkin-expiry"})
	t.Cleanup(func() {
		_, err := integrationDB.Exec(`DELETE FROM usage_billing_dedup WHERE api_key_id=$1`, key.ID)
		require.NoError(t, err)
	})
	command := func(cost float64) *service.UsageBillingCommand {
		return &service.UsageBillingCommand{RequestID: uuid.NewString(), UserID: id, APIKeyID: key.ID, BalanceCost: cost}
	}
	_, err := integrationDB.Exec(`UPDATE users SET balance=12 WHERE id=$1`, id)
	require.NoError(t, err)
	repo := NewUsageBillingRepository(integrationEntClient, integrationDB)
	now := time.Now()
	addCheckinBalanceFixture(t, integrationDB, id, "2026-01-01", 3, now)
	addCheckinBalanceFixture(t, integrationDB, id, "2026-01-02", 4, now)
	// A normal recharge remains permanent. Retries use the existing billing dedup.
	_, err = integrationDB.Exec(`UPDATE users SET balance=balance+5 WHERE id=$1`, id)
	require.NoError(t, err)
	cmds := []*service.UsageBillingCommand{command(2), command(2.25)}
	_, err = repo.Apply(ctx, cmds[0])
	require.NoError(t, err)
	_, err = repo.ApplyBatch(ctx, cmds)
	require.NoError(t, err)
	_, err = repo.ApplyBatch(ctx, cmds)
	require.NoError(t, err)
	var oldest, latest float64
	require.NoError(t, integrationDB.QueryRow(`SELECT min(remaining),max(remaining) FROM checkin_balance_credits WHERE user_id=$1`, id).Scan(&oldest, &latest))
	require.Zero(t, oldest)
	require.Equal(t, 2.75, latest)
	_, err = integrationDB.Exec(`UPDATE checkin_balance_credits SET expires_at=now()-interval '1 second' WHERE user_id=$1`, id)
	require.NoError(t, err)
	users := newUserRepositoryWithSQL(integrationEntClient, integrationDB)
	available, deadline, err := users.GetBillingBalance(ctx, id)
	require.NoError(t, err)
	require.Equal(t, 17.0, available)
	require.True(t, deadline.IsZero())
	// The first read already excludes expired money, even before the worker runs.
	profile, err := users.GetByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, available, profile.Balance)
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan error, 3)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; _, e := users.ExpireCheckinBalances(ctx, 200); results <- e }()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		_, e := repo.Apply(ctx, command(2))
		results <- e
	}()
	close(start)
	wg.Wait()
	close(results)
	for e := range results {
		require.NoError(t, e)
	}
	var balance, expired float64
	require.NoError(t, integrationDB.QueryRow(`SELECT balance,(SELECT sum(expired) FROM checkin_balance_credits WHERE user_id=$1) FROM users WHERE id=$1`, id).Scan(&balance, &expired))
	require.Equal(t, 15.0, balance)
	require.Equal(t, 2.75, expired)
}

func TestCheckinCreditCalendarMonthDebtAndHolds(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	id := achievementAccessUser(t, tx)
	_, err := tx.Exec(`UPDATE users SET balance=-2 WHERE id=$1`, id)
	require.NoError(t, err)
	// 16:35 UTC on Jan 30 is Jan 31 at 00:35 in Shanghai.
	grant := time.Date(2024, 1, 30, 16, 35, 11, 0, time.UTC)
	addCheckinBalanceFixture(t, tx, id, "2024-01-01", 5, grant)
	var remaining float64
	var deadline time.Time
	require.NoError(t, tx.QueryRow(`SELECT remaining,expires_at FROM checkin_balance_credits WHERE user_id=$1`, id).Scan(&remaining, &deadline))
	require.Equal(t, 3.0, remaining)
	require.Equal(t, time.Date(2024, 1, 31, 16, 0, 0, 0, time.UTC), deadline.UTC())
	var balance float64
	require.NoError(t, tx.QueryRow(`SELECT checkin_lock_balance($1)`, id).Scan(&balance))
	require.Zero(t, balance)
	_, err = tx.Exec(`UPDATE users SET balance=10 WHERE id=$1`, id)
	require.NoError(t, err)
	addCheckinBalanceFixture(t, tx, id, "2024-01-02", 5, time.Now())
	cmd := &service.BatchImageBalanceHoldCommand{UserID: id, BatchID: "first", HoldAmount: 6}
	hold, err := reserveUsageBillingBatchImageBalance(ctx, tx, cmd)
	require.NoError(t, err)
	require.Equal(t, 9.0, *hold.NewBalance)
	cmd.ActualAmount = 2
	captured, err := captureUsageBillingBatchImageBalance(ctx, tx, cmd)
	require.NoError(t, err)
	require.Equal(t, 13.0, *captured.NewBalance)
	require.NoError(t, tx.QueryRow(`SELECT remaining FROM checkin_balance_credits WHERE user_id=$1 AND day='2024-01-02'`, id).Scan(&remaining))
	require.Equal(t, 3.0, remaining)
	cmd.BatchID = "second"
	_, err = reserveUsageBillingBatchImageBalance(ctx, tx, cmd)
	require.NoError(t, err)
	_, err = tx.Exec(`UPDATE checkin_balance_credits SET expires_at=now() WHERE user_id=$1`, id)
	require.NoError(t, err)
	// A partially captured hold may cross the deadline. Only its permanent change is released.
	cmd.ActualAmount = 1
	captured, err = captureUsageBillingBatchImageBalance(ctx, tx, cmd)
	require.NoError(t, err)
	require.Equal(t, 10.0, *captured.NewBalance)
	require.Zero(t, *captured.FrozenBalance)
	var expired float64
	require.NoError(t, tx.QueryRow(`SELECT remaining,expired FROM checkin_balance_credits WHERE user_id=$1 AND day='2024-01-02'`, id).Scan(&remaining, &expired))
	require.Zero(t, remaining)
	require.Equal(t, 2.0, expired)
	addCheckinBalanceFixture(t, tx, id, "2024-01-03", 4, time.Now())
	cmd.BatchID = uuid.NewString()
	cmd.HoldAmount = 5
	_, err = reserveUsageBillingBatchImageBalance(ctx, tx, cmd)
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO usage_billing_dedup(request_id,api_key_id,request_fingerprint) VALUES($1,$2,'fixture')`, service.BatchImageHoldRequestID(cmd.BatchID), cmd.APIKeyID)
	require.NoError(t, err)
	_, err = tx.Exec(`UPDATE checkin_balance_credits SET expires_at=now() WHERE user_id=$1`, id)
	require.NoError(t, err)
	released, err := releaseUsageBillingBatchImageBalance(ctx, tx, cmd)
	require.NoError(t, err)
	require.Equal(t, 10.0, *released.NewBalance)
	require.Zero(t, *released.FrozenBalance)
}

func TestCheckinCreditMonthlyBoundaryUsesReceiptTime(t *testing.T) {
	tx := testTx(t)
	id := achievementAccessUser(t, tx)
	// Shanghai June 1 at 00:00 and June 29 at 23:59 share July 1 at 00:00.
	// The second and third receipts use older signed days, as makeup would.
	for _, grant := range []struct {
		day, month string
		received   time.Time
		deadline   time.Time
	}{
		{"2026-06-01", "2026-06-01", time.Date(2026, 5, 31, 16, 0, 0, 0, time.UTC), time.Date(2026, 6, 30, 16, 0, 0, 0, time.UTC)},
		{"2026-05-29", "2026-06-01", time.Date(2026, 6, 29, 15, 59, 0, 0, time.UTC), time.Date(2026, 6, 30, 16, 0, 0, 0, time.UTC)},
		{"2026-06-30", "2026-07-01", time.Date(2026, 6, 30, 16, 0, 0, 0, time.UTC), time.Date(2026, 7, 31, 16, 0, 0, 0, time.UTC)},
	} {
		addCheckinBalanceFixture(t, tx, id, grant.day, 2, grant.received)
		var month string
		var deadline time.Time
		require.NoError(t, tx.QueryRow(`SELECT credit_month::text,expires_at FROM checkin_balance_credits
			WHERE user_id=$1 AND day=$2`, id, grant.day).Scan(&month, &deadline))
		require.Equal(t, grant.month, month)
		require.Equal(t, grant.deadline, deadline.UTC())
	}
}

func TestCheckinCreditUsesCutoffAfterWalletLock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	id := achievementAccessUser(t, integrationDB)
	_, err := integrationDB.ExecContext(ctx, `UPDATE users SET balance = 10 WHERE id = $1`, id)
	require.NoError(t, err)
	addCheckinBalanceFixture(t, integrationDB, id, "2026-01-01", 5, time.Now())

	debit, err := integrationDB.Conn(ctx)
	require.NoError(t, err)
	defer func() { _ = debit.Close() }()
	var pid int
	require.NoError(t, debit.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid))

	locked, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = locked.Rollback() }()
	_, err = locked.ExecContext(ctx, `SELECT 1 FROM users WHERE id = $1 FOR UPDATE`, id)
	require.NoError(t, err)

	type outcome struct {
		balance    float64
		sufficient bool
		err        error
	}
	done := make(chan outcome, 1)
	go func() {
		var result outcome
		result.err = debit.QueryRowContext(ctx, `SELECT balance, sufficient FROM checkin_charge_balance($1, 1)`, id).
			Scan(&result.balance, &result.sufficient)
		done <- result
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := integrationDB.QueryRowContext(ctx, `SELECT EXISTS (
			SELECT 1 FROM pg_stat_activity WHERE pid = $1 AND wait_event_type = 'Lock'
		)`, pid).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	// Put the deadline between the debit's statement start and its lock acquisition.
	_, err = locked.ExecContext(ctx, `UPDATE checkin_balance_credits SET expires_at = clock_timestamp() WHERE user_id = $1`, id)
	require.NoError(t, err)
	require.NoError(t, locked.Commit())

	select {
	case result := <-done:
		require.NoError(t, result.err)
		require.True(t, result.sufficient)
		require.Equal(t, 9.0, result.balance)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var expired, remaining float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT expired, remaining FROM checkin_balance_credits
		WHERE user_id = $1`, id).Scan(&expired, &remaining))
	require.Equal(t, 5.0, expired)
	require.Zero(t, remaining)
}

func TestCheckinCreditCachedBalanceDeadline(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	cache := NewBillingCache(rdb).(*billingCache)
	until := time.Now().Add(10 * time.Second)
	require.NoError(t, cache.SetUserBalanceUntil(ctx, 7, 10, until))
	require.NoError(t, cache.DeductUserBalance(ctx, 7, 1))
	ttl, err := rdb.PTTL(ctx, billingBalanceKey(7)).Result()
	require.NoError(t, err)
	require.Positive(t, ttl)
	require.LessOrEqual(t, ttl, 10*time.Second)
	require.NoError(t, cache.SetUserBalanceUntil(ctx, 8, 5, time.Now().Add(-time.Second)))
	_, err = cache.GetUserBalance(ctx, 8)
	require.ErrorIs(t, err, redis.Nil)
}

func TestCheckinCreditOverdraftAndCappedChanges(t *testing.T) {
	ctx := context.Background()
	users := newUserRepositoryWithSQL(integrationEntClient, integrationDB)
	id := achievementAccessUser(t, integrationDB)
	addCheckinBalanceFixture(t, integrationDB, id, "2025-01-01", 4, time.Now().AddDate(0, -2, 0))
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	result, sufficient, err := deductUsageBillingBalance(ctx, tx, id, 1)
	require.NoError(t, err)
	require.False(t, sufficient)
	require.Equal(t, -1.0, result)
	require.NoError(t, tx.Rollback())
	change, err := users.SetBalance(ctx, id, 20)
	require.NoError(t, err)
	require.Zero(t, change.Old)
	require.Equal(t, 20.0, change.New)
	addCheckinBalanceFixture(t, integrationDB, id, "2025-01-02", 3, time.Now())
	_, err = users.AdjustBalance(ctx, id, -2)
	require.NoError(t, err)
	var gifted float64
	require.NoError(t, integrationDB.QueryRow(`SELECT remaining FROM checkin_balance_credits WHERE user_id=$1 AND day='2025-01-02'`, id).Scan(&gifted))
	require.Equal(t, 3.0, gifted)
	addCheckinBalanceFixture(t, integrationDB, id, "2024-01-01", 5, time.Now().AddDate(0, -2, 0))
	recovered, err := users.DeductAvailableBalance(ctx, id, 30)
	require.NoError(t, err)
	require.Equal(t, 21.0, recovered)
	var final float64
	require.NoError(t, integrationDB.QueryRow(`SELECT balance FROM users WHERE id=$1`, id).Scan(&final))
	require.Zero(t, final)
}

func TestCheckinCreditGatewayRejectsExpiredOnlyBalance(t *testing.T) {
	ctx := context.Background()
	uid := achievementAccessUser(t, integrationDB)
	addCheckinBalanceFixture(t, integrationDB, uid, "2025-12-01", 1, time.Now())
	_, err := integrationDB.Exec(`UPDATE checkin_balance_credits SET expires_at=clock_timestamp()+interval '3 seconds' WHERE user_id=$1`, uid)
	require.NoError(t, err)
	cache := NewBillingCache(testRedis(t))
	users := newUserRepositoryWithSQL(integrationEntClient, integrationDB)
	billing := service.NewBillingCacheService(cache, users, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(billing.Stop)
	user := &service.User{ID: uid}
	require.NoError(t, billing.CheckBillingEligibility(ctx, user, nil, nil, nil, ""))
	require.Eventually(t, func() bool {
		value, e := cache.GetUserBalance(ctx, uid)
		return e == nil && value == 1
	}, time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return errors.Is(billing.CheckBillingEligibility(ctx, user, nil, nil, nil, ""), service.ErrInsufficientBalance)
	}, 5*time.Second, 25*time.Millisecond)
}
