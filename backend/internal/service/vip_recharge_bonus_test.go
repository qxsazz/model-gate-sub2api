package service

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestVIPRechargeBonusLadderAndDisabledFallback(t *testing.T) {
	rules := DefaultVIPRules()
	rules.Enabled = true
	rules.RechargeBonusEnabled = true
	for _, tc := range []struct{ total, expected float64 }{{0, 1}, {100, 1.01}, {300, 1.02}, {600, 1.03}, {1500, 1.04}, {3000, 1.05}} {
		got, err := rules.RechargeMultiplier(tc.total, 1.05)
		require.NoError(t, err)
		require.Equal(t, tc.expected, got)
	}
	rules.Enabled = false
	got, err := rules.RechargeMultiplier(3000, 1.05)
	require.NoError(t, err)
	require.Equal(t, 1.05, got)
}
func TestVIPDiscountCap075PreservesSmallCuts(t *testing.T) {
	require.Equal(t, .225, VIPDiscountedRate(.3, .2, .1))
	require.Equal(t, .195, VIPDiscountedRate(.2, .17, .005))
	rules := DefaultVIPRules()
	rules.Groups = []VIPGroupRule{{GroupID: 1, Floor: .2, Discounts: []float64{.015, .03, .045, .06, .075}}}
	require.NoError(t, rules.Validate())
	rules.Groups[0].Discounts[4] = .076
	require.Error(t, rules.Validate())
}
