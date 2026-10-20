package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const canvasImageTaskTimeout = 6 * time.Minute

type canvasTask struct {
	DeletedAt      *time.Time      `json:"deleted_at,omitempty"`
	ID             string          `json:"id"`
	KeyID          int64           `json:"key_id"`
	Path           string          `json:"path"`
	Status         string          `json:"status"`
	Metadata       json.RawMessage `json:"metadata"`
	ResponseStatus int             `json:"response_status"`
	Result         json.RawMessage `json:"result"`
	Error          string          `json:"error"`
	Created        time.Time       `json:"created_at"`
	Updated        time.Time       `json:"updated_at"`
}

const canvasTaskColumns = `id,api_key_id,path,status,metadata,response_status,COALESCE(response_body,'null'::jsonb),error,created_at,updated_at,deleted_at`

func scanCanvasTask(row interface{ Scan(...any) error }) (canvasTask, error) {
	var t canvasTask
	err := row.Scan(&t.ID, &t.KeyID, &t.Path, &t.Status, &t.Metadata, &t.ResponseStatus, &t.Result, &t.Error, &t.Created, &t.Updated, &t.DeletedAt)
	return t, err
}
func canvasTaskPath(path string) bool {
	return path == "/images/generations" || path == "/images/edits" || (canvasGeminiGenerationPath.MatchString(path) && strings.HasSuffix(path, ":generateContent"))
}

// A caller-supplied UUID makes retries after a lost acknowledgement idempotent.
func (h *CanvasHandler) SubmitTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("task_id"))
	if err != nil || !canvasTaskPath(c.Param("path")) {
		response.BadRequest(c, "无效的生图任务")
		return
	}
	keyID, err := parseCanvasKeyID(c.Param("key_id"))
	if err != nil {
		response.BadRequest(c, "无效的 Key")
		return
	}
	key, err := h.keys.GetByID(c.Request.Context(), keyID)
	if err != nil || key.UserID != canvasUser(c) {
		c.Status(404)
		return
	}
	if !key.IsActive() || key.IsExpired() || key.IsQuotaExhausted() || key.Group == nil || !key.Group.KeyCapabilities().Image {
		response.Forbidden(c, "该 Key 不支持生图或已失效")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, h.maxBody))
	if err != nil || len(body) == 0 {
		response.BadRequest(c, "生图请求为空或超过站点限制")
		return
	}
	if asyncImageRequestStreams(c.GetHeader("Content-Type"), body) {
		response.BadRequest(c, "生图任务不支持流式响应")
		return
	}
	meta := json.RawMessage(`{}`)
	if header := c.GetHeader("X-Canvas-Task-Meta"); header != "" {
		decoded, e := url.QueryUnescape(header)
		if e != nil || len(decoded) > 32768 || !json.Valid([]byte(decoded)) {
			response.BadRequest(c, "无效的任务信息")
			return
		}
		meta = []byte(decoded)
	}
	var details map[string]any
	if json.Unmarshal(meta, &details) != nil || details == nil {
		response.BadRequest(c, "无效的任务信息")
		return
	}
	info := service.ParseGrokMediaRequest(c.GetHeader("Content-Type"), body)
	details["model"] = info.Model
	details["prompt"] = info.Prompt
	if match := canvasGeminiGenerationPath.FindStringSubmatch(c.Param("path")); match != nil {
		details["model"] = match[1]
		var payload struct {
			Contents []struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"contents"`
		}
		if json.Unmarshal(body, &payload) != nil {
			response.BadRequest(c, "无效的 Google 请求")
			return
		}
		var texts []string
		for _, v := range payload.Contents {
			for _, p := range v.Parts {
				if p.Text != "" {
					texts = append(texts, p.Text)
				}
			}
		}
		details["prompt"] = strings.Join(texts, "\n")
	}
	if prompt, _ := details["prompt"].(string); len(prompt) > 65536 {
		response.BadRequest(c, "提示词过长，请缩短后重试")
		return
	}
	meta, err = json.Marshal(details)
	if err != nil {
		response.BadRequest(c, "无效的任务信息")
		return
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(append([]byte(fmt.Sprintf("%d:%s:%s:", keyID, c.Param("path"), c.ContentType())), body...)))
	submitCtx, stopSubmit := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 10*time.Second)
	defer stopSubmit()
	tx, err := h.db.BeginTx(submitCtx, nil)
	if err != nil {
		canvasFailure(c, err)
		return
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(submitCtx, `SELECT pg_advisory_xact_lock($1)`, canvasUser(c)); err != nil {
		canvasFailure(c, err)
		return
	}
	var existingHash string
	err = tx.QueryRowContext(submitCtx, `SELECT request_hash FROM canvas_generation_tasks WHERE id=$1 AND user_id=$2`, id.String(), canvasUser(c)).Scan(&existingHash)
	if err == nil {
		if existingHash != hash {
			response.BadRequest(c, "任务编号已用于其他请求")
			return
		}
		_ = tx.Rollback()
		h.GetTask(c)
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		canvasFailure(c, err)
		return
	}
	var count int
	if err = tx.QueryRowContext(submitCtx, `SELECT count(*) FROM canvas_generation_tasks WHERE user_id=$1 AND status IN ('queued','running')`, canvasUser(c)).Scan(&count); err != nil {
		canvasFailure(c, err)
		return
	}
	if count >= 20 {
		c.JSON(http.StatusTooManyRequests, gin.H{"message": "进行中的生图任务过多，请等待部分任务完成"})
		return
	}
	_, err = tx.ExecContext(submitCtx, `INSERT INTO canvas_generation_tasks(id,user_id,api_key_id,path,content_type,request_body,request_hash,metadata,remote_addr,user_agent,request_host) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, id.String(), canvasUser(c), keyID, c.Param("path"), c.GetHeader("Content-Type"), body, hash, []byte(meta), net.JoinHostPort(c.ClientIP(), "0"), c.Request.UserAgent(), c.Request.Host)
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		canvasFailure(c, err)
		return
	}
	h.GetTask(c)
}
func parseCanvasKeyID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err == nil && id <= 0 {
		err = errors.New("invalid key id")
	}
	return id, err
}

