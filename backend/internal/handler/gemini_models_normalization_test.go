//go:build unit

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const canvasGeminiOpenAIModels = `{"object":"list","data":[{"id":"gemini-3.1-flash-lite-image","object":"model"},{"id":"gemini-3.1-flash-image-preview","object":"model"},{"id":"gemini-nano-banana-2.1","object":"model"},{"id":"gemini-3-pro-image-preview","object":"model"}]}`

type geminiNormalizationHeaderUpstream struct{ geminiMixedModelsUpstream }

func (u *geminiNormalizationHeaderUpstream) Do(req *http.Request, proxy string, id int64, concurrency int) (*http.Response, error) {
	res, err := u.geminiMixedModelsUpstream.Do(req, proxy, id, concurrency)
	if err == nil {
		res.Header.Set("Content-Type", "text/plain")
		res.Header.Set("Content-Length", strconv.Itoa(len(u.body)))
		res.Header.Set("Content-Encoding", "identity")
		res.Header.Set("ETag", `"upstream-body"`)
	}
	return res, err
}

func geminiModelsNormalizationFixture(t *testing.T, status int, body string, allowlist service.GroupModelAllowlist, mixed bool) (*GatewayHandler, *service.APIKey) {
	t.Helper()
	groupID := int64(20)
	accounts := []service.Account{{ID: 1, Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}}
	if mixed {
		accounts = append(accounts, service.Account{ID: 2, Platform: service.PlatformAntigravity, Extra: map[string]any{"mixed_scheduling": true}, Credentials: map[string]any{"model_mapping": map[string]any{"gemini-synced-custom": "gemini-3.8-flash-high", "gemini-3-pro-image-preview": "gemini-3-pro-image-preview"}}})
	}
	repo := &geminiAllowlistAccountRepoStub{gatewayModelsAccountRepoStub: gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{groupID: accounts}}}
	upstream := &geminiNormalizationHeaderUpstream{geminiMixedModelsUpstream{status: status, body: body}}
	handler := &GatewayHandler{geminiCompatService: service.NewGeminiMessagesCompatService(repo, nil, nil, nil, nil, nil, upstream, nil, &config.Config{})}
	key := &service.APIKey{ID: 479, UserID: 7, Key: "test-canvas-key", Status: service.StatusActive, GroupID: &groupID, Group: &service.Group{ID: groupID, Status: service.StatusActive, Platform: service.PlatformGemini, AllowImageGeneration: true, ModelAllowlist: allowlist}}
	return handler, key
}

func serveGeminiNormalizedModels(t *testing.T, status int, body string, allowlist service.GroupModelAllowlist, mixed bool) *httptest.ResponseRecorder {
	t.Helper()
	h, key := geminiModelsNormalizationFixture(t, status, body, allowlist, mixed)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
	c.Set(string(middleware.ContextKeyAPIKey), key)
	h.GeminiV1BetaListModels(c)
	return rec
}

func TestGeminiModelsNormalizationHandlerOpenAIImageModels(t *testing.T) {
	rec := serveGeminiNormalizedModels(t, 200, canvasGeminiOpenAIModels, service.GroupModelAllowlist{}, false)
	require.Equal(t, 200, rec.Code)
	require.JSONEq(t, `{"models":[{"name":"models/gemini-3.1-flash-lite-image"},{"name":"models/gemini-3.1-flash-image-preview"},{"name":"models/gemini-nano-banana-2.1"},{"name":"models/gemini-3-pro-image-preview"}]}`, rec.Body.String())
	require.Equal(t, "native-id", rec.Header().Get("X-Request-Id"))
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	require.Empty(t, rec.Header().Get("Content-Encoding"))
	require.Empty(t, rec.Header().Get("ETag"))
	length := rec.Header().Get("Content-Length")
	if length != "" {
		require.Equal(t, strconv.Itoa(rec.Body.Len()), length)
	}
}

