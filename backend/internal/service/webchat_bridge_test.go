package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type webchatTestKeys map[int64]*APIKey

func (keys webchatTestKeys) GetByID(_ context.Context, id int64) (*APIKey, error) {
	return keys[id], nil
}

type webchatTestTickets struct{ *miniredis.Miniredis }

func (s webchatTestTickets) Issue(_ context.Context, id string, data []byte, ttl time.Duration) (bool, error) {
	err := s.Set(id, string(data))
	s.SetTTL(id, ttl)
	return err == nil, err
}
func (s webchatTestTickets) Take(_ context.Context, id string) ([]byte, error) {
	value, err := s.Get(id)
	if err != nil {
		return nil, nil
	}
	s.Del(id)
	return []byte(value), nil
}
func (s webchatTestTickets) Drop(_ context.Context, id string) error { s.Del(id); return nil }
func (s webchatTestTickets) Pending(_ context.Context, id string) (bool, error) {
	return s.Exists(id), nil
}

func TestWebchatBridgeTurn(t *testing.T) {
	for _, mode := range []string{"ok", "tamper", "replay", "revoked", "reassigned", "quota", "no callback", "expired ticket", "sidecar restarted"} {
		t.Run(mode, func(t *testing.T) {
			cache := miniredis.RunT(t)
			rc := webchatTestTickets{cache}
			key := &APIKey{ID: 37, UserID: 9, Status: StatusActive, Key: "sk-example-never-leaves-gateway"}
			callbackKey := *key
			keys := webchatTestKeys{key.ID: &callbackKey}
			ctx := context.Background()
			clientIP := "2001:db8::92"
			messages := []WebchatMessage{{Role: "user", Content: "first turn"}, {Role: "assistant", Content: "first reply"}, {Role: "user", Content: "second turn"}}
			var calls, logins atomic.Int32

			gateway := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "[2001:db8::92]:0", r.RemoteAddr)
				assert.Equal(t, "Bearer "+key.Key, r.Header.Get("Authorization"))
				assert.Empty(t, r.Header.Get("Cookie"))
				assert.Empty(t, r.Header.Get(webchatTicketHeader))
				assert.Empty(t, r.Header.Get("X-Forwarded-For"))
				if r.URL.Path == "/v1/models" {
					_, _ = io.WriteString(w, `{"data":[{"id":"group-model"},{"id":"group-model"},{"id":"gpt-image-2"},{"id":"sora-2"},{"id":"tts-1"}]}`)
					return
				}
				assert.Equal(t, "/v1/chat/completions", r.URL.Path)
				calls.Add(1)
				var body map[string]json.RawMessage
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Len(t, body, 3)
				assert.JSONEq(t, `"group-model"`, string(body["model"]))
				expected, _ := json.Marshal(messages)
				assert.JSONEq(t, string(expected), string(body["messages"]))
				if mode == "quota" {
					http.Error(w, key.Key, http.StatusTooManyRequests)
					return
				}
				_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"actual gateway reply"}}],"sensitive_provider_metadata":"redacted"}`)
			})
			// Callback arrives on another ModelGate replica sharing only Redis.
			receiver := NewWebchatBridge(rc, nil)
			receiver.keys = keys
			receiver.url = "http://sidecar"
			receiver.email = "service"
			receiver.password = "secret"
			receiver.AttachGateway(gateway)
			callback := httptest.NewServer(http.HandlerFunc(receiver.Callback))
			defer callback.Close()
			sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				assert.NotContains(t, string(raw), key.Key)
				assert.NotContains(t, r.Header.Get("Authorization"), key.Key)
				if r.URL.Path == "/api/v1/auths/signin" {
					n := logins.Add(1)
					_, _ = fmt.Fprintf(w, `{"token":"service-jwt-%d"}`, n)
					return
				}
				assert.Equal(t, "/openai/chat/completions", r.URL.Path)
				if mode == "sidecar restarted" && r.Header.Get("Authorization") == "Bearer service-jwt-1" {
					w.WriteHeader(401)
					return
				}
				var request struct {
					Model    string            `json:"model"`
					Messages []WebchatMessage  `json:"messages"`
					Metadata map[string]string `json:"metadata"`
				}
				if !assert.NoError(t, json.Unmarshal(raw, &request)) {
					w.WriteHeader(500)
					return
				}
				assert.Equal(t, webchatAlias, request.Model)
				assert.Equal(t, messages, request.Messages)
				ticket := request.Metadata["message_id"]
				stored, err := cache.Get(ticket)
				assert.NoError(t, err)
				assert.NotContains(t, stored, key.Key)
				assert.NotContains(t, stored, "second turn")
				assert.Equal(t, webchatTicketTTL, cache.TTL(ticket))
				if mode == "no callback" {
					_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"unmetered reply"}}]}`)
					return
				}
				if mode == "expired ticket" {
					cache.FastForward(2 * time.Minute)
				}
				if mode == "revoked" {
					callbackKey.Status = StatusAPIKeyDisabled
				}
				if mode == "reassigned" {
					callbackKey.UserID++
				}
				if mode == "tamper" {
					request.Messages[0].Content = "other request"
				}
				body, _ := json.Marshal(map[string]any{"model": webchatAlias, "messages": request.Messages, "stream": false, "tools": []string{"should never reach gateway"}})
				send := func() (int, []byte) {
					req, err := http.NewRequest(http.MethodPost, callback.URL, bytes.NewReader(body))
					assert.NoError(t, err)
					req.Header.Set(webchatTicketHeader, ticket)
					res, err := http.DefaultClient.Do(req)
					if !assert.NoError(t, err) {
						return 500, nil
					}
					defer func() { _ = res.Body.Close() }()
					out, _ := io.ReadAll(res.Body)
					assert.NotContains(t, string(out), "sensitive_provider_metadata")
					assert.NotContains(t, string(out), key.Key)
					return res.StatusCode, out
				}
				status, out := send()
				if mode == "replay" {
					duplicate, _ := send()
					assert.Equal(t, 403, duplicate)
				}
				w.WriteHeader(status)
				_, _ = w.Write(out)
			}))
			defer sidecar.Close()
			sender := NewWebchatBridge(rc, nil)
			sender.keys = keys
			sender.url = sidecar.URL
			sender.email = "service@example.test"
			sender.password = "test-pass"
			sender.AttachGateway(gateway)
			models, err := sender.Models(ctx, key, clientIP)
			require.NoError(t, err)
			require.Equal(t, []string{"group-model"}, models)
			_, mediaErr := sender.Complete(ctx, key, clientIP, "gpt-image-2", messages)
			require.EqualError(t, mediaErr, "chat service returned HTTP 400")
			answer, err := sender.Complete(ctx, key, clientIP, "group-model", messages)
			switch mode {
			case "ok", "replay", "sidecar restarted":
				require.NoError(t, err)
				assert.Equal(t, "actual gateway reply", answer)
				assert.EqualValues(t, 1, calls.Load())
			default:
				var upstream *WebchatUpstreamError
				require.ErrorAs(t, err, &upstream)
				assert.Empty(t, answer)
				switch mode {
				case "no callback":
					assert.Equal(t, 502, upstream.Status)
				case "quota":
					assert.Equal(t, http.StatusTooManyRequests, upstream.Status)
				default:
					assert.Equal(t, 403, upstream.Status)
				}
				if mode == "quota" {
					assert.EqualValues(t, 1, calls.Load())
				} else {
					assert.Zero(t, calls.Load())
				}
			}
			if mode == "sidecar restarted" {
				assert.EqualValues(t, 2, logins.Load())
			}
			assert.Empty(t, cache.Keys())
		})
	}
}

