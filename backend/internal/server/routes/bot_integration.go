package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func RegisterBotIntegrationRoutes(v1 *gin.RouterGroup, h *handler.Handlers, cfg config.BotIntegrationConfig, redisClient *redis.Client) {
	bot := v1.Group("/integrations/bot")
	bot.Use(middleware.BotIntegrationAuth(cfg, redisClient))
	bot.POST("/binding-challenges", h.User.CreateBotBindingChallenge)
	bot.POST("/binding-emails", h.User.CreateBotBindingEmail)
	bot.POST("/bindings/confirm", h.User.ConfirmBotBinding)
	bot.POST("/checkin", h.User.BotCheckin)
}
