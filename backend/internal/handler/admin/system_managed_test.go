//go:build unit

package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestManagedDeploymentRejectsHTTPMutationsBeforeLocks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewSystemHandler(service.NewUpdateService(nil, nil, "0.2.10-mg.1", "release"), nil)
	router := gin.New()
	router.POST("/update", handler.PerformUpdate)
	router.POST("/rollback", handler.Rollback)
	router.GET("/rollback-versions", handler.GetRollbackVersions)
	for _, target := range []struct{ method, path, body string }{
		{"POST", "/update", ""}, {"POST", "/rollback", ""}, {"POST", "/rollback", `{"version":"0.1.1"}`}, {"GET", "/rollback-versions", ""},
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(target.method, target.path, strings.NewReader(target.body))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(response, request)
		require.Equal(t, http.StatusConflict, response.Code, response.Body.String())
		require.Contains(t, response.Body.String(), "DEPLOYMENT_MANAGED")
	}
}
