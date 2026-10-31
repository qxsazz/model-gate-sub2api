//go:build unit

package handler

import (
	"context"
	"net/http/httptest"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type achievementAccessHandlerRepo struct {
	service.UserRepository
	err    error
	userID int64
}

func (r *achievementAccessHandlerRepo) EnsureAchievementAccess(_ context.Context, id int64) error {
	r.userID = id
	return r.err
}

func TestAchievementProtectedHTTPEntrypoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, action := range []string{"topics", "quiz", "submit", "checkin", "claim", "claim_series", "preview", "use-card"} {
		t.Run(action, func(t *testing.T) {
			repo := &achievementAccessHandlerRepo{err: infraerrors.Forbidden("ACHIEVEMENT_RECHARGE_REQUIRED", "有效充值未达标")}
			h := &UserHandler{userService: service.NewUserService(repo, nil, nil, nil)}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/", nil)
			c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
			c.Params = gin.Params{{Key: "action", Value: action}, {Key: "kind", Value: "knowledge"}, {Key: "topic", Value: "intro"}}
			switch action {
			case "topics":
				h.GetAchievementTopics(c)
			case "quiz":
				h.GetAchievementQuiz(c)
			case "submit":
				h.SubmitAchievementQuiz(c)
			case "preview":
				h.PreviewAchievementCard(c)
			case "use-card":
				h.UseAchievementCard(c)
			default:
				h.ChangeAchievement(c)
			}
			require.Equal(t, 403, w.Code)
			require.Contains(t, w.Body.String(), "ACHIEVEMENT_RECHARGE_REQUIRED")
			require.Equal(t, int64(42), repo.userID)
		})
	}
}

func TestAchievementTopicsUseCurrentUserQualification(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &achievementAccessHandlerRepo{}
	h := &UserHandler{userService: service.NewUserService(repo, nil, nil, nil)}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 51})
	h.GetAchievementTopics(c)
	require.Equal(t, 200, w.Code)
	require.Equal(t, int64(51), repo.userID)
}
