package service

import (
	"context"
	"net/http"

	apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrVIPUnavailable = apperrors.New(http.StatusServiceUnavailable, "VIP_UNAVAILABLE", "Membership authorization is temporarily unavailable")
var ErrVIPRateUnavailable = apperrors.New(http.StatusServiceUnavailable, "VIP_RATE_UNAVAILABLE", "Your billing rate cannot be confirmed; please retry")

type requestRateSnapshot struct {
	userID int64
	rates  map[int64]float64
}
type requestRateContextKey struct{}
type vipDisabledRequestContextKey struct{}

func (k *APIKey) RequestRate(userID, groupID int64) (float64, bool) {
	if k != nil && userID == 0 && k.User != nil {
		userID = k.User.ID
	}
	if k == nil || k.requestRates == nil || k.requestRates.userID != userID {
		return 0, false
	}
	rate, ok := k.requestRates.rates[groupID]
	return rate, ok
}
func WithAPIKeyRequestRates(ctx context.Context, k *APIKey) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if k != nil {
		ctx = context.WithValue(ctx, vipDisabledRequestContextKey{}, k.vipDisabled)
	}
	if k == nil || k.requestRates == nil {
		if k != nil {
			return context.WithValue(ctx, requestRateContextKey{}, (*requestRateSnapshot)(nil))
		}
		return ctx
	}
	return context.WithValue(ctx, requestRateContextKey{}, k.requestRates)
}
func contextRequestRate(ctx context.Context, userID, groupID int64) (float64, bool, error) {
	if ctx == nil {
		return 0, false, nil
	}
	snapshot, _ := ctx.Value(requestRateContextKey{}).(*requestRateSnapshot)
	if snapshot == nil || snapshot.userID != userID {
		return 0, false, nil
	}
	rate, ok := snapshot.rates[groupID]
	if !ok {
		return 0, true, ErrVIPRateUnavailable
	}
	return rate, true, nil
}
func captureRequestRate(ctx context.Context, key *APIKey, resolve func(context.Context, int64, int64, float64) (float64, error)) (context.Context, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if key == nil || key.GroupID == nil || key.Group == nil {
		return ctx, nil
	}
	ctx = WithAPIKeyRequestRates(ctx, key)
	userID := key.UserID
	if userID == 0 && key.User != nil {
		userID = key.User.ID
	}
	if _, ok := key.RequestRate(userID, *key.GroupID); ok {
		return WithAPIKeyRequestRates(ctx, key), nil
	}
	rate, err := resolve(ctx, userID, *key.GroupID, key.Group.RateMultiplier)
	if err != nil {
		return ctx, err
	}
	if !finiteVIP(rate) || rate < 0 {
		return ctx, ErrVIPRateUnavailable
	}
	rates := map[int64]float64{}
	if key.requestRates != nil && key.requestRates.userID == userID {
		for id, v := range key.requestRates.rates {
			rates[id] = v
		}
	}
	rates[*key.GroupID] = rate
	key.requestRates = &requestRateSnapshot{userID: userID, rates: rates}
	return WithAPIKeyRequestRates(ctx, key), nil
}
func (s *GatewayService) CaptureRequestRate(ctx context.Context, key *APIKey) (context.Context, error) {
	return captureRequestRate(ctx, key, s.getUserGroupRateMultiplierStrict)
}

// An explicit billing-group fallback is a new quote, unlike composite routing
// which keeps the parent billing group and its existing request price.
func (s *GatewayService) CaptureFallbackRequestRate(ctx context.Context, key *APIKey) (context.Context, error) {
	if key == nil || key.User == nil || key.GroupID == nil || key.Group == nil {
		return ctx, ErrVIPRateUnavailable
	}
	if !key.User.CanBindGroup(*key.GroupID, key.Group.IsExclusive) {
		return ctx, ErrVIPRateUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := key.RequestRate(key.UserID, *key.GroupID); !ok {
		copy := *key
		copy.requestRates = nil
		fresh := context.WithValue(ctx, requestRateContextKey{}, (*requestRateSnapshot)(nil))
		_, err := s.CaptureRequestRate(fresh, &copy)
		if err != nil {
			return ctx, err
		}
		rates := map[int64]float64{}
		if key.requestRates != nil {
			for id, rate := range key.requestRates.rates {
				rates[id] = rate
			}
		}
		for id, rate := range copy.requestRates.rates {
			rates[id] = rate
		}
		key.requestRates = &requestRateSnapshot{userID: copy.requestRates.userID, rates: rates}
	}
	ctx = WithAPIKeyRequestRates(ctx, key)
	ctx = context.WithValue(ctx, gatewayTokenRequestBillingGroupCtxKey{}, key.Group)
	return ctx, nil
}
func (s *OpenAIGatewayService) CaptureRequestRate(ctx context.Context, key *APIKey) (context.Context, error) {
	return captureRequestRate(ctx, key, s.resolveUserGroupRateMultiplierStrict)
}
