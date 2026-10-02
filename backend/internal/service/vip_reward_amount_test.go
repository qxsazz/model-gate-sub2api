package service

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestVIPMilestoneRewardIsTwoPercent(t *testing.T) {
	for _, tc := range []struct{ threshold, reward float64 }{
		{100, 2}, {300, 6}, {600, 12}, {1500, 30}, {3000, 60}, {123.456, 2.47},
	} {
		require.Equal(t, tc.reward, VIPMilestoneReward(tc.threshold))
	}
}
