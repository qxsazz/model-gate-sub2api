//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

func TestAchievementCheckinPaysActiveAccountWithoutRecentActivity(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	var id int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,balance) VALUES($1,'fixture',0) RETURNING id`, uuid.NewString()+"@example.com").Scan(&id))
	_, err = tx.ExecContext(ctx, `UPDATE achievement_config SET payload=payload||'{"cash_enabled":true,"cash_scope":"all","budget_enabled":false}'::jsonb`)
	require.NoError(t, err)
	r := &userRepository{sql: tx}
	raw, err := r.AchievementMutation(ctx, id, "checkin", "", service.CheckinDate(time.Now()), "new-active-account")
	require.NoError(t, err)
	var receipt map[string]any
	require.NoError(t, json.Unmarshal(raw, &receipt))
	require.Equal(t, "eligible", receipt["reason"])
	require.Equal(t, .01, receipt["gross"])
	require.Equal(t, .01, receipt["net"])
	raw, err = r.AchievementMutation(ctx, id, "checkin", "", service.CheckinDate(time.Now()), "new-active-retry")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &receipt))
	require.Equal(t, true, receipt["replayed"])
	var balance float64
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, id).Scan(&balance))
	require.Equal(t, .01, balance)
}

func TestAchievementCheckinAccountingAndLifetimeClaim(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx, e := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, e)
	defer func() { _ = tx.Rollback() }()
	var id int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,role,status,balance) VALUES($1,'fixture','user','active',0) RETURNING id`, fmt.Sprintf("achievement-%d@example.com", time.Now().UnixNano())).Scan(&id))
	_, e = tx.ExecContext(ctx, `UPDATE vip_rules SET payload=jsonb_set(payload,'{enabled}','true');`)
	require.NoError(t, e)
	_, e = tx.ExecContext(ctx, `INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) VALUES($1,'admin_balance',$2,1000)`, id, fmt.Sprint(id))
	require.NoError(t, e)
	r := &userRepository{sql: tx}
	day := service.CheckinDate(time.Now())
	decode := func(raw json.RawMessage) map[string]any {
		var v map[string]any
		require.NoError(t, json.Unmarshal(raw, &v))
		return v
	}
	raw, e := r.AchievementMutation(ctx, id, "checkin", "", day, "first-key")
	require.NoError(t, e)
	require.Equal(t, 0.0, decode(raw)["gross"])
	require.NoError(t, r.SaveAchievementConfig(ctx, id, service.AchievementConfig{CashEnabled: true, MilestoneCashEnabled: true, CashAllowlist: []int64{id}, DailyBudget: 10, MonthlyBudget: 100}))
	raw, e = r.AchievementMutation(ctx, id, "checkin", "", day, "second-key")
	require.NoError(t, e)
	require.Equal(t, 0.0, decode(raw)["gross"])
	require.Equal(t, true, decode(raw)["replayed"])
	// A same-day opening cannot retroactively pay a checkin recorded with cash off.
	_, e = tx.ExecContext(ctx, `INSERT INTO vip_reward_debt(user_id,amount) VALUES($1,.4)`, id)
	require.NoError(t, e)
	raw, e = r.AchievementMutation(ctx, id, "claim", "R01", "", "")
	require.NoError(t, e)
	v := decode(raw)
	require.Equal(t, 1.0, v["gross"])
	require.Equal(t, .6, v["net"])
	require.Equal(t, .4, v["offset_amount"])
	raw, e = r.AchievementMutation(ctx, id, "claim", "R01", "", "")
	require.NoError(t, e)
	require.Equal(t, true, decode(raw)["replayed"])
	_, e = tx.ExecContext(ctx, `INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) VALUES($1,'refund',$2,-1000)`, id, fmt.Sprint(id))
	require.NoError(t, e)
	var balance, debt float64
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, id).Scan(&balance))
	require.Equal(t, 0.0, balance)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT amount FROM vip_reward_debt WHERE user_id=$1`, id).Scan(&debt))
	require.Equal(t, .4, debt)
	var reason string
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT achievement_cash_reason($1,payload) FROM achievement_config WHERE id=true`, id).Scan(&reason))
	require.Equal(t, "recent_activity_required", reason)
	_, e = tx.ExecContext(ctx, `INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) VALUES($1,'admin_balance',$2,1000)`, id, fmt.Sprint(id)+"-again")
	require.NoError(t, e)
	raw, e = r.AchievementMutation(ctx, id, "claim", "R01", "", "")
	require.NoError(t, e)
	require.Equal(t, true, decode(raw)["replayed"])
	require.NotNil(t, decode(raw)["revoked_at"])
	raw, e = r.AchievementSnapshot(ctx, id)
	require.NoError(t, e)
	require.Len(t, decode(raw)["medals"], 21)
}

