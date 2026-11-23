package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type webchatKeys interface {
	GetByID(context.Context, int64) (*service.APIKey, error)
	List(context.Context, int64, pagination.PaginationParams, service.APIKeyListFilters) ([]service.APIKey, *pagination.PaginationResult, error)
}

type webchatStore interface {
	List(context.Context, int64, int64) ([]service.WebchatConversation, error)
	Get(context.Context, int64, int64, string) (*service.WebchatConversation, error)
	GetAttachment(context.Context, int64, int64, string, string) (*service.WebchatAttachment, error)
	Delete(context.Context, int64, int64, string) error
	ReserveStart(context.Context, int64, int64) (string, error)
	ReleaseStart(context.Context, int64, int64, string) error
	BeginTurn(context.Context, int64, int64, string) (*service.WebchatConversation, string, error)
	SaveTurn(context.Context, int64, int64, string, int64, string, string, string, string, ...service.WebchatAttachment) (*service.WebchatConversation, error)
	ReleaseTurn(context.Context, int64, int64, string, string) error
}

type webchatBridge interface {
	Models(context.Context, *service.APIKey, string) ([]string, error)
	Complete(context.Context, *service.APIKey, string, string, []service.WebchatMessage) (string, error)
}

type WebchatHandler struct {
	keys   webchatKeys
	store  webchatStore
	bridge webchatBridge
}

func NewWebchatHandler(keys *service.APIKeyService, store *service.WebchatStore, bridge *service.WebchatBridge) *WebchatHandler {
	return &WebchatHandler{keys: keys, store: store, bridge: bridge}
}

// Conversation responses must never be reused after logging into a different account.
func webchatSubject(c *gin.Context) (int64, bool) {
	c.Header("Cache-Control", "private, no-store")
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID < 1 {
		response.Unauthorized(c, "请先登录")
		return 0, false
	}
	return subject.UserID, true
}

func usableWebchatKey(key *service.APIKey, userID int64, clientIP string) bool {
	if key == nil || key.UserID != userID || key.Key == "" || !key.IsActive() || key.IsExpired() || key.IsQuotaExhausted() {
		return false
	}
	if key.GroupID != nil && !key.Group.KeyCapabilities().Text {
		return false
	}
	if len(key.IPWhitelist) == 0 && len(key.IPBlacklist) == 0 {
		return true
	}
	allowed, _ := ip.CheckIPRestriction(clientIP, key.IPWhitelist, key.IPBlacklist)
	return allowed
}

func (h *WebchatHandler) Keys(c *gin.Context) {
	userID, ok := webchatSubject(c)
	if !ok {
		return
	}
	type option struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	items := make([]option, 0)
	clientIP := middleware.SecurityClientIP(c)
	params := pagination.PaginationParams{Page: 1, PageSize: 100, SortBy: "id", SortOrder: pagination.SortOrderAsc}
	for {
		keys, page, err := h.keys.List(c.Request.Context(), userID, params, service.APIKeyListFilters{Status: service.StatusActive})
		if err != nil {
			response.InternalError(c, "暂时无法读取密钥")
			return
		}
		for i := range keys {
			if usableWebchatKey(&keys[i], userID, clientIP) {
				items = append(items, option{ID: keys[i].ID, Name: keys[i].Name})
			}
		}
		if len(keys) == 0 || page == nil || params.Page >= page.Pages {
			break
		}
		params.Page++
	}
	response.Success(c, gin.H{"items": items})
}

func (h *WebchatHandler) ownKey(c *gin.Context, userID, keyID int64) (*service.APIKey, bool) {
	if keyID < 1 {
		response.BadRequest(c, "请选择本站密钥")
		return nil, false
	}
	key, err := h.keys.GetByID(c.Request.Context(), keyID)
	if err != nil && !errors.Is(err, service.ErrAPIKeyNotFound) {
		response.InternalError(c, "暂时无法验证密钥")
		return nil, false
	}
	if err != nil || !usableWebchatKey(key, userID, middleware.SecurityClientIP(c)) || key.ID != keyID {
		response.ErrorWithDetails(c, http.StatusForbidden, "所选密钥不可用于对话，请重新选择", "WEBCHAT_KEY_UNAVAILABLE", nil)
		return nil, false
	}
	return key, true
}

