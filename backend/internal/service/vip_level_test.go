package service

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestVIPEffectiveTierUsesExactAssignedLevelAndExpiry(t *testing.T) {
	rules := DefaultVIPRules()
	rules.Enabled = true
	now := time.Now()
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	for _, tc := range []struct {
		total     float64
		overrides []VIPOverride
		level     int
		manual    bool
	}{
		{0, []VIPOverride{{Benefit: "tier", Value: 5, ExpiresAt: &future}}, 5, true},
		{3000, []VIPOverride{{Benefit: "tier", Value: 1}}, 1, true},
		{3000, []VIPOverride{{Benefit: "tier", Value: 0}}, 0, true},
		{600, []VIPOverride{{Benefit: "tier", Value: 5, ExpiresAt: &past}}, 3, false},
		{0, []VIPOverride{{Benefit: "badge", Value: 5}}, 0, false},
	} {
		tier, manual, err := rules.EffectiveTier(tc.total, tc.overrides, now)
		require.NoError(t, err)
		require.Equal(t, tc.level, tier.Level)
		require.Equal(t, tc.manual, manual)
	}
	rules.Enabled = false
	tier, manual, err := rules.EffectiveTier(3000, []VIPOverride{{Benefit: "tier", Value: 5}}, now)
	require.NoError(t, err)
	require.Zero(t, tier.Level)
	require.False(t, manual)
}
func TestVIPLevelCommandValidation(t *testing.T) {
	for _, level := range []int{0, 1, 5} {
		require.NoError(t, (VIPLevelCommand{Level: level, Reason: "管理员调整"}).Validate())
	}
	for _, level := range []int{-1, 6} {
		require.Error(t, (VIPLevelCommand{Level: level, Reason: "管理员调整"}).Validate())
	}
	past := time.Now().Add(-time.Hour)
	require.Error(t, (VIPLevelCommand{Level: 2, Reason: "管理员调整", ExpiresAt: &past}).Validate())
	require.Error(t, (VIPLevelCommand{Level: 2, Reason: " "}).Validate())
}
