//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestUpstream0218ImageGateMigrationPreservesMGGroupsAndMigrationHistory(t *testing.T) {
	ctx := context.Background()
	// Both migrations use prefix 246; the runner must track their full filenames.
	var applied int
	err := integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations WHERE filename IN (
		'246_enable_gemini_image_generation_groups.sql', '246_vip_bonus_and_discount_cap.sql'
	)`).Scan(&applied)
	require.NoError(t, err)
	require.Equal(t, 2, applied)

	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `CREATE TEMP TABLE groups (
		platform text PRIMARY KEY, allow_image_generation boolean NOT NULL
	) ON COMMIT DROP;
	INSERT INTO groups VALUES ('gemini', false), ('antigravity', false), ('composite', false), ('grok', false), ('openai', true);`)
	require.NoError(t, err)
	script, err := migrations.FS.ReadFile("246_enable_gemini_image_generation_groups.sql")
	require.NoError(t, err)
	for range 2 {
		_, err = tx.ExecContext(ctx, string(script))
		require.NoError(t, err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT platform, allow_image_generation FROM groups`)
	require.NoError(t, err)
	defer rows.Close()
	got := map[string]bool{}
	for rows.Next() {
		var platform string
		var allowed bool
		require.NoError(t, rows.Scan(&platform, &allowed))
		got[platform] = allowed
	}
	require.NoError(t, rows.Err())
	require.Equal(t, map[string]bool{
		"gemini": true, "antigravity": true, "composite": false, "grok": false, "openai": true,
	}, got)
}
