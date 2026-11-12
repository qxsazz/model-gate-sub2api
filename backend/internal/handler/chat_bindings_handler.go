package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

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

func (h *UserHandler) CreateBotBindingEmail(c *gin.Context) {
	var input service.BotBindingChallengeInput
	if !decodeBotJSON(c, &input) {
		return
	}
	principal, ok := validateBotEvent(c, input.BotIdentityEvent)
	if !ok {
		return
	}
	if input.RequestedEmail == nil {
		response.ErrorFrom(c, infraerrors.BadRequest("BOT_BINDING_EMAIL_INVALID", "请填写有效的注册邮箱"))
		return
	}
	if !principal.AllowBindingEmail(c, input.Identity, *input.RequestedEmail) {
		return
	}
	ctx := c.Request.Context()
	smtp, err := h.emailService.GetSMTPConfig(ctx)
	if err != nil || (strings.TrimSpace(smtp.From) == "" && strings.TrimSpace(smtp.Username) == "") {
		response.ErrorFrom(c, infraerrors.New(503, "BOT_BINDING_MAIL_UNAVAILABLE", "绑定邮件暂不可发送，请联系管理员"))
		return
	}
	result, err := h.userService.CreateChatBindingChallenge(ctx, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !result.Replayed {
		groupName := h.botConfig.GroupName(input.Identity.Platform, input.Identity.BotID, input.Identity.GroupID)
		link := principal.PublicURL + "/chat-bindings/authorize/" + result.ChallengeID
		if err := h.authService.QueueChatBindingEmail(ctx, *input.RequestedEmail, groupName, link, result.ExpiresAt); err != nil {
			response.ErrorFrom(c, infraerrors.New(503, "BOT_BINDING_MAIL_UNAVAILABLE", "绑定邮件暂不可发送，请稍后重试"))
			return
		}
	}
	response.Success(c, gin.H{"status": "email_requested"})
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
	result.GroupName = h.botConfig.GroupName(result.Identity.Platform, result.Identity.BotID, result.Identity.GroupID)
	response.Success(c, result)
}

func (h *UserHandler) AuthorizeChatBindingChallenge(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var input struct {
		Confirm             *bool  `json:"confirm"`
		AccountConfirmation string `json:"account_confirmation"`
	}
	if !decodeBotJSON(c, &input) {
		return
	}
	if input.Confirm == nil {
		response.ErrorFrom(c, infraerrors.BadRequest("BOT_BINDING_CONFIRMATION_REQUIRED", "请选择是否确认绑定"))
		return
	}
	result, err := h.userService.AuthorizeChatBindingChallenge(c.Request.Context(), subject.UserID, c.Param("challenge_id"), input.AccountConfirmation, *input.Confirm)
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
	for i := range result {
		identity := result[i].Identity
		result[i].GroupName = h.botConfig.GroupName(identity.Platform, identity.BotID, identity.GroupID)
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
