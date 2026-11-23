// ModelGate keeps conversation state. Open WebUI's private OpenAI router receives
// only the current context and an opaque, single-use callback ticket.
package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	webchatAlias        = "modelgate-chat"
	webchatTicketHeader = "X-ModelGate-Ticket"
	webchatTicketTTL    = 90 * time.Second
	// Bounds the JSON envelope, including escaped text and provider metadata.
	webchatWireLimit = 4 << 20
)

type WebchatUpstreamError struct{ Status int }

func (e *WebchatUpstreamError) Error() string {
	return fmt.Sprintf("chat service returned HTTP %d", e.Status)
}

func webchatError(status int) *WebchatUpstreamError { return &WebchatUpstreamError{Status: status} }

type webchatKeyLookup interface {
	GetByID(context.Context, int64) (*APIKey, error)
}

// WebchatTicketCache shares short-lived callback authorizations across replicas.
// Take must atomically remove the item and return nil when it is absent.
type WebchatTicketCache interface {
	Issue(ctx context.Context, id string, data []byte, ttl time.Duration) (bool, error)
	Take(ctx context.Context, id string) ([]byte, error)
	Pending(ctx context.Context, id string) (bool, error)
	Drop(ctx context.Context, id string) error
}

type webchatTicket struct {
	UserID      int64
	KeyID       int64
	IP          string
	Model       string
	MessageHash [sha256.Size]byte
	Receipt     string
}

type WebchatBridge struct {
	tickets              WebchatTicketCache
	keys                 webchatKeyLookup
	gateway              http.Handler
	url, email, password string
	client               *http.Client
	tokenMu              sync.Mutex
	token                string
	tokenExpiry          time.Time
}

func NewWebchatBridge(cache WebchatTicketCache, keys *APIKeyService) *WebchatBridge {
	rawURL := strings.TrimRight(strings.TrimSpace(os.Getenv("WEBCHAT_OPEN_WEBUI_URL")), "/")
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		rawURL = ""
	}
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		defaultTransport = &http.Transport{}
	}
	transport := defaultTransport.Clone()
	transport.Proxy = nil // The internal service credential must only reach the configured sidecar.
	password := os.Getenv("WEBCHAT_OPEN_WEBUI_PASSWORD")
	if file := os.Getenv("WEBCHAT_OPEN_WEBUI_PASSWORD_FILE"); file != "" {
		data, err := os.ReadFile(file) //nolint:gosec // This credential path comes exclusively from the operator’s process environment.
		password = ""
		if err == nil {
			password = strings.TrimRight(string(data), "\r\n")
		}
	}
	return &WebchatBridge{tickets: cache, keys: keys, url: rawURL, email: os.Getenv("WEBCHAT_OPEN_WEBUI_EMAIL"), password: password, client: &http.Client{
		Transport: transport, Timeout: 6 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

// AttachGateway installs the same authenticated, metered routes used by API clients.
// Call before the server starts accepting requests.
func (b *WebchatBridge) AttachGateway(gateway http.Handler) { b.gateway = gateway }

type webchatResponse struct {
	header   http.Header
	closed   <-chan bool
	status   int
	body     bytes.Buffer
	oversize bool
}

func (w *webchatResponse) Header() http.Header      { return w.header }
func (w *webchatResponse) CloseNotify() <-chan bool { return w.closed }
func (w *webchatResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *webchatResponse) Write(p []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	if w.oversize || len(p) > webchatWireLimit-w.body.Len() {
		w.oversize = true
		return 0, errors.New("chat gateway response exceeds limit")
	}
	return w.body.Write(p)
}
func (w *webchatResponse) Flush() { w.WriteHeader(http.StatusOK) }

func (b *WebchatBridge) dispatch(ctx context.Context, key *APIKey, clientIP, method, path string, body []byte) ([]byte, *WebchatUpstreamError) {
	if b.gateway == nil {
		return nil, webchatError(503)
	}
	if key == nil || strings.TrimSpace(key.Key) == "" || !key.IsActive() || key.IsExpired() || key.IsQuotaExhausted() {
		return nil, webchatError(403)
	}
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return nil, webchatError(403)
	}
	req, err := http.NewRequestWithContext(ctx, method, path, bytes.NewReader(body))
	if err != nil {
		return nil, webchatError(502)
	}
	req.RequestURI = path
	req.RemoteAddr = net.JoinHostPort(ip.String(), "0")
	req.Header.Set("Authorization", "Bearer "+key.Key)
	req.Header.Set("Accept", "application/json")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	closed := make(chan bool)
	stop := context.AfterFunc(ctx, func() { close(closed) })
	defer stop()
	w := &webchatResponse{header: make(http.Header), closed: closed}
	b.gateway.ServeHTTP(w, req)
	if w.status < 200 || w.status >= 300 {
		return nil, webchatError(webchatStatus(w.status))
	}
	if w.oversize {
		return nil, webchatError(502)
	}
	return w.body.Bytes(), nil
}

func webchatStatus(status int) int {
	if status >= 400 && status < 600 {
		return status
	}
	return http.StatusBadGateway
}

func (b *WebchatBridge) Models(ctx context.Context, key *APIKey, clientIP string) ([]string, error) {
	if !b.configured() {
		return nil, webchatError(503)
	}
	body, err := b.dispatch(ctx, key, clientIP, http.MethodGet, "/v1/models", nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &result) != nil || result.Data == nil {
		return nil, webchatError(502)
	}
	names := make([]string, 0, len(result.Data))
	seen := map[string]bool{}
	for _, m := range result.Data {
		if ValidWebchatText(m.ID, WebchatMaxModelBytes) && keyModelPurpose(m.ID) == "text" && !seen[m.ID] {
			names = append(names, m.ID)
			seen[m.ID] = true
		}
	}
	return names, nil
}

func (b *WebchatBridge) configured() bool {
	return b.url != "" && b.email != "" && b.password != "" && b.tickets != nil && b.gateway != nil
}

func (b *WebchatBridge) post(ctx context.Context, path, token string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.url+path, bytes.NewReader(body))
	if err != nil {
		return nil, 0, webchatError(503)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := b.client.Do(req)
	if err != nil {
		return nil, 0, webchatError(503)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, res.StatusCode, webchatError(webchatStatus(res.StatusCode))
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, webchatWireLimit+1))
	if err != nil || len(data) > webchatWireLimit {
		return nil, res.StatusCode, webchatError(502)
	}
	return data, res.StatusCode, nil
}

