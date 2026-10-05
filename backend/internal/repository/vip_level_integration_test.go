//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestVIPAssignedGradeSynchronizesAllBenefitsWithoutPrincipal(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx := testTx(t)
	r := &userRepository{sql: tx}
	var actor, id, ordinary, exclusive, private int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,role) VALUES($1,'fixture','admin') RETURNING id`, uuid.NewString()+"@example.com").Scan(&actor))
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,concurrency,rpm_limit,balance) VALUES($1,'fixture',5,20,0) RETURNING id`, uuid.NewString()+"@example.com").Scan(&id))
	for _, g := range []struct {
		name      string
		exclusive bool
		id        *int64
	}{{"normal", false, &ordinary}, {"exclusive", true, &exclusive}, {"private", true, &private}} {
		require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO groups(name,platform,is_exclusive,rate_multiplier) VALUES($1,'openai',$2,.3) RETURNING id`, g.name+uuid.NewString(), g.exclusive).Scan(g.id))
	}
	rules := service.DefaultVIPRules()
	rules.Enabled = true
	rules.Tiers[2].RPM = 90
	rules.Groups = []service.VIPGroupRule{{GroupID: ordinary, Floor: .2, Discounts: []float64{.02, .04, .06, .075, .075}}, {GroupID: exclusive, Access: true, Discounts: []float64{0, 0, 0, 0, 0}}, {GroupID: private, Private: true, Discounts: []float64{0, 0, 0, 0, 0}}}
	require.NoError(t, r.VIPSaveRules(ctx, actor, rules))
	require.NoError(t, r.VIPSetOverride(ctx, actor, id, service.VIPOverride{Benefit: "concurrency", Value: 99, Reason: "old split override"}))
	require.NoError(t, r.VIPSetLevel(ctx, actor, id, &service.VIPLevelCommand{Level: 3, Reason: "统一会员权益"}, "统一会员权益"))
	snapshot, err := r.VIPSnapshot(ctx, id)
	require.NoError(t, err)
	require.Equal(t, 3, snapshot.Tier.Level)
	require.Zero(t, snapshot.GrowthTier.Level)
	require.Zero(t, snapshot.Total)
	require.Equal(t, 16, snapshot.Concurrency)
	require.Equal(t, 90, snapshot.RPM)
	require.Equal(t, 6.0, snapshot.RebatePercent)
	require.Equal(t, "manual", snapshot.TierSource)
	require.Len(t, snapshot.Overrides, 1)
	found := map[int64]service.VIPGroupView{}
	for _, g := range snapshot.Groups {
		found[g.ID] = g
	}
	require.Equal(t, .24, found[ordinary].Rate)
	require.True(t, found[exclusive].Granted)
	require.NotContains(t, found, private)
	var level int
	var rate, balance float64
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT achievement_tier(id),vip_rebate_rate(id),balance FROM users WHERE id=$1`, id).Scan(&level, &rate, &balance))
	require.Equal(t, 3, level)
	require.Equal(t, 6.0, rate)
	require.Zero(t, balance)
	raw, err := r.AchievementSnapshot(ctx, id)
	require.NoError(t, err)
	var daily struct {
		Tier   int     `json:"tier"`
		Amount float64 `json:"daily_amount"`
	}
	require.NoError(t, json.Unmarshal(raw, &daily))
	require.Equal(t, 3, daily.Tier)
	require.Equal(t, .25, daily.Amount)
	require.NoError(t, func() error { _, e := tx.ExecContext(ctx, "SAVEPOINT split"); return e }())
	require.Error(t, r.VIPSetOverride(ctx, actor, id, service.VIPOverride{Benefit: "badge", Value: 5, Reason: "cannot split again"}))
	_, err = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT split")
	require.NoError(t, err)
	require.NoError(t, r.VIPSetLevel(ctx, actor, id, nil, "恢复自动会员权益"))
	snapshot, err = r.VIPSnapshot(ctx, id)
	require.NoError(t, err)
	require.Zero(t, snapshot.Tier.Level)
	require.Equal(t, 5, snapshot.Concurrency)
	require.Equal(t, 20, snapshot.RPM)
	require.Zero(t, snapshot.Total)
	require.Empty(t, snapshot.Overrides)
	_, err = tx.ExecContext(ctx, "SAVEPOINT unauthorized")
	require.NoError(t, err)
	require.Error(t, r.VIPSetLevel(ctx, id, id, &service.VIPLevelCommand{Level: 5}, "禁止自行升级"))
	_, err = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT unauthorized")
	require.NoError(t, err)
	require.NoError(t, r.VIPInitialCredit(ctx, actor, id, 3000, "fixture genuine opening"))
	require.NoError(t, r.VIPSetLevel(ctx, actor, id, &service.VIPLevelCommand{Level: 0}, "指定普通会员"))
	snapshot, err = r.VIPSnapshot(ctx, id)
	require.NoError(t, err)
	require.Zero(t, snapshot.Tier.Level)
	require.Equal(t, 5, snapshot.GrowthTier.Level)
	require.Equal(t, 3000.0, snapshot.Total)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT achievement_tier($1),vip_rebate_rate($1)`, id).Scan(&level, &rate))
	require.Zero(t, level)
	require.Zero(t, rate)
	found = map[int64]service.VIPGroupView{}
	for _, g := range snapshot.Groups {
		found[g.ID] = g
	}
	require.False(t, found[exclusive].Granted)
	require.Equal(t, .3, found[ordinary].Rate)
	deadline := time.Now().Add(time.Hour)
	require.NoError(t, r.VIPSetLevel(ctx, actor, id, &service.VIPLevelCommand{Level: 1, ExpiresAt: &deadline}, "临时会员等级"))
	_, err = tx.ExecContext(ctx, `UPDATE vip_overrides SET expires_at=now()-interval '1 second' WHERE user_id=$1 AND benefit='tier'`, id)
	require.NoError(t, err)
	snapshot, err = r.VIPSnapshot(ctx, id)
	require.NoError(t, err)
	require.Equal(t, 5, snapshot.Tier.Level)
	require.Equal(t, "growth", snapshot.TierSource)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT achievement_tier($1),vip_rebate_rate($1)`, id).Scan(&level, &rate))
	require.Equal(t, 5, level)
	require.Equal(t, 10.0, rate)
	found = map[int64]service.VIPGroupView{}
	for _, g := range snapshot.Groups {
		found[g.ID] = g
	}
	require.True(t, found[exclusive].Granted)
	require.Equal(t, .225, found[ordinary].Rate)
	raw, err = r.AchievementSnapshot(ctx, id)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &daily))
	require.Equal(t, 5, daily.Tier)
	require.Equal(t, 1.0, daily.Amount)
	_, err = tx.ExecContext(ctx, "SAVEPOINT unavailable")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `UPDATE users SET status='disabled' WHERE id=$1`, id)
	require.NoError(t, err)
	require.Error(t, r.VIPSetLevel(ctx, actor, id, &service.VIPLevelCommand{Level: 3}, "不可用账号拒绝"))
	_, err = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT unavailable")
	require.NoError(t, err)
}