func (h *WebchatHandler) queryKey(c *gin.Context) (*service.APIKey, bool) {
	userID, ok := webchatSubject(c)
	if !ok {
		return nil, false
	}
	keyID, err := strconv.ParseInt(c.Query("key_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "请选择本站密钥")
		return nil, false
	}
	return h.ownKey(c, userID, keyID)
}

func (h *WebchatHandler) Models(c *gin.Context) {
	key, ok := h.queryKey(c)
	if !ok {
		return
	}
	models, err := h.bridge.Models(c.Request.Context(), key, middleware.SecurityClientIP(c))
	if err != nil {
		webchatBridgeError(c, err)
		return
	}
	type option struct {
		ID string `json:"id"`
	}
	items := make([]option, 0, len(models))
	for _, model := range models {
		items = append(items, option{ID: model})
	}
	response.Success(c, gin.H{"items": items})
}

func (h *WebchatHandler) Sessions(c *gin.Context) {
	key, ok := h.queryKey(c)
	if !ok {
		return
	}
	items, err := h.store.List(c.Request.Context(), key.UserID, key.ID)
	if err != nil {
		webchatStoreError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *WebchatHandler) sessionParams(c *gin.Context) (*service.APIKey, string, bool) {
	key, ok := h.queryKey(c)
	if !ok {
		return nil, "", false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		webchatStoreError(c, service.ErrWebchatNotFound)
		return nil, "", false
	}
	return key, id.String(), true
}

func (h *WebchatHandler) Session(c *gin.Context) {
	key, id, ok := h.sessionParams(c)
	if !ok {
		return
	}
	session, err := h.store.Get(c.Request.Context(), key.UserID, key.ID, id)
	if err != nil {
		webchatStoreError(c, err)
		return
	}
	response.Success(c, session)
}

func (h *WebchatHandler) GetAttachment(c *gin.Context) {
	key, sessionID, ok := h.sessionParams(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("attachment_id"))
	if err != nil {
		webchatAttachmentNotFound(c)
		return
	}
	file, err := h.store.GetAttachment(c.Request.Context(), key.UserID, key.ID, sessionID, id.String())
	if errors.Is(err, service.ErrWebchatNotFound) {
		webchatAttachmentNotFound(c)
		return
	}
	if err != nil {
		webchatStoreError(c, err)
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": file.Name}))
	c.Data(http.StatusOK, file.MIMEType, file.Data)
}

func webchatAttachmentNotFound(c *gin.Context) {
	response.ErrorWithDetails(c, http.StatusNotFound, "附件不存在或会话已过期", "WEBCHAT_ATTACHMENT_NOT_FOUND", nil)
}

func (h *WebchatHandler) DeleteSession(c *gin.Context) {
	key, id, ok := h.sessionParams(c)
	if !ok {
		return
	}
	if err := h.store.Delete(c.Request.Context(), key.UserID, key.ID, id); err != nil {
		webchatStoreError(c, err)
		return
	}
	response.Success(c, nil)
}

func webchatStoreError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrWebchatNotFound):
		response.ErrorWithDetails(c, http.StatusNotFound, "会话不存在或已过期", "WEBCHAT_SESSION_NOT_FOUND", nil)
	case errors.Is(err, service.ErrWebchatConflict):
		response.ErrorWithDetails(c, http.StatusConflict, "会话正在更新，请刷新后继续", "WEBCHAT_SESSION_CONFLICT", nil)
	case errors.Is(err, service.ErrWebchatLimit):
		response.ErrorWithDetails(c, http.StatusBadRequest, "已达到会话容量上限，请新建会话或删除旧会话", "WEBCHAT_HISTORY_LIMIT", nil)
	default:
		response.InternalError(c, "暂时无法访问会话")
	}
}