func (b *WebchatBridge) sessionToken(ctx context.Context, invalid string) (string, error) {
	b.tokenMu.Lock()
	defer b.tokenMu.Unlock()
	if b.token != "" && b.token != invalid && time.Now().Before(b.tokenExpiry) {
		return b.token, nil
	}
	b.token = ""
	body, _ := json.Marshal(map[string]string{"email": b.email, "password": b.password})
	loginCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	data, _, err := b.post(loginCtx, "/api/v1/auths/signin", "", body)
	if err != nil {
		return "", webchatError(503)
	}
	var result struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expires_at"`
	}
	if json.Unmarshal(data, &result) != nil || result.Token == "" {
		return "", webchatError(503)
	}
	b.token = result.Token
	b.tokenExpiry = time.Now().Add(24 * time.Hour)
	if result.ExpiresAt > 0 {
		b.tokenExpiry = time.Unix(result.ExpiresAt, 0).Add(-time.Minute)
	}
	return b.token, nil
}

func (b *WebchatBridge) Complete(ctx context.Context, key *APIKey, clientIP, model string, messages []WebchatMessage) (string, error) {
	if !b.configured() {
		return "", webchatError(503)
	}
	ip := net.ParseIP(clientIP)
	if ip == nil || key == nil || key.ID <= 0 || key.UserID <= 0 || !key.IsActive() || key.IsExpired() || key.IsQuotaExhausted() {
		return "", webchatError(403)
	}
	if !ValidWebchatText(model, WebchatMaxModelBytes) || keyModelPurpose(model) != "text" {
		return "", webchatError(http.StatusBadRequest)
	}
	token, err := b.sessionToken(ctx, "")
	if err != nil {
		return "", err
	}
	var raw [64]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", webchatError(503)
	}
	ticket := hex.EncodeToString(raw[:32])
	receipt := hex.EncodeToString(raw[32:])
	contextBytes, err := json.Marshal(messages)
	if err != nil {
		return "", webchatError(400)
	}
	state, _ := json.Marshal(webchatTicket{UserID: key.UserID, KeyID: key.ID, IP: ip.String(), Model: model, MessageHash: sha256.Sum256(contextBytes), Receipt: receipt})
	if ok, err := b.tickets.Issue(ctx, ticket, state, webchatTicketTTL); err != nil || !ok {
		return "", webchatError(503)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = b.tickets.Drop(cleanup, ticket)
	}()
	request, _ := json.Marshal(map[string]any{"model": webchatAlias, "messages": messages, "stream": false, "metadata": map[string]string{"message_id": ticket}})
	data, status, err := b.post(ctx, "/openai/chat/completions", token, request)
	if status == 401 {
		// A restarted sidecar can invalidate its service login. Only retry if the
		// callback has never consumed this ticket; the same ticket can bill once.
		if pending, _ := b.tickets.Pending(ctx, ticket); pending {
			if token, err = b.sessionToken(ctx, token); err == nil {
				data, _, err = b.post(ctx, "/openai/chat/completions", token, request)
			}
		}
	}
	if err != nil {
		return "", err
	}
	var result webchatCompletion
	if json.Unmarshal(data, &result) != nil || result.Receipt != receipt || result.text() == "" {
		return "", webchatError(502)
	}
	return result.text(), nil
}

