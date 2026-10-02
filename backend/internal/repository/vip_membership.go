package repository

import (
	"context"
	"database/sql"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
	"sort"
)

func vipRewardError(err error) error {
	if err == nil {
		return nil
	}
	var dbErr *pq.Error
	if errors.As(err, &dbErr) && dbErr.Code == "P0001" {
		switch dbErr.Message {
		case "User unavailable":
			return service.ErrVIPRewardAccountUnavailable.WithCause(err)
		case "VIP disabled":
			return service.ErrVIPRewardDisabled.WithCause(err)
		case "Invalid milestone":
			return service.ErrVIPRewardInvalidLevel.WithCause(err)
		case "Milestone not reached":
			return service.ErrVIPRewardThreshold.WithCause(err)
		case "Invalid reward":
			return service.ErrVIPRewardConfig.WithCause(err)
		}
	}
	return service.ErrVIPRewardUnavailable.WithCause(err)
}

func (r *userRepository) VIPClaimReward(ctx context.Context, id int64, level int) (float64, error) {
	rows, err := r.sql.QueryContext(ctx, `SELECT vip_claim_reward($1,$2)`, id, level)
	if err != nil {
		return 0, vipRewardError(err)
	}
	defer func() { _ = rows.Close() }()
	var amount float64
	if !rows.Next() {
		if err = rows.Err(); err != nil {
			return 0, vipRewardError(err)
		}
		return 0, vipRewardError(sql.ErrNoRows)
	}
	err = rows.Scan(&amount)
	if err != nil {
		return 0, vipRewardError(err)
	}
	return amount, vipRewardError(rows.Err())
}
func (r *userRepository) VIPMembership(ctx context.Context, id int64) (*service.VIPMembership, error) {
	snapshot, err := r.VIPSnapshot(ctx, id)
	if err != nil {
		return nil, err
	}
	result := &service.VIPMembership{Rewards: []service.VIPReward{}, Seats: []service.VIPHonorSeat{}, DiscountSummaries: map[int][]float64{}}
	rows, err := r.sql.QueryContext(ctx, `SELECT level,threshold,amount,revoked_at IS NOT NULL FROM vip_reward_claims WHERE user_id=$1`, id)
	if err != nil {
		return nil, err
	}
	claims := map[int]service.VIPReward{}
	for rows.Next() {
		var claim service.VIPReward
		var revoked bool
		if err = rows.Scan(&claim.Level, &claim.Threshold, &claim.Amount, &revoked); err != nil {
			break
		}
		claim.Status = "claimed"
		if revoked {
			claim.Status = "revoked"
		}
		result.Claimed += claim.Amount
		claims[claim.Level] = claim
	}
	rowErr := rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	if rowErr != nil {
		return nil, rowErr
	}
	for _, tier := range snapshot.Rules.Tiers {
		amount, _ := decimal.NewFromFloat(tier.Threshold).Mul(decimal.NewFromFloat(.01)).Round(2).Float64()
		reward := service.VIPReward{Level: tier.Level, Threshold: tier.Threshold, Amount: amount, Status: "locked"}
		if snapshot.Enabled && snapshot.Total >= tier.Threshold {
			reward.Status = "available"
		}
		if claim, ok := claims[tier.Level]; ok {
			reward = claim
		}
		result.Rewards = append(result.Rewards, reward)
		cuts := map[float64]bool{}
		for _, rule := range snapshot.Rules.Groups {
			if rule.Private || rule.Access || tier.Level > len(rule.Discounts) {
				continue
			}
			for _, group := range snapshot.Groups {
				if group.ID == rule.GroupID && !group.Exclusive {
					cut := decimal.NewFromFloat(group.BaseRate).Sub(decimal.NewFromFloat(service.VIPDiscountedRate(group.BaseRate, rule.Floor, rule.Discounts[tier.Level-1])))
					value, _ := cut.Float64()
					if value > 0 {
						cuts[value] = true
					}
				}
			}
		}
		values := []float64{}
		for cut := range cuts {
			values = append(values, cut)
		}
		sort.Sort(sort.Reverse(sort.Float64Slice(values)))
		result.DiscountSummaries[tier.Level] = values
	}
	rows, err = r.sql.QueryContext(ctx, `SELECT COALESCE((SELECT amount FROM vip_reward_debt WHERE user_id=$1),0)`, id)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		err = rows.Scan(&result.Debt)
	}
	rowErr = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	if rowErr != nil {
		return nil, rowErr
	}
	if !snapshot.Enabled {
		return result, nil
	}
	// Rank by effective badge, then net recharge. Only active ordinary users participate.
	rows, err = r.sql.QueryContext(ctx, `WITH totals AS (
 SELECT user_id,GREATEST(sum(amount),0) total FROM vip_recharge_ledger GROUP BY user_id), ranked AS (
 SELECT u.id,COALESCE(NULLIF(u.username,''),u.email) name,t.total,
 COALESCE((SELECT value::integer FROM vip_overrides WHERE user_id=u.id AND benefit='badge' AND (expires_at IS NULL OR expires_at>now())),
 (SELECT max((tier->>'level')::integer) FROM vip_rules,jsonb_array_elements(payload->'tiers') tier WHERE (tier->>'threshold')::numeric<=COALESCE(t.total,0)),0) level
 FROM users u LEFT JOIN totals t ON t.user_id=u.id WHERE u.deleted_at IS NULL AND u.status='active' AND u.role='user')
 SELECT name,level FROM ranked WHERE level>0 ORDER BY level DESC,total DESC NULLS LAST,id LIMIT 10`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var seat service.VIPHonorSeat
		if err = rows.Scan(&seat.Name, &seat.Level); err != nil {
			return nil, err
		}
		seat.Name = service.VIPPrivateName(seat.Name)
		result.Seats = append(result.Seats, seat)
	}
	return result, rows.Err()
}
