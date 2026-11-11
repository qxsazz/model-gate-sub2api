//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func botTestService(db sqlExecutor) *service.UserService {
	return service.NewUserService(&userRepository{sql: db}, nil, nil, nil)
}
func botTestIdentity(platform string) service.ChatIdentity {
	return service.ChatIdentity{Platform: platform, BotID: "synthetic-bot", GroupID: "synthetic-group", SenderID: uuid.NewString()}
}
func botTestEvent(identity service.ChatIdentity) service.BotIdentityEvent {
	return service.BotIdentityEvent{Identity: identity, EventID: uuid.NewString()}
}
func botTestBind(t *testing.T, s *service.UserService, uid int64, identity service.ChatIdentity) int64 {
	t.Helper()
	ctx := context.Background()
	ch, err := s.CreateChatBindingChallenge(ctx, botTestEvent(identity))
	require.NoError(t, err)
	auth, err := s.AuthorizeChatBindingChallenge(ctx, uid, ch.ChallengeID)
	require.NoError(t, err)
	receipt, err := s.ConfirmChatBinding(ctx, service.BotConfirmInput{BotIdentityEvent: botTestEvent(identity), Code: auth.Code})
	require.NoError(t, err)
	require.Equal(t, "bound", receipt.Status)
	bindings, err := s.ListChatBindings(ctx, uid)
	require.NoError(t, err)
	for _, binding := range bindings {
		if binding.Identity == identity {
			return binding.ID
		}
	}
	t.Fatal("binding missing")
	return 0
}
func botTestTx(t *testing.T) (*sql.Tx, *service.UserService) {
	t.Helper()
	_ = testEntClient(t)
	tx, err := integrationDB.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	return tx, botTestService(tx)
}
func botCheck(t *testing.T, s *service.UserService, input service.BotCheckinInput, status string) *service.BotReceipt {
	t.Helper()
	receipt, err := s.ChatBotCheckin(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, status, receipt.Status)
	return receipt
}

