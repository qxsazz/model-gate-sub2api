package handler

import (
	"context"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type vipPrivacyRepository struct {
	service.UserRepository
	service.VIPRepository
}

func (*vipPrivacyRepository) VIPSnapshot(context.Context, int64) (*service.VIPSnapshot, error) {
	return &service.VIPSnapshot{Rules: service.VIPRules{Groups: []service.VIPGroupRule{{GroupID: 21, Private: true}}, ExchangeRates: map[string]float64{"SECRET-CURRENCY": 1}}, Overrides: []service.VIPOverride{{Benefit: "badge", Value: 1, Reason: "INTERNAL-REASON"}}, Ledger: []service.VIPLedgerEntry{{Reason: "PRIVATE-OPENING-NOTE"}}}, nil
}
func TestVIPUserResponseDoesNotExposeInternalRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &UserHandler{userService: service.NewUserService(&vipPrivacyRepository{}, nil, nil, nil)}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/vip", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})
	h.GetVIP(c)
	if recorder.Code != http.StatusOK {
		t.Fatal(recorder.Body.String())
	}
	for _, secret := range []string{"SECRET-CURRENCY", "INTERNAL-REASON", "PRIVATE-OPENING-NOTE", "group_id"} {
		if strings.Contains(recorder.Body.String(), secret) {
			t.Fatalf("exposed %s", secret)
		}
	}
}
func TestVIPAdminResponseRetainsConfigurationForReview(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &UserHandler{userService: service.NewUserService(&vipPrivacyRepository{}, nil, nil, nil)}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/vip/users/1", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})
	h.GetVIP(c)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "INTERNAL-REASON") {
		t.Fatal(recorder.Body.String())
	}
}
