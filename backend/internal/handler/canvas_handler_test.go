package handler

import (
	"context"
	"database/sql"
	"encoding/json"
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
	m.ExpectQuery("SELECT id FROM canvas_assets WHERE").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(id))
	m.ExpectExec("UPDATE canvas_assets SET deleted_at").WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, h.Cleanup(context.Background()))
	_, err := os.Stat(h.assetPath(id))
	require.True(t, os.IsNotExist(err))
	require.NoError(t, m.ExpectationsWereMet())
	require.Equal(t, 168*time.Hour, canvasRetention)
}
func TestCanvasGatewayRejectsArbitraryPaths(t *testing.T) {
	for _, path := range []string{"/admin/users", "/videos/foo/content/extra", "/images/generations/../../admin", "//models"} {
		require.False(t, canvasGatewayPath("POST", path), path)
		require.False(t, canvasGatewayPath("GET", path), path)
	}
	require.True(t, canvasGatewayPath("POST", "/images/edits"))
	require.True(t, canvasGatewayPath("POST", "/videos/generations"))
	require.True(t, canvasGatewayPath("GET", "/videos/task/content"))
}

func TestCanvasGoogleGatewayPaths(t *testing.T) {
	for _, path := range []string{"/gemini/models/gemini-2.5-flash-image:generateContent", "/gemini/models/gemini-3.1-flash-image-preview:generateContent", "/gemini/models/gemini-2.5-flash:streamGenerateContent"} {
		require.True(t, canvasGatewayPath("POST", path), path)
		require.False(t, canvasGatewayPath("GET", path), path)
	}
	require.True(t, canvasGatewayPath("GET", "/gemini/models"))
	for _, path := range []string{"/gemini/models/foo:delete", "/gemini/models/foo/bar:generateContent", "/gemini/models/foo:generateContent/extra", "/gemini/models/../admin:generateContent", "/gemini/cachedContents"} {
		require.False(t, canvasGatewayPath("POST", path), path)
	}
}
func TestCanvasGoogleImagesAreArchivedWithOwnerPromptAndUsage(t *testing.T) {
	h, m, _, _ := canvasTest(t)
	m.ExpectBegin()
	m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	m.ExpectQuery("SELECT a.id FROM canvas_asset_bindings").WillReturnError(sql.ErrNoRows)
	m.ExpectQuery("SELECT id FROM canvas_assets").WillReturnError(sql.ErrNoRows)
	m.ExpectExec("INSERT INTO canvas_assets").WithArgs(sqlmock.AnyArg(), int64(7), sqlmock.AnyArg(), sqlmock.AnyArg(), "image/png", 68, int64(12), "gemini-2.5-flash-image", "gold bird", "generation", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	m.ExpectExec("UPDATE canvas_assets SET source").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec("INSERT INTO canvas_asset_bindings").WillReturnResult(sqlmock.NewResult(1, 1))
	m.ExpectCommit()
	result, err := h.archiveGeminiImages(context.Background(), 7, 12, "gemini-2.5-flash-image", "gold bird", []byte(`{"candidates":[{"content":{"parts":[{"text":"Your image"},{"inlineData":{"mimeType":"image/png","data":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jXzQAAAAASUVORK5CYII="},"thoughtSignature":"keep-me"}]}}],"usageMetadata":{"totalTokenCount":20}}`))
	require.NoError(t, err)
	require.NotContains(t, string(result), "inlineData")
	require.Contains(t, string(result), "thoughtSignature")
	require.Contains(t, string(result), "keep-me")
	require.Contains(t, string(result), "totalTokenCount")
	var data struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					FileData struct {
						FileURI string `json:"fileUri"`
					} `json:"fileData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	require.NoError(t, json.Unmarshal(result, &data))
	uri := data.Candidates[0].Content.Parts[1].FileData.FileURI
	require.True(t, strings.HasPrefix(uri, "/api/v1/canvas/assets/"))
	id := strings.TrimSuffix(strings.TrimPrefix(uri, "/api/v1/canvas/assets/"), "/content")
	_, err = os.Stat(h.assetPath(id))
	require.NoError(t, err)
	require.NoError(t, m.ExpectationsWereMet())
}

func TestCanvasPromptSourcesRequireAdministrator(t *testing.T) {
	for _, role := range []string{"", "user", "admin"} {
		for _, method := range []string{http.MethodGet, http.MethodPut} {
			t.Run(role+method, func(t *testing.T) {
				h, m, c, w := canvasTest(t)
				c.Set(string(middleware.ContextKeyUserRole), role)
				c.Params = gin.Params{{Key: "name", Value: "infinite-canvas:prompt_source_store_v2"}}
				c.Request = httptest.NewRequest(method, "/", strings.NewReader(`{"value":"{\"state\":{}}","revision":0}`))
				c.Request.Header.Set("Content-Type", "application/json")
				if role == "admin" {
					if method == http.MethodGet {
						m.ExpectQuery("SELECT payload,revision FROM canvas_states").WithArgs(int64(7), "infinite-canvas:prompt_source_store_v2").WillReturnError(sql.ErrNoRows)
					} else {
						m.ExpectQuery("INSERT INTO canvas_states").WithArgs(int64(7), "infinite-canvas:prompt_source_store_v2", `{"state":{}}`, int64(0)).WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(1))
					}
				}
				h.State(c)
				if role == "admin" {
					require.Equal(t, 200, w.Code)
				} else {
					require.Equal(t, 403, w.Code)
				}
				require.NoError(t, m.ExpectationsWereMet())
			})
		}
	}
}
