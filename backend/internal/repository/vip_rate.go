package repository

import (
	"context"
)

func (r *userGroupRateRepository) VIPEffectiveRate(ctx context.Context, userID, groupID int64) (*float64, bool, error) {
	repo := &userRepository{sql: r.sql}
	rules, err := repo.VIPRules(ctx)
	if err != nil {
		return nil, false, err
	}
	if !rules.Enabled {
		return nil, false, nil
	}
	state, err := repo.VIPSnapshot(ctx, userID)
	if err != nil {
		return nil, true, err
	}
	for _, g := range state.Groups {
		if g.ID == groupID {
			rate := g.Rate
			return &rate, true, nil
		}
	}
	return nil, true, nil
}
