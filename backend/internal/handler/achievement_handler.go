package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
)

func achievementTarget(c *gin.Context) (int64, bool) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil || id <= 0 {
		response.BadRequest(c, "无效的用户 ID")
		return 0, false
	}
	return id, true
}
func (h *UserHandler) GetAdminAchievements(c *gin.Context) {
	id, ok := achievementTarget(c)
	if !ok {
		return
	}
	v, e := h.userService.GetAdminAchievements(c.Request.Context(), id)
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, v)
}
func (h *UserHandler) PreviewAchievementBackfill(c *gin.Context) {
	id, ok := achievementTarget(c)
	if !ok {
		return
	}
	v, e := h.userService.PreviewAchievementBackfill(c.Request.Context(), id, c.Query("date"))
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, v)
}
func (h *UserHandler) ControlAdminAchievement(c *gin.Context) {
	id, ok := achievementTarget(c)
	if !ok {
		return
	}
	actor, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var body service.AchievementAdminCommand
	if e := c.ShouldBindJSON(&body); e != nil {
		response.BadRequest(c, "无效的操作参数")
		return
	}
	v, e := h.userService.ControlAdminAchievement(c.Request.Context(), actor.UserID, id, c.Param("action"), body)
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, v)
}
func (h *UserHandler) GetAchievementAudit(c *gin.Context) {
	v, e := h.userService.GetAchievementAudit(c.Request.Context())
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, v)
}

func (h *UserHandler) GetAchievementTopics(c *gin.Context) {
	response.Success(c, service.AchievementTopics())
}

func (h *UserHandler) SaveAchievementPreferences(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var body struct {
		Zodiac *string `json:"zodiac"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Zodiac == nil {
		response.BadRequest(c, "请选择星座，或传入空值取消选择")
		return
	}
	if err := h.userService.SaveAchievementZodiac(c.Request.Context(), subject.UserID, *body.Zodiac); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"zodiac": *body.Zodiac})
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
func (h *UserHandler) PreviewAchievementCard(c *gin.Context) {
	s, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	v, e := h.userService.PreviewAchievementCard(c.Request.Context(), s.UserID, c.Query("date"))
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, v)
}
func (h *UserHandler) UseAchievementCard(c *gin.Context) {
	s, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var in service.AchievementCardCommand
	if e := c.ShouldBindJSON(&in); e != nil {
		response.BadRequest(c, "无效的补签参数")
		return
	}
	v, e := h.userService.UseAchievementCard(c.Request.Context(), s.UserID, in)
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
