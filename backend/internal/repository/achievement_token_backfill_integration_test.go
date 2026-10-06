//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"strings"
	"testing"
)

func TestHistoricalTokenProgressIsIdempotentAndNeverPays(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	fixture, err := os.ReadFile("../../tools/achievements/testdata/token-progress.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(fixture))
	require.NoError(t, err)
	source, err := os.ReadFile("../../tools/achievements/token-source.sql")
	require.NoError(t, err)
	render := func(file string) string {
		raw, e := os.ReadFile("../../tools/achievements/" + file)
		require.NoError(t, e)
		return strings.ReplaceAll(string(raw), "/*TOKEN_SOURCE_SQL*/", string(source))
	}
	_, err = tx.ExecContext(ctx, `SELECT set_config('model_gate.token_history_cutoff','4',true)`)
	require.NoError(t, err)
	var raw string
	require.NoError(t, tx.QueryRowContext(ctx, render("token-snapshot.sql")).Scan(&raw))
	var manifest map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &manifest))
	require.Equal(t, float64(300), manifest["expected_tokens"])
	manifest["approved"] = true
	rawBytes, err := json.Marshal(manifest)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `SELECT set_config('model_gate.token_history_manifest',$1,true)`, string(rawBytes))
	require.NoError(t, err)
	apply := render("historical-token-progress.sql")
	for _, check := range []struct{ setup, message string }{
		{`INSERT INTO achievement_growth(user_id,tokens) VALUES(2,5)`, "Existing token progress"},
		{`UPDATE usage_logs SET input_tokens=input_tokens+1 WHERE id=1`, "source changed since review"},
		{`UPDATE achievement_config SET payload=payload||'{"cash_enabled":true}'`, "cash must be disabled"},
		{`INSERT INTO usage_logs(id) VALUES(5)`, "Usage changed after snapshot"},
	} {
		_, err = tx.ExecContext(ctx, `SAVEPOINT validation`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, check.setup)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, apply)
		require.ErrorContains(t, err, check.message)
		_, err = tx.ExecContext(ctx, `ROLLBACK TO validation`)
		require.NoError(t, err)
	}
	_, err = tx.ExecContext(ctx, apply)
	require.NoError(t, err)
	var tokens int64
	var balance float64
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT tokens FROM achievement_growth WHERE user_id=2`).Scan(&tokens))
	require.Equal(t, int64(300), tokens)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=2`).Scan(&balance))
	require.Equal(t, 20.0, balance)
	_, err = tx.ExecContext(ctx, `UPDATE achievement_growth SET tokens=tokens+40 WHERE user_id=2`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, apply)
	require.NoError(t, err)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT tokens FROM achievement_growth WHERE user_id=2`).Scan(&tokens))
	require.Equal(t, int64(340), tokens)
	_, err = tx.ExecContext(ctx, `SAVEPOINT conflict`)
	require.NoError(t, err)
	manifest["batch_id"] = "different-batch"
	rawBytes, err = json.Marshal(manifest)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `SELECT set_config('model_gate.token_history_manifest',$1,true)`, string(rawBytes))
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, apply)
	require.ErrorContains(t, err, "historical batch conflicts")
	_, err = tx.ExecContext(ctx, `ROLLBACK TO conflict`)
	require.NoError(t, err)
}