// Keep sidecar responses inside the standard API envelope. A gateway 401 describes
// the selected API key; returning a site-login 401 would sign the user out.
func webchatBridgeError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrWebchatModelUnavailable) {
		response.ErrorWithDetails(c, http.StatusForbidden, "当前密钥无法选择此模型，请刷新列表", "WEBCHAT_MODEL_UNAVAILABLE", nil)
		return
	}
	var upstream *service.WebchatUpstreamError
	if !errors.As(err, &upstream) {
		response.ErrorWithDetails(c, http.StatusBadGateway, "对话服务暂时不可用", "WEBCHAT_UPSTREAM_ERROR", nil)
		return
	}
	if upstream.Message != "" {
		status := upstream.Status
		if status == http.StatusUnauthorized {
			status = http.StatusForbidden
		}
		response.ErrorWithDetails(c, status, upstream.Message, "WEBCHAT_GATEWAY_ERROR", nil)
		return
	}
	switch upstream.Status {
	case http.StatusUnauthorized:
		response.ErrorWithDetails(c, http.StatusForbidden, "所选密钥不可用于对话，请重新选择", "WEBCHAT_KEY_UNAVAILABLE", nil)
	case http.StatusPaymentRequired, http.StatusForbidden:
		response.ErrorWithDetails(c, upstream.Status, "请检查所选密钥的余额、配额和模型权限", "WEBCHAT_ACCESS_DENIED", nil)
	case http.StatusRequestEntityTooLarge:
		response.ErrorWithDetails(c, upstream.Status, "模型请求超过大小限制，请减少附件或新建会话", "WEBCHAT_PAYLOAD_TOO_LARGE", nil)
	case http.StatusBadRequest, http.StatusNotFound, http.StatusUnprocessableEntity:
		response.ErrorWithDetails(c, upstream.Status, "模型拒绝了这次请求，未返回具体原因。", "WEBCHAT_MODEL_ERROR", nil)
	case http.StatusTooManyRequests:
		response.ErrorWithDetails(c, upstream.Status, "请求已达到使用限制，请稍后再试", "WEBCHAT_RATE_LIMITED", nil)
	case http.StatusServiceUnavailable:
		response.ErrorWithDetails(c, upstream.Status, "对话服务暂时不可用，请稍后再试", "WEBCHAT_UPSTREAM_UNAVAILABLE", nil)
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		response.ErrorWithDetails(c, http.StatusGatewayTimeout, "模型响应超时，请稍后再试", "WEBCHAT_TIMEOUT", nil)
	default:
		response.ErrorWithDetails(c, http.StatusBadGateway, "对话服务暂时不可用", "WEBCHAT_UPSTREAM_ERROR", nil)
	}
}

type webchatSendRequest struct {
	KeyID       int64           `json:"key_id"`
	SessionID   string          `json:"session_id"`
	Model       string          `json:"model"`
	Content     string          `json:"content"`
	Attachments json.RawMessage `json:"attachments,omitempty"`
}

// Defer allocating file strings until we have verified key ownership. Bound the
// count while decoding so a large array of empty objects cannot amplify memory.
func webchatUploads(raw json.RawMessage) ([]service.WebchatUpload, error) {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	t, err := d.Token()
	if err != nil || t != json.Delim('[') {
		return nil, service.ErrWebchatInvalidAttachment
	}
	var uploads []service.WebchatUpload
	for d.More() {
		var upload service.WebchatUpload
		if len(uploads) >= service.WebchatMaxAttachments || d.Decode(&upload) != nil {
			return nil, service.ErrWebchatInvalidAttachment
		}
		uploads = append(uploads, upload)
	}
	return uploads, nil
}

func webchatAttachmentError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrWebchatAttachmentBusy) {
		response.ErrorWithDetails(c, http.StatusServiceUnavailable, "图片处理繁忙，请稍后重试", "WEBCHAT_ATTACHMENT_BUSY", nil)
		return
	}
	response.ErrorWithDetails(c, http.StatusBadRequest, "请上传有效的 PNG、JPEG 或 WebP 图片，每条消息最多 3 张，合计不超过 4 MiB", "WEBCHAT_ATTACHMENT_INVALID", nil)
}