func TestGeminiModelsNormalizationHandlerFormats(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"native metadata", `{"models":[{"name":"models/Gemini-Alias","supportedGenerationMethods":["generateContent"],"inputTokenLimit":42,"custom":{"x":true}}],"nextPageToken":"next","other":1}`, ""},
		{"native wins over redundant data", `{"models":[{"name":"models/gemini-native","inputTokenLimit":42}],"nextPageToken":"next","object":"list","data":[{"id":"hidden-image"}]}`, `{"models":[{"name":"models/gemini-native","inputTokenLimit":42}],"nextPageToken":"next"}`},
		{"native empty wins", `{"models":[],"object":"list","data":[{"id":"hidden-image"}]}`, `{"models":[]}`},
		{"native wins without redundant list marker", `{"models":[{"name":"models/native"}],"data":[{"id":"hidden-image"}]}`, `{"models":[{"name":"models/native"}]}`},
		{"native preserves unrelated envelope metadata", `{"models":[{"name":"models/native"}],"object":"catalogue","data":[{"id":"hidden-image"}],"nextPageToken":"next"}`, `{"models":[{"name":"models/native"}],"object":"catalogue","nextPageToken":"next"}`},
		{"valid empty", `{"object":"list","data":[]}`, `{"models":[]}`},
		{"stable names and duplicates", `{"object":"list","data":[{"id":"models/Gemini-Alias"},{"id":"Gemini-Alias"},{"id":"gemini-alias"},{"id":"gemini-3.1-flash-image-preview","owned_by":"vendor","supportedGenerationMethods":["invented"]}]}`, `{"models":[{"name":"models/Gemini-Alias"},{"name":"models/gemini-alias"},{"name":"models/gemini-3.1-flash-image-preview"}]}`},
		{"invalid entries skipped", `{"object":"list","data":[null,123,"model",{}, {"id":null},{"id":123},{"id":""},{"id":"../x"},{"id":"models/models/x"},{"id":"  alias  "},{"id":"x/y"},{"id":"x?y"},{"id":"x#y"},{"id":"..."},{"id":"x\ny"},{"id":"user","object":"user"},{"id":"Valid.Alias-1","object":"model"}]}`, `{"models":[{"name":"models/Valid.Alias-1"}]}`},
		{"invalid native fallback", `{"models":null,"object":"list","data":[{"id":"gemini-native"}]}`, `{"models":[{"name":"models/gemini-native"}]}`},
		{"all invalid is not empty success", `{"object":"list","data":[{"id":""},{"id":3}]}`, ""},
		{"null data is not empty success", `{"object":"list","data":null}`, ""},
		{"object data is unknown", `{"object":"list","data":{}}`, ""},
		{"not a model list", `{"object":"user","data":[{"id":"a"}]}`, ""},
		{"arbitrary data", `{"data":[{"id":"a"}]}`, ""},
		{"missing names", `{"models":[{"other":true}]}`, ""},
		{"invalid json", `{"models":[`, ""},
		{"html", `<html>Bad gateway</html>`, ""},
		{"null", `null`, ""},
		{"unknown error", `{"error":{"message":"not available"}}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveGeminiNormalizedModels(t, 200, tc.body, service.GroupModelAllowlist{}, false)
			require.Equal(t, 200, rec.Code)
			if tc.want == "" {
				require.Equal(t, tc.body, rec.Body.String())
			} else {
				require.JSONEq(t, tc.want, rec.Body.String())
			}
		})
	}
}

func TestGeminiModelsNormalizationHandlerAllowlistAndMixedAccounts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		allow service.GroupModelAllowlist
		mixed bool
		names []string
	}{
		{"one image", service.GroupModelAllowlist{Enabled: true, Models: []string{"gemini-nano-banana-2.1"}}, false, []string{"models/gemini-nano-banana-2.1"}},
		{"prefixed name", service.GroupModelAllowlist{Enabled: true, Models: []string{"models/gemini-3-pro-image-preview"}}, false, []string{"models/gemini-3-pro-image-preview"}},
		{"wildcard", service.GroupModelAllowlist{Enabled: true, Models: []string{"gemini-3.1-*"}}, false, []string{"models/gemini-3.1-flash-lite-image", "models/gemini-3.1-flash-image-preview"}},
		{"no synthetic discovery", service.GroupModelAllowlist{Enabled: true, Models: []string{"upstream-did-not-list-this"}}, false, []string{}},
		{"mixed deduplicates", service.GroupModelAllowlist{}, true, []string{"models/gemini-3.1-flash-lite-image", "models/gemini-3.1-flash-image-preview", "models/gemini-nano-banana-2.1", "models/gemini-3-pro-image-preview", "models/gemini-synced-custom"}},
		{"mixed allowlist", service.GroupModelAllowlist{Enabled: true, Models: []string{"gemini-synced-custom"}}, true, []string{"models/gemini-synced-custom"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveGeminiNormalizedModels(t, 200, canvasGeminiOpenAIModels, tc.allow, tc.mixed)
			require.Equal(t, 200, rec.Code)
			var got struct {
				Models []struct {
					Name    string   `json:"name"`
					Methods []string `json:"supportedGenerationMethods"`
				}
				Data   json.RawMessage `json:"data"`
				Object json.RawMessage `json:"object"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
			names := []string{}
			for _, model := range got.Models {
				names = append(names, model.Name)
				if model.Name == "models/gemini-3.1-flash-lite-image" || model.Name == "models/gemini-3.1-flash-image-preview" || model.Name == "models/gemini-nano-banana-2.1" || model.Name == "models/gemini-3-pro-image-preview" {
					require.Empty(t, model.Methods, "conversion must not invent capabilities")
				}
			}
			if tc.mixed && !tc.allow.Enabled {
				// Antigravity also advertises its existing default passthroughs.
				require.Subset(t, names, tc.names)
				require.Equal(t, tc.names[:4], names[:4], "native model ordering must win")
				require.Equal(t, len(names), len(uniqueGeminiModelNames(names)))
			} else {
				require.Equal(t, tc.names, names)
			}
			require.Nil(t, got.Data)
			require.Nil(t, got.Object)
		})
	}
	// The native catalogue still wins, even when its redundant data catalogue
	// would otherwise contain a whitelisted name.
	rec := serveGeminiNormalizedModels(t, 200, `{"models":[{"name":"models/allowed","displayName":"Original","supportedGenerationMethods":["generateContent"]},{"name":"models/blocked"}],"object":"list","data":[{"id":"blocked"}],"nextPageToken":"next"}`, service.GroupModelAllowlist{Enabled: true, Models: []string{"allowed"}}, false)
	require.JSONEq(t, `{"models":[{"name":"models/allowed","displayName":"Original","supportedGenerationMethods":["generateContent"]}],"nextPageToken":"next"}`, rec.Body.String())
}

