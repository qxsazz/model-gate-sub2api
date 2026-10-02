package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	gocache "github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

type userGroupRateResolver struct {
	repo         UserGroupRateRepository
	cache        *gocache.Cache
	cacheTTL     time.Duration
	sf           *singleflight.Group
	logComponent string
}

func newUserGroupRateResolver(repo UserGroupRateRepository, cache *gocache.Cache, cacheTTL time.Duration, sf *singleflight.Group, logComponent string) *userGroupRateResolver {
	if cacheTTL <= 0 {
		cacheTTL = defaultUserGroupRateCacheTTL
	}
	if cache == nil {
		cache = gocache.New(cacheTTL, time.Minute)
	}
	if logComponent == "" {
		logComponent = "service.gateway"
	}
	if sf == nil {
		sf = &singleflight.Group{}
	}

	return &userGroupRateResolver{
		repo:         repo,
		cache:        cache,
		cacheTTL:     cacheTTL,
		sf:           sf,
		logComponent: logComponent,
	}
}

func (r *userGroupRateResolver) Resolve(ctx context.Context, userID, groupID int64, groupDefaultMultiplier float64) float64 {
	if r == nil || userID <= 0 || groupID <= 0 {
		return groupDefaultMultiplier
	}

	key := fmt.Sprintf("%d:%d", userID, groupID)
	if r.cache != nil {
		if cached, ok := r.cache.Get(key); ok {
			if multiplier, castOK := cached.(float64); castOK {
				userGroupRateCacheHitTotal.Add(1)
				return multiplier
			}
		}
	}
	if r.repo == nil {
		return groupDefaultMultiplier
	}
	userGroupRateCacheMissTotal.Add(1)

	value, err, shared := r.sf.Do(key, func() (any, error) {
		if r.cache != nil {
			if cached, ok := r.cache.Get(key); ok {
				if multiplier, castOK := cached.(float64); castOK {
					userGroupRateCacheHitTotal.Add(1)
					return multiplier, nil
				}
			}
		}

		userGroupRateCacheLoadTotal.Add(1)
		userRate, repoErr := r.repo.GetByUserAndGroup(ctx, userID, groupID)
		if repoErr != nil {
			return nil, repoErr
		}

		multiplier := groupDefaultMultiplier
		if userRate != nil {
			multiplier = *userRate
		}
		if r.cache != nil {
			r.cache.Set(key, multiplier, r.cacheTTL)
		}
		return multiplier, nil
	})
	if shared {
		userGroupRateCacheSFSharedTotal.Add(1)
	}
	if err != nil {
		userGroupRateCacheFallbackTotal.Add(1)
		logger.LegacyPrintf(r.logComponent, "get user group rate failed, fallback to group default: user=%d group=%d err=%v", userID, groupID, err)
		return groupDefaultMultiplier
	}

	multiplier, ok := value.(float64)
	if !ok {
		userGroupRateCacheFallbackTotal.Add(1)
		return groupDefaultMultiplier
	}
	return multiplier
}

// Strict resolution is used before forwarding and for settlement. Never turn an
// unknown discount or manual price into a silently different charge.
type strictRateCacheEntry struct{ manual *float64 }

func (r *userGroupRateResolver) ResolveStrict(ctx context.Context, userID, groupID int64, base float64) (float64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if rate, found, err := contextRequestRate(ctx, userID, groupID); found {
		return rate, err
	}
	if r == nil || userID <= 0 || groupID <= 0 {
		return base, nil
	}
	_, hasVIP := r.repo.(interface {
		VIPEffectiveRate(context.Context, int64, int64) (*float64, bool, error)
	})
	key := fmt.Sprintf("%d:%d", userID, groupID)
	cached := func() (strictRateCacheEntry, bool) {
		if r.cache == nil {
			return strictRateCacheEntry{}, false
		}
		value, ok := r.cache.Get(key)
		if !ok {
			return strictRateCacheEntry{}, false
		}
		if entry, ok := value.(strictRateCacheEntry); ok {
			return entry, true
		}
		// Legacy float entries are safe only when this repository has no VIP layer.
		if rate, ok := value.(float64); ok && !hasVIP {
			return strictRateCacheEntry{manual: &rate}, true
		}
		return strictRateCacheEntry{}, false
	}
	entry, hit := cached()
	if hit {
		userGroupRateCacheHitTotal.Add(1)
	} else if r.repo != nil {
		userGroupRateCacheMissTotal.Add(1)
		value, err, shared := r.sf.Do("strict:"+key, func() (any, error) {
			if entry, ok := cached(); ok {
				return entry, nil
			}
			userGroupRateCacheLoadTotal.Add(1)
			manual, err := r.repo.GetByUserAndGroup(ctx, userID, groupID)
			if err != nil {
				return nil, err
			}
			entry := strictRateCacheEntry{}
			if manual != nil {
				copy := *manual
				entry.manual = &copy
			}
			if r.cache != nil {
				r.cache.Set(key, entry, r.cacheTTL)
			}
			return entry, nil
		})
		if shared {
			userGroupRateCacheSFSharedTotal.Add(1)
		}
		if err != nil {
			userGroupRateCacheFallbackTotal.Add(1)
			return 0, ErrVIPRateUnavailable.WithCause(err)
		}
		entry = value.(strictRateCacheEntry)
	}
	manual := entry.manual
	if manual != nil {
		if !finiteVIP(*manual) || *manual < 0 {
			return 0, ErrVIPRateUnavailable
		}
		return *manual, nil
	}
	disabled, _ := ctx.Value(vipDisabledRequestContextKey{}).(bool)
	if vip, ok := r.repo.(interface {
		VIPEffectiveRate(context.Context, int64, int64) (*float64, bool, error)
	}); ok && !disabled {
		rate, active, err := vip.VIPEffectiveRate(ctx, userID, groupID)
		if err != nil {
			return 0, ErrVIPRateUnavailable.WithCause(err)
		}
		if active && rate != nil {
			if !finiteVIP(*rate) || *rate < 0 {
				return 0, ErrVIPRateUnavailable
			}
			return *rate, nil
		}
	}
	if !finiteVIP(base) || base < 0 {
		return 0, ErrVIPRateUnavailable
	}
	return base, nil
}
