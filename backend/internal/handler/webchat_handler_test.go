package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const testWebchatSession = "c68c5d90-cf48-4cc1-a398-fe632f06fd6e"

type webchatKeyStub struct {
	key   *service.APIKey
	pages [][]service.APIKey
}

func (s *webchatKeyStub) GetByID(_ context.Context, id int64) (*service.APIKey, error) {
	if s.key == nil || s.key.ID != id {
		return nil, service.ErrAPIKeyNotFound
	}
	return s.key, nil
}

func (s *webchatKeyStub) List(_ context.Context, _ int64, params pagination.PaginationParams, _ service.APIKeyListFilters) ([]service.APIKey, *pagination.PaginationResult, error) {
	return s.pages[params.Page-1], &pagination.PaginationResult{Pages: len(s.pages)}, nil
}

func validWebchatTestKey() *service.APIKey {
	return &service.APIKey{ID: 5, UserID: 41, Key: "fixture-owned-credential", Name: "我的密钥", Status: service.StatusActive}
}

func webchatTestRequest(t *testing.T, h *WebchatHandler, userID int64, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	engine := gin.New()
	if userID > 0 {
		engine.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID}) })
	}
	engine.GET("/keys", h.Keys)
	engine.GET("/models", h.Models)
	engine.GET("/sessions", h.Sessions)
	engine.GET("/sessions/:id", h.Session)
	engine.GET("/sessions/:id/attachments/:attachment_id", h.GetAttachment)
	engine.DELETE("/sessions/:id", h.DeleteSession)
	engine.POST("/messages", h.SendMessage)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "192.0.2.5:443"
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func TestWebchatKeyListOnlyExposesEligibleLabels(t *testing.T) {
	key := validWebchatTestKey()
	past := time.Now().Add(-time.Hour)
	stub := &webchatKeyStub{pages: [][]service.APIKey{{
		{ID: 7, UserID: 42, Key: "someone-else", Status: service.StatusActive},
		{ID: 8, UserID: 41, Key: "expired", Status: service.StatusActive, ExpiresAt: &past},
		{ID: 9, UserID: 41, Key: "disabled", Status: service.StatusAPIKeyDisabled},
		{ID: 10, UserID: 41, Key: "no-quota", Status: service.StatusActive, Quota: 1, QuotaUsed: 1},
		{ID: 11, UserID: 41, Key: "denied-ip", Status: service.StatusActive, IPBlacklist: []string{"192.0.2.5"}},
	}, {*key}}}
	w := webchatTestRequest(t, &WebchatHandler{keys: stub}, 41, http.MethodGet, "/keys", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"code":0,"message":"success","data":{"items":[{"id":5,"name":"我的密钥"}]}}`, w.Body.String())
	require.Contains(t, w.Header().Get("Cache-Control"), "no-store")
}

func TestWebchatGatesEverySessionAndGatewayRoute(t *testing.T) {
	key := validWebchatTestKey()
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/models?key_id=5", ""},
		{http.MethodGet, "/sessions?key_id=5", ""},
		{http.MethodGet, "/sessions/" + testWebchatSession + "?key_id=5", ""},
		{http.MethodGet, "/sessions/" + testWebchatSession + "/attachments/" + testWebchatFile + "?key_id=5", ""},
		{http.MethodDelete, "/sessions/" + testWebchatSession + "?key_id=5", ""},
		{http.MethodPost, "/messages", `{"key_id":5,"model":"m","content":"hello","user_id":41}`},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			h := &WebchatHandler{keys: &webchatKeyStub{key: key}}
			w := webchatTestRequest(t, h, 42, tc.method, tc.path, tc.body)
			require.Equal(t, http.StatusForbidden, w.Code)
			require.Contains(t, w.Body.String(), "WEBCHAT_KEY_UNAVAILABLE")
			h.keys = &webchatKeyStub{}
			w = webchatTestRequest(t, h, 41, tc.method, tc.path, tc.body)
			require.Equal(t, http.StatusForbidden, w.Code)
			require.Contains(t, w.Body.String(), "WEBCHAT_KEY_UNAVAILABLE")
			w = webchatTestRequest(t, h, 0, tc.method, tc.path, tc.body)
			require.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

type webchatTurnStore struct {
	webchatStore // Any unexpected call fails the test.
	history      []service.WebchatMessage
	beginErr     error
	began        []any
	saved        []any
	files        []service.WebchatAttachment
	released     bool
}

func (s *webchatTurnStore) ReserveStart(_ context.Context, user, key int64) (string, error) {
	s.began = []any{user, key}
	if s.beginErr != nil {
		return "", s.beginErr
	}
	return "start-lease", nil
}

func (s *webchatTurnStore) ReleaseStart(_ context.Context, _, _ int64, _ string) error {
	s.released = true
	return nil
}

func (s *webchatTurnStore) BeginTurn(_ context.Context, user, key int64, id string) (*service.WebchatConversation, string, error) {
	s.began = []any{user, key, id}
	if s.beginErr != nil {
		return nil, "", s.beginErr
	}
	return &service.WebchatConversation{Messages: s.history, Version: 1}, "turn-lease", nil
}
func (s *webchatTurnStore) SaveTurn(_ context.Context, user, key int64, id string, version int64, token, model, content, answer string, files ...service.WebchatAttachment) (*service.WebchatConversation, error) {
	s.files = files
	s.saved = []any{user, key, id, version, token, model, content, answer}
	return &service.WebchatConversation{ID: id, Version: version + 1, Messages: []service.WebchatMessage{{Role: "user", Content: content, Attachments: files}}}, nil
}
func (s *webchatTurnStore) ReleaseTurn(_ context.Context, _, _ int64, _, _ string) error {
	s.released = true
	return nil
}

type webchatBridgeStub struct {
	webchatBridge
	key      *service.APIKey
	clientIP string
	messages []service.WebchatMessage
	err      error
}

func (b *webchatBridgeStub) Complete(_ context.Context, key *service.APIKey, clientIP, _ string, messages []service.WebchatMessage) (string, error) {
	b.key, b.clientIP, b.messages = key, clientIP, messages
	return "next answer", b.err
}

func TestWebchatSendUsesOwnedKeyAndServerStoredContext(t *testing.T) {
	store := &webchatTurnStore{history: []service.WebchatMessage{{Role: "user", Content: "earlier"}, {Role: "assistant", Content: "earlier answer"}}}
	bridge := &webchatBridgeStub{}
	h := &WebchatHandler{keys: &webchatKeyStub{key: validWebchatTestKey()}, store: store, bridge: bridge}
	body, err := json.Marshal(map[string]any{
		"key_id": 5, "session_id": testWebchatSession, "model": "m", "content": "new question",
		"user_id": 2, "api_key": "other-token", "messages": []map[string]string{{"role": "system", "content": "spoofed context"}},
	})
	require.NoError(t, err)
	w := webchatTestRequest(t, h, 41, http.MethodPost, "/messages", string(body))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []any{int64(41), int64(5), testWebchatSession}, store.began)
	require.Equal(t, "fixture-owned-credential", bridge.key.Key)
	require.Equal(t, "192.0.2.5", bridge.clientIP)
	require.Equal(t, []service.WebchatMessage{{Role: "user", Content: "earlier"}, {Role: "assistant", Content: "earlier answer"}, {Role: "user", Content: "new question"}}, bridge.messages)
	require.Equal(t, []any{int64(41), int64(5), testWebchatSession, int64(1), "turn-lease", "m", "new question", "next answer"}, store.saved)
	require.False(t, store.released)
	require.NotContains(t, w.Body.String(), bridge.key.Key)
}

func TestWebchatTurnFailureAndExpiryDoNotSaveOrExposeContent(t *testing.T) {
	body := `{"key_id":5,"session_id":"` + testWebchatSession + `","model":"m","content":"new question"}`
	for _, tc := range []struct {
		name                     string
		storeErr, errorFromModel error
		wantStatus               int
		release                  bool
	}{
		{"expired", service.ErrWebchatNotFound, nil, 404, false},
		{"busy", service.ErrWebchatConflict, nil, 409, false},
		{"gateway unavailable", nil, errors.New("sensitive internal payload"), 502, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &webchatTurnStore{beginErr: tc.storeErr}
			bridge := &webchatBridgeStub{err: tc.errorFromModel}
			h := &WebchatHandler{keys: &webchatKeyStub{key: validWebchatTestKey()}, store: store, bridge: bridge}
			w := webchatTestRequest(t, h, 41, http.MethodPost, "/messages", body)
			require.Equal(t, tc.wantStatus, w.Code)
			require.Equal(t, tc.release, store.released)
			require.Nil(t, store.saved)
			require.NotContains(t, w.Body.String(), "sensitive")
			if tc.storeErr != nil {
				require.Nil(t, bridge.key)
			}
		})
	}
}

func TestWebchatRejectsUnsupportedInputBeforeInference(t *testing.T) {
	for _, content := range []string{"", "   ", "line\x00break", strings.Repeat("界", service.WebchatMaxUserBytes/3+1)} {
		body, err := json.Marshal(webchatSendRequest{KeyID: 5, Model: "m", Content: content})
		require.NoError(t, err)
		w := webchatTestRequest(t, &WebchatHandler{}, 41, http.MethodPost, "/messages", string(body))
		require.Equal(t, http.StatusBadRequest, w.Code)
	}
}

func TestWebchatDoesNotSignUserOutWhenGatewayRejectsSelectedKey(t *testing.T) {
	bridge := &webchatBridgeStub{err: &service.WebchatUpstreamError{Status: http.StatusUnauthorized}}
	store := &webchatTurnStore{}
	h := &WebchatHandler{keys: &webchatKeyStub{key: validWebchatTestKey()}, store: store, bridge: bridge}
	body := `{"key_id":5,"session_id":"` + testWebchatSession + `","model":"m","content":"a"}`
	w := webchatTestRequest(t, h, 41, http.MethodPost, "/messages", body)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "WEBCHAT_KEY_UNAVAILABLE")
	require.True(t, store.released)
}

func TestWebchatReservesFirstSessionBeforeCallingModel(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		reserveErr, modelErr error
		want                 int
		wantRelease          bool
	}{
		{"success", nil, nil, 200, false},
		{"at capacity", service.ErrWebchatLimit, nil, 400, false},
		{"model error releases reserved slot", nil, errors.New("unavailable"), 502, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &webchatTurnStore{beginErr: tc.reserveErr}
			bridge := &webchatBridgeStub{err: tc.modelErr}
			h := &WebchatHandler{keys: &webchatKeyStub{key: validWebchatTestKey()}, store: store, bridge: bridge}
			w := webchatTestRequest(t, h, 41, http.MethodPost, "/messages", `{"key_id":5,"model":"m","content":"first question"}`)
			require.Equal(t, tc.want, w.Code)
			require.Equal(t, []any{int64(41), int64(5)}, store.began)
			require.Equal(t, tc.wantRelease, store.released)
			if tc.want == 200 {
				require.Equal(t, []any{int64(41), int64(5), "", int64(0), "start-lease", "m", "first question", "next answer"}, store.saved)
			} else {
				require.Nil(t, store.saved)
			}
			if tc.reserveErr != nil {
				require.Nil(t, bridge.key)
			}
		})
	}
}

const testWebchatFile = "02e6f692-abb7-4898-bd4d-857077b12f12"
const testWebchatWebP = "UklGRjgAAABXRUJQVlA4ICwAAACQAQCdASoBAAEAAgA0JaACdLoAA5gA/vmTb/+QH/+QH/+QH/8gP+IXeyAwAA=="

func TestWebchatSendsAttachmentOnlyAndKeepsPrivateBytesOutOfJSON(t *testing.T) {
	store := &webchatTurnStore{}
	bridge := &webchatBridgeStub{}
	h := &WebchatHandler{keys: &webchatKeyStub{key: validWebchatTestKey()}, store: store, bridge: bridge}
	original, err := base64.StdEncoding.DecodeString(testWebchatWebP)
	require.NoError(t, err)
	w := webchatTestRequest(t, h, 41, http.MethodPost, "/messages",
		`{"key_id":5,"model":"m","attachments":[{"name":"example.webp","mime_type":"image/webp","data":"`+testWebchatWebP+`"}]}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Len(t, bridge.messages, 1)
	require.Len(t, store.files, 1)
	require.Equal(t, store.files, bridge.messages[0].Attachments)
	require.Equal(t, original, store.files[0].Data)
	require.Empty(t, store.files[0].Text)
	require.Contains(t, w.Body.String(), "example.webp")
	require.NotContains(t, w.Body.String(), testWebchatWebP)
}

