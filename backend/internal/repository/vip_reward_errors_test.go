//go:build unit

package repository

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestVIPClaimErrorsHaveSafeReasons(t *testing.T) {
	for _, tc := range []struct {
		message, reason string
		code            int
	}{
		{"User unavailable", "VIP_REWARD_ACCOUNT_UNAVAILABLE", 403},
		{"VIP disabled", "VIP_REWARD_DISABLED", 409},
		{"Invalid milestone", "VIP_REWARD_INVALID_LEVEL", 400},
		{"Milestone not reached", "VIP_REWARD_THRESHOLD_NOT_REACHED", 400},
		{"Invalid reward", "VIP_REWARD_CONFIG_INVALID", 500},
		{"password=SECRET internal SQL", "VIP_REWARD_UNAVAILABLE", 500},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			repo := &userRepository{sql: db}
			mock.ExpectQuery(`SELECT vip_claim_reward`).WithArgs(int64(77), 1).WillReturnError(&pq.Error{Code: "P0001", Message: tc.message})
			_, err = repo.VIPClaimReward(context.Background(), 77, 1)
			require.Equal(t, tc.code, infraerrors.Code(err))
			require.Equal(t, tc.reason, infraerrors.Reason(err))
			require.NotContains(t, infraerrors.Message(err), "SECRET")
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	mock.ExpectQuery(`SELECT vip_claim_reward`).WithArgs(int64(77), 1).WillReturnRows(sqlmock.NewRows([]string{"amount"}))
	_, err = (&userRepository{sql: db}).VIPClaimReward(context.Background(), 77, 1)
	require.Equal(t, 500, infraerrors.Code(err), "no result must not masquerade as an already-claimed success")
	require.NoError(t, mock.ExpectationsWereMet())
}
