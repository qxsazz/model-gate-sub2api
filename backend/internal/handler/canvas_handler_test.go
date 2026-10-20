package handler

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func canvasTest(t *testing.T) (*CanvasHandler, sqlmock.Sqlmock, *gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return &CanvasHandler{db: db, dir: t.TempDir(), maxBody: 256 << 20}, mock, c, w
}
func TestCanvasContentOwnershipExpiryAndReview(t *testing.T) {
	for _, tt := range []struct {
		name   string
		owner  int64
		expiry time.Time
		status string
		want   int
	}{
		{"other user", 8, time.Now().Add(time.Hour), "unreviewed", 404},
		{"expired", 7, time.Now().Add(-time.Second), "unreviewed", 410},
		{"blocked", 7, time.Now().Add(time.Hour), "blocked", 403},
		{"own content", 7, time.Now().Add(time.Hour), "unreviewed", 200},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, m, c, w := canvasTest(t)
			id := uuid.NewString()
			require.NoError(t, os.WriteFile(h.assetPath(id), []byte("private image"), 0600))
			m.ExpectQuery("SELECT user_id,mime_type").WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"user_id", "mime", "status", "expiry", "deleted"}).AddRow(tt.owner, "image/png", tt.status, tt.expiry, nil))
			h.content(c, id, false)
			c.Writer.WriteHeaderNow()
			require.Equal(t, tt.want, w.Code)
			if tt.want != 200 {
				require.NotContains(t, w.Body.String(), "private image")
			}
			require.NoError(t, m.ExpectationsWereMet())
		})
	}
}
func TestCanvasStateUsesAuthenticatedOwnerAndRejectsStaleRevision(t *testing.T) {
	h, m, c, w := canvasTest(t)
	c.Params = gin.Params{{Key: "name", Value: "infinite-canvas:canvas_store"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"value":"{\"state\":{}}","revision":2}`))
	c.Request.Header.Set("Content-Type", "application/json")
	m.ExpectQuery("INSERT INTO canvas_states").WithArgs(int64(7), "infinite-canvas:canvas_store", `{"state":{}}`, int64(2)).WillReturnError(sql.ErrNoRows)
	m.ExpectQuery("UPDATE canvas_states").WithArgs(int64(7), "infinite-canvas:canvas_store", `{"state":{}}`, int64(2)).WillReturnError(sql.ErrNoRows)
	h.State(c)
	require.Equal(t, 409, w.Code)
	require.NoError(t, m.ExpectationsWereMet())
}
func TestCanvasStateDoesNotStoreInlineImages(t *testing.T) {
	h, m, c, w := canvasTest(t)
	c.Params = gin.Params{{Key: "name", Value: "infinite-canvas:canvas_store"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"value":"{\"content\":\"data:image/png;base64,eA==\"}","revision":0}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.State(c)
	require.Equal(t, 400, w.Code)
	require.NoError(t, m.ExpectationsWereMet())
}
func TestCanvasAdminReadIsAudited(t *testing.T) {
	h, m, c, w := canvasTest(t)
	id := uuid.NewString()
	require.NoError(t, os.WriteFile(h.assetPath(id), []byte("private"), 0600))
	m.ExpectQuery("SELECT user_id,mime_type").WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"user", "mime", "status", "expiry", "deleted"}).AddRow(8, "image/png", "blocked", time.Now().Add(time.Hour), nil))
	m.ExpectExec("INSERT INTO canvas_reviews").WithArgs(id, int64(7)).WillReturnResult(sqlmock.NewResult(1, 1))
	h.content(c, id, true)
	require.Equal(t, 200, w.Code)
	require.NoError(t, m.ExpectationsWereMet())
}
func TestCanvasCleanupDeletesExpiredFilesAndMarksMetadata(t *testing.T) {
	h, m, _, _ := canvasTest(t)
	id := uuid.NewString()
	require.NoError(t, os.WriteFile(h.assetPath(id), []byte("expired"), 0600))
	m.ExpectQuery("SELECT id FROM canvas_assets WHERE expires_at").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(id))
	m.ExpectExec("UPDATE canvas_assets SET deleted_at").WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, h.Cleanup(context.Background()))
	_, err := os.Stat(h.assetPath(id))
	require.True(t, os.IsNotExist(err))
	require.NoError(t, m.ExpectationsWereMet())
	require.Equal(t, 168*time.Hour, canvasRetention)
}
func TestCanvasGatewayRejectsArbitraryPaths(t *testing.T) {
	for _, path := range []string{"/responses", "/admin/users", "/videos/foo/content/extra", "/images/generations/../../admin", "//models"} {
		require.False(t, canvasGatewayPath("POST", path), path)
		require.False(t, canvasGatewayPath("GET", path), path)
	}
	require.True(t, canvasGatewayPath("POST", "/images/edits"))
	require.True(t, canvasGatewayPath("POST", "/videos/generations"))
	require.True(t, canvasGatewayPath("GET", "/videos/task/content"))
}