func TestWebchatRejectsInvalidUploadsBeforeModelUse(t *testing.T) {
	for _, raw := range []string{
		`[{"name":"test.txt","mime_type":"text/plain","url":"https://example.invalid/file"}]`,
		`[{}, {}, {}, {}]`,
		`{"name":"test.txt"}`,
		`[{"name":"test.png","mime_type":"image/png","data":"invalid base64"}]`,
		`[{"name":"note.txt","mime_type":"text/plain","data":"aGVsbG8="}]`,
		`[{"name":"image.webp","mime_type":"image/webp","data":"` + testWebchatWebP + `"},{"name":"note.md","mime_type":"text/markdown","data":"aGVsbG8="}]`,
	} {
		store := &webchatTurnStore{}
		bridge := &webchatBridgeStub{}
		h := &WebchatHandler{keys: &webchatKeyStub{key: validWebchatTestKey()}, store: store, bridge: bridge}
		w := webchatTestRequest(t, h, 41, http.MethodPost, "/messages", `{"key_id":5,"model":"m","attachments":`+raw+`}`)
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "WEBCHAT_ATTACHMENT_INVALID")
		require.Nil(t, bridge.messages)
		require.Nil(t, store.files)
		if store.began != nil {
			require.True(t, store.released)
		}
	}
}

