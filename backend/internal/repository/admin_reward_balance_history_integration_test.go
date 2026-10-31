//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRewardBalanceHistoryPreservesOldRecordsAndNeverRecredits(t *testing.T) {
	ctx := context.Background()
	client := testEntTx(t).Client()
	var uid, other int64
	rows, err := client.QueryContext(ctx, `INSERT INTO users(email,password_hash,balance) VALUES($1,'fixture',10),($2,'fixture',20) RETURNING id`, uuid.NewString()+"@test.invalid", uuid.NewString()+"@test.invalid")
	require.NoError(t, err)
	require.True(t, rows.Next())
	require.NoError(t, rows.Scan(&uid))
	require.True(t, rows.Next())
	require.NoError(t, rows.Scan(&other))
	require.NoError(t, rows.Close())
	exec := func(query string, args ...any) {
		_, err := client.ExecContext(ctx, query, args...)
		require.NoError(t, err)
	}
	exec(`INSERT INTO redeem_codes(code,type,value,status,used_by,used_at) VALUES($1,'balance',10,'used',$2,now()-interval '2 days')`, "PAY-existing-"+uuid.NewString()[:8], uid)
	exec(`INSERT INTO achievement_checkins(user_id,day,request_key,tier,revision,gross,net,offset_amount,reason,streak,source,created_at)
 VALUES($1,current_date-1,'historical',1,1,.05,.03,.02,'card_backfill',1,'card',now()-interval '1 day'),
 ($1,current_date-2,'cash-off',0,1,0,0,0,'cash_disabled',1,'user',now()-interval '2 days'),
 ($2,current_date-1,'other-user',0,1,100,100,0,'eligible',1,'user',now())`, uid, other)
	exec(`INSERT INTO achievement_claims(user_id,key,gross,net,offset_amount,revision,revoked_at) VALUES($1,'T02',.5,.5,0,1,now())`, uid)
	exec(`INSERT INTO achievement_series_claims(user_id,key,gross,net,offset_amount,revision) VALUES($1,'A-K',1.4,1.4,0,1)`, uid)
	redeems := NewRedeemCodeRepository(client)
	svc := service.NewAdminService(nil, nil, nil, nil, nil, nil, redeems, nil, nil, nil, nil, nil, nil, client, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	readBalance := func() float64 {
		rows, err := client.QueryContext(ctx, `SELECT balance::double precision FROM users WHERE id=$1`, uid)
		require.NoError(t, err)
		defer rows.Close()
		require.True(t, rows.Next())
		var balance float64
		require.NoError(t, rows.Scan(&balance))
		return balance
	}
	before := readBalance()
	first, total, paid, err := svc.GetUserBalanceHistory(ctx, uid, 1, 20, "")
	require.NoError(t, err)
	require.EqualValues(t, 4, total)
	require.Equal(t, 10.0, paid)
	require.Len(t, first, 4)
	require.Equal(t, before, readBalance())
	byType := map[string]service.RedeemCode{}
	for _, record := range first {
		byType[record.Type] = record
	}
	require.Equal(t, .03, byType[service.BalanceHistoryCheckin].Value)
	require.Contains(t, byType[service.BalanceHistoryCheckin].Notes, "补签卡签到")
	require.Contains(t, byType[service.BalanceHistoryCheckin].Notes, "抵扣奖励欠款 $0.02")
	require.Contains(t, byType[service.BalanceHistoryAchievement].Notes, "后续已撤回")
	require.Equal(t, fmt.Sprintf("ACH-%d-T02", uid), byType[service.BalanceHistoryAchievement].Code)
	require.Equal(t, fmt.Sprintf("SERIES-%d-A-K", uid), byType[service.BalanceHistoryAchievementSeries].Code)
	for _, kind := range []string{service.BalanceHistoryCheckin, service.BalanceHistoryAchievement, service.BalanceHistoryAchievementSeries} {
		filtered, count, recharged, err := svc.GetUserBalanceHistory(ctx, uid, 1, 20, kind)
		require.NoError(t, err)
		require.EqualValues(t, 1, count)
		require.Len(t, filtered, 1)
		require.Equal(t, kind, filtered[0].Type)
		require.Equal(t, 10.0, recharged)
		empty, count, _, err := svc.GetUserBalanceHistory(ctx, uid, 2, 20, kind)
		require.NoError(t, err)
		require.Empty(t, empty)
		require.EqualValues(t, 1, count)
	}
	// Equal timestamps and interleaved sources must paginate without duplicates.
	var codes []string
	for page := 1; page <= 4; page++ {
		items, _, _, err := svc.GetUserBalanceHistory(ctx, uid, page, 1, "")
		require.NoError(t, err)
		require.Len(t, items, 1)
		codes = append(codes, items[0].Code)
	}
	for i := range first {
		require.Equal(t, first[i].Code, codes[i])
	}
	// New live payouts and retries appear automatically, once, without any
	// history backfill job or extra writes triggered by the history endpoint.
	exec(`INSERT INTO payment_orders(user_id,amount,pay_amount,expires_at,status,payment_type,provider_snapshot,out_trade_no,paid_at,completed_at)
 VALUES($1,10,10,now(),'COMPLETED','alipay','{"currency":"CNY","recharge_principal":10}'::jsonb,$2,now(),now())`, uid, uuid.NewString())
	exec(`UPDATE achievement_config SET payload=payload||'{"cash_enabled":true,"cash_scope":"all","milestone_cash_enabled":true,"budget_enabled":false}'::jsonb`)
	exec(`INSERT INTO achievement_growth(user_id,tokens,last_balance_paid_at) VALUES($1,1000000,now())`, uid)
	exec(`INSERT INTO vip_reward_debt(user_id,amount) VALUES($1,.005)`, uid)
	r := &userRepository{sql: client}
	for i := 0; i < 2; i++ {
		_, err = r.AchievementMutation(ctx, uid, "checkin", "", service.CheckinDate(time.Now()), fmt.Sprint("live-", i))
		require.NoError(t, err)
		_, err = r.AchievementMutation(ctx, uid, "claim", "T01", "", "")
		require.NoError(t, err)
	}
	afterPayout := readBalance()
	require.InDelta(t, before+.005+.1, afterPayout, 1e-8)
	for i := 0; i < 2; i++ {
		items, count, recharged, err := svc.GetUserBalanceHistory(ctx, uid, 1, 20, "")
		require.NoError(t, err)
		require.EqualValues(t, 6, count)
		require.Len(t, items, 6)
		require.Equal(t, 10.0, recharged)
		require.Equal(t, afterPayout, readBalance())
	}
	old, count, _, err := svc.GetUserBalanceHistory(ctx, uid, 1, 20, "balance")
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	require.Len(t, old, 1)
}