func TestChatBindingOwnershipIdentityAndSingleUse(t *testing.T) {
	tx, s := botTestTx(t)
	ctx := context.Background()
	uid := achievementAccessUser(t, tx)
	other := achievementAccessUser(t, tx)
	identity := botTestIdentity("qq_official")
	event := botTestEvent(identity)
	ch, err := s.CreateChatBindingChallenge(ctx, event)
	require.NoError(t, err)
	replay, err := s.CreateChatBindingChallenge(ctx, event)
	require.NoError(t, err)
	require.Equal(t, ch.ChallengeID, replay.ChallengeID)
	require.True(t, replay.Replayed)
	preview, err := s.GetChatBindingChallenge(ctx, uid, ch.ChallengeID)
	require.NoError(t, err)
	require.Equal(t, identity, preview.Identity)
	auth, err := s.AuthorizeChatBindingChallenge(ctx, uid, ch.ChallengeID)
	require.NoError(t, err)
	_, err = s.AuthorizeChatBindingChallenge(ctx, other, ch.ChallengeID)
	require.Equal(t, "BOT_BINDING_CHALLENGE_CLAIMED", infraerrors.Reason(err))
	_, err = s.GetChatBindingChallenge(ctx, other, ch.ChallengeID)
	require.Equal(t, "BOT_BINDING_CHALLENGE_CLAIMED", infraerrors.Reason(err))
	var stored string
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT code_hash FROM chat_binding_challenges WHERE id=$1`, ch.ChallengeID).Scan(&stored))
	require.Len(t, stored, 64)
	require.NotContains(t, stored, strings.ReplaceAll(auth.Code, "-", ""))
	for _, field := range []string{"sender", "group", "bot", "platform"} {
		wrong := identity
		switch field {
		case "sender":
			wrong.SenderID = "other"
		case "group":
			wrong.GroupID = "other"
		case "bot":
			wrong.BotID = "other"
		case "platform":
			wrong.Platform = "wechat_native"
		}
		_, err = s.ConfirmChatBinding(ctx, service.BotConfirmInput{BotIdentityEvent: botTestEvent(wrong), Code: auth.Code})
		require.Equal(t, "BOT_BINDING_CODE_INVALID", infraerrors.Reason(err))
	}
	input := service.BotConfirmInput{BotIdentityEvent: botTestEvent(identity), Code: auth.Code}
	receipt, err := s.ConfirmChatBinding(ctx, input)
	require.NoError(t, err)
	require.Equal(t, "bound", receipt.Status)
	require.False(t, receipt.Replayed)
	receipt, err = s.ConfirmChatBinding(ctx, input)
	require.NoError(t, err)
	require.True(t, receipt.Replayed)
	_, err = s.AuthorizeChatBindingChallenge(ctx, uid, ch.ChallengeID)
	require.Equal(t, "BOT_BINDING_CHALLENGE_USED", infraerrors.Reason(err))
	bindings, err := s.ListChatBindings(ctx, uid)
	require.NoError(t, err)
	require.Len(t, bindings, 1)
	require.Equal(t, "BOT_BINDING_NOT_FOUND", infraerrors.Reason(s.RevokeChatBinding(ctx, other, bindings[0].ID)))
	require.NoError(t, s.RevokeChatBinding(ctx, uid, bindings[0].ID))
	// Retrying a successful confirmation after unlink never creates another binding.
	receipt, err = s.ConfirmChatBinding(ctx, input)
	require.NoError(t, err)
	require.True(t, receipt.Replayed)
	bindings, err = s.ListChatBindings(ctx, uid)
	require.NoError(t, err)
	require.Empty(t, bindings)
}

func TestChatBindingExpiryAttemptsAndSupersession(t *testing.T) {
	tx, s := botTestTx(t)
	ctx := context.Background()
	uid := achievementAccessUser(t, tx)
	for _, mode := range []string{"attempts", "expiry", "superseded"} {
		t.Run(mode, func(t *testing.T) {
			identity := botTestIdentity("wechat_native")
			ch, err := s.CreateChatBindingChallenge(ctx, botTestEvent(identity))
			require.NoError(t, err)
			auth, err := s.AuthorizeChatBindingChallenge(ctx, uid, ch.ChallengeID)
			require.NoError(t, err)
			switch mode {
			case "attempts":
				for i := 0; i < 5; i++ {
					bad := service.BotConfirmInput{BotIdentityEvent: botTestEvent(identity), Code: "AAAA-AAAA-AAAA-AAAA"}
					_, err = s.ConfirmChatBinding(ctx, bad)
					require.Equal(t, "BOT_BINDING_CODE_INVALID", infraerrors.Reason(err))
					_, err = s.ConfirmChatBinding(ctx, bad)
					require.Equal(t, "BOT_BINDING_CODE_INVALID", infraerrors.Reason(err))
				}
				var attempts int
				require.NoError(t, tx.QueryRowContext(ctx, `SELECT failed_attempts FROM chat_binding_challenges WHERE id=$1`, ch.ChallengeID).Scan(&attempts))
				require.Equal(t, 5, attempts)
			case "expiry":
				_, err = tx.ExecContext(ctx, `UPDATE chat_binding_challenges SET expires_at=now()-interval '1 second' WHERE id=$1`, ch.ChallengeID)
				require.NoError(t, err)
			case "superseded":
				_, err = s.CreateChatBindingChallenge(ctx, botTestEvent(identity))
				require.NoError(t, err)
			}
			_, err = s.AuthorizeChatBindingChallenge(ctx, uid, ch.ChallengeID)
			require.Equal(t, "BOT_BINDING_CHALLENGE_EXPIRED", infraerrors.Reason(err))
			_, err = s.ConfirmChatBinding(ctx, service.BotConfirmInput{BotIdentityEvent: botTestEvent(identity), Code: auth.Code})
			require.Error(t, err)
			var count int
			require.NoError(t, tx.QueryRowContext(ctx, `SELECT count(*) FROM chat_bindings WHERE identity_key=$1`, identity.Key()).Scan(&count))
			require.Zero(t, count)
		})
	}
}

func TestChatBotEligibilityReceiptsAndRefunds(t *testing.T) {
	tx, s := botTestTx(t)
	ctx := context.Background()
	_, err := tx.ExecContext(ctx, `UPDATE achievement_config SET payload=payload||'{"cash_enabled":true,"cash_scope":"all","budget_enabled":false}'`)
	require.NoError(t, err)
	for _, principal := range []float64{0, 9.99, 10} {
		t.Run(fmt.Sprint(principal), func(t *testing.T) {
			uid := achievementAccessUser(t, tx)
			identity := botTestIdentity("qq_official")
			botTestBind(t, s, uid, identity)
			// Administrative balance and spending do not establish online principal.
			_, err := tx.ExecContext(ctx, `UPDATE users SET balance=1000 WHERE id=$1`, uid)
			require.NoError(t, err)
			if principal > 0 {
				seedAchievementRecharge(t, tx, uid, principal)
			}
			input := service.BotCheckinInput{BotIdentityEvent: botTestEvent(identity), EventAt: time.Now()}
			expected := "ineligible"
			if principal == 10 {
				expected = "checked_in"
			}
			first := botCheck(t, s, input, expected)
			require.False(t, first.Replayed)
			repeated := botCheck(t, s, input, expected)
			require.True(t, repeated.Replayed)
			var count int
			require.NoError(t, tx.QueryRowContext(ctx, `SELECT count(*) FROM achievement_checkins WHERE user_id=$1`, uid).Scan(&count))
			if principal < 10 {
				require.Zero(t, count)
				var balance float64
				require.NoError(t, tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, uid).Scan(&balance))
				require.Equal(t, 1000.0, balance)
				seedAchievementRecharge(t, tx, uid, math.Round((10-principal)*100)/100)
				require.True(t, botCheck(t, s, input, "ineligible").Replayed)
				input.EventID = uuid.NewString()
				botCheck(t, s, input, "checked_in")
			} else {
				require.Equal(t, 1, count)
			}
			input.EventID = uuid.NewString()
			botCheck(t, s, input, "already_checked_in")
		})
	}
	uid := achievementAccessUser(t, tx)
	order := seedAchievementRecharge(t, tx, uid, 10)
	identity := botTestIdentity("wechat_native")
	botTestBind(t, s, uid, identity)
	_, err = tx.ExecContext(ctx, `UPDATE payment_orders SET status='PARTIALLY_REFUNDED',refund_amount=1,refund_at=now() WHERE id=$1`, order)
	require.NoError(t, err)
	botCheck(t, s, service.BotCheckinInput{BotIdentityEvent: botTestEvent(identity), EventAt: time.Now()}, "ineligible")
	seedAchievementRecharge(t, tx, uid, 1)
	_, err = tx.ExecContext(ctx, `UPDATE users SET balance=0 WHERE id=$1`, uid)
	require.NoError(t, err)
	botCheck(t, s, service.BotCheckinInput{BotIdentityEvent: botTestEvent(identity), EventAt: time.Now()}, "checked_in")
	_, err = tx.ExecContext(ctx, `UPDATE users SET status='disabled' WHERE id=$1`, uid)
	require.NoError(t, err)
	botCheck(t, s, service.BotCheckinInput{BotIdentityEvent: botTestEvent(identity), EventAt: time.Now()}, "account_unavailable")
}

func TestChatBotReplayAfterUnbindAndRebind(t *testing.T) {
	tx, s := botTestTx(t)
	ctx := context.Background()
	uid := achievementAccessUser(t, tx)
	other := achievementAccessUser(t, tx)
	seedAchievementRecharge(t, tx, uid, 10)
	seedAchievementRecharge(t, tx, other, 10)
	identity := botTestIdentity("qq_official")
	oldID := botTestBind(t, s, uid, identity)
	input := service.BotCheckinInput{BotIdentityEvent: botTestEvent(identity), EventAt: time.Now()}
	first := botCheck(t, s, input, "checked_in")
	require.NoError(t, s.RevokeChatBinding(ctx, uid, oldID))
	unbound := service.BotCheckinInput{BotIdentityEvent: botTestEvent(identity), EventAt: time.Now()}
	botCheck(t, s, unbound, "not_bound")
	challenge, err := s.CreateChatBindingChallenge(ctx, botTestEvent(identity))
	require.NoError(t, err)
	auth, err := s.AuthorizeChatBindingChallenge(ctx, other, challenge.ChallengeID)
	require.NoError(t, err)
	require.NoError(t, s.RevokeChatBinding(ctx, uid, oldID)) // old unlink retry cannot revoke the new challenge
	_, err = s.ConfirmChatBinding(ctx, service.BotConfirmInput{BotIdentityEvent: botTestEvent(identity), Code: auth.Code})
	require.NoError(t, err)
	replay := botCheck(t, s, input, "checked_in")
	require.True(t, replay.Replayed)
	require.Equal(t, first.RequestKey, replay.RequestKey)
	require.True(t, botCheck(t, s, unbound, "not_bound").Replayed)
	var originalUser int64
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT user_id FROM chat_bot_events WHERE request_key=$1`, first.RequestKey).Scan(&originalUser))
	require.Equal(t, uid, originalUser)
	var count int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT count(*) FROM achievement_checkins WHERE user_id=$1`, other).Scan(&count))
	require.Zero(t, count)
	changed := input
	changed.EventAt = changed.EventAt.Add(time.Second)
	_, err = s.ChatBotCheckin(ctx, changed)
	require.Equal(t, "BOT_EVENT_CONFLICT", infraerrors.Reason(err))
	input.EventID = uuid.NewString()
	botCheck(t, s, input, "checked_in")
}

func TestChatBotUnavailableAdmissionIsImmutableAndDoesNotWriteCheckin(t *testing.T) {
	tx, s := botTestTx(t)
	ctx := context.Background()
	uid := achievementAccessUser(t, tx)
	order := seedAchievementRecharge(t, tx, uid, 10)
	identity := botTestIdentity("qq_official")
	botTestBind(t, s, uid, identity)
	_, err := tx.ExecContext(ctx, `UPDATE payment_orders SET provider_snapshot='{"currency":"CNY","recharge_principal":"invalid"}' WHERE id=$1`, order)
	require.NoError(t, err)
	input := service.BotCheckinInput{BotIdentityEvent: botTestEvent(identity), EventAt: time.Now()}
	botCheck(t, s, input, "checkin_unavailable")
	var count int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT count(*) FROM achievement_checkins WHERE user_id=$1`, uid).Scan(&count))
	require.Zero(t, count)
	_, err = tx.ExecContext(ctx, `UPDATE payment_orders SET provider_snapshot='{"currency":"CNY","recharge_principal":10}' WHERE id=$1`, order)
	require.NoError(t, err)
	require.True(t, botCheck(t, s, input, "checkin_unavailable").Replayed)
	input.EventID = uuid.NewString()
	botCheck(t, s, input, "checked_in")
}

