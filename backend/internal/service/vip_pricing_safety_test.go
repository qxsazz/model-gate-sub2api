package service

import (
	"context"
	"errors"
	gocache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type vipPricingSafetyRepo struct {
	userGroupRateResolverRepoStub
	vipRate  *float64
	vipErr   error
	active   bool
	vipCalls int
}

func TestVIPCachedLegacyDefaultDoesNotOverrideDiscount(t *testing.T) {
	price := .26
	repo := &vipPricingSafetyRepo{active: true, vipRate: &price}
	cache := gocache.New(time.Minute, time.Minute)
	cache.Set("1:2", .3, time.Minute)
	resolver := newUserGroupRateResolver(repo, cache, time.Minute, nil, "test")
	rate, err := resolver.ResolveStrict(context.Background(), 1, 2, .3)
	require.NoError(t, err)
	require.Equal(t, .26, rate)
}
func TestVIPConfirmedDisabledRequestDoesNotQueryVIPPricing(t *testing.T) {
	repo := &vipPricingSafetyRepo{active: true, vipErr: errors.New("VIP tables unavailable")}
	resolver := newUserGroupRateResolver(repo, nil, time.Minute, nil, "test")
	ctx := WithAPIKeyRequestRates(context.Background(), &APIKey{vipDisabled: true})
	rate, err := resolver.ResolveStrict(ctx, 1, 2, .3)
	require.NoError(t, err)
	require.Equal(t, .3, rate)
	require.Zero(t, repo.vipCalls)
}

func TestVIPNewLogicalRequestClearsPreviousRateAndMode(t *testing.T) {
	price := .26
	repo := &vipPricingSafetyRepo{active: true, vipRate: &price}
	svc := &GatewayService{userGroupRateResolver: newUserGroupRateResolver(repo, nil, time.Minute, nil, "test")}
	gid := int64(2)
	newKey := func(disabled bool) *APIKey {
		return &APIKey{UserID: 1, User: &User{ID: 1}, GroupID: &gid, Group: &Group{ID: 2, RateMultiplier: .3}, vipDisabled: disabled}
	}
	ctx, err := svc.CaptureRequestRate(context.Background(), newKey(false))
	require.NoError(t, err)
	key := newKey(true)
	ctx, err = svc.CaptureRequestRate(ctx, key)
	require.NoError(t, err)
	rate, ok := key.RequestRate(1, 2)
	require.True(t, ok)
	require.Equal(t, .3, rate)
	key = newKey(false)
	_, err = svc.CaptureRequestRate(ctx, key)
	require.NoError(t, err)
	rate, ok = key.RequestRate(1, 2)
	require.True(t, ok)
	require.Equal(t, .26, rate)
}

func (r *vipPricingSafetyRepo) VIPEffectiveRate(context.Context, int64, int64) (*float64, bool, error) {
	r.vipCalls++
	return r.vipRate, r.active, r.vipErr
}

func TestVIPPricingManualRateWinsWithoutVIPQuery(t *testing.T) {
	manual := .15
	repo := &vipPricingSafetyRepo{userGroupRateResolverRepoStub: userGroupRateResolverRepoStub{rate: &manual}, active: true, vipErr: errors.New("VIP unavailable")}
	resolver := newUserGroupRateResolver(repo, nil, time.Minute, nil, "test")
	got, err := resolver.ResolveStrict(context.Background(), 1, 2, .3)
	require.NoError(t, err)
	require.Equal(t, .15, got)
	require.Zero(t, repo.vipCalls)
}
func TestVIPPricingFailureCannotSilentlyUseBaseRate(t *testing.T) {
	repo := &vipPricingSafetyRepo{active: true, vipErr: errors.New("VIP unavailable")}
	resolver := newUserGroupRateResolver(repo, nil, time.Minute, nil, "test")
	_, err := resolver.ResolveStrict(context.Background(), 1, 2, .3)
	require.ErrorIs(t, err, ErrVIPRateUnavailable)
}
func TestVIPFrozenPriceSurvivesLaterRuleChanges(t *testing.T) {
	discount := .26
	repo := &vipPricingSafetyRepo{vipRate: &discount, active: true}
	svc := &GatewayService{userGroupRateResolver: newUserGroupRateResolver(repo, nil, time.Minute, nil, "test")}
	gid := int64(2)
	key := &APIKey{UserID: 1, User: &User{ID: 1}, GroupID: &gid, Group: &Group{ID: 2, RateMultiplier: .3}}
	ctx, err := svc.CaptureRequestRate(context.Background(), key)
	require.NoError(t, err)
	repo.vipErr = errors.New("database unavailable")
	require.Equal(t, .26, svc.ResolveUserGroupRateMultiplier(ctx, 1, 2, .3))
	rate, ok := key.RequestRate(1, 2)
	require.True(t, ok)
	require.Equal(t, .26, rate)
	deps := svc.inflightEstimateDeps()
	text, _ := deps.rates(context.Background(), key)
	require.Equal(t, .26, text)
}

func TestVIPFallbackCapturesNewQuoteWithoutChangingOriginal(t *testing.T) {
	price := .26
	repo := &vipPricingSafetyRepo{active: true, vipRate: &price}
	svc := &GatewayService{userGroupRateResolver: newUserGroupRateResolver(repo, nil, time.Minute, nil, "test")}
	first := int64(1)
	key := &APIKey{UserID: 7, User: &User{ID: 7}, GroupID: &first, Group: &Group{ID: 1, RateMultiplier: .3}}
	ctx, err := svc.CaptureRequestRate(context.Background(), key)
	require.NoError(t, err)
	copy := *key
	next := int64(2)
	copy.GroupID = &next
	copy.Group = &Group{ID: 2, RateMultiplier: .4, Hydrated: true, Platform: PlatformAnthropic, Status: StatusActive}
	newPrice := .36
	repo.vipRate = &newPrice
	ctx, err = svc.CaptureFallbackRequestRate(ctx, &copy)
	require.NoError(t, err)
	rate, ok := copy.RequestRate(7, 2)
	require.True(t, ok)
	require.Equal(t, .36, rate)
	rate, ok = key.RequestRate(7, 1)
	require.True(t, ok)
	require.Equal(t, .26, rate)
	_, ok = key.RequestRate(7, 2)
	require.False(t, ok, "fallback must not mutate the original snapshot")
	billingGroup := gatewayTokenRequestBillingGroupFromContext(ctx)
	require.NotNil(t, billingGroup)
	require.Equal(t, int64(2), billingGroup.ID)
}
