//go:build integration

package repository

import (
	"context"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestVIPGroupRulesSaveAfterDeletingGroupAndAddingReplacement(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx := testTx(t)
	repo := &userRepository{sql: tx}
	var actor, existing, inactive, deleted, newGroup int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,balance) VALUES($1,'fixture',100) RETURNING id`, uuid.NewString()+"@test.invalid").Scan(&actor))
	for _, fixture := range []struct {
		status string
		id     *int64
	}{{"active", &existing}, {"inactive", &inactive}, {"active", &deleted}, {"active", &newGroup}} {
		require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO groups(name,platform,rate_multiplier,status) VALUES($1,'openai',.4,$2) RETURNING id`, uuid.NewString(), fixture.status).Scan(fixture.id))
	}
	rules := service.DefaultVIPRules()
	rules.Groups = []service.VIPGroupRule{
		{GroupID: existing, Floor: .2, Discounts: []float64{.01, .02, .03, .04, .05}},
		{GroupID: inactive, Floor: .1, Discounts: []float64{0, 0, 0, 0, 0}},
		{GroupID: deleted, Floor: .2, Discounts: []float64{0, 0, 0, 0, 0}},
	}
	require.NoError(t, repo.VIPSaveRules(ctx, actor, rules))
	_, err := tx.ExecContext(ctx, `UPDATE groups SET deleted_at=now() WHERE id=$1`, deleted)
	require.NoError(t, err)
	stale, err := repo.VIPRules(ctx)
	require.NoError(t, err)
	require.Len(t, stale.Groups, 3, "group deletion currently leaves an old VIP reference")
	err = repo.VIPSaveRules(ctx, actor, stale)
	require.ErrorContains(t, err, "分组已删除或不存在")
	require.ErrorContains(t, err, "请刷新分组权益后重新保存")
	require.Contains(t, err.Error(), strconv.FormatInt(deleted, 10))
	unchanged, err := repo.VIPRules(ctx)
	require.NoError(t, err)
	require.Equal(t, stale, unchanged, "a rejected save must not partially change rules")
	// The UI reconciles against all non-deleted groups, including inactive ones.
	// Keep the old inactive rule, edit the current group, and configure the new one.
	reconciled := stale
	reconciled.Groups = []service.VIPGroupRule{stale.Groups[0], stale.Groups[1], {GroupID: newGroup, Floor: .3, Discounts: []float64{.01, .02, .03, .04, .05}}}
	reconciled.Groups[0].Floor = .19
	require.NoError(t, repo.VIPSaveRules(ctx, actor, reconciled))
	saved, err := repo.VIPRules(ctx)
	require.NoError(t, err)
	require.Equal(t, reconciled, saved)
	require.Equal(t, rules.Groups[1], saved.Groups[1], "disabled groups retain their configured rights")
	var balance float64
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, actor).Scan(&balance))
	require.Equal(t, 100.0, balance)
}
