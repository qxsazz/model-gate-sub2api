//go:build unit

package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAchievementMinimumRechargeValidation(t *testing.T) {
	require.NoError(t, (AchievementConfig{CashScope: "all"}).Validate(), "legacy requests omit the threshold")
	for _, amount := range []float64{0, -1, .001, 1000001, math.Inf(1), math.NaN()} {
		cfg := AchievementConfig{MinimumRechargeCNY: &amount, CashScope: "all"}
		require.Error(t, cfg.Validate())
	}
	for _, amount := range []float64{.01, 10, 12.35} {
		cfg := AchievementConfig{MinimumRechargeCNY: &amount, CashScope: "all"}
		require.NoError(t, cfg.Validate())
	}
}