func TestAchievementActivityDistinctPassesAndImmutableReplay(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx, e := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, e)
	defer func() { _ = tx.Rollback() }()
	var id int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash) VALUES($1,'fixture') RETURNING id`, fmt.Sprintf("activity-%d@example.com", time.Now().UnixNano())).Scan(&id))
	r := &userRepository{sql: tx}
	answers := json.RawMessage(`[0,1,2,0,1,2,0,1,2,0]`)
	for i, topic := range []string{"intro", "endpoint", "request", "cost"} {
		kind := "practice"
		if i == 0 {
			kind = "knowledge"
		}
		_, e = r.SaveActivityAttempt(ctx, id, kind, topic, fmt.Sprintf("00000000-0000-0000-0000-%012d", i+1), fmt.Sprintf("request-%d", i), answers, service.ActivityGrade{Score: 10, Passed: true})
		require.NoError(t, e)
	}
	_, e = r.SaveActivityAttempt(ctx, id, "knowledge", "intro", "00000000-0000-0000-0000-000000000009", "request-0", answers, service.ActivityGrade{Score: 0, Passed: false})
	require.NoError(t, e)
	var knowledge, practice, themes int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT achievement_progress($1,'A-K01'),achievement_progress($1,'A-X01'),achievement_progress($1,'A-C01')`, id).Scan(&knowledge, &practice, &themes))
	require.Equal(t, 1, knowledge)
	require.Equal(t, 3, practice)
	require.Equal(t, 1, themes)
	_, e = tx.ExecContext(ctx, `SAVEPOINT conflict`)
	require.NoError(t, e)
	_, e = r.SaveActivityAttempt(ctx, id, "practice", "debug", uuid.NewString(), "request-0", answers, service.ActivityGrade{Score: 10, Passed: true})
	require.Error(t, e)
	_, e = tx.ExecContext(ctx, `ROLLBACK TO conflict`)
	require.NoError(t, e)
	for i := 4; i < 30; i++ {
		_, e = r.SaveActivityAttempt(ctx, id, "knowledge", "intro", uuid.NewString(), fmt.Sprintf("request-%d", i), answers, service.ActivityGrade{Score: 0})
		require.NoError(t, e)
	}
	_, e = tx.ExecContext(ctx, `SAVEPOINT attempt_limit`)
	require.NoError(t, e)
	_, e = r.SaveActivityAttempt(ctx, id, "knowledge", "intro", uuid.NewString(), "request-31", answers, service.ActivityGrade{Score: 0})
	require.Error(t, e)
	_, e = tx.ExecContext(ctx, `ROLLBACK TO attempt_limit`)
	require.NoError(t, e)
}

