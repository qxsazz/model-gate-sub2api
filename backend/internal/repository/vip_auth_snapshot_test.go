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
	mock.ExpectQuery(`SELECT payload`).WillReturnRows(sqlmock.NewRows([]string{"payload"}).AddRow(`{"enabled":false,"tiers":[],"groups":[]}`))
	snapshot, err := (&userRepository{sql: db}).VIPAuthSnapshot(context.Background(), 7)
	require.NoError(t, err)
	require.False(t, snapshot.Enabled)
	require.Empty(t, snapshot.Ledger)
	require.NoError(t, mock.ExpectationsWereMet())
}