func TestWebchatRejectsOversizedRetainedAttachmentsBeforeBilling(t *testing.T) {
	for _, old := range []service.WebchatAttachment{
		{Size: service.WebchatMaxSessionAttachmentBytes},
		{Text: strings.Repeat("a", service.WebchatMaxSessionExtractedBytes+1)},
	} {
		store := &webchatTurnStore{history: []service.WebchatMessage{{Role: "user", Attachments: []service.WebchatAttachment{old}}}}
		bridge := &webchatBridgeStub{}
		h := &WebchatHandler{keys: &webchatKeyStub{key: validWebchatTestKey()}, store: store, bridge: bridge}
		w := webchatTestRequest(t, h, 41, http.MethodPost, "/messages",
			`{"key_id":5,"model":"m","session_id":"`+testWebchatSession+`","content":"summarize","attachments":[{"name":"new.webp","mime_type":"image/webp","data":"`+testWebchatWebP+`"}]}`)
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "WEBCHAT_HISTORY_LIMIT")
		require.Nil(t, bridge.messages)
		require.True(t, store.released)
	}
}

func TestWebchatContinuesStoredDocumentWithNewImage(t *testing.T) {
	old := service.WebchatAttachment{ID: testWebchatFile, Name: "notes.txt", MIMEType: "text/plain", Data: []byte("notes"), Text: "notes", Size: 5}
	history := service.WebchatMessage{Role: "user", Attachments: []service.WebchatAttachment{old}}
	store := &webchatTurnStore{history: []service.WebchatMessage{history}}
	bridge := &webchatBridgeStub{}
	h := &WebchatHandler{keys: &webchatKeyStub{key: validWebchatTestKey()}, store: store, bridge: bridge}

	w := webchatTestRequest(t, h, 41, http.MethodPost, "/messages",
		`{"key_id":5,"model":"m","session_id":"`+testWebchatSession+`","attachments":[{"name":"new.webp","mime_type":"image/webp","data":"`+testWebchatWebP+`"}]}`)
	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, bridge.messages, 2)
	require.Equal(t, history, bridge.messages[0])
	require.Len(t, store.files, 1)
	require.Equal(t, "new.webp", store.files[0].Name)
}

