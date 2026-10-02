//go:build integration

package repository

import (
	"context"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestVIPAdminClaimsSameMilestonesOnce(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	var uid int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,role,status,balance) VALUES($1,'fixture','admin','active',0) RETURNING id`, fmt.Sprintf("vip-admin-%d@example.com", time.Now().UnixNano())).Scan(&uid))
	_, err = tx.ExecContext(ctx, `UPDATE vip_rules SET payload=jsonb_set(payload,'{enabled}','true') WHERE id=true`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) VALUES($1,'opening',$2,300)`, uid, fmt.Sprint(uid))
	require.NoError(t, err)
	repo := &userRepository{sql: tx}
	amount, err := repo.VIPClaimReward(ctx, uid, 1)
	require.NoError(t, err)
	require.Equal(t, 1.0, amount)
	amount, err = repo.VIPClaimReward(ctx, uid, 1)
	require.NoError(t, err)
	require.Equal(t, 0.0, amount)
	amount, err = repo.VIPClaimReward(ctx, uid, 2)
	require.NoError(t, err)
	require.Equal(t, 3.0, amount)
	_, err = tx.ExecContext(ctx, `SAVEPOINT denied`)
	require.NoError(t, err)
	_, err = repo.VIPClaimReward(ctx, uid, 3)
	require.ErrorIs(t, err, service.ErrVIPRewardThreshold)
	_, err = tx.ExecContext(ctx, `ROLLBACK TO denied`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `UPDATE users SET status='disabled' WHERE id=$1`, uid)
	require.NoError(t, err)
	_, err = repo.VIPClaimReward(ctx, uid, 1)
	require.ErrorIs(t, err, service.ErrVIPRewardAccountUnavailable)
}

func TestVIPRewardsClaimRefundDebtAndNoRepeat(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	var uid int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,balance) VALUES($1,'fixture',0) RETURNING id`, fmt.Sprintf("vip-reward-%d@example.com", time.Now().UnixNano())).Scan(&uid))
	_, err = tx.ExecContext(ctx, `UPDATE vip_rules SET payload=jsonb_set(payload,'{enabled}','true');`)
	require.NoError(t, err)
	var amount float64
	_, err = tx.ExecContext(ctx, `SAVEPOINT denied`)
	require.NoError(t, err)
	require.Error(t, tx.QueryRowContext(ctx, `SELECT vip_claim_reward($1,1)`, uid).Scan(&amount))
	_, err = tx.ExecContext(ctx, `ROLLBACK TO denied`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) VALUES($1,'opening',$2,300)`, uid, fmt.Sprint(uid))
	require.NoError(t, err)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT vip_claim_reward($1,1)`, uid).Scan(&amount))
	require.Equal(t, 1.0, amount)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT vip_claim_reward($1,1)`, uid).Scan(&amount))
	require.Equal(t, 0.0, amount)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT vip_claim_reward($1,2)`, uid).Scan(&amount))
	require.Equal(t, 3.0, amount)
	var balance, debt float64
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, uid).Scan(&balance))
	require.Equal(t, 4.0, balance)
	_, err = tx.ExecContext(ctx, `UPDATE users SET balance=1 WHERE id=$1`, uid)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) VALUES($1,'admin_balance',$2,-250)`, uid, fmt.Sprint(uid)+":refund")
	require.NoError(t, err)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, uid).Scan(&balance))
	require.Equal(t, 0.0, balance)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT amount FROM vip_reward_debt WHERE user_id=$1`, uid).Scan(&debt))
	require.Equal(t, 3.0, debt)
	_, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance+2 WHERE id=$1`, uid)
	require.NoError(t, err)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, uid).Scan(&balance))
	require.Equal(t, 0.0, balance)
	_, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance+5 WHERE id=$1`, uid)
	require.NoError(t, err)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, uid).Scan(&balance))
	require.Equal(t, 4.0, balance)
	_, err = tx.ExecContext(ctx, `INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) VALUES($1,'opening',$2,250)`, uid, fmt.Sprint(uid)+":restore")
	require.NoError(t, err)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT vip_claim_reward($1,2)`, uid).Scan(&amount))
	require.Equal(t, 0.0, amount)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT sum(amount) FROM vip_recharge_ledger WHERE user_id=$1`, uid).Scan(&amount))
	require.Equal(t, 300.0, amount)
}
