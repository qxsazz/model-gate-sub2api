package service

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

type dailyPolicyTestRepo struct {
	UserRepository
	captured AchievementConfig
	calls    int
}

func (r *dailyPolicyTestRepo) AchievementSnapshot(context.Context, int64) (json.RawMessage, error) {
	return nil, nil
}
func (r *dailyPolicyTestRepo) AchievementMutation(context.Context, int64, string, string, string, string) (json.RawMessage, error) {
	return nil, nil
}
func (r *dailyPolicyTestRepo) AchievementConfig(context.Context) (*AchievementConfig, error) {
	return &r.captured, nil
}
func (r *dailyPolicyTestRepo) SaveAchievementConfig(_ context.Context, _ int64, c AchievementConfig) error {
	r.captured = c
	r.calls++
	return nil
}
func TestAchievementServicePropagatesAndValidatesDailyPolicy(t *testing.T) {
	repo := &dailyPolicyTestRepo{}
	svc := &UserService{userRepo: repo}
	ctx := context.Background()
	amounts := []float64{.02, .06, .12, .3, .6, 1.2}
	require.NoError(t, svc.SetAchievementConfig(ctx, 1, AchievementConfig{CashScope: "all", DailyRewards: amounts}))
	require.Equal(t, amounts, repo.captured.DailyRewards)
	require.Error(t, svc.SetAchievementConfig(ctx, 1, AchievementConfig{CashScope: "all", DailyRewards: []float64{1}}))
	require.Equal(t, 1, repo.calls)
}
