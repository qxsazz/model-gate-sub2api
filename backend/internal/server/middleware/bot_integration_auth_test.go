package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestBotIntegrationCredentialScopeAndRateLimits(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr(), MaxRetries: -1, DialTimeout: 100 * time.Millisecond})
	t.Cleanup(func() { _ = client.Close() })
	token := "mgbot_" + strings.Repeat("x", 43)
	digest := sha256.Sum256([]byte(token))
	clients, err := json.Marshal([]config.BotIntegrationClient{{ID: "test", TokenSHA256: hex.EncodeToString(digest[:]), Scopes: []config.BotIntegrationScope{{Platform: "qq_official", BotID: "bot-1", GroupIDs: []string{"group-1"}}}}})
	require.NoError(t, err)
	cfg := config.BotIntegrationConfig{Enabled: true, PublicURL: "https://staging.model-gate.cc", ClientsJSON: string(clients)}
	identity := service.ChatIdentity{Platform: "qq_official", BotID: "bot-1", GroupID: "group-1", SenderID: "member-openid"}
	engine := gin.New()
	engine.Use(BotIntegrationAuth(cfg, client))
	engine.POST("/", func(c *gin.Context) {
		if _, ok := AuthorizeBotIdentity(c, identity); ok {
			c.Status(204)
		}
	})
	request := func(value string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest("POST", "/", nil)
		r.Header.Set("Authorization", value)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, r)
		return w
	}
	for _, credential := range []string{"", "Bearer ordinary-user-key", "Bearer eyJhbGciOiJIUzI1NiJ9.jwt.signature", "Bearer mgbot_" + strings.Repeat("z", 43)} {
		require.Equal(t, 401, request(credential).Code)
	}
	require.Equal(t, 204, request("Bearer "+token).Code)
	for _, change := range []func(){func() { identity.GroupID = "other" }, func() { identity.BotID = "other" }, func() { identity.Platform = "wechat_native" }} {
		identity = service.ChatIdentity{Platform: "qq_official", BotID: "bot-1", GroupID: "group-1", SenderID: "member-openid"}
		change()
		require.Equal(t, 403, request("Bearer "+token).Code)
	}
	identity = service.ChatIdentity{Platform: "qq_official", BotID: "bot-1", GroupID: "group-1", SenderID: "rate-member"}
	for i := 0; i < 30; i++ {
		require.Equal(t, 204, request("Bearer "+token).Code)
	}
	limited := request("Bearer " + token)
	require.Equal(t, 429, limited.Code)
	require.NotEmpty(t, limited.Header().Get("Retry-After"))
	require.Equal(t, "no-store", limited.Header().Get("Cache-Control"))
	redisServer.Close()
	require.Equal(t, 503, request("Bearer "+token).Code)
	disabled := gin.New()
	disabled.Use(BotIntegrationAuth(config.BotIntegrationConfig{}, nil))
	disabled.POST("/", func(c *gin.Context) { t.Error("disabled route ran") })
	w := httptest.NewRecorder()
	disabled.ServeHTTP(w, httptest.NewRequest("POST", "/", nil))
	require.Equal(t, 404, w.Code)
}
