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
func TestVIPGroupFloorBoundsLargeDiscountsAndPreservesSmallCuts(t *testing.T) {
	require.Equal(t, .2, VIPDiscountedRate(.3, .2, .1))
	require.Equal(t, .195, VIPDiscountedRate(.2, .17, .005))
	rules := DefaultVIPRules()
	rules.Groups = []VIPGroupRule{{GroupID: 1, Floor: .2, Discounts: []float64{.015, .03, .06, .1, .15}}}
	require.NoError(t, rules.Validate())
	rules.Groups[0].Discounts[4] = .09
	require.Error(t, rules.Validate())
}

func TestVIPDiscountedRateSupportsZeroAndGroupSpecificFloors(t *testing.T) {
	for _, tc := range []struct{ base, floor, discount, want float64 }{
		{.35, .2, .15, .2}, {.35, 0, .35, 0}, {.35, .2, .3, .2},
		{.01, 0, .15, 0}, {0, 0, .15, 0}, {.35, .4, .15, .35},
	} {
		require.Equal(t, tc.want, VIPDiscountedRate(tc.base, tc.floor, tc.discount))
	}
}
