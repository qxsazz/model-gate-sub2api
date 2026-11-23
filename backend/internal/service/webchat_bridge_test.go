package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	for _, mode := range []string{"ok", "tamper", "replay", "revoked", "reassigned", "quota", "no callback", "expired ticket", "sidecar restarted", "attachments", "tamper-image", "same-size image", "tamper-document", "callback busy"} {
		t.Run(mode, func(t *testing.T) {
			cache := miniredis.RunT(t)
			rc := webchatTestTickets{cache}
			key := &APIKey{ID: 37, UserID: 9, Status: StatusActive, Key: "sk-example-never-leaves-gateway"}
			callbackKey := *key
			keys := webchatTestKeys{key.ID: &callbackKey}
			ctx := context.Background()
			clientIP := "2001:db8::92"
			messages := []WebchatMessage{{Role: "user", Content: "first turn"}, {Role: "assistant", Content: "first reply"}, {Role: "user", Content: "second turn"}}
			media := mode == "attachments" || mode == "tamper-image" || mode == "same-size image" || mode == "tamper-document" || mode == "callback busy"
			if media {
				doc := WebchatAttachment{ID: "private-doc-id", Name: "report.pdf", MIMEType: "application/pdf", Size: len("PDF binary"), Data: []byte("PDF binary"), Text: "季度营收 & 32"}
				img := webchatTestImage(t)
				messages[0].Attachments = []WebchatAttachment{doc, img}
				messages[2].Content = "Tell me about the earlier report and image."
			}
			expected, _ := json.Marshal(webchatWireMessages(messages))
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
					Model    string               `json:"model"`
					Messages []webchatWireMessage `json:"messages"`
					Metadata map[string]string    `json:"metadata"`
				}
				if !assert.NoError(t, json.Unmarshal(raw, &request)) {
					w.WriteHeader(500)
					return
				}
				assert.Equal(t, webchatAlias, request.Model)
				actual, _ := json.Marshal(request.Messages)
				assert.JSONEq(t, string(expected), string(actual))
				if media {
					assert.NotContains(t, string(raw), "private-doc-id")
					assert.NotContains(t, string(raw), "PDF binary")
					assert.Contains(t, string(actual), "data:image/png;base64,")
					assert.Contains(t, string(actual), "季度营收")
				}
				ticket := request.Metadata["message_id"]
				stored, err := cache.Get(ticket)
				assert.NoError(t, err)
				assert.NotContains(t, stored, key.Key)
				assert.NotContains(t, stored, "second turn")
				assert.Equal(t, webchatTicketTTL, cache.TTL(ticket))
				assert.NotContains(t, stored, "季度营收")
				assert.NotContains(t, stored, "data:image")
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
				if mode == "tamper-image" || mode == "same-size image" || mode == "tamper-document" {
					parts, ok := request.Messages[0].Content.([]any)
					if !assert.True(t, ok) {
						w.WriteHeader(500)
						return
					}
					switch mode {
					case "tamper-image":
						parts[1] = map[string]any{"type": "image_url", "image_url": map[string]string{"url": "https://example.invalid/external"}}
					case "same-size image":
						img := webchatTestImage(t)
						img.Data[len(img.Data)-1] ^= 1
						parts[1] = map[string]any{"type": "image_url", "image_url": map[string]string{"url": "data:image/png;base64," + base64.StdEncoding.EncodeToString(img.Data)}}
					case "tamper-document":
						parts[0] = map[string]any{"type": "text", "text": "unrelated file"}
					}
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
			if mode == "callback busy" {
				require.True(t, receiver.callbackBudget.TryAcquire(webchatAttachmentBudget))
				defer receiver.callbackBudget.Release(webchatAttachmentBudget)
			}
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
			case "ok", "replay", "sidecar restarted", "attachments":
				require.NoError(t, err)
				assert.Equal(t, "actual gateway reply", answer)
				assert.EqualValues(t, 1, calls.Load())
			default:
				var upstream *WebchatUpstreamError
				require.ErrorAs(t, err, &upstream)
				assert.Empty(t, answer)
				switch mode {
				case "callback busy":
					assert.Equal(t, 503, upstream.Status)
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

func webchatTestImage(t *testing.T) WebchatAttachment {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGMwqeAEAAGZALbqWecyAAAAAElFTkSuQmCC")
	require.NoError(t, err)
	return WebchatAttachment{ID: "private-image-id", Name: "image.png", MIMEType: "image/png", Size: len(raw), Data: raw}
}

func TestWebchatWireMessagesHydratedContentOnly(t *testing.T) {
	img := webchatTestImage(t)
	document := WebchatAttachment{ID: "private-doc-id", Name: "note.txt", MIMEType: "text/plain", Size: 4, Data: []byte("ABCD"), Text: "ABCD"}
	messages := []WebchatMessage{{Role: "user", Attachments: []WebchatAttachment{img}}, {Role: "assistant", Content: "an image"}, {Role: "user", Attachments: []WebchatAttachment{document}}, {Role: "assistant", Content: "a document"}, {Role: "user", Content: "compare them"}}
	weight, err := webchatAttachmentWeight(messages)
	require.Nil(t, err)
	require.EqualValues(t, 1<<20, weight)
	wire := webchatWireMessages(messages)
	data, encodeErr := json.Marshal(wire)
	require.NoError(t, encodeErr)
	assert.NotContains(t, string(data), "private-")
	expected := "data:image/png;base64," + base64.StdEncoding.EncodeToString(img.Data)
	assert.JSONEq(t, `[{"role":"user","content":[{"type":"image_url","image_url":{"url":"`+expected+`"}}]},{"role":"assistant","content":"an image"},{"role":"user","content":"note.txt\nABCD"},{"role":"assistant","content":"a document"},{"role":"user","content":"compare them"}]`, string(data))
	for _, a := range []WebchatAttachment{
		{MIMEType: "image/png", Name: "x.png", Size: 3, Data: []byte("url")},
		{MIMEType: "https://example.invalid", Name: "x", Size: 3, Data: []byte("raw")},
		{MIMEType: "image/png", Name: "missing.png", Size: 23},
		{MIMEType: "application/pdf", Name: "scanned.pdf", Size: 3, Data: []byte("raw")},
	} {
		_, bad := webchatAttachmentWeight([]WebchatMessage{{Role: "user", Attachments: []WebchatAttachment{a}}})
		require.NotNil(t, bad)
	}
}

func TestWebchatAttachmentAdmission(t *testing.T) {
	bridge := NewWebchatBridge(webchatTestTickets{miniredis.RunT(t)}, nil)
	started := make(chan struct{}, 3)
	unblock := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auths/signin" {
			_, _ = io.WriteString(w, `{"token":"test-login"}`)
			return
		}
		started <- struct{}{}
		<-unblock
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	var callers sync.WaitGroup
	defer callers.Wait()
	defer close(unblock)
	bridge.url = server.URL
	bridge.email = "example"
	bridge.password = "test"
	bridge.AttachGateway(http.NotFoundHandler())
	msgs := []WebchatMessage{{Role: "user", Content: "image", Attachments: []WebchatAttachment{webchatTestImage(t)}}}
	small, err := webchatAttachmentWeight(msgs)
	require.Nil(t, err)
	// Multiple independent users' small uploads fit alongside a maximum-size context.
	require.True(t, bridge.sendBudget.TryAcquire(WebchatMaxSessionAttachmentBytes))
	defer bridge.sendBudget.Release(WebchatMaxSessionAttachmentBytes)
	call := func(id int64, inputs []WebchatMessage) error {
		_, err := bridge.Complete(context.Background(), &APIKey{ID: id, UserID: id, Status: StatusActive}, "127.0.0.1", "chat", inputs)
		return err
	}
	wait := func() {
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Fatal("chat did not reach Open WebUI")
		}
	}
	for id := range 2 {
		callers.Go(func() { _ = call(int64(id+1), msgs) })
	}
	wait()
	wait()
	remainder := webchatAttachmentBudget - WebchatMaxSessionAttachmentBytes - 2*small
	require.True(t, bridge.sendBudget.TryAcquire(remainder))
	defer bridge.sendBudget.Release(remainder)
	var upstream *WebchatUpstreamError
	require.ErrorAs(t, call(3, msgs), &upstream)
	require.Equal(t, http.StatusServiceUnavailable, upstream.Status)
	callers.Go(func() { _ = call(4, []WebchatMessage{{Role: "user", Content: "text only"}}) })
	wait()
	require.True(t, bridge.callbackBudget.TryAcquire(small))
	bridge.callbackBudget.Release(small)
}

func TestWebchatLargeImageCallbackUsesRequestBudget(t *testing.T) {
	// A valid PNG with an uncompressed text chunk fits the 4 MiB upload limit
	// while the equivalent OpenAI base64 request exceeds the 4 MiB reply cap.
	img := webchatTestImage(t)
	payload := append([]byte("Description\x00"), bytes.Repeat([]byte("a"), (3<<20)+128)...)
	chunk := append([]byte("tEXt"), payload...)
	var png bytes.Buffer
	_, _ = png.Write(img.Data[:len(img.Data)-12])
	require.NoError(t, binary.Write(&png, binary.BigEndian, uint32(len(payload))))
	_, _ = png.Write(chunk)
	require.NoError(t, binary.Write(&png, binary.BigEndian, crc32.ChecksumIEEE(chunk)))
	_, _ = png.Write(img.Data[len(img.Data)-12:])
	img.Data = png.Bytes()
	img.Size = len(img.Data)
	messages := []WebchatMessage{{Role: "user", Attachments: []WebchatAttachment{img}}}
	weight, inputErr := webchatAttachmentWeight(messages)
	require.Nil(t, inputErr)
	wire, err := json.Marshal(webchatWireMessages(messages))
	require.NoError(t, err)
	require.Greater(t, len(wire), webchatResponseLimit)
	rc := webchatTestTickets{miniredis.RunT(t)}
	b := NewWebchatBridge(rc, nil)
	b.url = "http://example.invalid"
	b.email = "example"
	b.password = "test"
	b.keys = webchatTestKeys{31: {ID: 31, UserID: 21, Key: "test-site-key", Status: StatusActive}}
	b.AttachGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Greater(t, r.ContentLength, int64(webchatResponseLimit))
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"received image"}}]}`)
	}))
	ticket := strings.Repeat("a", 64)
	record, _ := json.Marshal(webchatTicket{UserID: 21, KeyID: 31, IP: "127.0.0.1", Model: "chat", MessageHash: sha256.Sum256(wire), Receipt: "receipt", AttachmentWeight: weight, CallbackLimit: int64(len(wire) + 1024)})
	issued, err := rc.Issue(context.Background(), ticket, record, webchatTicketTTL)
	require.NoError(t, err)
	require.True(t, issued)
	body, _ := json.Marshal(map[string]any{"model": webchatAlias, "messages": json.RawMessage(wire)})
	request := httptest.NewRequest(http.MethodPost, "/api/internal/webchat/v1/chat/completions", bytes.NewReader(body))
	request.Header.Set(webchatTicketHeader, ticket)
	response := httptest.NewRecorder()
	b.Callback(response, request)
	require.Equal(t, 200, response.Code)
	require.Contains(t, response.Body.String(), "received image")
	oversize := &webchatResponse{header: make(http.Header)}
	_, err = oversize.Write(body)
	require.Error(t, err)
	require.True(t, oversize.oversize)
}
