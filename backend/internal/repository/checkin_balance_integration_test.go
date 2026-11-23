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
	addCheckinBalanceFixture(t, integrationDB, id, "2026-01-01", 3, now.Add(-48*time.Hour))
	addCheckinBalanceFixture(t, integrationDB, id, "2026-01-02", 4, now.Add(-24*time.Hour))
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
	require.True(t, time.Date(2024, 2, 28, 16, 35, 11, 0, time.UTC).Equal(deadline))
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