func (h *WebchatHandler) SendMessage(c *gin.Context) {
	userID, ok := webchatSubject(c)
	if !ok {
		return
	}
	var req webchatSendRequest
	// A code point may occupy six ASCII bytes when JSON-escaped.
	maxBody := base64.StdEncoding.EncodedLen(service.WebchatMaxTurnAttachmentBytes) + 6*(service.WebchatMaxUserBytes+service.WebchatMaxModelBytes) + 8*1024
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, int64(maxBody))
	if err := c.ShouldBindJSON(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			response.ErrorWithDetails(c, http.StatusRequestEntityTooLarge, "消息和图片超过大小限制", "WEBCHAT_ATTACHMENT_INVALID", nil)
		} else {
			response.BadRequest(c, "消息格式无效")
		}
		return
	}
	req.Model = strings.TrimSpace(req.Model)
	if !service.ValidWebchatText(req.Model, service.WebchatMaxModelBytes) || len(req.Content) > service.WebchatMaxUserBytes {
		response.BadRequest(c, "请选择模型并输入有效长度的消息")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		req.Content = ""
	}
	if (req.Content != "" || len(req.Attachments) == 0) && !service.ValidWebchatText(req.Content, service.WebchatMaxUserBytes) {
		response.BadRequest(c, "请选择模型并输入有效长度的消息")
		return
	}
	key, ok := h.ownKey(c, userID, req.KeyID)
	if !ok {
		return
	}

	uploads, err := webchatUploads(req.Attachments)
	req.Attachments = nil
	if err != nil {
		webchatAttachmentError(c, err)
		return
	}
	if req.Content == "" && len(uploads) == 0 {
		response.BadRequest(c, "请输入消息或添加图片")
		return
	}

	ctx := c.Request.Context()
	var history []service.WebchatMessage
	var version int64
	var lease string
	if req.SessionID == "" {
		token, err := h.store.ReserveStart(ctx, userID, key.ID)
		if err != nil {
			webchatStoreError(c, err)
			return
		}
		lease = token
	} else {
		id, err := uuid.Parse(req.SessionID)
		if err != nil {
			webchatStoreError(c, service.ErrWebchatNotFound)
			return
		}
		req.SessionID = id.String()
		session, token, err := h.store.BeginTurn(ctx, userID, key.ID, req.SessionID)
		if err != nil {
			webchatStoreError(c, err)
			return
		}
		history, version, lease = session.Messages, session.Version, token
	}
	defer func() {
		if lease == "" {
			return
		}
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		if req.SessionID == "" {
			_ = h.store.ReleaseStart(cleanup, userID, key.ID, lease)
		} else {
			_ = h.store.ReleaseTurn(cleanup, userID, key.ID, req.SessionID, lease)
		}
	}()
	attachments, err := service.ParseWebchatAttachments(ctx, uploads)
	if err != nil {
		webchatAttachmentError(c, err)
		return
	}
	messages := append(history, service.WebchatMessage{Role: "user", Content: req.Content, Attachments: attachments})
	if !webchatHistoryFits(messages) {
		webchatStoreError(c, service.ErrWebchatLimit)
		return
	}
	answer, err := h.bridge.Complete(ctx, key, middleware.SecurityClientIP(c), req.Model, messages)
	if err != nil {
		webchatBridgeError(c, err)
		return
	}

	// A completed, billable response should survive the browser closing before delivery.
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 8*time.Second)
	defer cancel()
	session, err := h.store.SaveTurn(saveCtx, userID, key.ID, req.SessionID, version, lease, req.Model, req.Content, answer, attachments...)
	if err != nil {
		webchatStoreError(c, err)
		return
	}
	lease = "" // SaveTurn released the lease atomically with persisting the pair.
	response.Success(c, session)
}

// Reserve room for the largest supported answer before incurring model usage.
func webchatHistoryFits(messages []service.WebchatMessage) bool {
	textSize := service.WebchatMaxAssistantBytes
	var fileSize, extractedSize int
	for _, m := range messages {
		textSize += len(m.Content)
		for _, a := range m.Attachments {
			fileSize += a.Size
			extractedSize += len(a.Text)
		}
	}
	return textSize <= service.WebchatMaxHistoryBytes && fileSize <= service.WebchatMaxSessionAttachmentBytes && extractedSize <= service.WebchatMaxSessionExtractedBytes
}
