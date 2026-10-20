package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const canvasRetention = 7 * 24 * time.Hour

var errCanvasAssetDeleted = errors.New("canvas asset was deleted by administrator")

type CanvasHandler struct {
	db      *sql.DB
	keys    *service.APIKeyService
	dir     string
	maxBody int64
	gateway http.Handler
}

func NewCanvasHandler(db *sql.DB, keys *service.APIKeyService, cfg *config.Config) *CanvasHandler {
	return &CanvasHandler{db: db, keys: keys, dir: filepath.Join(cfg.Pricing.DataDir, "canvas-private"), maxBody: cfg.Gateway.MaxBodySize}
}

func (h *CanvasHandler) SetGateway(gateway http.Handler) { h.gateway = gateway }
func canvasUser(c *gin.Context) int64 {
	s, _ := middleware.GetAuthSubjectFromContext(c)
	return s.UserID
}
func canvasFailure(c *gin.Context, err error) {
	if errors.Is(err, errCanvasAssetDeleted) {
		c.JSON(http.StatusGone, gin.H{"message": "该素材已被管理员删除"})
		return
	}
	log.Printf("canvas: %v", err)
	response.InternalError(c, "画布服务暂时不可用，请重试")
}

// Files are private and never exposed by the frontend static file server.
func (h *CanvasHandler) assetPath(id string) string { return filepath.Join(h.dir, id) }
func (h *CanvasHandler) save(ctx context.Context, user int64, key, mime string, data []byte, keyID int64, model, prompt, source string) (string, error) {
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()
	// Serialize aliases for identical content, so a generated image and its canvas node share review and expiry.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("%d:%s", user, hash)); err != nil {
		return "", err
	}
	var removed bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM canvas_assets WHERE user_id=$1 AND content_hash=$2 AND deleted_at IS NOT NULL AND expires_at>now())`, user, hash).Scan(&removed); err != nil {
		return "", err
	}
	if removed {
		return "", errCanvasAssetDeleted
	}
	var id string
	err = tx.QueryRowContext(ctx, `SELECT a.id FROM canvas_asset_bindings b JOIN canvas_assets a ON a.id=b.asset_id WHERE b.user_id=$1 AND b.storage_key=$2 AND a.content_hash=$3 AND a.deleted_at IS NULL AND a.expires_at>now()`, user, key, hash).Scan(&id)
	if err == nil {
		return id, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	err = tx.QueryRowContext(ctx, `SELECT id FROM canvas_assets WHERE user_id=$1 AND content_hash=$2 AND deleted_at IS NULL AND expires_at>now() ORDER BY created_at LIMIT 1`, user, hash).Scan(&id)
	created := false
	if errors.Is(err, sql.ErrNoRows) {
		id = uuid.NewString()
		created = true
		if err = os.MkdirAll(h.dir, 0700); err != nil {
			return "", err
		}
		if err = os.WriteFile(h.assetPath(id), data, 0600); err != nil {
			return "", err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO canvas_assets(id,user_id,storage_key,content_hash,mime_type,byte_size,api_key_id,model,prompt,source,expires_at) VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,0),$8,$9,$10,$11)`, id, user, id, hash, mime, len(data), keyID, model, prompt, source, time.Now().Add(canvasRetention))
	}
	if err == nil && source == "generation" {
		_, err = tx.ExecContext(ctx, `UPDATE canvas_assets SET source='generation',api_key_id=$2,model=$3,prompt=$4 WHERE id=$1`, id, keyID, model, prompt)
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO canvas_asset_bindings(user_id,storage_key,asset_id) VALUES($1,$2,$3) ON CONFLICT(user_id,storage_key) DO UPDATE SET asset_id=EXCLUDED.asset_id`, user, key, id)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil && created {
		_ = os.Remove(h.assetPath(id))
	}
	return id, err
}

type canvasImageSink struct {
	h             *CanvasHandler
	user, keyID   int64
	model, prompt string
}

func (s canvasImageSink) Save(ctx context.Context, key, mime string, data []byte) (string, error) {
	id, err := s.h.save(ctx, s.user, "generated:"+key, mime, data, s.keyID, s.model, s.prompt, "generation")
	return "/api/v1/canvas/assets/" + id + "/content", err
}

func (h *CanvasHandler) State(c *gin.Context) {
	name := c.Param("name")
	if !strings.HasPrefix(name, "infinite-canvas:") || strings.Contains(name, "config") || strings.Contains(name, "plugin") {
		response.BadRequest(c, "不支持的画布状态")
		return
	}
	user := canvasUser(c)
	if c.Request.Method == http.MethodGet {
		var payload []byte
		var revision int64
		err := h.db.QueryRowContext(c.Request.Context(), `SELECT payload,revision FROM canvas_states WHERE user_id=$1 AND name=$2`, user, name).Scan(&payload, &revision)
		if errors.Is(err, sql.ErrNoRows) {
			response.Success(c, gin.H{"value": nil, "revision": 0})
			return
		}
		if err != nil {
			canvasFailure(c, err)
			return
		}
		response.Success(c, gin.H{"value": string(payload), "revision": revision})
		return
	}
	var req struct {
		Value    string `json:"value"`
		Revision int64  `json:"revision"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxBody)
	if c.ShouldBindJSON(&req) != nil || !json.Valid([]byte(req.Value)) {
		response.BadRequest(c, "画布数据无效")
		return
	}
	// Media bytes and credentials belong in dedicated stores, never in project JSON.
	if strings.Contains(req.Value, "data:image/") || strings.Contains(req.Value, "data:video/") {
		response.BadRequest(c, "请先上传素材再保存画布")
		return
	}
	var revision int64
	err := h.db.QueryRowContext(c.Request.Context(), `INSERT INTO canvas_states(user_id,name,payload,revision) SELECT $1,$2,$3::jsonb,1 WHERE $4=0 ON CONFLICT(user_id,name) DO NOTHING RETURNING revision`, user, name, req.Value, req.Revision).Scan(&revision)
	if errors.Is(err, sql.ErrNoRows) {
		err = h.db.QueryRowContext(c.Request.Context(), `UPDATE canvas_states SET payload=$3::jsonb,revision=revision+1,updated_at=now() WHERE user_id=$1 AND name=$2 AND revision=$4 RETURNING revision`, user, name, req.Value, req.Revision).Scan(&revision)
	}
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusConflict, gin.H{"message": "画布已在其他窗口更新，请刷新后继续"})
		return
	}
	if err != nil {
		canvasFailure(c, err)
		return
	}
	response.Success(c, gin.H{"revision": revision})
}