func TestAchievementConcurrentBudgetAndClaim(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	repo := &userRepository{sql: integrationDB}
	original, e := repo.AchievementConfig(ctx)
	require.NoError(t, e)
	var ids []int64
	for i := 0; i < 2; i++ {
		var id int64
		require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,role,status,balance) VALUES($1,'fixture','user','active',0) RETURNING id`, uuid.NewString()+"@example.com").Scan(&id))
		ids = append(ids, id)
		_, e = integrationDB.ExecContext(ctx, `INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) VALUES($1,'admin_balance',$2,1)`, id, uuid.NewString())
		require.NoError(t, e)
	}
	day := service.CheckinDate(time.Now())
	var spent float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COALESCE((SELECT spent FROM achievement_budget WHERE day=$1),0)`, day).Scan(&spent))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM users WHERE id IN($1,$2)`, ids[0], ids[1])
		_ = repo.SaveAchievementConfig(ctx, ids[0], *original)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM achievement_budget WHERE day=$1`, day)
		if spent > 0 {
			_, _ = integrationDB.ExecContext(ctx, `INSERT INTO achievement_budget(day,spent) VALUES($1,$2)`, day, spent)
		}
	})
	_, e = integrationDB.ExecContext(ctx, `DELETE FROM achievement_budget`)
	require.NoError(t, e)
	require.NoError(t, repo.SaveAchievementConfig(ctx, ids[0], service.AchievementConfig{CashEnabled: true, MilestoneCashEnabled: true, CashAllowlist: ids, BudgetEnabled: true, DailyBudget: .01, MonthlyBudget: .01}))
	run := func(action string) {
		var wg sync.WaitGroup
		errors := make(chan error, 24)
		for i := 0; i < 24; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				id := ids[i%2]
				if action == "claim" {
					id = ids[0]
				}
				_, e := repo.AchievementMutation(ctx, id, action, "T01", day, uuid.NewString())
				errors <- e
			}(i)
		}
		wg.Wait()
		close(errors)
		for e := range errors {
			require.NoError(t, e)
		}
	}
	run("checkin")
	var count int
	var balance, total float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*),sum(gross) FROM achievement_checkins WHERE user_id IN($1,$2)`, ids[0], ids[1]).Scan(&count, &total))
	require.Equal(t, 2, count)
	require.Equal(t, .01, total)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT sum(balance) FROM users WHERE id IN($1,$2)`, ids[0], ids[1]).Scan(&balance))
	require.Equal(t, .01, balance)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT spent FROM achievement_budget WHERE day=$1`, day).Scan(&total))
	require.Equal(t, .01, total)
	require.NoError(t, repo.SaveAchievementConfig(ctx, ids[0], service.AchievementConfig{CashEnabled: true, MilestoneCashEnabled: true, CashAllowlist: ids, DailyBudget: 10, MonthlyBudget: 100}))
	_, e = integrationDB.ExecContext(ctx, `INSERT INTO achievement_growth(user_id,tokens) VALUES($1,1000000)`, ids[0])
	require.NoError(t, e)
	run("claim")
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*),sum(gross) FROM achievement_claims WHERE user_id=$1`, ids[0]).Scan(&count, &total))
	require.Equal(t, 1, count)
	require.Equal(t, .10, total)
}

func TestAchievementBillingAppliedOnceAndRollback(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	u := mustCreateUser(t, client, &service.User{Email: uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 10})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: u.ID, Key: "sk-test-" + uuid.NewString(), Name: "growth"})
	r := NewUsageBillingRepository(client, integrationDB)
	cmd := &service.UsageBillingCommand{RequestID: uuid.NewString(), APIKeyID: key.ID, UserID: u.ID, BalanceCost: .1, InputTokens: 100, OutputTokens: 20, CacheReadTokens: 30}
	result, e := r.Apply(ctx, cmd)
	require.NoError(t, e)
	require.True(t, result.Applied)
	result, e = r.Apply(ctx, cmd)
	require.NoError(t, e)
	require.False(t, result.Applied)
	var tokens int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT tokens FROM achievement_growth WHERE user_id=$1`, u.ID).Scan(&tokens))
	require.Equal(t, int64(150), tokens)
	// Overflow the progress CHECK after the wallet deduction. The whole billing
	// transaction, including its dedup entitlement, must roll back.
	_, e = integrationDB.ExecContext(ctx, `UPDATE achievement_growth SET tokens=9223372036854775807 WHERE user_id=$1`, u.ID)
	require.NoError(t, e)
	failed := &service.UsageBillingCommand{RequestID: uuid.NewString(), APIKeyID: key.ID, UserID: u.ID, BalanceCost: .1, InputTokens: 1}
	_, e = r.Apply(ctx, failed)
	require.Error(t, e)
	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, u.ID).Scan(&balance))
	require.Equal(t, 9.9, balance)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM usage_billing_dedup WHERE request_id=$1 AND api_key_id=$2`, failed.RequestID, key.ID).Scan(&count))
	require.Zero(t, count)
}

