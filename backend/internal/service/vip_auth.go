package service

import (
	"context"
	"time"
)

// Cache only a confirmed disabled state, briefly. Enabled grants are never cached.
func (s *APIKeyService) vipEnabled(ctx context.Context, repo VIPRepository) (bool, error) {
	s.vipModeMu.Lock()
	if time.Now().Before(s.vipDisabledUntil) {
		s.vipModeMu.Unlock()
		return false, nil
	}
	generation := s.vipModeGeneration
	s.vipModeMu.Unlock()
	var enabled bool
	var err error
	if mode, ok := repo.(interface {
		VIPMode(context.Context) (bool, error)
	}); ok {
		enabled, err = mode.VIPMode(ctx)
	} else {
		var rules VIPRules
		rules, err = repo.VIPRules(ctx)
		enabled = rules.Enabled
	}
	if err != nil {
		return false, ErrVIPUnavailable.WithCause(err)
	}
	if !enabled {
		s.vipModeMu.Lock()
		if generation == s.vipModeGeneration {
			s.vipDisabledUntil = time.Now().Add(time.Second)
		}
		s.vipModeMu.Unlock()
	}
	return enabled, nil
}
func (s *APIKeyService) InvalidateVIPConfig() {
	s.vipModeMu.Lock()
	s.vipModeGeneration++
	s.vipDisabledUntil = time.Time{}
	s.vipModeMu.Unlock()
}

// Resolve VIP on each authentication to avoid stale cached grants after refunds.
// The shared auth-cache User snapshot is never mutated.
func (s *APIKeyService) applyVIP(ctx context.Context, key *APIKey) error {
	if key == nil || key.User == nil {
		return nil
	}
	repo, ok := s.userRepo.(VIPRepository)
	if !ok {
		return nil
	}
	enabled, err := s.vipEnabled(ctx, repo)
	if err != nil {
		return err
	}
	if !enabled {
		key.vipDisabled = true
		return nil
	}
	var state *VIPSnapshot
	if auth, ok := repo.(interface {
		VIPAuthSnapshot(context.Context, int64) (*VIPSnapshot, error)
	}); ok {
		state, err = auth.VIPAuthSnapshot(ctx, key.UserID)
	} else {
		state, err = repo.VIPSnapshot(ctx, key.UserID)
	}
	if err != nil {
		return ErrVIPUnavailable.WithCause(err)
	}
	if state == nil {
		return ErrVIPUnavailable
	}
	if !state.Enabled {
		key.vipDisabled = true
		return nil
	}
	copy := *key.User
	manualGroups := state.ManualGroups
	if manualGroups == nil {
		// Repositories without fresh grant inputs retain the original reload path.
		current, err := s.userRepo.GetByID(ctx, key.UserID)
		if err != nil {
			return ErrVIPUnavailable.WithCause(err)
		}
		if current == nil {
			return ErrVIPUnavailable
		}
		manualGroups = current.AllowedGroups
	}
	copy.AllowedGroups = append([]int64{}, manualGroups...)
	copy.Concurrency = state.Concurrency
	copy.RPMLimit = state.RPM
	for _, g := range state.Groups {
		if g.Exclusive && g.Granted {
			copy.AllowedGroups = append(copy.AllowedGroups, g.ID)
		}
	}
	key.User = &copy
	rates := map[int64]float64{}
	for _, g := range state.Groups {
		if !finiteVIP(g.Rate) || g.Rate < 0 {
			return ErrVIPRateUnavailable
		}
		rates[g.ID] = g.Rate
	}
	// Subscription groups do not receive VIP discounts but retain manual pricing.
	if key.GroupID != nil && key.Group != nil && key.Group.IsSubscriptionType() {
		rate := key.Group.RateMultiplier
		if s.userGroupRateRepo != nil {
			manual, err := s.userGroupRateRepo.GetByUserAndGroup(ctx, key.UserID, *key.GroupID)
			if err != nil {
				return ErrVIPRateUnavailable.WithCause(err)
			}
			if manual != nil {
				rate = *manual
			}
		}
		if !finiteVIP(rate) || rate < 0 {
			return ErrVIPRateUnavailable
		}
		rates[*key.GroupID] = rate
	}
	key.requestRates = &requestRateSnapshot{userID: key.UserID, rates: rates}
	return nil
}
func (s *APIKeyService) vipCanBind(ctx context.Context, userID, groupID int64) bool {
	repo, ok := s.userRepo.(VIPRepository)
	if !ok {
		return false
	}
	state, err := repo.VIPSnapshot(ctx, userID)
	if err != nil || !state.Enabled {
		return false
	}
	for _, g := range state.Groups {
		if g.ID == groupID {
			return g.Granted
		}
	}
	return false
}
