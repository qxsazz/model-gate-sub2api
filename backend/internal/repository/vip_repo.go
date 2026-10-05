package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"time"
)

func (r *userRepository) VIPRules(ctx context.Context) (service.VIPRules, error) {
	rows, err := r.sql.QueryContext(ctx, `SELECT payload||jsonb_build_object('daily_rewards',(SELECT jsonb_agg(amount ORDER BY tier) FROM achievement_daily_policy)) FROM vip_rules WHERE id=true`)
	if err != nil {
		return service.VIPRules{}, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return service.VIPRules{}, fmt.Errorf("VIP configuration missing")
	}
	var raw []byte
	if err = rows.Scan(&raw); err != nil {
		return service.VIPRules{}, err
	}
	var rules service.VIPRules
	err = json.Unmarshal(raw, &rules)
	return rules, err
}
func (r *userRepository) VIPSnapshot(ctx context.Context, id int64) (*service.VIPSnapshot, error) {
	return r.vipSnapshot(ctx, id, false)
}
func (r *userRepository) VIPMode(ctx context.Context) (bool, error) {
	rows, err := r.sql.QueryContext(ctx, `SELECT (payload->>'enabled')::boolean FROM vip_rules WHERE id=true`)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err = rows.Err(); err != nil {
			return false, err
		}
		return false, fmt.Errorf("VIP configuration missing")
	}
	var enabled bool
	err = rows.Scan(&enabled)
	if err == nil {
		err = rows.Err()
	}
	return enabled, err
}
func (r *userRepository) VIPAuthSnapshot(ctx context.Context, id int64) (*service.VIPSnapshot, error) {
	return r.vipSnapshot(ctx, id, true)
}
func (r *userRepository) vipSnapshot(ctx context.Context, id int64, authOnly bool) (*service.VIPSnapshot, error) {
	rules, err := r.VIPRules(ctx)
	if err != nil {
		return nil, err
	}
	result := &service.VIPSnapshot{Enabled: rules.Enabled, Rules: rules, Groups: []service.VIPGroupView{}, Ledger: []service.VIPLedgerEntry{}, Overrides: []service.VIPOverride{}}
	if authOnly && !rules.Enabled {
		return result, nil
	}
	var restrictPublic, unifiedManaged bool
	result.User = &service.VIPUserSummary{}
	rows, err := r.sql.QueryContext(ctx, `SELECT u.concurrency,u.rpm_limit,GREATEST(COALESCE((SELECT SUM(amount) FROM vip_recharge_ledger WHERE user_id=u.id),0),0),u.restrict_public_groups,u.id,u.email,COALESCE(u.username,''),EXISTS(SELECT 1 FROM vip_user_level_history WHERE user_id=u.id) FROM users u WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		_ = rows.Close()
		return nil, service.ErrUserNotFound
	}
	err = rows.Scan(&result.Concurrency, &result.RPM, &result.Total, &restrictPublic, &result.User.ID, &result.User.Email, &result.User.Username, &unifiedManaged)
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	growthRules := rules
	growthRules.Enabled = true
	result.GrowthTier = growthRules.Tier(result.Total)
	result.BaseConcurrency = result.Concurrency
	result.BaseRPM = result.RPM
	for _, t := range rules.Tiers {
		if t.Threshold > result.Total {
			copy := t
			result.Next = &copy
			break
		}
	}
	overrides := map[string]float64{}
	rows, err = r.sql.QueryContext(ctx, `SELECT benefit,value,expires_at,reason FROM vip_overrides WHERE user_id=$1 AND (expires_at IS NULL OR expires_at>now())`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var o service.VIPOverride
		if err = rows.Scan(&o.Benefit, &o.Value, &o.ExpiresAt, &o.Reason); err != nil {
			break
		}
		result.Overrides = append(result.Overrides, o)
		if rules.Enabled {
			overrides[o.Benefit] = o.Value
		}
	}
	rowErr := rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	if rowErr != nil {
		return nil, rowErr
	}
	tier, manualTier, err := rules.EffectiveTier(result.Total, result.Overrides, time.Now())
	if err != nil {
		return nil, err
	}
	result.Tier = tier
	result.TierSource = "growth"
	if !rules.Enabled {
		result.TierSource = "disabled"
	} else if manualTier {
		result.TierSource = "manual"
	}
	for _, o := range result.Overrides {
		if o.Benefit == "tier" {
			copy := o
			result.LevelOverride = &copy
			break
		}
	}
	benefitTotal := result.Total
	if unifiedManaged {
		overrides = map[string]float64{}
	}
	if manualTier {
		overrides = map[string]float64{}
		benefitTotal = tier.Threshold
	}
	result.BadgeLevel = tier.Level
	result.RebatePercent = tier.RebatePercent
	if tier.Concurrency > result.Concurrency {
		result.Concurrency = tier.Concurrency
	}
	if result.RPM > 0 && tier.RPM > result.RPM {
		result.RPM = tier.RPM
	}
	if v, ok := overrides["badge"]; ok {
		result.BadgeLevel = int(v)
	}
	if v, ok := overrides["concurrency"]; ok {
		result.Concurrency = int(v)
	}
	if v, ok := overrides["rpm"]; ok {
		result.RPM = int(v)
	}
	if v, ok := overrides["rebate"]; ok {
		result.RebatePercent = v
	}
	byGroup := map[int64]service.VIPGroupRule{}
	for _, g := range rules.Groups {
		byGroup[g.GroupID] = g
	}
	rows, err = r.sql.QueryContext(ctx, `SELECT g.id,g.name,g.platform,g.is_exclusive,g.subscription_type,g.rate_multiplier,
 EXISTS(SELECT 1 FROM user_allowed_groups WHERE user_id=$1 AND group_id=g.id),ur.rate_multiplier
 FROM groups g LEFT JOIN user_group_rate_multipliers ur ON ur.group_id=g.id AND ur.user_id=$1
 WHERE g.deleted_at IS NULL AND g.status='active' ORDER BY g.sort_order,g.id`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var g service.VIPGroupView
		var manual *float64
		if err = rows.Scan(&g.ID, &g.Name, &g.Platform, &g.Exclusive, &g.Subscription, &g.BaseRate, &g.Granted, &manual); err != nil {
			break
		}
		rule, has := byGroup[g.ID]
		autoAccess := rules.Enabled && has && rule.Access && !rule.Private && g.Exclusive && g.Subscription == "standard" && benefitTotal >= rules.AccessThreshold
		if v, ok := overrides["access"]; ok {
			autoAccess = v == 1 && has && rule.Access && !rule.Private && g.Exclusive && g.Subscription == "standard"
		}
		g.Granted = g.Granted || (!g.Exclusive && !restrictPublic) || autoAccess
		if !g.Exclusive && !g.Granted {
			continue
		}
		if g.Exclusive && !g.Granted && (!has || !rule.Access || rule.Private) {
			continue
		}
		if g.Subscription != "standard" {
			continue
		}
		g.Rate = g.BaseRate
		g.Participating = has && !g.Exclusive && !rule.Private && !rule.Access
		if rules.Enabled && g.Participating {
			cut := 0.0
			if result.Tier.Level > 0 {
				cut = rule.Discounts[result.Tier.Level-1]
			}
			if v, ok := overrides["discount"]; ok {
				cut = v
			}
			g.Rate = service.VIPDiscountedRate(g.BaseRate, rule.Floor, cut)
		}
		if manual != nil {
			g.Rate = *manual
		}
		result.Groups = append(result.Groups, g)
	}
	rowErr = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	if rowErr != nil {
		return nil, rowErr
	}
	if authOnly {
		return result, nil
	}
	rows, err = r.sql.QueryContext(ctx, `SELECT id,source,amount,reason,created_at FROM vip_recharge_ledger WHERE user_id=$1 ORDER BY id DESC LIMIT 50`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var e service.VIPLedgerEntry
		if err = rows.Scan(&e.ID, &e.Source, &e.Amount, &e.Reason, &e.CreatedAt); err != nil {
			return nil, err
		}
		result.Ledger = append(result.Ledger, e)
	}
	return result, rows.Err()
}
func (r *userRepository) VIPSaveRules(ctx context.Context, actor int64, rules service.VIPRules) error {
	if err := rules.Validate(); err != nil {
		return err
	}
	ids := []int64{}
	for _, g := range rules.Groups {
		ids = append(ids, g.GroupID)
	}
	rows, err := r.sql.QueryContext(ctx, `SELECT id,name,is_exclusive,subscription_type,rate_multiplier FROM groups WHERE id=ANY($1) AND deleted_at IS NULL`, pq.Array(ids))
	if err != nil {
		return err
	}
	found := 0
	for rows.Next() {
		var id int64
		var name, subscription string
		var exclusive bool
		var base float64
		if err = rows.Scan(&id, &name, &exclusive, &subscription, &base); err != nil {
			break
		}
		found++
		for _, g := range rules.Groups {
			if g.GroupID != id {
				continue
			}
			if g.Floor > base {
				err = fmt.Errorf("floor exceeds base for %s", name)
				break
			}
			if g.Access && (!exclusive || subscription != "standard" || name == "zth-plus" || name == "zth-pro" || name == "ceshi" || name == "ceshi-gemini") {
				err = fmt.Errorf("group %s cannot be automatically granted", name)
				break
			}
			if exclusive {
				for _, d := range g.Discounts {
					if d != 0 {
						err = fmt.Errorf("exclusive discount not allowed")
					}
				}
			}
		}
		if err != nil {
			break
		}
	}
	rowErr := rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	if rowErr != nil {
		return rowErr
	}
	if found != len(ids) {
		return fmt.Errorf("unknown group")
	}
	raw, err := json.Marshal(rules)
	if err != nil {
		return err
	}
	var daily any
	if len(rules.DailyRewards) > 0 {
		encoded, e := json.Marshal(rules.DailyRewards)
		if e != nil {
			return e
		}
		daily = string(encoded)
	}
	_, err = r.sql.ExecContext(ctx, `SELECT vip_save_rules_with_daily($1,$2::jsonb,$3::jsonb)`, actor, string(raw), daily)
	return err
}
func (r *userRepository) VIPSetOverride(ctx context.Context, actor, id int64, o service.VIPOverride) error {
	raw, _ := json.Marshal(o)
	_, err := r.sql.ExecContext(ctx, `SELECT vip_set_legacy_override($1,$2,$3,$4,$5,$6,$7::jsonb)`, id, o.Benefit, o.Value, o.ExpiresAt, o.Reason, actor, string(raw))
	var db *pq.Error
	if errors.As(err, &db) && db.Message == "VIP_LEVEL_MANAGED" {
		return infraerrors.BadRequest(db.Message, "该用户已使用统一VIP等级，请通过等级设置更新整套权益")
	}

	return err
}
func (r *userRepository) VIPClearOverride(ctx context.Context, actor, id int64, benefit string) error {
	_, err := r.sql.ExecContext(ctx, `WITH changed AS (DELETE FROM vip_overrides WHERE user_id=$1 AND benefit=$2 RETURNING user_id)
 INSERT INTO vip_audit(actor_id,user_id,action,detail) SELECT $3,user_id,'restore_auto',jsonb_build_object('benefit',$2::text) FROM changed`, id, benefit, actor)
	return err
}
func (r *userRepository) VIPInitialCredit(ctx context.Context, actor, id int64, amount float64, reason string) error {
	// One explicit opening record per user, independent of mutable legacy totals.
	_, err := r.sql.ExecContext(ctx, `WITH inserted AS (
 INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount,reason,actor_id)
 VALUES($1,'opening',($1::bigint)::text,$2,$3,$4) ON CONFLICT(source,source_id) DO NOTHING RETURNING user_id)
 INSERT INTO vip_audit(actor_id,user_id,action,detail) SELECT $4,user_id,'opening',jsonb_build_object('amount',$2::numeric,'reason',$3::text) FROM inserted`, id, amount, reason, actor)
	return err
}

var _ service.VIPRepository = (*userRepository)(nil)
