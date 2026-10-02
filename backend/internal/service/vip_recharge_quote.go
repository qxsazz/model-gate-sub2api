package service

import "context"

type VIPRechargeQuote struct {
	Multiplier float64
	Level      int
	Enabled    bool
}

// Checkout and order creation use the same rule; order creation rechecks and
// freezes it so a stale browser cannot choose a bonus or alter an old order.
func (s *PaymentService) GetVIPRechargeQuote(ctx context.Context, userID int64, legacy float64) (VIPRechargeQuote, error) {
	quote := VIPRechargeQuote{Multiplier: legacy}
	repo, ok := s.userRepo.(VIPRepository)
	if !ok {
		return quote, nil
	}
	rules, err := repo.VIPRules(ctx)
	if err != nil {
		return quote, ErrVIPUnavailable.WithCause(err)
	}
	if !rules.Enabled || !rules.RechargeBonusEnabled {
		return quote, nil
	}
	var state *VIPSnapshot
	if auth, ok := repo.(interface {
		VIPAuthSnapshot(context.Context, int64) (*VIPSnapshot, error)
	}); ok {
		state, err = auth.VIPAuthSnapshot(ctx, userID)
	} else {
		state, err = repo.VIPSnapshot(ctx, userID)
	}
	if err != nil {
		return quote, ErrVIPUnavailable.WithCause(err)
	}
	if state == nil {
		return quote, ErrVIPUnavailable
	}
	if !finiteVIP(state.Total) {
		return quote, ErrVIPRateUnavailable
	}
	quote.Multiplier, err = state.Rules.RechargeMultiplier(state.Total, legacy)
	if err != nil {
		return quote, err
	}
	quote.Level = state.Rules.Tier(state.Total).Level
	quote.Enabled = state.Rules.Enabled && state.Rules.RechargeBonusEnabled
	return quote, err
}
