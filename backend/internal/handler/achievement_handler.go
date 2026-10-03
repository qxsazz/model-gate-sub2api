package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *UserHandler) GetAchievementTopics(c *gin.Context) {
	response.Success(c, service.AchievementTopics())
}
func (h *UserHandler) GetAchievementQuiz(c *gin.Context) {
	v, e := service.AchievementQuiz(c.Param("kind"), c.Param("topic"))
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, v)
}
func (h *UserHandler) SubmitAchievementQuiz(c *gin.Context) {
	s, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var in struct {
		Answers    []int  `json:"answers"`
		RequestKey string `json:"request_key"`
	}
	if e := c.ShouldBindJSON(&in); e != nil {
		response.BadRequest(c, "无效的答案")
		return
	}
	v, e := h.userService.SubmitAchievementActivity(c.Request.Context(), s.UserID, c.Param("kind"), c.Param("topic"), in.RequestKey, in.Answers)
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, v)
}

func (h *UserHandler) GetAchievements(c *gin.Context) {
	s, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	v, e := h.userService.GetAchievements(c.Request.Context(), s.UserID)
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, v)
}
func (h *UserHandler) ChangeAchievement(c *gin.Context) {
	s, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var in struct {
		Key        string `json:"key"`
		Date       string `json:"date"`
		RequestKey string `json:"request_key"`
	}
	if e := c.ShouldBindJSON(&in); e != nil {
		response.BadRequest(c, "无效的请求")
		return
	}
	v, e := h.userService.ChangeAchievement(c.Request.Context(), s.UserID, c.Param("action"), in.Key, in.Date, in.RequestKey)
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, v)
}
func (h *UserHandler) GetAchievementConfig(c *gin.Context) {
	v, e := h.userService.GetAchievementConfig(c.Request.Context())
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, v)
}
func (h *UserHandler) SaveAchievementConfig(c *gin.Context) {
	s, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var cfg service.AchievementConfig
	if e := c.ShouldBindJSON(&cfg); e != nil {
		response.BadRequest(c, "无效的配置")
		return
	}
	if e := h.userService.SetAchievementConfig(c.Request.Context(), s.UserID, cfg); e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, map[string]bool{"saved": true})
}
