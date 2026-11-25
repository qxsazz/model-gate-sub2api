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
		"263_chat_trial_credit.sql":                      true,
	}
	laterMigrations := map[string]bool{
		"251_group_force_openai_ultrafast.sql":         true,
		"251_group_usage_rollup_nonblocking_build.sql": true,
		"252_channel_pricing_ultrafast_multiplier.sql": true,
		"264_admin_recharge_principal.sql":             true,
		"265_webchat_sessions.sql":                     true,
		"266_webchat_attachments.sql":                  true,
		"267_checkin_credit_expiry.sql":                true,
		"268_staging_admin_checkin.sql":                true,
	}
	names, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	old := fstest.MapFS{}
	staging := fstest.MapFS{}
	for _, name := range names {
		if laterMigrations[name] {
			continue
		}
		data, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		staging[name] = &fstest.MapFile{Data: data}
		if !additions[name] {
			old[name] = staging[name]
		}
	}
	require.NoError(t, applyMigrationsFS(ctx, db, old))
	uid := achievementAccessUser(t, db)
	seedAchievementRecharge(t, db, uid, 10)
	_, err = db.ExecContext(ctx, `INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount,reason)
		VALUES($1,'admin_balance','before-admin-principal',100,'Backend balance adjustment')`, uid)
	require.NoError(t, err)
	require.NoError(t, applyMigrationsFS(ctx, db, staging))
	_, err = db.ExecContext(ctx, `SELECT chat_configure_trial_credit(true,3)`)
	require.NoError(t, err)
	var cutoff time.Time
	require.NoError(t, db.QueryRowContext(ctx, `SELECT eligible_after FROM chat_trial_credit_policy`).Scan(&cutoff))
	var newUser int64
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,balance,created_at)
		VALUES('staging-upgrade@example.invalid','fixture',0,clock_timestamp()) RETURNING id`).Scan(&newUser))
	svc := botTestService(db)
	qq := botTestIdentity("qq_official")
	botTestBind(t, svc, newUser, qq)
	event := botTestEvent(qq)
	claim, err := svc.ChatBotClaimTrialCredit(ctx, event)
	require.NoError(t, err)
	require.Equal(t, "claimed", claim.Status)

	// This reward was already mixed with cash under the previous deployment.
	addCheckinBalanceFixture(t, db, uid, "2026-01-01", 2, time.Now().AddDate(0, -2, 0))
	require.NoError(t, ApplyMigrations(ctx, db))
	var pastLots int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM checkin_balance_credits WHERE user_id=$1`, uid).Scan(&pastLots))
	require.Zero(t, pastLots)
	require.NoError(t, ApplyMigrations(ctx, db)) // startup retry validates checksums and does not reapply
	require.Equal(t, 10.0, achievementAccessResult(t, db, uid)["qualified_recharge_cny"])
	var unmarked bool
	require.NoError(t, db.QueryRowContext(ctx, `SELECT admin_principal_delta_cny IS NULL FROM vip_recharge_ledger
		WHERE source='admin_balance' AND source_id='before-admin-principal'`).Scan(&unmarked))
	require.True(t, unmarked)
	var policyPreserved bool
	require.NoError(t, db.QueryRowContext(ctx, `SELECT enabled AND amount=3 AND eligible_after=$1
		FROM chat_trial_credit_policy`, cutoff).Scan(&policyPreserved))
	require.True(t, policyPreserved)
	replay, err := svc.ChatBotClaimTrialCredit(ctx, event)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.Equal(t, claim.RequestKey, replay.RequestKey)
	require.Equal(t, claim.Status, replay.Status)
	wx := botTestIdentity("wechat_native")
	botTestBind(t, svc, newUser, wx)
	second, err := svc.ChatBotClaimTrialCredit(ctx, botTestEvent(wx))
	require.NoError(t, err)
	require.Equal(t, "already_claimed", second.Status)
	var balance float64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, newUser).Scan(&balance))
	require.Equal(t, 3.0, balance)
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
