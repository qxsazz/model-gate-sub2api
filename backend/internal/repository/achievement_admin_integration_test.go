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

func TestAchievementAdminBackfillUsesHistoricalTierAndRebuildsStreak(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx, e := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, e)
	defer func() { _ = tx.Rollback() }()
	var actor, id int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,role) VALUES($1,'fixture','admin') RETURNING id`, uuid.NewString()+"@example.com").Scan(&actor))
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,created_at,balance) VALUES($1,'fixture',now()-interval '10 days',0) RETURNING id`, uuid.NewString()+"@example.com").Scan(&id))
	day := service.CheckinDate(time.Now().Add(-24 * time.Hour))
	today := service.CheckinDate(time.Now())
	_, e = tx.ExecContext(ctx, `INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount,created_at) VALUES($1,'admin_balance',$2,100,now()-interval '2 days'),($1,'admin_balance',$3,500,now())`, id, uuid.NewString(), uuid.NewString())
	require.NoError(t, e)
	_, e = tx.ExecContext(ctx, `INSERT INTO vip_audit(actor_id,action,detail,created_at) VALUES($1,'rules','{"enabled":true,"tiers":[{"level":1,"threshold":100},{"level":2,"threshold":300},{"level":3,"threshold":600}]}',now()-interval '3 days')`, actor)
	require.NoError(t, e)
	_, e = tx.ExecContext(ctx, `INSERT INTO achievement_daily_history(effective_at,rewards) VALUES(now()-interval '3 days','[0.01,0.05,0.10,0.25,0.50,1.00]')`)
	require.NoError(t, e)
	_, e = tx.ExecContext(ctx, `INSERT INTO achievement_checkins(user_id,day,request_key,tier,revision,reason,streak) VALUES($1,$2,'fixture-today',0,1,'cash_disabled',1)`, id, today)
	require.NoError(t, e)
	r := &userRepository{sql: tx}
	raw, e := r.AchievementBackfillPreview(ctx, id, day)
	require.NoError(t, e)
	var p map[string]any
	require.NoError(t, json.Unmarshal(raw, &p))
	require.Equal(t, true, p["policy_known"])
	require.Equal(t, 1.0, p["tier"])
	require.Equal(t, .05, p["gross"])
	expectedGross := .05
	expectedTier := 1
	c := service.AchievementAdminCommand{Date: day, Reason: "核验遗漏日期", RequestKey: uuid.NewString(), ExpectedGross: &expectedGross, ExpectedTier: &expectedTier}
	_, e = tx.ExecContext(ctx, `SAVEPOINT changed_preview`)
	require.NoError(t, e)
	wrong := .10
	bad := c
	bad.ExpectedGross = &wrong
	_, e = r.AdminAchievementMutation(ctx, actor, id, "backfill", bad)
	require.Error(t, e)
	_, e = tx.ExecContext(ctx, `ROLLBACK TO changed_preview`)
	require.NoError(t, e)
	raw, e = r.AdminAchievementMutation(ctx, actor, id, "backfill", c)
	require.NoError(t, e)
	require.NoError(t, json.Unmarshal(raw, &p))
	require.Equal(t, .05, p["net"])
	raw, e = r.AdminAchievementMutation(ctx, actor, id, "backfill", c)
	require.NoError(t, e)
	require.NoError(t, json.Unmarshal(raw, &p))
	require.Equal(t, true, p["replayed"])
	var balance float64
	var streak int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, id).Scan(&balance))
	require.Equal(t, .05, balance)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT streak FROM achievement_checkins WHERE user_id=$1 AND day=$2`, id, today).Scan(&streak))
	require.Equal(t, 2, streak)
}

func TestAchievementAdminGrantCancelRestoreNeverDuplicatesCash(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx, e := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, e)
	defer func() { _ = tx.Rollback() }()
	var actor, id int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,role) VALUES($1,'fixture','admin') RETURNING id`, uuid.NewString()+"@example.com").Scan(&actor))
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,balance) VALUES($1,'fixture',0) RETURNING id`, uuid.NewString()+"@example.com").Scan(&id))
	r := &userRepository{sql: tx}
	c := service.AchievementAdminCommand{Key: "T01", Reason: "管理员核验授予", RequestKey: uuid.NewString(), GrantReward: true}
	_, e = r.AdminAchievementMutation(ctx, actor, id, "grant", c)
	require.NoError(t, e)
	_, e = r.AdminAchievementMutation(ctx, actor, id, "grant", c)
	require.NoError(t, e)
	_, e = r.AchievementMutation(ctx, id, "equip", "T01", "", "")
	require.NoError(t, e)
	c.GrantReward = false
	c.RequestKey = uuid.NewString()
	_, e = r.AdminAchievementMutation(ctx, actor, id, "revoke", c)
	require.NoError(t, e)
	var balance float64
	var unlocked bool
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT balance,achievement_is_unlocked(id,'T01') FROM users WHERE id=$1`, id).Scan(&balance, &unlocked))
	require.Equal(t, .1, balance)
	require.False(t, unlocked)
	c.GrantReward = true
	c.RequestKey = uuid.NewString()
	_, e = r.AdminAchievementMutation(ctx, actor, id, "grant", c)
	require.NoError(t, e)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, id).Scan(&balance))
	require.Equal(t, .1, balance)
	c.GrantReward = false
	c.ReclaimReward = true
	c.RequestKey = uuid.NewString()
	_, e = r.AdminAchievementMutation(ctx, actor, id, "revoke", c)
	require.NoError(t, e)
	_, e = r.AdminAchievementMutation(ctx, actor, id, "revoke", c)
	require.NoError(t, e)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, id).Scan(&balance))
	require.Zero(t, balance)
	c.ReclaimReward = false
	c.RequestKey = uuid.NewString()
	_, e = r.AdminAchievementMutation(ctx, actor, id, "restore", c)
	require.NoError(t, e)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT achievement_is_unlocked($1,'T01')`, id).Scan(&unlocked))
	require.False(t, unlocked)
	_, e = tx.ExecContext(ctx, `SAVEPOINT denied_actor`)
	require.NoError(t, e)
	_, e = r.AdminAchievementMutation(ctx, id, id, "grant", c)
	require.ErrorContains(t, e, "管理员")
	_, e = tx.ExecContext(ctx, `ROLLBACK TO denied_actor`)
	require.NoError(t, e)
	_, e = tx.ExecContext(ctx, `SAVEPOINT conflicting_request`)
	require.NoError(t, e)
	_, e = r.AdminAchievementMutation(ctx, actor, id, "grant", c)
	require.ErrorContains(t, e, "请求编号")
	_, e = tx.ExecContext(ctx, `ROLLBACK TO conflicting_request`)
	require.NoError(t, e)
	var count int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT count(*) FROM achievement_overrides WHERE user_id=$1`, id).Scan(&count))
	require.Zero(t, count)
}

func TestAchievementManualRechargeBadgeSurvivesAutomaticRefundUnequip(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx, e := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, e)
	defer func() { _ = tx.Rollback() }()
	var actor, id int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,role) VALUES($1,'fixture','admin') RETURNING id`, uuid.NewString()+"@example.com").Scan(&actor))
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash) VALUES($1,'fixture') RETURNING id`, uuid.NewString()+"@example.com").Scan(&id))
	r := &userRepository{sql: tx}
	_, e = r.AdminAchievementMutation(ctx, actor, id, "grant", service.AchievementAdminCommand{Key: "R01", Reason: "特别荣誉授予", RequestKey: uuid.NewString()})
	require.NoError(t, e)
	_, e = r.AchievementMutation(ctx, id, "equip", "R01", "", "")
	require.NoError(t, e)
	_, e = tx.ExecContext(ctx, `INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) VALUES($1,'refund',$2,-1)`, id, uuid.NewString())
	require.NoError(t, e)
	var equipped string
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT key FROM achievement_equipment WHERE user_id=$1`, id).Scan(&equipped))
	require.Equal(t, "R01", equipped)
}
