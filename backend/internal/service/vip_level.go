package service

import (
	"context"
	"fmt"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"strings"
	"time"
	"unicode/utf8"
)

type VIPLevelCommand struct {
	Level     int        `json:"level"`
	ExpiresAt *time.Time `json:"expires_at"`
	Reason    string     `json:"reason"`
}

func (c VIPLevelCommand) Validate() error {
	if c.Level < 0 || c.Level > 5 || utf8.RuneCountInString(strings.TrimSpace(c.Reason)) < 3 || utf8.RuneCountInString(c.Reason) > 500 {
		return infraerrors.BadRequest("VIP_LEVEL_INVALID", "请选择普通会员或 VIP 1–5，并填写至少三个字的原因")
	}
	if c.ExpiresAt != nil && !c.ExpiresAt.After(time.Now()) {
		return infraerrors.BadRequest("VIP_LEVEL_EXPIRY_INVALID", "到期时间须晚于当前时间")
	}
	return nil
}

type vipLevelRepository interface {
	VIPSetLevel(context.Context, int64, int64, *VIPLevelCommand, string) error
}

func (s *UserService) SetVIPLevel(ctx context.Context, actor, id int64, c VIPLevelCommand) error {
	if err := c.Validate(); err != nil {
		return err
	}
	return s.changeVIPLevel(ctx, actor, id, &c, c.Reason)
}
func (s *UserService) RestoreVIPLevel(ctx context.Context, actor, id int64, reason string) error {
	if err := (VIPLevelCommand{Reason: reason}).Validate(); err != nil {
		return err
	}
	return s.changeVIPLevel(ctx, actor, id, nil, reason)
}
func (s *UserService) changeVIPLevel(ctx context.Context, actor, id int64, c *VIPLevelCommand, reason string) error {
	repo, ok := s.userRepo.(vipLevelRepository)
	if !ok {
		return fmt.Errorf("VIP level repository unavailable")
	}
	if err := repo.VIPSetLevel(ctx, actor, id, c, reason); err != nil {
		return err
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, id)
	}
	return nil
}

// The assigned grade is exact, not a minimum; expiry falls back to genuine growth.
func (r VIPRules) EffectiveTier(total float64, overrides []VIPOverride, at time.Time) (VIPTier, bool, error) {
	if !r.Enabled {
		return VIPTier{}, false, nil
	}
	for _, o := range overrides {
		if o.Benefit != "tier" || (o.ExpiresAt != nil && !o.ExpiresAt.After(at)) {
			continue
		}
		if !finiteVIP(o.Value) || o.Value < 0 || o.Value > 5 || o.Value != float64(int(o.Value)) {
			return VIPTier{}, false, ErrVIPUnavailable
		}
		if o.Value == 0 {
			return VIPTier{}, true, nil
		}
		for _, t := range r.Tiers {
			if t.Level == int(o.Value) {
				return t, true, nil
			}
		}
		return VIPTier{}, false, ErrVIPUnavailable
	}
	return r.Tier(total), false, nil
}
