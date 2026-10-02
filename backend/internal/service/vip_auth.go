package service

import "context"

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
	state, err := repo.VIPSnapshot(ctx, key.UserID)
	if err != nil {
		return err
	}
	if !state.Enabled {
		return nil
	}
	copy := *key.User
	copy.AllowedGroups = append([]int64{}, copy.AllowedGroups...)
	// Reload manual grants as well so revoked cache entries cannot retain access.
	current, err := s.userRepo.GetByID(ctx, key.UserID)
	if err != nil {
		return err
	}
	copy.AllowedGroups = append([]int64{}, current.AllowedGroups...)
	copy.Concurrency = state.Concurrency
	copy.RPMLimit = state.RPM
	for _, g := range state.Groups {
		if g.Exclusive && g.Granted {
			copy.AllowedGroups = append(copy.AllowedGroups, g.ID)
		}
	}
	key.User = &copy
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
