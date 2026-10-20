//go:build unit

package handler

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCanvasTaskGetScopesResultsToAuthenticatedUser(t *testing.T) {
	for _, found := range []bool{true, false} {
		t.Run(map[bool]string{true: "own task", false: "other user or missing task"}[found], func(t *testing.T) {
			h, m, c, w := canvasTest(t)
			id := uuid.NewString()
			c.Params = gin.Params{{Key: "task_id", Value: id}}
			query := m.ExpectQuery("SELECT .* FROM canvas_generation_tasks WHERE id").WithArgs(id, int64(7))
			if found {
				query.WillReturnRows(sqlmock.NewRows([]string{"id", "key", "path", "status", "meta", "code", "result", "error", "created", "updated", "deleted"}).AddRow(id, 12, "/images/generations", "running", []byte(`{"source":"image-workbench"}`), 0, []byte(`null`), "", time.Now(), time.Now(), nil))
			} else {
				query.WillReturnError(sql.ErrNoRows)
			}
			h.GetTask(c)
			c.Writer.WriteHeaderNow()
			if found {
				require.Equal(t, 200, w.Code)
				require.Contains(t, w.Body.String(), "running")
			} else {
				require.Equal(t, 404, w.Code)
			}
			require.NoError(t, m.ExpectationsWereMet())
		})
	}
}
func TestCanvasImageQueueEmptyDoesNotExecute(t *testing.T) {
	h, m, _, _ := canvasTest(t)
	m.ExpectQuery("UPDATE canvas_generation_tasks SET status='running'").WillReturnError(sql.ErrNoRows)
	worked, err := h.ProcessImageTask(context.Background())
	require.NoError(t, err)
	require.False(t, worked)
	require.NoError(t, m.ExpectationsWereMet())
}
func TestCanvasImageTaskPathOnlyAllowsArchivedImageOperations(t *testing.T) {
	for _, p := range []string{"/images/generations", "/images/edits", "/gemini/models/gemini-3.1-flash-image-preview:generateContent"} {
		require.True(t, canvasTaskPath(p))
	}
	for _, p := range []string{"/responses", "/admin/users", "/videos/generations", "/gemini/models/x:streamGenerateContent", "/images/generations/async", "/images/edits/../admin"} {
		require.False(t, canvasTaskPath(p))
	}
}
func TestCanvasAdminDeleteAuditsAndRemovesPrivateFile(t *testing.T) {
	h, m, c, w := canvasTest(t)
	id := uuid.NewString()
	require.NoError(t, os.WriteFile(h.assetPath(id), []byte("private"), 0600))
	c.Params = gin.Params{{Key: "id", Value: id}}
	c.Request.Method = http.MethodDelete
	c.Request.Body = http.NoBody
	c.Request.Body = ioBody(`{"reason":"违规素材处理"}`)
	c.Request.Header.Set("Content-Type", "application/json")
	m.ExpectBegin()
	m.ExpectQuery("SELECT user_id,content_hash").WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"user", "hash"}).AddRow(8, "hash"))
	m.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs("8:hash").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT deleted_at").WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"deleted"}).AddRow(nil))
	m.ExpectExec("UPDATE canvas_assets SET deleted_at=now").WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec("INSERT INTO canvas_reviews").WithArgs(id, int64(7), "违规素材处理").WillReturnResult(sqlmock.NewResult(1, 1))
	m.ExpectCommit()
	m.ExpectExec("UPDATE canvas_assets SET file_deleted_at").WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
	h.DeleteAsset(c)
	require.Equal(t, 200, w.Code)
	_, err := os.Stat(h.assetPath(id))
	require.True(t, os.IsNotExist(err))
	require.NoError(t, m.ExpectationsWereMet())
}
func TestCanvasAdminDeleteRequiresReason(t *testing.T) {
	h, m, c, w := canvasTest(t)
	c.Params = gin.Params{{Key: "id", Value: uuid.NewString()}}
	c.Request.Body = ioBody(`{"reason":" "}`)
	c.Request.Header.Set("Content-Type", "application/json")
	h.DeleteAsset(c)
	require.Equal(t, 400, w.Code)
	require.NoError(t, m.ExpectationsWereMet())
}

func ioBody(value string) io.ReadCloser { return io.NopCloser(strings.NewReader(value)) }

func TestCanvasDeletedAssetCannotBeReuploadedFromCachedCopy(t *testing.T) {
	h, m, _, _ := canvasTest(t)
	m.ExpectBegin()
	m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"deleted"}).AddRow(true))
	m.ExpectRollback()
	_, err := h.save(context.Background(), 7, "new-cache-key", "image/png", []byte("deleted photo"), 0, "", "", "upload")
	require.ErrorIs(t, err, errCanvasAssetDeleted)
	require.NoError(t, m.ExpectationsWereMet())
}
func TestCanvasDeletedContentUnavailableEvenToAdministrator(t *testing.T) {
	for _, admin := range []bool{false, true} {
		h, m, c, w := canvasTest(t)
		id := uuid.NewString()
		m.ExpectQuery("SELECT user_id,mime_type").WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"user", "mime", "status", "expiry", "deleted"}).AddRow(7, "image/png", "approved", time.Now().Add(time.Hour), time.Now()))
		h.content(c, id, admin)
		require.Equal(t, 410, w.Code)
		require.NoError(t, m.ExpectationsWereMet())
	}
}

func TestCanvasTaskDeleteRequiresOwnerAndTerminalState(t *testing.T) {
	for _, tc := range []struct {
		name    string
		pending bool
		missing bool
		want    int
	}{
		{"completed", false, false, 200}, {"running", true, false, 409}, {"other user", false, true, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, m, c, w := canvasTest(t)
			id := uuid.NewString()
			c.Params = gin.Params{{Key: "task_id", Value: id}}
			query := m.ExpectQuery("UPDATE canvas_generation_tasks SET deleted_at=").WithArgs(id, int64(7))
			if tc.missing {
				query.WillReturnError(sql.ErrNoRows)
			} else {
				query.WillReturnRows(sqlmock.NewRows([]string{"pending"}).AddRow(tc.pending))
			}
			h.DeleteTask(c)
			c.Writer.WriteHeaderNow()
			require.Equal(t, tc.want, w.Code)
			require.NoError(t, m.ExpectationsWereMet())
		})
	}
}

func TestCanvasTaskCancellationCannotCancelAnotherUsersWorker(t *testing.T) {
	for _, affected := range []int64{0, 1} {
		t.Run(map[int64]string{0: "not owner", 1: "owner"}[affected], func(t *testing.T) {
			h, m, c, _ := canvasTest(t)
			id := uuid.NewString()
			c.Params = gin.Params{{Key: "task_id", Value: id}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			h.taskCancels.Store(id, cancel)
			m.ExpectExec("UPDATE canvas_generation_tasks SET cancel_requested=true").WithArgs(id, int64(7)).WillReturnResult(sqlmock.NewResult(0, affected))
			m.ExpectQuery("SELECT .* FROM canvas_generation_tasks WHERE id").WithArgs(id, int64(7)).WillReturnError(sql.ErrNoRows)
			h.CancelTask(c)
			require.Equal(t, affected == 1, ctx.Err() != nil)
			require.NoError(t, m.ExpectationsWereMet())
		})
	}
}
