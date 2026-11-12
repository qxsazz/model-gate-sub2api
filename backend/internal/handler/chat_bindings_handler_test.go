package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBotRequestRejectsAccountInjectionAndAmbiguousBody(t *testing.T) {
	base := `{"identity":{"platform":"qq_official","bot_id":"b","group_id":"g","sender_id":"s"},"event_id":"e","event_at":"2026-11-11T12:00:00Z"}`
	for _, body := range []string{strings.TrimSuffix(base, "}") + `,"user_id":123}`, strings.Replace(base, `"sender_id":"s"`, `"sender_id":"s","email":"victim@example.com"`, 1), base + base, strings.Repeat(" ", 8192) + base, `null`} {
		engine := gin.New()
		engine.POST("/", func(c *gin.Context) {
			var input service.BotCheckinInput
			if !decodeBotJSON(c, &input) {
				return
			}
			if err := input.Validate(); err != nil {
				c.Status(400)
				return
			}
			t.Error("invalid request accepted")
		})
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest("POST", "/", strings.NewReader(body)))
		require.Equal(t, 400, w.Code)
	}
}
