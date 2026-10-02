package handler

import (
	"context"
	"errors"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type vipClaimRepository struct {
	service.UserRepository
	service.VIPMembershipRepository
	amount float64
	err    error
	uid    int64
}

func (r *vipClaimRepository) VIPClaimReward(_ context.Context, uid int64, _ int) (float64, error) {
	r.uid = uid
	return r.amount, r.err
}
func TestVIPRewardClaimUsesOwnIdentityAndAuditsResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name   string
		amount float64
		err    error
		status int
		result string
	}{
		{"credited", 1, nil, 200, "credited"}, {"duplicate", 0, nil, 200, "already_claimed"},
		{"below threshold", 0, service.ErrVIPRewardThreshold, 400, "rejected"},
		{"database failure", 0, errors.New("password=SECRET"), 500, "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &vipClaimRepository{amount: tc.amount, err: tc.err}
			h := &UserHandler{userService: service.NewUserService(repo, nil, nil, nil)}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/user/vip/rewards/1/claim", strings.NewReader(`{"user_id":999}`))
			c.Params = gin.Params{{Key: "level", Value: "1"}}
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 77})
			c.Set(string(middleware.ContextKeyUserRole), "admin")
			h.ClaimVIPReward(c)
			require.Equal(t, tc.status, recorder.Code)
			require.Equal(t, int64(77), repo.uid)
			extra, exists := c.Get("audit_extra")
			require.True(t, exists)
			details, typed := extra.(map[string]any)
			require.True(t, typed)
			require.Equal(t, tc.result, details["result"])
			require.Equal(t, 1, details["reward_level"])
			require.NotContains(t, recorder.Body.String(), "SECRET")
			if tc.err == nil {
				require.Equal(t, tc.amount, details["reward_amount"])
			} else {
				require.NotEmpty(t, details["error_code"])
			}
		})
	}
}