func (h *CanvasHandler) GetTask(c *gin.Context) {
	if _, err := uuid.Parse(c.Param("task_id")); err != nil {
		c.Status(404)
		return
	}
	task, err := scanCanvasTask(h.db.QueryRowContext(c.Request.Context(), `SELECT `+canvasTaskColumns+` FROM canvas_generation_tasks WHERE id=$1 AND user_id=$2 AND expires_at>now()`, c.Param("task_id"), canvasUser(c)))
	if errors.Is(err, sql.ErrNoRows) {
		c.Status(404)
		return
	}
	if err != nil {
		canvasFailure(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, task)
}
func (h *CanvasHandler) ListTasks(c *gin.Context) {
	rows, err := h.db.QueryContext(c.Request.Context(), `SELECT `+canvasTaskColumns+` FROM canvas_generation_tasks WHERE user_id=$1 AND expires_at>now() ORDER BY (status IN ('queued','running')) DESC,(deleted_at IS NULL) DESC,created_at DESC LIMIT 220`, canvasUser(c))
	if err != nil {
		canvasFailure(c, err)
		return
	}
	defer func() { _ = rows.Close() }()
	tasks := []canvasTask{}
	for rows.Next() {
		task, e := scanCanvasTask(rows)
		if e != nil {
			canvasFailure(c, e)
			return
		}
		tasks = append(tasks, task)
	}
	if err = rows.Err(); err != nil {
		canvasFailure(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, gin.H{"tasks": tasks})
}

// Each queued request is claimed once. Stale running requests are never resubmitted:
// synchronous providers cannot tell us whether a disconnected attempt was billed.
func (h *CanvasHandler) ProcessImageTask(ctx context.Context) (bool, error) {
	var id, path, ct, remote, agent, host string
	var user, keyID int64
	var body []byte
	err := h.db.QueryRowContext(ctx, `UPDATE canvas_generation_tasks SET status='running',updated_at=now() WHERE id=(SELECT id FROM canvas_generation_tasks WHERE status='queued' AND expires_at>now() ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1) AND status='queued' RETURNING id,user_id,api_key_id,path,content_type,request_body,remote_addr,user_agent,request_host`).Scan(&id, &user, &keyID, &path, &ct, &body, &remote, &agent, &host)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	h.executeImageTask(ctx, id, user, keyID, path, ct, body, remote, agent, host)
	return true, nil
}
func (h *CanvasHandler) executeImageTask(parent context.Context, id string, user, keyID int64, path, ct string, body []byte, remote, agent, host string) {
	ctx, cancel := context.WithTimeout(context.WithValue(parent, ctxkey.ManagedImageTask, true), canvasImageTaskTimeout)
	defer cancel()
	h.taskCancels.Store(id, cancel)
	defer h.taskCancels.Delete(id)
	stopped := make(chan struct{})
	defer close(stopped)
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopped:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				var cancelled bool
				e := h.db.QueryRowContext(ctx, `UPDATE canvas_generation_tasks SET updated_at=now() WHERE id=$1 AND status='running' RETURNING cancel_requested`, id).Scan(&cancelled)
				if e != nil {
					log.Printf("canvas task heartbeat: %v", e)
				}
				if cancelled {
					cancel()
					return
				}

			}
		}
	}()
	status := "failed"
	code := 500
	result := []byte(`null`)
	message := "任务执行失败，请查看记录后重试"
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("canvas task execution panic: %v", recovered)
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			status = "failed"
			code = http.StatusGatewayTimeout
			message = "生图已超过6分钟，本站已终止上游连接。请确认生成记录后手动重试，上游可能已计费。"
			result, _ = json.Marshal(gin.H{"message": message})
		} else if ctx.Err() != nil {
			code = http.StatusServiceUnavailable
			result = []byte(`null`)
			status = "interrupted"
			message = "任务已中断，结果待确认，请查看生成记录后再重试"
		}
		saveCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if _, e := h.db.ExecContext(saveCtx, `UPDATE canvas_generation_tasks SET status=$2,response_status=$3,response_body=$4,error=$5,request_body=NULL,updated_at=now() WHERE id=$1 AND status='running'`, id, status, code, result, message); e != nil {
			log.Printf("canvas task completion: %v", e)
		}
	}()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "http://canvas.internal/api/v1/canvas/gateway", bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("User-Agent", agent)
	req.RemoteAddr = remote
	req.Host = host
	c.Request = req
	c.Params = gin.Params{{Key: "key_id", Value: fmt.Sprint(keyID)}, {Key: "path", Value: path}}
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: user})
	h.Gateway(c)
	code = rec.Code
	if json.Valid(rec.Body.Bytes()) {
		result = rec.Body.Bytes()
	}
	if code >= 200 && code < 300 {
		status = "completed"
		message = ""
	} else {
		message = "生图失败，请查看错误信息后重试"
	}
}
func (h *CanvasHandler) StartImageTasks(ctx context.Context) {
	for i := 0; i < 4; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				worked, err := h.ProcessImageTask(ctx)
				if err != nil {
					log.Printf("canvas image task: %v", err)
				}
				if !worked {
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Second):
					}
				}
			}
		}()
	}
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, err := h.db.ExecContext(ctx, `UPDATE canvas_generation_tasks SET status='interrupted',error='任务已中断，结果待确认，请查看生成记录后再重试',request_body=NULL,updated_at=now() WHERE status='running' AND updated_at<now()-interval '2 minutes'`)
				if err != nil {
					log.Printf("canvas task recovery: %v", err)
				}
				_, err = h.db.ExecContext(ctx, `DELETE FROM canvas_generation_tasks WHERE expires_at<=now() AND status NOT IN ('running')`)
				if err != nil {
					log.Printf("canvas task cleanup: %v", err)
				}
			}
		}
	}()
}

