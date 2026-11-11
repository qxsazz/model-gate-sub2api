package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func decodeBotJSON(c *gin.Context, target any) bool {
	c.Header("Cache-Control", "no-store")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("BOT_REQUEST_INVALID", "请求字段或 JSON 格式无效"))
		return false
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		response.ErrorFrom(c, infraerrors.BadRequest("BOT_REQUEST_INVALID", "只允许一个 JSON 对象"))
		return false
	}
	return true
}

func validateBotEvent(c *gin.Context, event service.BotIdentityEvent) (*middleware2.BotPrincipal, bool) {
	if err := event.Validate(); err != nil {
		response.ErrorFrom(c, err)
		return nil, false
	}
	return middleware2.AuthorizeBotIdentity(c, event.Identity)
}

func (h *UserHandler) CreateBotBindingChallenge(c *gin.Context) {
	var input service.BotBindingChallengeInput
	if !decodeBotJSON(c, &input) {
		return
	}
	principal, ok := validateBotEvent(c, input.BotIdentityEvent)
	if !ok {
		return
	}
	result, err := h.userService.CreateChatBindingChallenge(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"challenge_id": result.ChallengeID, "binding_url": principal.PublicURL + "/chat-bindings/authorize/" + result.ChallengeID, "expires_at": result.ExpiresAt, "replayed": result.Replayed})
}

func (h *UserHandler) ConfirmBotBinding(c *gin.Context) {
	var input service.BotConfirmInput
	if !decodeBotJSON(c, &input) {
		return
	}
	if _, ok := validateBotEvent(c, input.BotIdentityEvent); !ok {
		return
	}
	result, err := h.userService.ConfirmChatBinding(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UserHandler) BotCheckin(c *gin.Context) {
	var input service.BotCheckinInput
	if !decodeBotJSON(c, &input) {
		return
	}
	if _, ok := validateBotEvent(c, input.BotIdentityEvent); !ok {
		return
	}
	result, err := h.userService.ChatBotCheckin(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UserHandler) GetChatBindingChallenge(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	result, err := h.userService.GetChatBindingChallenge(c.Request.Context(), subject.UserID, c.Param("challenge_id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UserHandler) AuthorizeChatBindingChallenge(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var input struct {
		Confirm             bool   `json:"confirm"`
		AccountConfirmation string `json:"account_confirmation"`
	}
	if !decodeBotJSON(c, &input) {
		return
	}
	if !input.Confirm {
		response.ErrorFrom(c, infraerrors.BadRequest("BOT_BINDING_CONFIRMATION_REQUIRED", "请确认当前登录账号和群聊来源"))
		return
	}
	result, err := h.userService.AuthorizeChatBindingChallenge(c.Request.Context(), subject.UserID, c.Param("challenge_id"), input.AccountConfirmation)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UserHandler) ListChatBindings(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	result, err := h.userService.ListChatBindings(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UserHandler) RevokeChatBinding(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("binding_id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("BOT_BINDING_INVALID", "绑定编号无效"))
		return
	}
	if err = h.userService.RevokeChatBinding(c.Request.Context(), subject.UserID, id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"revoked": true})
}
