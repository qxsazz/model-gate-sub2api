//go:build integration

package repository

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestChatBotUpgradeKeepsEveryHistoricalMigrationChecksum(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, selectDockerImage(ctx, postgresImageTag), tcpostgres.WithDatabase("bot_upgrade"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("postgres"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	defer func() { _ = container.Terminate(context.Background()) }()
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := openSQLWithRetry(ctx, dsn, 30*time.Second)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	additions := map[string]bool{
		"249_add_usage_logs_api_key_cost_index_notx.sql": true,
		"250_openai_cache_write_ttl.sql":                 true,
		"260_chat_bot_bindings.sql":                      true,
		"261_chat_bot_email_binding.sql":                 true,
		"262_chat_binding_decline.sql":                   true,
	}
	names, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	old := fstest.MapFS{}
	for _, name := range names {
		if !additions[name] {
			data, err := migrations.FS.ReadFile(name)
			require.NoError(t, err)
			old[name] = &fstest.MapFile{Data: data}
		}
	}
	require.NoError(t, applyMigrationsFS(ctx, db, old))
	uid := achievementAccessUser(t, db)
	seedAchievementRecharge(t, db, uid, 10)
	require.NoError(t, ApplyMigrations(ctx, db))
	require.NoError(t, ApplyMigrations(ctx, db)) // startup retry validates checksums and does not reapply
	rows, err := db.QueryContext(ctx, `SELECT filename,checksum FROM schema_migrations ORDER BY filename`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	seen := 0
	for rows.Next() {
		var name, checksum string
		require.NoError(t, rows.Scan(&name, &checksum))
		data, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("%x", sha256.Sum256([]byte(strings.TrimSpace(string(data))))), checksum, name)
		seen++
	}
	require.NoError(t, rows.Err())
	require.Equal(t, len(names), seen)
	// A migrated, pre-existing qualified account can use the new protocol.
	botTestBind(t, botTestService(db), uid, botTestIdentity("qq_official"))
}