func TestWebchatBridgePrivateConfiguration(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "password")
	require.NoError(t, os.WriteFile(secret, []byte("mounted-password\r\n"), 0600))
	t.Setenv("WEBCHAT_OPEN_WEBUI_URL", "http://sidecar:8080/")
	t.Setenv("WEBCHAT_OPEN_WEBUI_PASSWORD", "fallback")
	t.Setenv("WEBCHAT_OPEN_WEBUI_PASSWORD_FILE", secret)
	require.Equal(t, "mounted-password", NewWebchatBridge(nil, nil).password)
	t.Setenv("WEBCHAT_OPEN_WEBUI_PASSWORD_FILE", secret+"-absent")
	require.Empty(t, NewWebchatBridge(nil, nil).password)
	t.Setenv("WEBCHAT_OPEN_WEBUI_URL", "http://user:password@sidecar/")
	require.Empty(t, NewWebchatBridge(nil, nil).url)
	w := httptest.NewRecorder()
	NewWebchatBridge(nil, nil).Callback(w, httptest.NewRequest(http.MethodPost, "/api/internal/webchat/v1/chat/completions", nil))
	assert.Equal(t, 403, w.Code)
}

func TestWebchatBridgeAssistantLimit(t *testing.T) {
	var result webchatCompletion
	content := strings.Repeat("a", WebchatMaxAssistantBytes+1)
	data, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": WebchatMessage{Role: "assistant", Content: content}}}})
	require.NoError(t, json.Unmarshal(data, &result))
	assert.Empty(t, result.text())
}
