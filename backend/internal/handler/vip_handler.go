package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
)

func (h *UserHandler) GetVIPMembership(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Unauthenticated")
		return
	}
	data, err := h.userService.GetVIPMembership(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}
func (h *UserHandler) ClaimVIPReward(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Unauthenticated")
		return
	}
	level, err := strconv.Atoi(c.Param("level"))
	if err != nil {
		response.BadRequest(c, "Invalid milestone")
		return
	}
	amount, err := h.userService.ClaimVIPReward(c.Request.Context(), subject.UserID, level)
	if err != nil {
		response.BadRequest(c, "奖励暂不可领取，请刷新会员中心确认资格。")
		return
	}
	response.Success(c, map[string]float64{"amount": amount})
}

func (h *UserHandler) GetVIP(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Unauthenticated")
		return
	}
	id := subject.UserID
	if param := c.Param("id"); param != "" {
		var err error
		id, err = strconv.ParseInt(param, 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "Invalid user ID")
			return
		}
	}
	data, err := h.userService.GetVIP(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if c.Param("id") == "" {
		data.Rules.Groups = nil
		data.Rules.ExchangeRates = nil
		for i := range data.Overrides {
			data.Overrides[i].Reason = ""
		}
		for i := range data.Ledger {
			data.Ledger[i].Reason = ""
		}
	}
	response.Success(c, data)
}
func (h *UserHandler) GetVIPConfig(c *gin.Context) {
	data, err := h.userService.VIPConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}
func (h *UserHandler) SaveVIPConfig(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Unauthenticated")
		return
	}
	var rules service.VIPRules
	if err := c.ShouldBindJSON(&rules); err != nil {
		response.BadRequest(c, "Invalid VIP rules")
		return
	}
	if err := h.userService.SaveVIPConfig(c.Request.Context(), subject.UserID, rules); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, map[string]bool{"saved": true})
}
func (h *UserHandler) SetVIPOverride(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Unauthenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	var o service.VIPOverride
	if err = c.ShouldBindJSON(&o); err != nil {
		response.BadRequest(c, "Invalid override")
		return
	}
	if err = h.userService.SetVIPOverride(c.Request.Context(), subject.UserID, id, o); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, map[string]bool{"saved": true})
}
func (h *UserHandler) ClearVIPOverride(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Unauthenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	if err = h.userService.ClearVIPOverride(c.Request.Context(), subject.UserID, id, c.Param("benefit")); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, map[string]bool{"saved": true})
}
func (h *UserHandler) InitialVIPCredit(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Unauthenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	var body struct {
		Amount float64 `json:"amount"`
		Reason string  `json:"reason"`
	}
	if err = c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "Invalid opening record")
		return
	}
	if err = h.userService.InitialVIPCredit(c.Request.Context(), subject.UserID, id, body.Amount, body.Reason); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, map[string]bool{"saved": true})
}
