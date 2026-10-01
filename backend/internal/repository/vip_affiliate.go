package repository

import "context"

func (r *affiliateRepository) VIPRebatePercent(ctx context.Context, userID int64) (float64, bool, error) {
	if r.vipSQL == nil {
		return 0, false, nil
	}
	repo := &userRepository{sql: r.vipSQL}
	rules, err := repo.VIPRules(ctx)
	if err != nil {
		return 0, false, err
	}
	if !rules.Enabled {
		return 0, false, nil
	}
	state, err := repo.VIPSnapshot(ctx, userID)
	if err != nil {
		return 0, true, err
	}
	return state.RebatePercent, true, nil
}
