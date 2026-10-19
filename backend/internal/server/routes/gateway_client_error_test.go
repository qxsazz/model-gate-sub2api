package routes

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGatewayClientErrorRoutesCoverAuthAndExcludeAdmin(t *testing.T) {
	r := gin.New()
	h := &handler.Handlers{Gateway: &handler.GatewayHandler{}, OpenAIGateway: &handler.OpenAIGatewayHandler{}, AsyncImage: handler.NewAsyncImageHandler(nil, nil)}
	auth := middleware.APIKeyAuthMiddleware(func(c *gin.Context) {
		c.AbortWithStatusJSON(401, gin.H{"code": "INVALID_API_KEY", "message": "original authentication message"})
	})
	cfg := &config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1024 * 1024, TextMaxBodySize: 1024 * 1024}}
	RegisterGatewayRoutes(r, h, auth, nil, nil, nil, nil, nil, cfg)
	r.POST("/api/v1/admin/example", func(c *gin.Context) { c.JSON(401, gin.H{"error": gin.H{"message": "original admin message"}}) })
	for _, path := range []string{"/v1/responses", "/v1/messages", "/v1/chat/completions", "/responses", "/backend-api/codex/responses", "/v1beta/models/gemini:generateContent", "/antigravity/v1/messages", "/antigravity/v1beta/models/gemini:generateContent"} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("POST", path, strings.NewReader(`{}`)))
			require.Equal(t, 401, w.Code)
			if strings.Contains(path, "v1beta") {
				require.Equal(t, int64(401), gjson.Get(w.Body.String(), "error.code").Int())
				require.Equal(t, "UNAUTHENTICATED", gjson.Get(w.Body.String(), "error.status").String())
				require.Contains(t, gjson.Get(w.Body.String(), "error.message").String(), "【认证失败｜401】")
			} else {
				require.Equal(t, "INVALID_API_KEY", gjson.Get(w.Body.String(), "code").String())
				require.Contains(t, gjson.Get(w.Body.String(), "message").String(), "【认证失败｜401】")
			}
		})
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/antigravity/models", nil))
	require.Equal(t, 401, w.Code)
	require.Contains(t, gjson.Get(w.Body.String(), "message").String(), "【认证失败｜401】")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/admin/example", nil))
	require.Equal(t, "original admin message", gjson.Get(w.Body.String(), "error.message").String())
}
