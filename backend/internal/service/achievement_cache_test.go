package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type achievementCacheRepo struct {
	dailyPolicyTestRepo
	err error
}

func (r *achievementCacheRepo) AchievementMutation(context.Context, int64, string, string, string, string) (json.RawMessage, error) {
	return nil, r.err
}
func (r *achievementCacheRepo) UseAchievementCard(context.Context, int64, AchievementCardCommand) (json.RawMessage, error) {
	return nil, r.err
}
func (r *achievementCacheRepo) AchievementCardPreview(context.Context, int64, string) (json.RawMessage, error) {
	return nil, nil
}
func (r *achievementCacheRepo) AdminAchievementMutation(context.Context, int64, int64, string, AchievementAdminCommand) (json.RawMessage, error) {
	return nil, r.err
}
func (r *achievementCacheRepo) AdminAchievementSnapshot(context.Context, int64) (json.RawMessage, error) {
	return nil, nil
}
func (r *achievementCacheRepo) AchievementBackfillPreview(context.Context, int64, string) (json.RawMessage, error) {
	return nil, nil
}
func (r *achievementCacheRepo) AchievementAudit(context.Context) (json.RawMessage, error) {
	return nil, nil
}

type achievementCacheRecorder struct {
	BillingCache
	APIKeyAuthCacheInvalidator
	auth, balances []int64
}

func (c *achievementCacheRecorder) InvalidateAuthCacheByUserID(_ context.Context, id int64) {
	c.auth = append(c.auth, id)
}
func (c *achievementCacheRecorder) InvalidateUserBalance(ctx context.Context, id int64) error {
	c.balances = append(c.balances, id)
	return ctx.Err()
}

func TestAchievementBalanceCacheInvalidation(t *testing.T) {
	amount, tier := .1, 2
	for _, failed := range []bool{false, true} {
		for _, action := range []string{"checkin", "card", "admin"} {
			t.Run(action+map[bool]string{false: " success", true: " failure"}[failed], func(t *testing.T) {
				repo := &achievementCacheRepo{}
				if failed {
					repo.err = errors.New("mutation failed")
				}
				cache := &achievementCacheRecorder{}
				svc := &UserService{userRepo: repo, billingCache: cache, authCacheInvalidator: cache}
				ctx := context.Background()
				var err error
				switch action {
				case "checkin":
					_, err = svc.ChangeAchievement(ctx, 7, "checkin", "", "2026-10-04", "cache-test-1234")
				case "card":
					_, err = svc.UseAchievementCard(ctx, 7, AchievementCardCommand{Date: "2026-10-03", RequestKey: "9049586f-2ea3-4931-9106-25d13e1224be", ExpectedGross: &amount, ExpectedTier: &tier})
				case "admin":
					_, err = svc.ControlAdminAchievement(ctx, 1, 7, "grant", AchievementAdminCommand{Key: "T01", Reason: "核验后授予", RequestKey: "9049586f-2ea3-4931-9106-25d13e1224be"})
				}
				if failed {
					require.Error(t, err)
					require.Empty(t, cache.auth)
					require.Empty(t, cache.balances)
				} else {
					require.NoError(t, err)
					require.Equal(t, []int64{7}, cache.auth)
					require.Equal(t, []int64{7}, cache.balances)
				}
			})
		}
	}
}
