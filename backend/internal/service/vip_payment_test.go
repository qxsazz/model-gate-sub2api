package service

import (
	"math"
	"testing"

	apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestVIPCNYRechargePrincipalExcludesBonus(t *testing.T) {
	rules := DefaultVIPRules()
	rules.Enabled = true
	require.Equal(t, 1.0, rules.ExchangeRates["CNY"])
	snapshot, err := applyVIPPaymentSnapshot(map[string]any{"provider": "fixture"}, "CNY", 100, rules)
	require.NoError(t, err)
	require.Equal(t, 105.0, calculateCreditedBalance(100, 1.05))
	require.Equal(t, 100.0, snapshot["vip_principal_usd"])
	require.Equal(t, 1.0, snapshot["vip_fx"])
	require.Equal(t, "fixture", snapshot["provider"])
	rules.ExchangeRates["CNY"] = .142857142857
	snapshot, err = applyVIPPaymentSnapshot(nil, "CNY", 100, rules)
	require.NoError(t, err)
	require.Equal(t, 14.28571429, snapshot["vip_principal_usd"])
}

func TestVIPPaymentExchangeRateValidation(t *testing.T) {
	rules := DefaultVIPRules()
	rules.Enabled = true
	delete(rules.ExchangeRates, "CNY")
	_, err := applyVIPPaymentSnapshot(nil, "CNY", 100, rules)
	require.Equal(t, 503, apperrors.Code(err))
	require.Equal(t, "VIP_EXCHANGE_RATE_NOT_CONFIGURED", apperrors.Reason(err))
	rules.Enabled = false
	snapshot, err := applyVIPPaymentSnapshot(nil, "CNY", 100, rules)
	require.NoError(t, err)
	require.Nil(t, snapshot)
	rules.Enabled = true
	for _, value := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		rules.ExchangeRates["CNY"] = value
		_, err = applyVIPPaymentSnapshot(nil, "CNY", 100, rules)
		require.Equal(t, "VIP_EXCHANGE_RATE_INVALID", apperrors.Reason(err))
	}
	rules.ExchangeRates["CNY"] = 1
	_, err = applyVIPPaymentSnapshot(nil, "CNY", math.NaN(), rules)
	require.Error(t, err)
}
