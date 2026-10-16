//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModelGateCacheQuoteAndSettlementKeepFrozenRateAndStoragePrice(t *testing.T) {
	svc, logs := newGeminiCacheBillingGatewayForTest(t)
	rate := .26
	repo := &vipPricingSafetyRepo{active: true, vipRate: &rate}
	svc.userGroupRateResolver = newUserGroupRateResolver(repo, nil, 0, nil, "test")
	groupID := int64(2)
	key := &APIKey{ID: 1, UserID: 7, User: &User{ID: 7}, GroupID: &groupID,
		Group: &Group{ID: groupID, Platform: PlatformGemini, Hydrated: true, RateMultiplier: 1}}
	ctx, err := svc.CaptureRequestRate(context.Background(), key)
	require.NoError(t, err)
	input := &RecordUsageInput{APIKey: key, User: key.User, Account: &Account{ID: 4, Platform: PlatformGemini},
		Result: &ForwardResult{RequestID: "cache-price-snapshot", Model: "gemini-3.8-flash", CacheStorageTokenHours: 20000}}
	price, err := svc.QuoteCacheStorageUnitPrice(ctx, input)
	require.NoError(t, err)
	input.Result.CacheStorageUnitPrice = &price
	quote, err := svc.QuoteUsageCost(ctx, input)
	require.NoError(t, err)
	require.InDelta(t, .01*.26, quote, 1e-12)
	// Settlement runs without the request context, after VIP/pricing may change.
	repo.vipErr = errors.New("VIP temporarily unavailable")
	svc.billingService.pricingService.pricingData = nil
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	require.InDelta(t, quote, logs.lastLog.ActualCost, 1e-12)
	require.InDelta(t, .26, logs.lastLog.RateMultiplier, 1e-12)
}

func TestModelGateCacheQuoteRejectsUnconfirmedVIPRate(t *testing.T) {
	svc, _ := newGeminiCacheBillingGatewayForTest(t)
	repo := &vipPricingSafetyRepo{active: true, vipErr: errors.New("VIP unavailable")}
	svc.userGroupRateResolver = newUserGroupRateResolver(repo, nil, 0, nil, "test")
	id := int64(2)
	key := &APIKey{User: &User{ID: 7}, GroupID: &id, Group: &Group{ID: id, RateMultiplier: 1}}
	amount, err := svc.QuoteUsageCost(context.Background(), &RecordUsageInput{APIKey: key,
		Result: &ForwardResult{Model: "gemini-3.8-flash", CacheStorageTokenHours: 20000}})
	require.ErrorIs(t, err, ErrVIPRateUnavailable)
	require.Zero(t, amount)
}

func TestModelGateCacheQuoteRejectsMissingStoragePrice(t *testing.T) {
	svc, _ := newGeminiCacheBillingGatewayForTest(t)
	_, err := svc.QuoteUsageCost(context.Background(), &RecordUsageInput{APIKey: &APIKey{},
		Result: &ForwardResult{Model: "unpriced-cache-model", CacheStorageTokenHours: 1}})
	require.ErrorContains(t, err, "storage pricing")
}
