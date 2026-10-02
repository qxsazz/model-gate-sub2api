//go:build integration

package repository

import (
	"context"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestVIPRepositoryGrowthPermissionsAndManualPrices(t *testing.T) {
	// The harness applies all SQL migrations; every fixture here rolls back.
	_ = testEntClient(t)
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	repo := &userRepository{sql: tx}
	var user, ordinary, exclusive, private int64
	email := fmt.Sprintf("vip-%d@example.com", time.Now().UnixNano())
	require.NoError(t, tx.QueryRowContext(ctx, "INSERT INTO users(email,password_hash,concurrency,balance) VALUES($1,'fixture',5,100) RETURNING id", email).Scan(&user))
	for _, fixture := range []struct {
		name      string
		exclusive bool
		target    *int64
	}{{"vip-ordinary-" + email, false, &ordinary}, {"vip-exclusive-" + email, true, &exclusive}, {"zth-plus-" + email, true, &private}} {
		require.NoError(t, tx.QueryRowContext(ctx, "INSERT INTO groups(name,platform,is_exclusive,rate_multiplier) VALUES($1,'openai',$2,0.3) RETURNING id", fixture.name, fixture.exclusive).Scan(fixture.target))
	}
	rules := service.DefaultVIPRules()
	rules.Enabled = true
	rules.Groups = []service.VIPGroupRule{{GroupID: ordinary, Floor: .2, Discounts: []float64{.02, .04, .06, .08, .1}}, {GroupID: exclusive, Access: true, Discounts: []float64{0, 0, 0, 0, 0}}, {GroupID: private, Private: true, Discounts: []float64{0, 0, 0, 0, 0}}}
	require.NoError(t, repo.VIPSaveRules(ctx, user, rules))
	require.NoError(t, repo.VIPInitialCredit(ctx, user, user, 100, "fixture opening"))
	require.NoError(t, repo.VIPInitialCredit(ctx, user, user, 100, "fixture duplicate"))
	state, err := repo.VIPSnapshot(ctx, user)
	require.NoError(t, err)
	require.Equal(t, 100.0, state.Total)
	require.Equal(t, 1, state.Tier.Level)
	require.Equal(t, 8, state.Concurrency)
	authState, err := repo.VIPAuthSnapshot(ctx, user)
	require.NoError(t, err)
	require.Equal(t, state.Total, authState.Total)
	require.Equal(t, state.Groups, authState.Groups)
	require.Empty(t, authState.Ledger, "authentication must not load display history")
	find := func(id int64) *service.VIPGroupView {
		for i := range state.Groups {
			if state.Groups[i].ID == id {
				return &state.Groups[i]
			}
		}
		return nil
	}
	require.Equal(t, .28, find(ordinary).Rate)
	require.True(t, find(exclusive).Granted)
	require.Nil(t, find(private))
	_, err = tx.ExecContext(ctx, "INSERT INTO user_allowed_groups(user_id,group_id) VALUES($1,$2)", user, private)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "INSERT INTO user_group_rate_multipliers(user_id,group_id,rate_multiplier) VALUES($1,$2,0.25)", user, ordinary)
	require.NoError(t, err)
	require.NoError(t, repo.VIPSetOverride(ctx, user, user, service.VIPOverride{Benefit: "concurrency", Value: 12, Reason: "manual fixture"}))
	_, err = tx.ExecContext(ctx, "INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) VALUES($1,'admin_balance',$2,-1)", user, email)
	require.NoError(t, err)
	state, err = repo.VIPSnapshot(ctx, user)
	require.NoError(t, err)
	require.Equal(t, 0, state.Tier.Level)
	require.Equal(t, 12, state.Concurrency)
	require.Equal(t, .25, find(ordinary).Rate)
	require.False(t, find(exclusive).Granted)
	require.True(t, find(private).Granted)
	require.NoError(t, repo.VIPClearOverride(ctx, user, user, "concurrency"))
	state, err = repo.VIPSnapshot(ctx, user)
	require.NoError(t, err)
	require.Equal(t, 5, state.Concurrency)
}