func (h *CanvasHandler) CancelTask(c *gin.Context) {
	if _, err := uuid.Parse(c.Param("task_id")); err != nil {
		c.Status(404)
		return
	}
	changed, err := h.db.ExecContext(c.Request.Context(), `UPDATE canvas_generation_tasks SET cancel_requested=true,status=CASE WHEN status='queued' THEN 'failed' ELSE status END,error=CASE WHEN status='queued' THEN '任务已取消' ELSE error END,request_body=CASE WHEN status='queued' THEN NULL ELSE request_body END,updated_at=now() WHERE id=$1 AND user_id=$2 AND status IN ('queued','running')`, c.Param("task_id"), canvasUser(c))
	if err != nil {
		canvasFailure(c, err)
		return
	}
	if count, _ := changed.RowsAffected(); count > 0 {
		if cancel, ok := h.taskCancels.Load(c.Param("task_id")); ok {
			cancel.(context.CancelFunc)()
		}
	}
	h.GetTask(c)
}

// Hide terminal tasks while retaining idempotency and recovery receipts until normal expiry.
// Active tasks must be stopped before deletion so removal cannot orphan an upstream request.
func (h *CanvasHandler) DeleteTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("task_id"))
	if err != nil {
		c.Status(404)
		return
	}
	var pending bool
	err = h.db.QueryRowContext(c.Request.Context(), `UPDATE canvas_generation_tasks SET deleted_at=CASE WHEN status IN ('queued','running') THEN deleted_at ELSE COALESCE(deleted_at,now()) END WHERE id=$1 AND user_id=$2 AND expires_at>now() RETURNING status IN ('queued','running')`, id.String(), canvasUser(c)).Scan(&pending)
	if errors.Is(err, sql.ErrNoRows) {
		c.Status(404)
		return
	}
	if err != nil {
		canvasFailure(c, err)
		return
	}
	if pending {
		c.JSON(409, gin.H{"message": "请先停止任务，待状态更新后再删除"})
		return
	}
	response.Success(c, gin.H{"deleted": true})
}