type webchatDownloadStore struct {
	webchatStore
	found   bool
	request []any
}

func (s *webchatDownloadStore) GetAttachment(_ context.Context, user, key int64, session, id string) (*service.WebchatAttachment, error) {
	s.request = []any{user, key, session, id}
	if !s.found {
		return nil, service.ErrWebchatNotFound
	}
	return &service.WebchatAttachment{ID: id, Name: "聊天记录.txt", MIMEType: "text/plain", Size: 7, Data: []byte("private")}, nil
}

func TestWebchatPrivateAttachmentDownload(t *testing.T) {
	store := &webchatDownloadStore{found: true}
	h := &WebchatHandler{keys: &webchatKeyStub{key: validWebchatTestKey()}, store: store}
	path := "/sessions/" + testWebchatSession + "/attachments/" + testWebchatFile + "?key_id=5"
	w := webchatTestRequest(t, h, 41, http.MethodGet, path, "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []any{int64(41), int64(5), testWebchatSession, testWebchatFile}, store.request)
	require.Equal(t, "private", w.Body.String())
	require.Contains(t, w.Header().Get("Cache-Control"), "no-store")
	require.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	disposition, params, err := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
	require.NoError(t, err)
	require.Equal(t, "attachment", disposition)
	require.Equal(t, "聊天记录.txt", params["filename"])
	store.found = false
	w = webchatTestRequest(t, h, 41, http.MethodGet, path, "")
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "WEBCHAT_ATTACHMENT_NOT_FOUND")
	require.NotContains(t, w.Body.String(), "private")
}
