package handler

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type wsReauthFixtureRepository struct {
	service.APIKeyRepository
	key         *service.APIKey
	concurrency int
}

func (r *wsReauthFixtureRepository) GetByKey(context.Context, string) (*service.APIKey, error) {
	key := *r.key
	if key.Status == "" {
		key.Status = service.StatusActive
	}
	if key.User != nil {
		user := *key.User
		if user.Status == "" {
			user.Status = service.StatusActive
		}
		if user.Concurrency == 0 {
			user.Concurrency = r.concurrency
		}
		key.User = &user
		if key.UserID == 0 {
			key.UserID = user.ID
		}
	}
	if key.Group != nil {
		group := *key.Group
		key.Group = &group
	} else if key.GroupID != nil {
		key.Group = &service.Group{ID: *key.GroupID, Platform: service.PlatformOpenAI, RateMultiplier: 1}
	}
	if key.Group != nil {
		key.Group.Hydrated = true
		if key.Group.Status == "" {
			key.Group.Status = service.StatusActive
		}
		if key.Group.Platform == "" {
			key.Group.Platform = service.PlatformOpenAI
		}
	}
	return &key, nil
}

func (r *wsReauthFixtureRepository) GetByKeyForAuth(ctx context.Context, key string) (*service.APIKey, error) {
	return r.GetByKey(ctx, key)
}
func (*wsReauthFixtureRepository) UpdateLastUsed(context.Context, int64, time.Time) error { return nil }
func (*wsReauthFixtureRepository) IncrementQuotaUsed(context.Context, int64, float64) (float64, error) {
	return 0, nil
}
func (*wsReauthFixtureRepository) IncrementRateLimitUsage(context.Context, int64, float64) error {
	return nil
}
func (*wsReauthFixtureRepository) GetRateLimitData(context.Context, int64) (*service.APIKeyRateLimitData, error) {
	return &service.APIKeyRateLimitData{}, nil
}

// These harnesses inject an authenticated context instead of running middleware.
// A real fixture repository is required now that each WS turn re-authenticates.
func newWSReauthFixtureService(key *service.APIKey, concurrency int) *service.APIKeyService {
	if key.Key == "" {
		key.Key = "ws-reauth-fixture-key"
	}
	return service.NewAPIKeyService(&wsReauthFixtureRepository{key: key, concurrency: concurrency}, nil, nil, nil, nil, nil, &config.Config{RunMode: config.RunModeSimple})
}
