//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type calendarClockExecutor struct {
	sqlExecutor
	day string
}

func (c calendarClockExecutor) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	// Freeze the snapshot's server clock, retaining its actual production query.
	query = strings.ReplaceAll(query, "clock_timestamp()", "$2::timestamptz")
	return c.sqlExecutor.QueryContext(ctx, query, append(args, c.day+"T12:00:00+08:00")...)
}

func TestAchievementCalendarMonthBoundaries(t *testing.T) {
	_ = testEntClient(t)
	for _, day := range []string{"2026-10-05", "2027-03-01", "2028-03-01", "2027-03-31", "2027-01-01"} {
		t.Run(day, func(t *testing.T) {
			ctx := context.Background()
			tx := testTx(t)
			r := &userRepository{sql: calendarClockExecutor{sqlExecutor: tx, day: day}}
			var id int64
			var firstDay, minimum string
			require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,created_at) VALUES($1,'fixture',$2::timestamptz-interval '100 days') RETURNING id`, uuid.NewString()+"@example.com", day).Scan(&id))
			require.NoError(t, tx.QueryRowContext(ctx, `SELECT (date_trunc('month',$1::date)-interval '1 month')::date::text,($1::date-30)::text`, day).Scan(&firstDay, &minimum))
			_, err := tx.ExecContext(ctx, `INSERT INTO achievement_checkins(user_id,day,request_key,tier,revision,reason,streak) SELECT $1,d,d::text,0,1,'cash_disabled',1 FROM (VALUES($2::date),($3::date)) v(d) ON CONFLICT DO NOTHING`, id, firstDay, minimum)
			require.NoError(t, err)
			raw, err := r.AchievementSnapshot(ctx, id)
			require.NoError(t, err)
			var snapshot struct {
				Calendar []string `json:"calendar"`
				MinDate  string   `json:"card_min_date"`
			}
			require.NoError(t, json.Unmarshal(raw, &snapshot))
			require.Contains(t, snapshot.Calendar, firstDay)
			require.Contains(t, snapshot.Calendar, minimum)
			require.Equal(t, minimum, snapshot.MinDate)
		})
	}
}

func TestAchievementCalendarIncludesWholePreviousMonth(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx := testTx(t)
	r := &userRepository{sql: tx}
	var id int64
	var firstDay string
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,created_at) VALUES($1,'fixture',now()-interval '100 days') RETURNING id`, uuid.NewString()+"@example.com").Scan(&id))
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT (date_trunc('month',clock_timestamp() AT TIME ZONE 'Asia/Shanghai')-interval '1 month')::date::text`).Scan(&firstDay))
	_, err := tx.ExecContext(ctx, `INSERT INTO achievement_checkins(user_id,day,request_key,tier,revision,reason,streak) VALUES($1,$2::date,'previous-month',0,1,'cash_disabled',1)`, id, firstDay)
	require.NoError(t, err)
	raw, err := r.AchievementSnapshot(ctx, id)
	require.NoError(t, err)
	var snapshot struct {
		Calendar []string `json:"calendar"`
		MinDate  string   `json:"card_min_date"`
	}
	require.NoError(t, json.Unmarshal(raw, &snapshot))
	require.Contains(t, snapshot.Calendar, firstDay, "displaying the previous month must preserve its signed dates")
	var minimum string
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT ((clock_timestamp() AT TIME ZONE 'Asia/Shanghai')::date-30)::text`).Scan(&minimum))
	require.Equal(t, minimum, snapshot.MinDate, "calendar display must not extend makeup eligibility")
}