type webchatCompletion struct {
	Choices []struct {
		Message WebchatMessage `json:"message"`
	} `json:"choices"`
	Receipt string `json:"modelgate_receipt,omitempty"`
}

func (c *webchatCompletion) text() string {
	if len(c.Choices) != 1 {
		return ""
	}
	msg := c.Choices[0].Message
	if msg.Role != "assistant" || !ValidWebchatText(msg.Content, WebchatMaxAssistantBytes) {
		return ""
	}
	return msg.Content
}

// Callback accepts only the private Open WebUI connection's single-turn ticket.
// The actual model, user, key and address are bound before dispatch. A second
// caller (including another replica) cannot consume the same authorization.
func (b *WebchatBridge) Callback(w http.ResponseWriter, r *http.Request) {
	reject := func(status int) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, `{"error":{"message":"chat request unavailable"}}`)
	}
	ticket := r.Header.Get(webchatTicketHeader)
	if len(ticket) != 64 {
		reject(403)
		return
	}
	if _, err := hex.DecodeString(ticket); err != nil {
		reject(403)
		return
	}
	if !b.configured() {
		reject(503)
		return
	}
	stateJSON, err := b.tickets.Take(r.Context(), ticket)
	if err != nil {
		reject(503)
		return
	}
	if len(stateJSON) == 0 {
		reject(403)
		return
	}
	var state webchatTicket
	if json.Unmarshal(stateJSON, &state) != nil {
		reject(503)
		return
	}
	var in struct {
		Model    string           `json:"model"`
		Messages []WebchatMessage `json:"messages"`
		Stream   bool             `json:"stream"`
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, webchatWireLimit))
	if err != nil || json.Unmarshal(raw, &in) != nil || in.Model != webchatAlias || in.Stream {
		reject(400)
		return
	}
	body, _ := json.Marshal(in.Messages)
	if sha256.Sum256(body) != state.MessageHash {
		reject(403)
		return
	}
	key, err := b.keys.GetByID(r.Context(), state.KeyID)
	if err != nil || key == nil || key.ID != state.KeyID || key.UserID != state.UserID {
		reject(403)
		return
	}
	payload, _ := json.Marshal(map[string]any{"model": state.Model, "messages": in.Messages, "stream": false})
	out, gatewayErr := b.dispatch(r.Context(), key, state.IP, http.MethodPost, "/v1/chat/completions", payload)
	if gatewayErr != nil {
		reject(gatewayErr.Status)
		return
	}
	var result webchatCompletion
	if json.Unmarshal(out, &result) != nil || result.text() == "" {
		reject(502)
		return
	}
	// The sidecar simply relays this envelope. A stale or misrouted provider
	// connection cannot produce a successful chat without using this user's key.
	result.Receipt = state.Receipt
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(&result)
}