func TestAchievementHistoricalGrowthIsNotRecentCashActivity(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx, e := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, e)
	defer func() { _ = tx.Rollback() }()
	var id int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash) VALUES($1,'fixture') RETURNING id`, uuid.NewString()+"@example.com").Scan(&id))
	r := &userRepository{sql: tx}
	require.NoError(t, r.SaveAchievementConfig(ctx, id, service.AchievementConfig{CashEnabled: true, CashAllowlist: []int64{id}, DailyBudget: 10, MonthlyBudget: 100}))
	_, e = tx.ExecContext(ctx, `INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) VALUES($1,'opening',$2,300),($1,'admin_balance','historical-admin-audit:fixture',100)`, id, uuid.NewString())
	require.NoError(t, e)
	var reason string
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT achievement_cash_reason($1,payload) FROM achievement_config WHERE id=true`, id).Scan(&reason))
	require.Equal(t, "recent_activity_required", reason)
	_, e = tx.ExecContext(ctx, `INSERT INTO achievement_growth(user_id,tokens,last_balance_paid_at) VALUES($1,1,now())`, id)
	require.NoError(t, e)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT achievement_cash_reason($1,payload) FROM achievement_config WHERE id=true`, id).Scan(&reason))
	require.Equal(t, "eligible", reason)
}

func TestAchievementDailyPolicyTiersAndStreak(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx, e := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, e)
	defer func() { _ = tx.Rollback() }()
	_, e = tx.ExecContext(ctx, `UPDATE vip_rules SET payload=jsonb_set(payload,'{enabled}','true')`)
	require.NoError(t, e)
	principals := []float64{1, 100, 300, 600, 1500, 3000, 3000}
	want := []float64{.01, .05, .10, .25, .50, 1, .01}
	var ids []int64
	for _, principal := range principals {
		var id int64
		require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash) VALUES($1,'fixture') RETURNING id`, uuid.NewString()+"@example.com").Scan(&id))
		ids = append(ids, id)
		_, e = tx.ExecContext(ctx, `INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) VALUES($1,'admin_balance',$2,$3)`, id, uuid.NewString(), principal)
		require.NoError(t, e)
	}
	r := &userRepository{sql: tx}
	require.NoError(t, r.SaveAchievementConfig(ctx, ids[0], service.AchievementConfig{CashEnabled: true, CashAllowlist: ids, DailyBudget: 10, MonthlyBudget: 100}))
	day := service.CheckinDate(time.Now())
	_, e = tx.ExecContext(ctx, `INSERT INTO achievement_checkins(user_id,day,request_key,tier,revision,reason,streak) VALUES($1,$2::date-1,'historical-sign-fixture',0,1,'cash_disabled',99)`, ids[0], day)
	require.NoError(t, e)
	for i, id := range ids {
		if i == 6 {
			_, e = tx.ExecContext(ctx, `UPDATE vip_rules SET payload=jsonb_set(payload,'{enabled}','false')`)
			require.NoError(t, e)
		}
		raw, e := r.AchievementMutation(ctx, id, "checkin", "", day, uuid.NewString())
		require.NoError(t, e)
		var receipt map[string]any
		require.NoError(t, json.Unmarshal(raw, &receipt))
		require.Equal(t, want[i], receipt["gross"])
		if i == 0 {
			require.Equal(t, float64(100), receipt["streak"])
		}
	}
	var longest int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT achievement_progress($1,'S01')`, ids[0]).Scan(&longest))
	require.Equal(t, 100, longest)
}
