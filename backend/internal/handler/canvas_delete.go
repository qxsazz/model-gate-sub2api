package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Mark unavailable and audit atomically before attempting physical deletion.
// The cleanup worker retries removal without ever reopening access.
func (h *CanvasHandler) DeleteAsset(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.Status(404)
		return
	}
	var input struct {
		Reason string `json:"reason" binding:"required,max=2000"`
	}
	if c.ShouldBindJSON(&input) != nil || strings.TrimSpace(input.Reason) == "" {
		response.BadRequest(c, "请填写删除原因")
		return
	}
	tx, err := h.db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		canvasFailure(c, err)
		return
	}
	defer func() { _ = tx.Rollback() }()
	var owner int64
	var hash string
	err = tx.QueryRowContext(c.Request.Context(), `SELECT user_id,content_hash FROM canvas_assets WHERE id=$1`, id).Scan(&owner, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		c.Status(404)
		return
	}
	if err != nil {
		canvasFailure(c, err)
		return
	}
	if _, err = tx.ExecContext(c.Request.Context(), `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("%d:%s", owner, hash)); err != nil {
		canvasFailure(c, err)
		return
	}
	var deleted sql.NullTime
	err = tx.QueryRowContext(c.Request.Context(), `SELECT deleted_at FROM canvas_assets WHERE id=$1 FOR UPDATE`, id).Scan(&deleted)
	if errors.Is(err, sql.ErrNoRows) {
		c.Status(404)
		return
	}
	if err != nil {
		canvasFailure(c, err)
		return
	}
	if !deleted.Valid {
		_, err = tx.ExecContext(c.Request.Context(), `UPDATE canvas_assets SET deleted_at=now() WHERE id=$1`, id)
		if err == nil {
			_, err = tx.ExecContext(c.Request.Context(), `INSERT INTO canvas_reviews(asset_id,actor_user_id,action,note) VALUES($1,$2,'delete',$3)`, id, canvasUser(c), strings.TrimSpace(input.Reason))
		}
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		canvasFailure(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pending := false
	if err = os.Remove(h.assetPath(id)); err != nil && !os.IsNotExist(err) {
		pending = true
		log.Printf("canvas asset deletion deferred: %v", err)
	} else if _, err = h.db.ExecContext(ctx, `UPDATE canvas_assets SET file_deleted_at=now() WHERE id=$1`, id); err != nil {
		pending = true
		log.Printf("canvas asset deletion bookkeeping: %v", err)
	}
	response.Success(c, gin.H{"deleted": true, "file_cleanup_pending": pending})
}
