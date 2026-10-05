package service

import (
	"context"
	"fmt"
	"time"
)

type vipRebateSnapshotContextKey struct{}

type VIPGroupView struct {
	ID            int64   `json:"id"`
	Name          string  `json:"name"`
	Platform      string  `json:"platform"`
	Exclusive     bool    `json:"exclusive"`
	Subscription  string  `json:"subscription"`
	BaseRate      float64 `json:"base_rate"`
	Rate          float64 `json:"rate"`
	Granted       bool    `json:"granted"`
	Participating bool    `json:"participating"`
}
type VIPOverride struct {
	Benefit   string     `json:"benefit"`
	Value     float64    `json:"value"`
	ExpiresAt *time.Time `json:"expires_at"`
	Reason    string     `json:"reason"`
}
type VIPLedgerEntry struct {
	ID        int64     `json:"id"`
	Source    string    `json:"source"`
	Amount    float64   `json:"amount"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}
type VIPSnapshot struct {
	User            *VIPUserSummary  `json:"user,omitempty"`
	BaseConcurrency int              `json:"base_concurrency"`
	BaseRPM         int              `json:"base_rpm"`
	GrowthTier      VIPTier          `json:"growth_tier"`
	TierSource      string           `json:"tier_source"`
	LevelOverride   *VIPOverride     `json:"level_override"`
	Enabled         bool             `json:"enabled"`
	Total           float64          `json:"total"`
	Tier            VIPTier          `json:"tier"`
	BadgeLevel      int              `json:"badge_level"`
	Concurrency     int              `json:"concurrency"`
	RPM             int              `json:"rpm"`
	RebatePercent   float64          `json:"rebate_percent"`
	Next            *VIPTier         `json:"next"`
	Rules           VIPRules         `json:"rules"`
	Groups          []VIPGroupView   `json:"groups"`
	Ledger          []VIPLedgerEntry `json:"ledger"`
	Overrides       []VIPOverride    `json:"overrides"`
}
type VIPUserSummary struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}
type VIPRepository interface {
	VIPSnapshot(context.Context, int64) (*VIPSnapshot, error)
	VIPRules(context.Context) (VIPRules, error)
	VIPSaveRules(context.Context, int64, VIPRules) error
	VIPSetOverride(context.Context, int64, int64, VIPOverride) error
	VIPClearOverride(context.Context, int64, int64, string) error
	VIPInitialCredit(context.Context, int64, int64, float64, string) error
}

func (s *UserService) vipRepository() (VIPRepository, error) {
	r, ok := s.userRepo.(VIPRepository)
	if !ok {
		return nil, fmt.Errorf("VIP repository unavailable")
	}
	return r, nil
}
func (s *UserService) GetVIP(ctx context.Context, id int64) (*VIPSnapshot, error) {
	r, e := s.vipRepository()
	if e != nil {
		return nil, e
	}
	return r.VIPSnapshot(ctx, id)
}
func (s *UserService) VIPConfig(ctx context.Context) (VIPRules, error) {
	r, e := s.vipRepository()
	if e != nil {
		return VIPRules{}, e
	}
	return r.VIPRules(ctx)
}
func (s *UserService) SaveVIPConfig(ctx context.Context, actor int64, rules VIPRules) error {
	if e := rules.Validate(); e != nil {
		return e
	}
	r, e := s.vipRepository()
	if e != nil {
		return e
	}
	if err := r.VIPSaveRules(ctx, actor, rules); err != nil {
		return err
	}
	if cache, ok := s.authCacheInvalidator.(interface{ InvalidateVIPConfig() }); ok {
		cache.InvalidateVIPConfig()
	}
	return nil
}
func (s *UserService) SetVIPOverride(ctx context.Context, actor, id int64, o VIPOverride) error {
	if !finiteVIP(o.Value) || o.Value < 0 || len(o.Reason) < 3 {
		return fmt.Errorf("invalid override")
	}
	switch o.Benefit {
	case "discount":
		if o.Value > .075 {
			return fmt.Errorf("discount exceeds 0.075")
		}
	case "access":
		if o.Value != 0 && o.Value != 1 {
			return fmt.Errorf("access must be 0 or 1")
		}
	case "badge":
		if o.Value > 5 || o.Value != float64(int(o.Value)) {
			return fmt.Errorf("invalid badge")
		}
	case "concurrency", "rpm":
		if o.Value > 1000 || o.Value != float64(int(o.Value)) {
			return fmt.Errorf("invalid limit")
		}
	case "rebate":
		if o.Value > 10 {
			return fmt.Errorf("rebate exceeds 10 percent")
		}
	default:
		return fmt.Errorf("unknown benefit")
	}
	if o.ExpiresAt != nil && !o.ExpiresAt.After(time.Now()) {
		return fmt.Errorf("expiry must be in the future")
	}
	r, e := s.vipRepository()
	if e != nil {
		return e
	}
	return r.VIPSetOverride(ctx, actor, id, o)
}
func (s *UserService) ClearVIPOverride(ctx context.Context, actor, id int64, benefit string) error {
	if benefit == "tier" {
		return s.RestoreVIPLevel(ctx, actor, id, "兼容入口恢复自动等级")
	}
	r, e := s.vipRepository()
	if e != nil {
		return e
	}
	return r.VIPClearOverride(ctx, actor, id, benefit)
}
func (s *UserService) InitialVIPCredit(ctx context.Context, actor, id int64, amount float64, reason string) error {
	if !finiteVIP(amount) || amount < 0 || len(reason) < 3 {
		return fmt.Errorf("invalid opening recharge record")
	}
	r, e := s.vipRepository()
	if e != nil {
		return e
	}
	return r.VIPInitialCredit(ctx, actor, id, amount, reason)
}
