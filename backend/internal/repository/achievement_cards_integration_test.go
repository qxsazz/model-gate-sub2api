//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAchievementCardsFinancialScenarios(t *testing.T) {
	_ = testEntClient(t)
	tx := testTx(t)
	script, err := os.ReadFile("testdata/achievement_cards_smoke.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(context.Background(), string(script))
	require.NoError(t, err)
}

func TestAchievementOneCardConcurrentBackfill(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	r := &userRepository{sql: integrationDB}
	original, err := r.AchievementConfig(ctx)
	require.NoError(t, err)
	var id int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,balance,created_at) VALUES($1,'fixture',0,now()-interval '10 days') RETURNING id`, uuid.NewString()+"@example.com").Scan(&id))
	today := service.CheckinDate(time.Now())
	var spent float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COALESCE((SELECT spent FROM achievement_budget WHERE day=$1),0)`, today).Scan(&spent))
	var vipAt, dailyAt time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO achievement_vip_history(effective_at,rules) VALUES(clock_timestamp()-interval '20 days','{"enabled":false,"tiers":[]}') RETURNING effective_at`).Scan(&vipAt))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO achievement_daily_history(effective_at,rewards) VALUES(clock_timestamp()-interval '20 days','[0.01,0.05,0.10,0.25,0.50,1.00]') RETURNING effective_at`).Scan(&dailyAt))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, id)
		_ = r.SaveAchievementConfig(ctx, id, *original)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM achievement_budget WHERE day=$1`, today)
		if spent > 0 {
			_, _ = integrationDB.ExecContext(ctx, `INSERT INTO achievement_budget(day,spent) VALUES($1,$2)`, today, spent)
		}
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM achievement_vip_history WHERE effective_at=$1`, vipAt)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM achievement_daily_history WHERE effective_at=$1`, dailyAt)
	})
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO achievement_card_claims(user_id,key,amount) VALUES($1,'A-K01',1)`, id)
	require.NoError(t, err)
	cfg := *original
	cfg.CashEnabled = true
	cfg.CashScope = "all"
	cfg.BudgetEnabled = false
	require.NoError(t, r.SaveAchievementConfig(ctx, id, cfg))
	var wg sync.WaitGroup
	results := make(chan error, 16)
	gross := .01
	tier := 0
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			day := service.CheckinDate(time.Now().Add(-time.Duration(i%2+1) * 24 * time.Hour))
			_, e := r.UseAchievementCard(ctx, id, service.AchievementCardCommand{Date: day, RequestKey: uuid.NewString(), ExpectedGross: &gross, ExpectedTier: &tier})
			results <- e
		}(i)
	}
	wg.Wait()
	close(results)
	var failed int
	for e := range results {
		if e != nil {
			require.ErrorContains(t, e, "补签卡不足")
			failed++
		}
	}
	// Existing-day retries may succeed without spending a second card.
	require.Greater(t, failed, 0)
	var cards, records, uses int
	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance,achievement_card_balance(id) FROM users WHERE id=$1`, id).Scan(&balance, &cards))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM achievement_checkins WHERE user_id=$1`, id).Scan(&records))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM achievement_card_ledger WHERE user_id=$1 AND kind='use'`, id).Scan(&uses))
	require.Equal(t, .01, balance)
	require.Zero(t, cards)
	require.Equal(t, 1, records)
	require.Equal(t, 1, uses)
}

func TestAchievementCardsSeriesAndHistoricalBackfill(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	var id int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,balance,created_at) VALUES($1,'fixture',0,now()-interval '10 days') RETURNING id`, uuid.NewString()+"@example.com").Scan(&id))
	_, err = tx.ExecContext(ctx, `UPDATE vip_rules SET payload=jsonb_set(payload,'{enabled}','true'); UPDATE achievement_config SET payload=payload||'{"cash_enabled":true,"cash_scope":"all","budget_enabled":false,"milestone_cash_enabled":false}'::jsonb;
 INSERT INTO achievement_vip_history(effective_at,rules) VALUES(now()-interval '40 days','{"enabled":true,"tiers":[{"level":1,"threshold":100},{"level":2,"threshold":300}]}');
 INSERT INTO achievement_daily_history(effective_at,rewards) VALUES(now()-interval '40 days','[0.01,0.05,0.10,0.25,0.50,1.00]')`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount,created_at) VALUES($1,'admin_balance',$2,100,now()-interval '2 days'),($1,'admin_balance',$3,200,now())`, id, uuid.NewString(), uuid.NewString())
	require.NoError(t, err)
	for _, topic := range []string{"intro", "protocol", "models", "billing", "security", "troubleshoot"} {
		_, err = tx.ExecContext(ctx, `INSERT INTO achievement_activity_passes(user_id,kind,topic) VALUES($1,'knowledge',$2)`, id, topic)
		require.NoError(t, err)
	}
	r := &userRepository{sql: tx}
	decode := func(raw json.RawMessage) map[string]any {
		var out map[string]any
		require.NoError(t, json.Unmarshal(raw, &out))
		return out
	}
	for i, key := range []string{"A-K01", "A-K02", "A-K03"} {
		raw, e := r.AchievementMutation(ctx, id, "claim", key, "", "")
		require.NoError(t, e)
		require.Equal(t, float64(i+1), decode(raw)["cards_awarded"])
	}
	raw, err := r.AchievementMutation(ctx, id, "claim", "A-K01", "", "")
	require.NoError(t, err)
	require.Equal(t, true, decode(raw)["replayed"])
	var balance float64
	var cards int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT balance,achievement_card_balance(id) FROM users WHERE id=$1`, id).Scan(&balance, &cards))
	require.Zero(t, balance)
	require.Equal(t, 6, cards)
	day := service.CheckinDate(time.Now().Add(-24 * time.Hour))
	raw, err = r.AchievementCardPreview(ctx, id, day)
	require.NoError(t, err)
	require.Equal(t, 1., decode(raw)["tier"])
	require.Equal(t, .05, decode(raw)["gross"])
	gross := .05
	tier := 1
	cmd := service.AchievementCardCommand{Date: day, RequestKey: uuid.NewString(), ExpectedGross: &gross, ExpectedTier: &tier}
	raw, err = r.UseAchievementCard(ctx, id, cmd)
	require.NoError(t, err)
	require.Equal(t, .05, decode(raw)["net"])
	require.Equal(t, 5., decode(raw)["card_balance"])
	raw, err = r.UseAchievementCard(ctx, id, cmd)
	require.NoError(t, err)
	require.Equal(t, true, decode(raw)["replayed"])
	cmd.RequestKey = uuid.NewString()
	raw, err = r.UseAchievementCard(ctx, id, cmd)
	require.NoError(t, err)
	require.Equal(t, true, decode(raw)["existing"])
	require.Equal(t, 0., decode(raw)["cards_spent"])
	_, err = tx.ExecContext(ctx, `UPDATE achievement_config SET payload=jsonb_set(payload,'{milestone_cash_enabled}','true')`)
	require.NoError(t, err)
	raw, err = r.AchievementMutation(ctx, id, "claim_series", "A-K", "", "")
	require.NoError(t, err)
	require.Equal(t, 1.4, decode(raw)["gross"])
	raw, err = r.AchievementMutation(ctx, id, "claim_series", "A-K", "", "")
	require.NoError(t, err)
	require.Equal(t, true, decode(raw)["replayed"])
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT balance,achievement_card_balance(id) FROM users WHERE id=$1`, id).Scan(&balance, &cards))
	require.Equal(t, 1.45, balance)
	require.Equal(t, 5, cards)
	raw, err = r.AchievementSnapshot(ctx, id)
	require.NoError(t, err)
	require.Equal(t, 5., decode(raw)["card_balance"])
	require.Len(t, decode(raw)["series"], 3)
}

func TestAchievementCardFailuresDoNotConsumeCards(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	var id int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,balance,created_at) VALUES($1,'fixture',0,now()-interval '10 days') RETURNING id`, uuid.NewString()+"@example.com").Scan(&id))
	_, err = tx.ExecContext(ctx, `INSERT INTO achievement_card_claims(user_id,key,amount) VALUES($1,'A-K01',1)`, id)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `UPDATE achievement_config SET payload=payload||'{"cash_enabled":true,"cash_scope":"all","budget_enabled":false}'::jsonb;
 INSERT INTO achievement_vip_history(effective_at,rules) VALUES(now()-interval '40 days','{"enabled":false,"tiers":[]}');
 INSERT INTO achievement_daily_history(effective_at,rewards) VALUES(now()-interval '40 days','[0.01,0.05,0.10,0.25,0.50,1.00]')`)
	require.NoError(t, err)
	r := &userRepository{sql: tx}
	gross := .01
	tier := 0
	good := service.AchievementCardCommand{Date: service.CheckinDate(time.Now().Add(-24 * time.Hour)), RequestKey: uuid.NewString(), ExpectedGross: &gross, ExpectedTier: &tier}
	for _, scenario := range []string{"today", "outside_window", "before_registration", "amount_changed", "cash_disabled", "budget_exhausted", "no_cards"} {
		_, err = tx.ExecContext(ctx, `SAVEPOINT card_failure`)
		require.NoError(t, err)
		cmd := good
		switch scenario {
		case "today":
			cmd.Date = service.CheckinDate(time.Now())
		case "outside_window":
			cmd.Date = service.CheckinDate(time.Now().Add(-31 * 24 * time.Hour))
		case "before_registration":
			cmd.Date = service.CheckinDate(time.Now().Add(-11 * 24 * time.Hour))
		case "amount_changed":
			v := .1
			cmd.ExpectedGross = &v
		case "cash_disabled":
			_, err = tx.ExecContext(ctx, `UPDATE achievement_config SET payload=jsonb_set(payload,'{cash_enabled}','false')`)
		case "budget_exhausted":
			_, err = tx.ExecContext(ctx, `UPDATE achievement_config SET payload=payload||'{"budget_enabled":true,"daily_budget":0,"monthly_budget":0}'::jsonb`)
		case "no_cards":
			_, err = tx.ExecContext(ctx, `UPDATE achievement_card_claims SET used=amount WHERE user_id=$1`, id)
		}
		require.NoError(t, err)
		_, err = r.UseAchievementCard(ctx, id, cmd)
		require.Error(t, err, scenario)
		_, err = tx.ExecContext(ctx, `ROLLBACK TO card_failure`)
		require.NoError(t, err)
		var cards int
		var balance float64
		require.NoError(t, tx.QueryRowContext(ctx, `SELECT balance,achievement_card_balance(id) FROM users WHERE id=$1`, id).Scan(&balance, &cards))
		require.Zero(t, balance)
		require.Equal(t, 1, cards)
	}
}
