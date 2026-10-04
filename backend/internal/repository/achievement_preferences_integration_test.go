//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAchievementCompanionAccountIsolation(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx := testTx(t)
	r := &userRepository{sql: tx}
	var first, second int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,created_at) VALUES($1,'fixture',((clock_timestamp() AT TIME ZONE 'Asia/Shanghai')::date::timestamp AT TIME ZONE 'Asia/Shanghai') - interval '97 days') RETURNING id`, uuid.NewString()+"@example.com").Scan(&first))
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash) VALUES($1,'fixture') RETURNING id`, uuid.NewString()+"@example.com").Scan(&second))
	require.NoError(t, r.SaveAchievementZodiac(ctx, first, "libra"))
	require.NoError(t, r.SaveAchievementZodiac(ctx, second, "aries"))
	var value struct {
		Zodiac            string `json:"zodiac"`
		CompanionshipDays int    `json:"companionship_days"`
		JoinedDate        string `json:"joined_date"`
	}
	raw, err := r.AchievementSnapshot(ctx, first)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &value))
	require.Equal(t, "libra", value.Zodiac)
	require.Equal(t, 98, value.CompanionshipDays)
	require.NotEmpty(t, value.JoinedDate)
	raw, err = r.AchievementSnapshot(ctx, second)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &value))
	require.Equal(t, "aries", value.Zodiac)
	require.Equal(t, 1, value.CompanionshipDays)
	require.NoError(t, r.SaveAchievementZodiac(ctx, first, ""))
	raw, err = r.AchievementSnapshot(ctx, first)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &value))
	require.Empty(t, value.Zodiac)
	_, err = tx.ExecContext(ctx, `UPDATE users SET status='disabled' WHERE id=$1`, first)
	require.NoError(t, err)
	require.Error(t, r.SaveAchievementZodiac(ctx, first, "pisces"))
}
