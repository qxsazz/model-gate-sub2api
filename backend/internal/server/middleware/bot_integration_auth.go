package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	ratelimit "github.com/Wei-Shaw/sub2api/internal/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const botPrincipalContextKey = "modelgate_bot_principal"

type BotPrincipal struct {
	PublicURL string
	client    config.BotIntegrationClient
	limiter   panelRateLimitAllower
}

func BotIntegrationAuth(cfg config.BotIntegrationConfig, redisClient *redis.Client) gin.HandlerFunc {
	clients, parseErr := cfg.Clients()
	limiter := ratelimit.NewRateLimiter(redisClient)
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		if !cfg.Enabled {
			AbortWithError(c, 404, "BOT_INTEGRATION_DISABLED", "群聊接入未启用")
			return
		}
		if parseErr != nil {
			AbortWithError(c, 503, "BOT_INTEGRATION_UNAVAILABLE", "群聊接入暂不可用")
			return
		}
		parts := strings.SplitN(c.GetHeader("Authorization"), " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || len(parts[1]) < 40 || len(parts[1]) > 200 || !strings.HasPrefix(parts[1], "mgbot_") {
			AbortWithError(c, 401, "BOT_UNAUTHORIZED", "需要 Bot 专用服务凭据")
			return
		}
		digest := sha256.Sum256([]byte(parts[1]))
		matched := -1
		for idx, client := range clients {
			expected, _ := hex.DecodeString(client.TokenSHA256)
			if subtle.ConstantTimeCompare(digest[:], expected) == 1 {
				matched = idx
			}
		}
		if matched < 0 {
			AbortWithError(c, 401, "BOT_UNAUTHORIZED", "Bot 服务凭据无效")
			return
		}
		principal := BotPrincipal{PublicURL: strings.TrimSuffix(cfg.PublicURL, "/"), client: clients[matched], limiter: limiter}
		if !botRequestLimit(c, limiter, "chat-bot:client:"+principal.client.ID, 240) {
			return
		}
		c.Set(botPrincipalContextKey, principal)
		c.Next()
	}
}

func botRequestLimit(c *gin.Context, limiter panelRateLimitAllower, key string, limit int) bool {
	result, err := limiter.Allow(c.Request.Context(), key, limit, time.Minute)
	if err != nil {
		AbortWithError(c, 503, "BOT_RATE_LIMIT_UNAVAILABLE", "暂时无法核验请求频率，请稍后重试")
		return false
	}
	if !result.Allowed {
		abortPanelRateLimited(c, result.RetryAfter)
		return false
	}
	return true
}

func AuthorizeBotIdentity(c *gin.Context, identity service.ChatIdentity) (*BotPrincipal, bool) {
	value, ok := c.Get(botPrincipalContextKey)
	if !ok {
		AbortWithError(c, 401, "BOT_UNAUTHORIZED", "需要 Bot 专用服务凭据")
		return nil, false
	}
	principal, ok := value.(BotPrincipal)
	if !ok {
		AbortWithError(c, 401, "BOT_UNAUTHORIZED", "需要 Bot 专用服务凭据")
		return nil, false
	}
	allowed := false
	for _, scope := range principal.client.Scopes {
		if scope.Platform != identity.Platform || scope.BotID != identity.BotID {
			continue
		}
		for _, group := range scope.GroupIDs {
			if group == identity.GroupID {
				allowed = true
			}
		}
	}
	if !allowed {
		AbortWithError(c, 403, "BOT_SCOPE_FORBIDDEN", "该平台、Bot 实例或群未获授权")
		return nil, false
	}
	if !botRequestLimit(c, principal.limiter, "chat-bot:identity:"+identity.Key(), 30) {
		return nil, false
	}
	return &principal, true
}
