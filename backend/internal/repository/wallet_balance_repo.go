package repository

import (
	"context"
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.WalletBalanceRepository = (*userRepository)(nil)

// The wallet and achievement views use the same statement-time cutoffs and lot totals.
// The outer query must alias the user row as u.
const checkinCreditSummaryJoin = `
CROSS JOIN LATERAL (
    SELECT local_month::date AS month_start,
           (local_month + interval '1 month') AT TIME ZONE 'Asia/Shanghai' AS expires_at
    FROM (SELECT date_trunc('month', statement_timestamp() AT TIME ZONE 'Asia/Shanghai') AS local_month) calendar
) checkin_period
CROSS JOIN LATERAL (
    SELECT COALESCE(sum(c.remaining) FILTER (WHERE c.expires_at <= statement_timestamp()), 0) AS expired,
           COALESCE(sum(c.remaining) FILTER (WHERE c.expires_at > statement_timestamp()
               AND c.credit_month = checkin_period.month_start), 0) AS monthly,
           COALESCE(sum(c.remaining) FILTER (WHERE c.expires_at > statement_timestamp()
               AND c.credit_month IS NULL), 0) AS legacy,
           min(c.expires_at) FILTER (WHERE c.expires_at > statement_timestamp()
               AND c.credit_month IS NULL) AS legacy_expiry
    FROM checkin_balance_credits c WHERE c.user_id = u.id AND c.remaining > 0
) checkin_totals
LEFT JOIN LATERAL (
    SELECT sum(c.remaining) AS amount FROM checkin_balance_credits c
    WHERE c.user_id = u.id AND c.remaining > 0 AND c.credit_month IS NULL
        AND c.expires_at = checkin_totals.legacy_expiry
) legacy_due ON checkin_totals.legacy_expiry IS NOT NULL
`

func (r *userRepository) GetWalletBalance(ctx context.Context, userID int64) (service.WalletBalance, error) {
	const query = `
SELECT u.balance - checkin_totals.expired, u.frozen_balance,
       u.balance - checkin_totals.expired - checkin_totals.monthly - checkin_totals.legacy,
       checkin_totals.monthly, to_char(checkin_period.month_start, 'YYYY-MM'), checkin_period.expires_at,
       checkin_totals.legacy, COALESCE(legacy_due.amount, 0), checkin_totals.legacy_expiry
FROM users u ` + checkinCreditSummaryJoin + `
WHERE u.id = $1 AND u.deleted_at IS NULL
`
	rows, err := r.sql.QueryContext(ctx, query, userID)
	if err != nil {
		return service.WalletBalance{}, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return service.WalletBalance{}, err
		}
		return service.WalletBalance{}, service.ErrUserNotFound
	}

	var balance service.WalletBalance
	var legacy service.LegacyCheckinCredit
	var legacyExpiry sql.NullTime
	if err := rows.Scan(
		&balance.AvailableBalance, &balance.FrozenBalance, &balance.OtherAvailable,
		&balance.CheckinCredit.Available, &balance.CheckinCredit.Month, &balance.CheckinCredit.ExpiresAt,
		&legacy.Available, &legacy.NextAmount, &legacyExpiry,
	); err != nil {
		return service.WalletBalance{}, err
	}
	if legacyExpiry.Valid {
		legacy.NextExpiry = legacyExpiry.Time
		balance.LegacyCheckinCredit = &legacy
	}
	return balance, rows.Err()
}
