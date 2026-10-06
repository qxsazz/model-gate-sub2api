//go:build unit

package repository

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAchievementGrowthSharesBillingTransaction(t *testing.T) {
	db, m, e := sqlmock.New()
	require.NoError(t, e)
	defer func() { _ = db.Close() }()
	m.ExpectBegin()
	tx, e := db.BeginTx(context.Background(), nil)
	require.NoError(t, e)
	m.ExpectQuery(conditionalBalanceDeductSQL).WithArgs(.01, int64(42)).WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(9.99))
	m.ExpectExec("INSERT INTO achievement_growth").WithArgs(int64(42), int64(150), true).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	e = (&usageBillingRepository{}).applyUsageBillingEffects(context.Background(), tx, &service.UsageBillingCommand{UserID: 42, BalanceCost: .01, InputTokens: 100, OutputTokens: 50}, &service.UsageBillingApplyResult{})
	require.NoError(t, e)
	require.NoError(t, tx.Commit())
	require.NoError(t, m.ExpectationsWereMet())
}