func uniqueGeminiModelNames(names []string) map[string]bool {
	seen := make(map[string]bool)
	for _, name := range names {
		seen[name] = true
	}
	return seen
}

func TestGeminiModelsNormalizationHandlerErrorsRemainErrors(t *testing.T) {
	for _, status := range []int{401, 403, 429, 500, 502, 503} {
		for _, body := range []string{canvasGeminiOpenAIModels, `{"error":{"message":"upstream failure"}}`, `<html>upstream failure</html>`} {
			rec := serveGeminiNormalizedModels(t, status, body, service.GroupModelAllowlist{}, true)
			require.Equal(t, status, rec.Code)
			require.Equal(t, body, rec.Body.String(), "non-200 results must not be normalized or merged")
		}
	}
}

func TestCanvasGatewayGeminiModelsNormalization(t *testing.T) {
	for _, imageAllowed := range []bool{true, false} {
		h, key := geminiModelsNormalizationFixture(t, 200, canvasGeminiOpenAIModels, service.GroupModelAllowlist{Enabled: true, Models: []string{"gemini-*"}}, false)
		key.Group.AllowImageGeneration = imageAllowed
		gateway := gin.New()
		gateway.Use(func(c *gin.Context) {
			require.Equal(t, "Bearer test-canvas-key", c.GetHeader("Authorization"))
			require.Empty(t, c.GetHeader("Cookie"))
			require.Empty(t, c.GetHeader("X-Goog-Api-Key"))
			c.Set(string(middleware.ContextKeyAPIKey), key)
			c.Next()
		})
		gateway.GET("/v1beta/models", h.GeminiV1BetaListModels)
		repo := &dailyUsageAPIKeyRepoStub{keys: map[int64]*service.APIKey{key.ID: key}}
		canvas := &CanvasHandler{keys: service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, &config.Config{}), maxBody: 1024, gateway: gateway}
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
			c.Next()
		})
		router.GET("/api/v1/canvas/gateway/:key_id/*path", canvas.Gateway)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/canvas/gateway/479/gemini/models", nil)
		req.Header.Set("Cookie", "user-session-cookie")
		req.Header.Set("X-Goog-Api-Key", "untrusted-client-value")
		router.ServeHTTP(rec, req)
		require.Equal(t, 200, rec.Code, rec.Body.String())
		require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
		var body struct {
			Models []struct {
				Name string `json:"name"`
			}
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Len(t, body.Models, 4, "the real broker must forward the normalized model catalogue")
		require.Equal(t, imageAllowed, key.Group.KeyCapabilities().Image, "listing compatibility must not grant image permissions")
	}
}