func TestChatBotShanghaiDayAndZeroCash(t *testing.T) {
	tx, s := botTestTx(t)
	ctx := context.Background()
	for _, policy := range []struct{ json, reason string }{
		{`{"cash_enabled":false}`, "cash_disabled"},
		{`{"cash_enabled":true,"cash_scope":"allowlist","cash_allowlist":[]}`, "not_in_cash_pilot"},
		{`{"cash_enabled":true,"cash_scope":"all","budget_enabled":true,"daily_budget":0}`, "budget_exhausted"},
	} {
		t.Run(policy.reason, func(t *testing.T) {
			uid := achievementAccessUser(t, tx)
			seedAchievementRecharge(t, tx, uid, 10)
			identity := botTestIdentity("wechat_native")
			botTestBind(t, s, uid, identity)
			_, err := tx.ExecContext(ctx, `UPDATE achievement_config SET payload=payload||$1::jsonb`, policy.json)
			require.NoError(t, err)
			loc, err := time.LoadLocation("Asia/Shanghai")
			require.NoError(t, err)
			now := time.Now().In(loc)
			midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
			input := service.BotCheckinInput{BotIdentityEvent: botTestEvent(identity), EventAt: midnight.Add(-time.Nanosecond)}
			old := botCheck(t, s, input, "date_changed")
			require.Equal(t, midnight.Format("2006-01-02"), old.CheckinDate)
			require.True(t, botCheck(t, s, input, "date_changed").Replayed)
			input = service.BotCheckinInput{BotIdentityEvent: botTestEvent(identity), EventAt: midnight}
			botCheck(t, s, input, "checked_in")
			var gross, balance float64
			var reason string
			var streak int
			require.NoError(t, tx.QueryRowContext(ctx, `SELECT c.gross,c.reason,c.streak,u.balance FROM achievement_checkins c JOIN users u ON u.id=c.user_id WHERE c.user_id=$1`, uid).Scan(&gross, &reason, &streak, &balance))
			require.Zero(t, gross)
			require.Zero(t, balance)
			require.Equal(t, policy.reason, reason)
			require.Equal(t, 1, streak)
		})
	}
}

