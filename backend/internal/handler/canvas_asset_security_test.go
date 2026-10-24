//go:build unit

package handler

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCanvasPrivateContentCannotServeActiveDocumentsInline(t *testing.T) {
	for _, mime := range []string{"text/html; charset=utf-8", "image/svg+xml", "IMAGE/SVG+XML; charset=utf-8", "application/xhtml+xml", "invalid media type"} {
		t.Run(mime, func(t *testing.T) {
			h, db, c, w := canvasTest(t)
			id := uuid.NewString()
			require.NoError(t, os.WriteFile(h.assetPath(id), []byte(`<svg onload="alert(1)"></svg>`), 0600))
			db.ExpectQuery("SELECT user_id,mime_type").WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"user", "mime", "status", "expiry", "deleted"}).AddRow(7, mime, "allowed", time.Now().Add(time.Hour), nil))
			h.content(c, id, false)
			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
			require.Contains(t, w.Header().Get("Content-Disposition"), "attachment;")
			require.Contains(t, w.Header().Get("Content-Security-Policy"), "sandbox")
			require.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
			require.NoError(t, db.ExpectationsWereMet())
		})
	}
}

func TestCanvasNormalMediaKeepsInlineContentType(t *testing.T) {
	for _, mime := range []string{"image/png", "video/mp4", "audio/mpeg"} {
		t.Run(mime, func(t *testing.T) {
			h, db, c, w := canvasTest(t)
			id := uuid.NewString()
			require.NoError(t, os.WriteFile(h.assetPath(id), []byte("media"), 0600))
			db.ExpectQuery("SELECT user_id,mime_type").WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"user", "mime", "status", "expiry", "deleted"}).AddRow(7, mime, "allowed", time.Now().Add(time.Hour), nil))
			h.content(c, id, false)
			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, mime, w.Header().Get("Content-Type"))
			require.Empty(t, w.Header().Get("Content-Disposition"))
			require.NoError(t, db.ExpectationsWereMet())
		})
	}
}
