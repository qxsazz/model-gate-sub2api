//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestVIPBonusMigrationScalesLargeRulesAndPreservesSmallRules(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	raw := `{"enabled":false,"tiers":[{"level":1},{"level":2},{"level":3},{"level":4},{"level":5}],"groups":[{"group_id":1,"discounts":[0.02,0.04,0.06,0.08,0.1],"floor":0.2},{"group_id":2,"discounts":[0.005,0.01,0.015,0.02,0.03],"floor":0.17}]}`
	_, err = tx.ExecContext(ctx, `UPDATE vip_rules SET payload=$1::jsonb WHERE id=true`, raw)
	require.NoError(t, err)
	migration, err := os.ReadFile("../../migrations/246_vip_bonus_and_discount_cap.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	var result []byte
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT payload FROM vip_rules WHERE id=true`).Scan(&result))
	var rules service.VIPRules
	require.NoError(t, json.Unmarshal(result, &rules))
	require.False(t, rules.Enabled, "migration must not activate VIP")
	require.True(t, rules.RechargeBonusEnabled)
	require.Equal(t, []float64{.015, .03, .045, .06, .075}, rules.Groups[0].Discounts)
	require.Equal(t, []float64{.005, .01, .015, .02, .03}, rules.Groups[1].Discounts)
	for i, tier := range rules.Tiers {
		require.Equal(t, float64(i+1), tier.RechargeBonusPercent)
	}
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	var again []byte
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT payload FROM vip_rules WHERE id=true`).Scan(&again))
	require.JSONEq(t, string(result), string(again), "rerunning must not rescale discounts")
}