func TestChatBotWebsiteQQWechatConcurrentOneReward(t *testing.T) {
	_ = testEntClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var config string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT payload::text FROM achievement_config`).Scan(&config))
	_, err := integrationDB.ExecContext(ctx, `UPDATE achievement_config SET payload=payload||'{"cash_enabled":true,"cash_scope":"all","budget_enabled":false}'`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `UPDATE achievement_config SET payload=$1::jsonb`, config)
	})
	s := botTestService(integrationDB)
	uid := achievementAccessUser(t, integrationDB)
	seedAchievementRecharge(t, integrationDB, uid, 10)
	qq, wx := botTestIdentity("qq_official"), botTestIdentity("wechat_native")
	botTestBind(t, s, uid, qq)
	botTestBind(t, s, uid, wx)
	start := make(chan struct{})
	failures := make(chan error, 18)
	var wg sync.WaitGroup
	for i := 0; i < 18; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			if i%3 == 0 {
				_, err := s.ChangeAchievement(ctx, uid, "checkin", "", service.CheckinDate(time.Now()), uuid.NewString())
				failures <- err
				return
			}
			identity := qq
			if i%3 == 2 {
				identity = wx
			}
			_, err := s.ChatBotCheckin(ctx, service.BotCheckinInput{BotIdentityEvent: botTestEvent(identity), EventAt: time.Now()})
			failures <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(failures)
	for err := range failures {
		require.NoError(t, err)
	}
	var count int
	var gross, balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*),COALESCE(sum(gross),0) FROM achievement_checkins WHERE user_id=$1`, uid).Scan(&count, &gross))
	require.Equal(t, 1, count)
	require.Greater(t, gross, 0.0)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, uid).Scan(&balance))
	require.Equal(t, gross, balance)
}