func (h *CanvasHandler) Upload(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		response.BadRequest(c, "素材标识不能为空")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxBody)
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.BadRequest(c, "素材超过站点上传限制")
		return
	}
	mime := http.DetectContentType(data)
	if !strings.HasPrefix(mime, "image/") && !strings.HasPrefix(mime, "video/") && !strings.HasPrefix(mime, "audio/") {
		response.BadRequest(c, "不支持的素材格式")
		return
	}
	id, err := h.save(c.Request.Context(), canvasUser(c), key, mime, data, 0, "", "", "upload")
	if err != nil {
		canvasFailure(c, err)
		return
	}
	response.Success(c, gin.H{"id": id, "retention_days": 7})
}
func (h *CanvasHandler) ReadStorage(c *gin.Context) {
	var id string
	err := h.db.QueryRowContext(c.Request.Context(), `SELECT asset_id FROM canvas_asset_bindings WHERE user_id=$1 AND storage_key=$2`, canvasUser(c), c.Param("key")).Scan(&id)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	h.content(c, id, false)
}
func (h *CanvasHandler) Content(c *gin.Context)      { h.content(c, c.Param("id"), false) }
func (h *CanvasHandler) AdminContent(c *gin.Context) { h.content(c, c.Param("id"), true) }
func (h *CanvasHandler) content(c *gin.Context, id string, admin bool) {
	if _, err := uuid.Parse(id); err != nil {
		c.Status(404)
		return
	}
	var user int64
	var mime, status string
	var expiry time.Time
	var deleted sql.NullTime
	err := h.db.QueryRowContext(c.Request.Context(), `SELECT user_id,mime_type,review_status,expires_at,deleted_at FROM canvas_assets WHERE id=$1`, id).Scan(&user, &mime, &status, &expiry, &deleted)
	if err != nil || (!admin && user != canvasUser(c)) {
		c.Status(404)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	if deleted.Valid || !time.Now().Before(expiry) {
		c.JSON(http.StatusGone, gin.H{"message": "素材已删除或超过7天保存期限"})
		return
	}
	if !admin && status == "blocked" {
		c.JSON(http.StatusForbidden, gin.H{"message": "该素材已被管理员屏蔽"})
		return
	}
	if admin {
		if _, err = h.db.ExecContext(c.Request.Context(), `INSERT INTO canvas_reviews(asset_id,actor_user_id,action) VALUES($1,$2,'view')`, id, canvasUser(c)); err != nil {
			canvasFailure(c, err)
			return
		}
	}
	c.Header("Content-Type", mime)
	c.File(h.assetPath(id))
}

var canvasGeminiGenerationPath = regexp.MustCompile(`^/gemini/models/([A-Za-z0-9][A-Za-z0-9._-]*):(generateContent|streamGenerateContent)$`)

// Only the explicit hosted media endpoints can be called through the broker.
func canvasGatewayPath(method, path string) bool {
	if method == "POST" {
		return canvasGeminiGenerationPath.MatchString(path) || path == "/responses" || path == "/audio/speech" || path == "/images/generations" || path == "/images/edits" || path == "/videos/generations"
	}
	if method != "GET" {
		return false
	}
	if path == "/models" || path == "/gemini/models" {
		return true
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) >= 2 && len(parts) <= 3 && parts[0] == "videos" && parts[1] != "" && (len(parts) == 2 || parts[2] == "content")
}
func (h *CanvasHandler) Gateway(c *gin.Context) {
	path := c.Param("path")
	if !canvasGatewayPath(c.Request.Method, path) {
		c.Status(404)
		return
	}
	keyID, _ := strconv.ParseInt(c.Param("key_id"), 10, 64)
	key, err := h.keys.GetByID(c.Request.Context(), keyID)
	if err != nil || key.UserID != canvasUser(c) {
		c.Status(404)
		return
	}
	if !key.IsActive() || key.IsExpired() || key.IsQuotaExhausted() {
		c.JSON(403, gin.H{"message": "Key已失效或额度不足，请切换Key"})
		return
	}
	var model, prompt string
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, h.maxBody))
	if err != nil {
		response.BadRequest(c, "请求超过站点限制")
		return
	}
	if c.Request.Method == "POST" {
		info := service.ParseGrokMediaRequest(c.GetHeader("Content-Type"), body)
		model, prompt = info.Model, info.Prompt
		if match := canvasGeminiGenerationPath.FindStringSubmatch(path); match != nil {
			model = match[1]
			var payload struct {
				Contents []struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"contents"`
			}
			if json.Unmarshal(body, &payload) != nil {
				response.BadRequest(c, "Google 请求格式无效")
				return
			}
			var texts []string
			for _, content := range payload.Contents {
				for _, part := range content.Parts {
					if part.Text != "" {
						texts = append(texts, part.Text)
					}
				}
			}
			prompt = strings.Join(texts, "\n")
		}
	}

	if path == "/responses" && service.IsImageGenerationIntent("/v1/responses", model, body) {
		response.BadRequest(c, "请通过生图工作台生成图片，以便归档作品")
		return
	}
	if strings.HasSuffix(path, ":streamGenerateContent") {
		var payload map[string]any
		if json.Unmarshal(body, &payload) != nil {
			response.BadRequest(c, "Google 请求格式无效")
			return
		}
		generation, _ := payload["generationConfig"].(map[string]any)
		if generation == nil {
			generation = map[string]any{}
		}
		// Streaming is used by text nodes. Images use non-streaming generation and private archiving.
		generation["responseModalities"] = []string{"TEXT"}
		payload["generationConfig"] = generation
		body, err = json.Marshal(payload)
		if err != nil {
			canvasFailure(c, err)
			return
		}
	}
	if strings.HasPrefix(path, "/videos/") && c.Request.Method == "GET" {
		task := strings.Split(strings.Trim(path, "/"), "/")[1]
		var exists bool
		err = h.db.QueryRowContext(c.Request.Context(), `SELECT EXISTS(SELECT 1 FROM canvas_video_tasks WHERE user_id=$1 AND api_key_id=$2 AND task_id=$3)`, canvasUser(c), keyID, task).Scan(&exists)
		if err != nil {
			canvasFailure(c, err)
			return
		}
		if !exists {
			c.Status(404)
			return
		}
	}
	req := c.Request.Clone(c.Request.Context())
	copiedURL := *req.URL
	req.URL = &copiedURL
	req.URL.Path = "/v1" + path
	if strings.HasPrefix(path, "/gemini/") {
		req.URL.Path = "/v1beta/" + strings.TrimPrefix(path, "/gemini/")
	}
	req.URL.RawPath = ""
	req.URL.RawQuery = ""
	if strings.HasSuffix(path, ":streamGenerateContent") {
		req.URL.RawQuery = "alt=sse"
	}
	req.RequestURI = req.URL.RequestURI()
	req.Header = c.Request.Header.Clone()
	req.Header.Set("Authorization", "Bearer "+key.Key)
	req.Header.Del("Cookie")
	req.Header.Del("X-Goog-Api-Key")
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	// Use the existing gateway middleware, routing and billing, preserving the real client address.
	recorder := httptest.NewRecorder()
	h.gateway.ServeHTTP(recorder, req)
	result := recorder.Body.Bytes()
	if recorder.Code >= 200 && recorder.Code < 300 && strings.HasPrefix(path, "/images/") {
		uploader := service.NewImageResultUploader(canvasImageSink{h, canvasUser(c), keyID, model, prompt}, "canvas", h.maxBody, nil)
		result, err = uploader.Rewrite(c.Request.Context(), uuid.NewString(), result)
		if err != nil {
			canvasFailure(c, fmt.Errorf("generation succeeded but archive failed: %w", err))
			return
		}
	}
	if recorder.Code >= 200 && recorder.Code < 300 && strings.HasSuffix(path, ":generateContent") && canvasGeminiGenerationPath.MatchString(path) {
		result, err = h.archiveGeminiImages(c.Request.Context(), canvasUser(c), keyID, model, prompt, result)
		if err != nil {
			canvasFailure(c, fmt.Errorf("gemini generation archive failed: %w", err))
			return
		}
	}
	if recorder.Code >= 200 && recorder.Code < 300 && path == "/audio/speech" {
		if _, err = h.save(c.Request.Context(), canvasUser(c), "generated:"+uuid.NewString(), recorder.Header().Get("Content-Type"), result, keyID, model, prompt, "generation"); err != nil {
			canvasFailure(c, err)
			return
		}
	}
	if recorder.Code >= 200 && recorder.Code < 300 && path == "/videos/generations" {
		var v struct {
			ID        string `json:"id"`
			RequestID string `json:"request_id"`
		}
		_ = json.Unmarshal(result, &v)
		if v.ID == "" {
			v.ID = v.RequestID
		}
		if v.ID != "" {
			_, err = h.db.ExecContext(c.Request.Context(), `INSERT INTO canvas_video_tasks(user_id,api_key_id,task_id,model,prompt,remote_addr,user_agent,request_host) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING`, canvasUser(c), keyID, v.ID, model, prompt, c.Request.RemoteAddr, c.Request.UserAgent(), c.Request.Host)
			if err != nil {
				canvasFailure(c, err)
				return
			}
		}
	}
	if recorder.Code >= 200 && recorder.Code < 300 && strings.HasPrefix(path, "/videos/") && strings.HasSuffix(path, "/content") {
		task := strings.Split(strings.Trim(path, "/"), "/")[1]
		err = h.db.QueryRowContext(c.Request.Context(), `SELECT model,prompt FROM canvas_video_tasks WHERE user_id=$1 AND api_key_id=$2 AND task_id=$3`, canvasUser(c), keyID, task).Scan(&model, &prompt)
		if err == nil {
			var id string
			id, err = h.save(c.Request.Context(), canvasUser(c), "generated-video:"+task, recorder.Header().Get("Content-Type"), result, keyID, model, prompt, "generation")
			if err == nil {
				_, err = h.db.ExecContext(c.Request.Context(), `UPDATE canvas_video_tasks SET asset_id=$4,status='completed' WHERE user_id=$1 AND api_key_id=$2 AND task_id=$3`, canvasUser(c), keyID, task, id)
			}
		}
		if err != nil {
			canvasFailure(c, err)
			return
		}
	}
	c.Header("Cache-Control", "no-store")
	c.Data(recorder.Code, recorder.Header().Get("Content-Type"), result)
}

func (h *CanvasHandler) Capabilities(c *gin.Context) {
	all := []gin.H{}
	for page := 1; ; page++ {
		keys, paging, err := h.keys.List(c.Request.Context(), canvasUser(c), pagination.PaginationParams{Page: page, PageSize: 100}, service.APIKeyListFilters{})
		if err != nil {
			canvasFailure(c, err)
			return
		}
		for _, key := range keys {
			if !key.IsActive() || key.IsExpired() || key.IsQuotaExhausted() || key.Group == nil || key.Group.Status != "active" {
				continue
			}
			platform := key.Group.Platform
			capabilities := key.Group.KeyCapabilities()
			formats := []string{}
			if platform == service.PlatformAnthropic || platform == service.PlatformOpenAI || platform == service.PlatformGrok || platform == service.PlatformComposite {
				formats = append(formats, "openai")
			}
			if platform == service.PlatformGemini || platform == service.PlatformAntigravity || platform == service.PlatformComposite {
				formats = append(formats, "gemini")
			}
			if len(formats) > 0 {
				all = append(all, gin.H{"id": key.ID, "name": key.Name, "image": capabilities.Image, "video": capabilities.Video, "text": capabilities.Text, "audio": platform == service.PlatformOpenAI || platform == service.PlatformComposite, "formats": formats})
			}
		}
		if paging == nil || page*100 >= int(paging.Total) {
			break
		}
	}
	response.Success(c, gin.H{"keys": all, "retention_days": 7})
}

type canvasAsset struct {
	ID        string        `json:"id"`
	UserID    int64         `json:"user_id"`
	Email     string        `json:"email"`
	MIME      string        `json:"mime_type"`
	Bytes     int64         `json:"bytes"`
	KeyID     sql.NullInt64 `json:"-"`
	Model     string        `json:"model"`
	Prompt    string        `json:"prompt"`
	Source    string        `json:"source"`
	Review    string        `json:"review_status"`
	Note      string        `json:"review_note"`
	Created   time.Time     `json:"created_at"`
	Expires   time.Time     `json:"expires_at"`
	Deleted   sql.NullTime  `json:"-"`
	Available bool          `json:"available"`
}

func (h *CanvasHandler) AdminList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	user, _ := strconv.ParseInt(c.Query("user_id"), 10, 64)
	status := c.Query("status")
	rows, err := h.db.QueryContext(c.Request.Context(), `SELECT a.id,a.user_id,COALESCE(u.email,''),a.mime_type,a.byte_size,a.api_key_id,a.model,a.prompt,a.source,a.review_status,a.review_note,a.created_at,a.expires_at,a.deleted_at FROM canvas_assets a LEFT JOIN users u ON u.id=a.user_id WHERE ($1=0 OR a.user_id=$1) AND ($2='' OR a.review_status=$2) ORDER BY a.created_at DESC LIMIT 30 OFFSET $3`, user, status, (page-1)*30)
	if err != nil {
		canvasFailure(c, err)
		return
	}
	defer func() { _ = rows.Close() }()
	items := []canvasAsset{}
	for rows.Next() {
		var a canvasAsset
		if err = rows.Scan(&a.ID, &a.UserID, &a.Email, &a.MIME, &a.Bytes, &a.KeyID, &a.Model, &a.Prompt, &a.Source, &a.Review, &a.Note, &a.Created, &a.Expires, &a.Deleted); err != nil {
			canvasFailure(c, err)
			return
		}
		a.Available = !a.Deleted.Valid && time.Now().Before(a.Expires)
		items = append(items, a)
	}
	if err = rows.Err(); err != nil {
		canvasFailure(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "page": page, "has_more": len(items) == 30})
}
func (h *CanvasHandler) Review(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.Status(404)
		return
	}
	var req struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if c.ShouldBindJSON(&req) != nil || (req.Status != "approved" && req.Status != "blocked" && req.Status != "unreviewed") {
		response.BadRequest(c, "审核状态无效")
		return
	}
	if req.Status == "blocked" && strings.TrimSpace(req.Note) == "" {
		response.BadRequest(c, "请填写屏蔽原因")
		return
	}
	tx, err := h.db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		canvasFailure(c, err)
		return
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(c.Request.Context(), `UPDATE canvas_assets SET review_status=$2,review_note=$3 WHERE id=$1 AND deleted_at IS NULL AND expires_at>now()`, id, req.Status, req.Note)
	if err != nil {
		canvasFailure(c, err)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		c.Status(404)
		return
	}
	_, err = tx.ExecContext(c.Request.Context(), `INSERT INTO canvas_reviews(asset_id,actor_user_id,action,note) VALUES($1,$2,$3,$4)`, id, canvasUser(c), req.Status, req.Note)
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		canvasFailure(c, err)
		return
	}
	response.Success(c, gin.H{"status": req.Status})
}

// Access expires at the exact deadline; this worker retries physical deletion after failures.
func (h *CanvasHandler) Cleanup(ctx context.Context) error {
	rows, err := h.db.QueryContext(ctx, `SELECT id FROM canvas_assets WHERE (expires_at<=now() OR deleted_at IS NOT NULL) AND file_deleted_at IS NULL`)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err = os.Remove(h.assetPath(id)); err != nil && !os.IsNotExist(err) {
			return err
		}
		if _, err = h.db.ExecContext(ctx, `UPDATE canvas_assets SET deleted_at=COALESCE(deleted_at,now()),file_deleted_at=now() WHERE id=$1`, id); err != nil {
			return err
		}
	}
	return nil
}
func (h *CanvasHandler) StartCleanup(ctx context.Context) {
	h.StartImageTasks(ctx)
	// A slow video provider must not delay seven-day file deletion.
	for name, job := range map[string]func(context.Context) error{"cleanup": h.Cleanup, "video archiving": h.ProcessVideos} {
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				if err := job(ctx); err != nil {
					log.Printf("canvas %s: %v", name, err)
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	}
}

// Pending video tasks survive navigation and server restarts. Completion still uses the gateway's billing deduplication.
func (h *CanvasHandler) ProcessVideos(ctx context.Context) error {
	rows, err := h.db.QueryContext(ctx, `SELECT user_id,api_key_id,task_id,model,prompt,remote_addr,user_agent,request_host FROM canvas_video_tasks WHERE status='pending' AND created_at>now()-interval '7 days'`)
	if err != nil {
		return err
	}
	type task struct {
		user, key                              int64
		id, model, prompt, remote, agent, host string
	}
	var tasks []task
	for rows.Next() {
		var t task
		if err = rows.Scan(&t.user, &t.key, &t.id, &t.model, &t.prompt, &t.remote, &t.agent, &t.host); err != nil {
			_ = rows.Close()
			return err
		}
		tasks = append(tasks, t)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, t := range tasks {
		if err = ctx.Err(); err != nil {
			return err
		}
		key, e := h.keys.GetByID(ctx, t.key)
		if e != nil || key.UserID != t.user || !key.IsActive() {
			continue
		}
		call := func(path string) *httptest.ResponseRecorder {
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://canvas.internal/v1/videos/"+url.PathEscape(t.id)+path, nil)
			req.RemoteAddr = t.remote
			req.Host = t.host
			req.Header.Set("User-Agent", t.agent)
			req.Header.Set("Authorization", "Bearer "+key.Key)
			w := httptest.NewRecorder()
			h.gateway.ServeHTTP(w, req)
			return w
		}
		w := call("")
		if w.Code != 200 {
			continue
		}
		var state struct {
			Status string `json:"status"`
			Video  struct {
				URL string `json:"url"`
			} `json:"video"`
		}
		if json.Unmarshal(w.Body.Bytes(), &state) != nil {
			continue
		}
		if state.Status == "failed" || state.Status == "expired" || state.Status == "cancelled" {
			_, err = h.db.ExecContext(ctx, `UPDATE canvas_video_tasks SET status='failed' WHERE user_id=$1 AND api_key_id=$2 AND task_id=$3`, t.user, t.key, t.id)
			if err != nil {
				return err
			}
			continue
		}
		if state.Status != "done" && state.Status != "completed" && state.Video.URL == "" {
			continue
		}
		w = call("/content")
		if w.Code != 200 {
			continue
		}
		id, e := h.save(ctx, t.user, "generated-video:"+t.id, w.Header().Get("Content-Type"), w.Body.Bytes(), t.key, t.model, t.prompt, "generation")
		if e != nil {
			return e
		}
		_, err = h.db.ExecContext(ctx, `UPDATE canvas_video_tasks SET asset_id=$4,status='completed' WHERE user_id=$1 AND api_key_id=$2 AND task_id=$3`, t.user, t.key, t.id, id)
		if err != nil {
			return err
		}
	}
	return nil
}

// Archive native Google image parts before returning, preserving text, usage and thought signatures.
func (h *CanvasHandler) archiveGeminiImages(ctx context.Context, user, keyID int64, model, prompt string, body []byte) ([]byte, error) {
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	candidates, _ := result["candidates"].([]any)
	for _, candidate := range candidates {
		item, _ := candidate.(map[string]any)
		content, _ := item["content"].(map[string]any)
		parts, _ := content["parts"].([]any)
		for _, rawPart := range parts {
			part, _ := rawPart.(map[string]any)
			inline, _ := part["inlineData"].(map[string]any)
			if inline == nil {
				inline, _ = part["inline_data"].(map[string]any)
			}
			if inline == nil {
				continue
			}
			mime, _ := inline["mimeType"].(string)
			if mime == "" {
				mime, _ = inline["mime_type"].(string)
			}
			if !strings.HasPrefix(mime, "image/") {
				continue
			}
			encoded, _ := inline["data"].(string)
			bytes, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				return nil, err
			}
			if !strings.HasPrefix(http.DetectContentType(bytes), "image/") {
				return nil, fmt.Errorf("invalid Google image content")
			}
			id, err := h.save(ctx, user, "generated:"+uuid.NewString(), mime, bytes, keyID, model, prompt, "generation")
			if err != nil {
				return nil, err
			}
			delete(part, "inlineData")
			delete(part, "inline_data")
			part["fileData"] = map[string]any{"mimeType": mime, "fileUri": "/api/v1/canvas/assets/" + id + "/content"}
		}
	}
	return json.Marshal(result)
}
