//go:build unit

package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestVIPDisabledAuthRepositoryDoesNotQueryUserOrLedger(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	mock.ExpectQuery(`SELECT jsonb_build_object`).WithArgs(int64(7), true).WillReturnRows(sqlmock.NewRows([]string{"snapshot"}).AddRow(`{"rules":{"enabled":false,"tiers":[],"groups":[]},"user":null,"groups":[],"overrides":[]}`))
	snapshot, err := (&userRepository{sql: db}).VIPAuthSnapshot(context.Background(), 7)
	require.NoError(t, err)
	require.False(t, snapshot.Enabled)
	require.Empty(t, snapshot.Ledger)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVIPAuthSnapshotLoadsFreshInputsInOneQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	mock.ExpectQuery(`SELECT jsonb_build_object`).WithArgs(int64(7), true).WillReturnRows(sqlmock.NewRows([]string{"snapshot"}).AddRow(`{"rules":{"enabled":true,"tiers":[],"groups":[]},"user":{"id":7,"email":"fixture@example.com","username":"fixture"},"concurrency":5,"rpm":20,"total":0,"restrict_public":false,"managed":false,"manual_groups":[21],"overrides":[],"groups":[]}`))
	snapshot, err := (&userRepository{sql: db}).VIPAuthSnapshot(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, 5, snapshot.Concurrency)
	require.Equal(t, 20, snapshot.RPM)
	require.Equal(t, []int64{21}, snapshot.ManualGroups)
	require.Empty(t, snapshot.Ledger)
	require.NoError(t, mock.ExpectationsWereMet())
}
