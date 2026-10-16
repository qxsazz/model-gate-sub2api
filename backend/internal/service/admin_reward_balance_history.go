package service

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// Project immutable reward receipts into the existing history response. This is
// deliberately read-only: old and new rewards become visible without replaying
// credit functions, creating redeemable codes, or contributing to paid recharge
// / VIP growth. A receipt's net is the actual balance increase after debt offsets.
const rewardBalanceHistoryCTE = `WITH rewards AS (
 SELECT 'SIGN-' || user_id || '-' || to_char(day,'YYYYMMDD') AS code,
        'checkin_balance' AS type, net, gross, offset_amount, created_at,
        CASE source WHEN 'admin' THEN '管理员补签' WHEN 'card' THEN '补签卡签到' ELSE '每日签到' END
          || ' · ' || day::text || ' · VIP' || tier AS description,
        NULL::timestamptz AS revoked_at
 FROM achievement_checkins WHERE user_id=$1 AND gross>0
 UNION ALL
 SELECT 'ACH-' || c.user_id || '-' || c.key, 'achievement_balance',
        c.net, c.gross, c.offset_amount, c.created_at,
        '成就奖励 · ' || a.name || '（' || c.key || '）', c.revoked_at
 FROM achievement_claims c JOIN achievement_catalog a ON a.key=c.key
 WHERE c.user_id=$1 AND c.gross>0
 UNION ALL
 SELECT 'SERIES-' || c.user_id || '-' || c.key, 'achievement_series_balance',
        c.net, c.gross, c.offset_amount, c.created_at,
        '系列成就奖励 · ' || a.name || '（' || c.key || '）', c.revoked_at
 FROM achievement_series_claims c JOIN achievement_series a ON a.key=c.key
 WHERE c.user_id=$1 AND c.gross>0
) `

func isRewardBalanceHistoryType(kind string) bool {
	return kind == BalanceHistoryCheckin || kind == BalanceHistoryAchievement || kind == BalanceHistoryAchievementSeries
}

func (s *adminServiceImpl) listRewardBalanceHistory(ctx context.Context, userID int64, params pagination.PaginationParams, kind string) ([]RedeemCode, int64, error) {
	if s == nil || s.entClient == nil || userID <= 0 {
		return nil, 0, nil
	}
	// Count and page in one statement/snapshot, including empty out-of-range
	// pages. LIMIT is intentionally not capped by PaginationParams.Limit here:
	// the all-sources merge needs offset+pageSize candidates from each source.
	limit := params.PageSize
	if limit < 1 {
		limit = params.Limit()
	}
	rows, err := s.entClient.QueryContext(ctx, rewardBalanceHistoryCTE+`
 SELECT totals.total, page.code, page.type, page.net::double precision,
        page.gross::double precision, page.offset_amount::double precision,
        page.created_at, page.description, page.revoked_at
 FROM (SELECT count(*) AS total FROM rewards WHERE ($2='' OR type=$2)) totals
 LEFT JOIN LATERAL (
   SELECT * FROM rewards WHERE ($2='' OR type=$2)
   ORDER BY created_at DESC, type ASC, code DESC OFFSET $3 LIMIT $4
 ) page ON true
 ORDER BY page.created_at DESC, page.type ASC, page.code DESC`, userID, kind, params.Offset(), limit)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	codes := []RedeemCode{}
	var total int64
	for rows.Next() {
		// The LEFT JOIN emits a totals-only row when the page is empty.
		var code, source, description *string
		var net, gross, offset *float64
		var createdAt, revokedAt *time.Time
		if err := rows.Scan(&total, &code, &source, &net, &gross, &offset, &createdAt, &description, &revokedAt); err != nil {
			return nil, 0, err
		}
		if code == nil {
			continue
		}
		notes := rewardBalanceHistoryNotes(*description, *gross, *net, *offset, revokedAt)
		usedBy := userID
		codes = append(codes, RedeemCode{
			ID: rewardBalanceHistoryID(*code), Code: *code, Type: *source,
			Value: *net, Status: StatusUsed, UsedBy: &usedBy,
			UsedAt: createdAt, CreatedAt: *createdAt, Notes: notes,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return codes, total, nil
}

// Keep numeric IDs stable and within JavaScript's safe integer range for legacy
// clients. The full source code is the authoritative identity, not this hash.
func rewardBalanceHistoryID(code string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(code))
	return -int64(h.Sum64()%((1<<53)-1) + 1)
}

func rewardBalanceHistoryNotes(description string, gross, net, offset float64, revokedAt *time.Time) string {
	amount := func(value float64) string {
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.8f", value), "0"), ".")
	}
	notes := description + "；奖励 $" + amount(gross) + "；实际入账 $" + amount(net)
	if offset > 0 {
		notes += "；抵扣奖励欠款 $" + amount(offset)
	}
	if revokedAt != nil {
		notes += "；该奖励后续已撤回（保留原始入账记录）"
	}
	return notes
}
