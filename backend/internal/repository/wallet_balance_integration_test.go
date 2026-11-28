//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWalletBalanceIncludesOnlySpendableCheckinCredits(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	userID := achievementAccessUser(t, tx)
	repo := newUserRepositoryWithSQL(testEntClient(t), tx)
	_, err := tx.Exec(`UPDATE users SET balance = 20, frozen_balance = 4 WHERE id = $1`, userID)
	require.NoError(t, err)

	// A makeup reward keeps the historical sign-in date, but is granted this month.
	for _, grant := range []struct {
		day    string
		amount float64
	}{
		{"2024-05-01", 7},
		{"2024-05-02", 3},
		{"2024-05-03", 2},
		{"2024-05-04", 1},
	} {
		addCheckinBalanceFixture(t, tx, userID, grant.day, grant.amount, time.Now())
	}
	_, err = tx.Exec(`
UPDATE checkin_balance_credits
SET credit_month = NULL,
    expires_at = statement_timestamp() + CASE day
        WHEN DATE '2024-05-02' THEN interval '1 day'
        WHEN DATE '2024-05-03' THEN interval '2 days'
        ELSE interval '-1 day'
    END
WHERE user_id = $1 AND day <> DATE '2024-05-01'`, userID)
	require.NoError(t, err)

	balance, err := repo.GetWalletBalance(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, 32.0, balance.AvailableBalance)
	require.Equal(t, 20.0, balance.OtherAvailable)
	require.Equal(t, 4.0, balance.FrozenBalance)
	require.Equal(t, 7.0, balance.CheckinCredit.Available)
	require.NotNil(t, balance.LegacyCheckinCredit)
	require.Equal(t, 5.0, balance.LegacyCheckinCredit.Available)
	require.Equal(t, 3.0, balance.LegacyCheckinCredit.NextAmount)
	var month string
	var expiry time.Time
	require.NoError(t, tx.QueryRow(`SELECT to_char(credit_month, 'YYYY-MM'), expires_at FROM checkin_balance_credits WHERE user_id = $1 AND day = DATE '2024-05-01'`, userID).Scan(&month, &expiry))
	require.Equal(t, month, balance.CheckinCredit.Month)
	require.True(t, expiry.Equal(balance.CheckinCredit.ExpiresAt))

	raw, err := repo.AchievementSnapshot(ctx, userID)
	require.NoError(t, err)
	var snapshot struct {
		CheckinCredit struct {
			Available  float64   `json:"available"`
			Month      string    `json:"month"`
			ExpiresAt  time.Time `json:"expires_at"`
			NextAmount float64   `json:"next_amount"`
			NextExpiry time.Time `json:"next_expiry"`
		} `json:"checkin_credit"`
		LegacyCheckinCredit struct {
			Available float64 `json:"available"`
		} `json:"legacy_checkin_credit"`
	}
	require.NoError(t, json.Unmarshal(raw, &snapshot))
	require.Equal(t, balance.CheckinCredit.Available, snapshot.CheckinCredit.Available)
	require.Equal(t, balance.CheckinCredit.Month, snapshot.CheckinCredit.Month)
	require.True(t, balance.CheckinCredit.ExpiresAt.Equal(snapshot.CheckinCredit.ExpiresAt))
	require.Equal(t, snapshot.CheckinCredit.Available, snapshot.CheckinCredit.NextAmount)
	require.True(t, snapshot.CheckinCredit.ExpiresAt.Equal(snapshot.CheckinCredit.NextExpiry))
	require.Equal(t, balance.LegacyCheckinCredit.Available, snapshot.LegacyCheckinCredit.Available)

	// Reading a logically expired balance does not settle it or change the audit ledger.
	var rawBalance, expiredLot float64
	require.NoError(t, tx.QueryRow(`SELECT balance, (SELECT remaining FROM checkin_balance_credits WHERE user_id = $1 AND day = DATE '2024-05-04') FROM users WHERE id = $1`, userID).Scan(&rawBalance, &expiredLot))
	require.Equal(t, 33.0, rawBalance)
	require.Equal(t, 1.0, expiredLot)
}
