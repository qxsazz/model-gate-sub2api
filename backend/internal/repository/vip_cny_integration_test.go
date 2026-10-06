//go:build integration

package repository

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVIPCNYMigrationPreservesExplicitRate(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	migration, err := os.ReadFile("../../migrations/244_vip_cny_exchange_rate.sql")
	require.NoError(t, err)
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `UPDATE vip_rules SET payload=jsonb_set(payload,'{exchange_rates}','{"USD":1}'::jsonb) WHERE id=true`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	var rate float64
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT (payload->'exchange_rates'->>'CNY')::double precision FROM vip_rules WHERE id=true`).Scan(&rate))
	require.Equal(t, 1.0, rate)
	_, err = tx.ExecContext(ctx, `UPDATE vip_rules SET payload=jsonb_set(payload,'{exchange_rates,CNY}','0.15'::jsonb) WHERE id=true`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT (payload->'exchange_rates'->>'CNY')::double precision FROM vip_rules WHERE id=true`).Scan(&rate))
	require.Equal(t, .15, rate)
}