func TestVIPHistoricalAssignedGradeAndExpiryForMakeup(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx := testTx(t)
	r := &userRepository{sql: tx}
	var actor, id int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,role) VALUES($1,'fixture','admin') RETURNING id`, uuid.NewString()+"@example.com").Scan(&actor))
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,created_at) VALUES($1,'fixture',now()-interval '10 days') RETURNING id`, uuid.NewString()+"@example.com").Scan(&id))
	rules := service.DefaultVIPRules()
	rules.Enabled = true
	require.NoError(t, r.VIPSaveRules(ctx, actor, rules))
	encoded, _ := json.Marshal(rules)
	_, err := tx.ExecContext(ctx, `INSERT INTO achievement_vip_history(effective_at,rules) VALUES(now()-interval '20 days',$1::jsonb);`, string(encoded))
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO achievement_daily_history(effective_at,rewards) VALUES(now()-interval '20 days','[0.01,0.05,0.1,0.25,0.5,1]')`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO vip_user_level_history(user_id,level,expires_at,effective_at,actor_id,reason) VALUES($1,5,now()-interval '1 day',now()-interval '4 days',$2,'历史会员'),($1,1,NULL,now()-interval '12 hours',$2,'当前会员')`, id, actor)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO vip_overrides(user_id,benefit,value,actor_id,reason) VALUES($1,'tier',1,$2,'当前会员')`, id, actor)
	require.NoError(t, err)
	day := service.CheckinDate(time.Now().Add(-48 * time.Hour))
	raw, err := r.AchievementBackfillPreview(ctx, id, day)
	require.NoError(t, err)
	var historical struct {
		Tier  int     `json:"tier"`
		Gross float64 `json:"gross"`
		Known bool    `json:"policy_known"`
	}
	require.NoError(t, json.Unmarshal(raw, &historical))
	require.True(t, historical.Known)
	require.Equal(t, 5, historical.Tier)
	require.Equal(t, 1.0, historical.Gross)
	snapshot, err := r.VIPSnapshot(ctx, id)
	require.NoError(t, err)
	require.Equal(t, 1, snapshot.Tier.Level)
	require.NoError(t, r.VIPSetLevel(ctx, actor, id, nil, "恢复自动等级"))
	raw, err = r.AchievementBackfillPreview(ctx, id, day)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &historical))
	require.Equal(t, 5, historical.Tier)
}

func TestVIPAndCheckinShareOneDailyPolicy(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx := testTx(t)
	r := &userRepository{sql: tx}
	rules, err := r.VIPRules(ctx)
	require.NoError(t, err)
	rules.DailyRewards = []float64{.02, .06, .12, .3, .6, 1.2}
	require.NoError(t, r.VIPSaveRules(ctx, 1, rules))
	config, err := r.AchievementConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, rules.DailyRewards, config.DailyRewards)
	config.DailyRewards = []float64{.03, .07, .14, .35, .7, 1.4}
	require.NoError(t, r.SaveAchievementConfig(ctx, 1, *config))
	rules, err = r.VIPRules(ctx)
	require.NoError(t, err)
	require.Equal(t, config.DailyRewards, rules.DailyRewards)
	var copies bool
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT payload ? 'daily_rewards' FROM vip_rules WHERE id=true`).Scan(&copies))
	require.False(t, copies)
}
