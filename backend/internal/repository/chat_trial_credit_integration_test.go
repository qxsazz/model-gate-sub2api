//go:build integration

package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestChatTrialCreditAwardsOneGrantAcrossQQAndWeChat(t *testing.T) {
	_ = testEntClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db := integrationDB
	svc := botTestService(db)
	oldAccount := achievementAccessUser(t, db)
	oldIdentity := botTestIdentity("qq_official")
	botTestBind(t, svc, oldAccount, oldIdentity)

	receipt, err := svc.ChatBotClaimTrialCredit(ctx, botTestEvent(oldIdentity))
	require.NoError(t, err)
	require.Equal(t, "disabled", receipt.Status)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`UPDATE chat_trial_credit_policy SET enabled=false, amount=0, eligible_after=NULL WHERE id=true`)
	})
	_, err = db.ExecContext(ctx, `SELECT chat_configure_trial_credit(true,3)`)
	require.NoError(t, err)
	var cutoff time.Time
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT eligible_after FROM chat_trial_credit_policy WHERE id=true`).Scan(&cutoff))
	receipt, err = svc.ChatBotClaimTrialCredit(ctx, botTestEvent(oldIdentity))
	require.NoError(t, err)
	require.Equal(t, "not_new", receipt.Status)

	email := "trial-credit-" + uuid.NewString() + "@example.invalid"
	var uid int64
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO users(email,password_hash,balance,created_at)
		  VALUES($1,'fixture',0,clock_timestamp()) RETURNING id`, email).Scan(&uid))
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM redeem_codes WHERE type='trial_credit' AND used_by=$1`, uid)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users WHERE id=$1`, uid)
	})
	qq, wx := botTestIdentity("qq_official"), botTestIdentity("wechat_native")
	unbound := botTestEvent(wx)
	receipt, err = svc.ChatBotClaimTrialCredit(ctx, unbound)
	require.NoError(t, err)
	require.Equal(t, "not_bound", receipt.Status)
	botTestBind(t, svc, uid, qq)
	wxBinding := botTestBind(t, svc, uid, wx)
	receipt, err = svc.ChatBotClaimTrialCredit(ctx, unbound)
	require.NoError(t, err)
	require.True(t, receipt.Replayed)
	require.Equal(t, "not_bound", receipt.Status)

	inputs := []service.BotIdentityEvent{botTestEvent(qq), botTestEvent(wx)}
	var receipts [2]*service.BotReceipt
	var errors [2]error
	start := make(chan struct{})
	var calls sync.WaitGroup
	for i, input := range inputs {
		calls.Go(func() {
			<-start
			receipts[i], errors[i] = svc.ChatBotClaimTrialCredit(ctx, input)
		})
	}
	close(start)
	calls.Wait()
	for _, err := range errors {
		require.NoError(t, err)
	}
	require.ElementsMatch(t, []string{"claimed", "already_claimed"},
		[]string{receipts[0].Status, receipts[1].Status})
	for i, input := range inputs {
		replay, err := svc.ChatBotClaimTrialCredit(ctx, input)
		require.NoError(t, err)
		require.True(t, replay.Replayed)
		require.Equal(t, receipts[i].Status, replay.Status)
	}

	require.NoError(t, svc.RevokeChatBinding(ctx, uid, wxBinding))
	botTestBind(t, svc, uid, wx)
	_, err = db.ExecContext(ctx, `SELECT chat_configure_trial_credit(false,3)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SELECT chat_configure_trial_credit(true,4)`)
	require.NoError(t, err)
	var resumedCutoff time.Time
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT eligible_after FROM chat_trial_credit_policy WHERE id=true`).Scan(&resumedCutoff))
	require.Equal(t, cutoff, resumedCutoff)
	receipt, err = svc.ChatBotClaimTrialCredit(ctx, botTestEvent(wx))
	require.NoError(t, err)
	require.Equal(t, "already_claimed", receipt.Status)

	var grantCount, historyCount int
	var balance, principal float64
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT balance,
		       (achievement_access_snapshot(id)->>'qualified_recharge_cny')::numeric,
		       (SELECT count(*) FROM chat_trial_credit_claims WHERE user_id=users.id),
		       (SELECT count(*) FROM redeem_codes WHERE used_by=users.id AND type='trial_credit' AND value=3)
		FROM users WHERE id=$1`, uid).Scan(&balance, &principal, &grantCount, &historyCount))
	require.Equal(t, 3.0, balance)
	require.Zero(t, principal)
	require.Equal(t, 1, grantCount)
	require.Equal(t, 1, historyCount)
}
