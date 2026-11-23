package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	WebchatMaxSessions               = 50
	WebchatMaxTurns                  = 32
	WebchatMaxHistoryBytes           = 512 << 10
	WebchatMaxUserBytes              = 32 << 10
	WebchatMaxAssistantBytes         = 128 << 10
	WebchatMaxModelBytes             = 256
	WebchatMaxAttachments            = 3
	WebchatMaxTurnAttachmentBytes    = 4 << 20
	WebchatMaxSessionAttachmentBytes = 12 << 20
	WebchatMaxExtractedFileBytes     = 128 << 10
	WebchatMaxSessionExtractedBytes  = 512 << 10

	// Elapsed hours keep the retention window independent of the database session's time zone and DST.
	webchatLifetime = "interval '120 hours'"
	webchatLive     = "updated_at > statement_timestamp() - " + webchatLifetime
	webchatScope    = "user_id=$1 AND api_key_id=$2 AND id=$3 AND " + webchatLive
	webchatColumns  = "id,api_key_id,title,model,version,created_at,updated_at,updated_at + " + webchatLifetime
)

var (
	ErrWebchatNotFound = errors.New("webchat conversation or key not found")
	ErrWebchatConflict = errors.New("webchat conversation is busy or has changed")
	ErrWebchatLimit    = errors.New("webchat size or session limit exceeded")
)

type WebchatAttachment struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MIMEType string `json:"mime_type"`
	Size     int    `json:"size"`
	// These are available only to the internal model bridge and the authorized download handler.
	Data []byte `json:"-"`
	Text string `json:"-"`
}

type WebchatMessage struct {
	Role        string              `json:"role"`
	Content     string              `json:"content"`
	Attachments []WebchatAttachment `json:"attachments,omitempty"`
}

type WebchatConversation struct {
	ID        string           `json:"id"`
	APIKeyID  int64            `json:"api_key_id"`
	Title     string           `json:"title"`
	Model     string           `json:"model"`
	Version   int64            `json:"version"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	ExpiresAt time.Time        `json:"expires_at"`
	Messages  []WebchatMessage `json:"messages,omitempty"`
}

type WebchatStore struct {
	db        *sql.DB
	cleanupMu sync.Mutex
	cancel    context.CancelFunc
	done      chan struct{}
}

func NewWebchatStore(db *sql.DB) *WebchatStore { return &WebchatStore{db: db} }

// Start and Stop own exactly one worker per store; multiple replicas share cleanup safely.
func (s *WebchatStore) Start() {
	s.cleanupMu.Lock()
	defer s.cleanupMu.Unlock()
	if s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.cancel, s.done = cancel, done
	go func() {
		defer close(done)
		tick := time.NewTicker(time.Minute)
		defer tick.Stop()
		for {
			for batch := 0; batch < 4; batch++ {
				n, err := s.CleanupExpired(ctx)
				if err != nil {
					if ctx.Err() == nil {
						slog.Warn("webchat history cleanup failed", "error", err)
					}
					break
				}
				if n < 256 {
					break
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
}

func (s *WebchatStore) Stop() {
	if s == nil {
		return
	}
	s.cleanupMu.Lock()
	defer s.cleanupMu.Unlock()
	if s.cancel == nil {
		return
	}
	s.cancel()
	<-s.done
	s.cancel, s.done = nil, nil
}

func (s *WebchatStore) CleanupExpired(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `WITH expired AS (
        SELECT id FROM webchat_sessions WHERE updated_at <= statement_timestamp() - `+webchatLifetime+`
        ORDER BY updated_at,id LIMIT 256 FOR UPDATE SKIP LOCKED
    ), deleted AS (
        DELETE FROM webchat_sessions s USING expired e WHERE s.id=e.id RETURNING s.id
    ), abandoned AS (
        SELECT token FROM webchat_start_leases WHERE expires_at<=statement_timestamp()
        ORDER BY expires_at,token LIMIT (SELECT 256-count(*) FROM deleted) FOR UPDATE SKIP LOCKED
    ), released AS (
        DELETE FROM webchat_start_leases s USING abandoned a WHERE s.token=a.token RETURNING s.token
    ) SELECT (SELECT count(*) FROM deleted)+(SELECT count(*) FROM released)`).Scan(&count)
	return count, err
}

type webchatRow interface{ Scan(...any) error }

// All SQL selectors use the same ordered metadata projection, including for list responses.
func scanWebchat(row webchatRow, withMessages bool, lease *string) (*WebchatConversation, error) {
	var c WebchatConversation
	var raw []byte
	dest := []any{&c.ID, &c.APIKeyID, &c.Title, &c.Model, &c.Version, &c.CreatedAt, &c.UpdatedAt, &c.ExpiresAt}
	if withMessages {
		dest = append(dest, &raw)
	}
	if lease != nil {
		dest = append(dest, lease)
	}
	if err := row.Scan(dest...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWebchatNotFound
		}
		return nil, err
	}
	if withMessages {
		if err := json.Unmarshal(raw, &c.Messages); err != nil {
			return nil, fmt.Errorf("decode stored webchat messages: %w", err)
		}
	}
	c.CreatedAt, c.UpdatedAt, c.ExpiresAt = c.CreatedAt.UTC(), c.UpdatedAt.UTC(), c.ExpiresAt.UTC()
	return &c, nil
}

func (s *WebchatStore) List(ctx context.Context, userID, keyID int64) ([]WebchatConversation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+webchatColumns+` FROM webchat_sessions
	WHERE user_id=$1 AND api_key_id=$2 AND `+webchatLive+`
	ORDER BY updated_at DESC,id LIMIT $3`, userID, keyID, WebchatMaxSessions)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]WebchatConversation, 0)
	for rows.Next() {
		c, err := scanWebchat(rows, false, nil)
		if err != nil {
			return nil, err
		}
		items = append(items, *c)
	}
	return items, rows.Err()
}

func validWebchatUUID(id string) bool {
	// Google UUID also accepts URNs. Bind only normal UUIDs accepted by the database.
	return len(id) == 36 && uuid.Validate(id) == nil
}

func (s *WebchatStore) Get(ctx context.Context, userID, keyID int64, id string) (*WebchatConversation, error) {
	if !validWebchatUUID(id) {
		return nil, ErrWebchatNotFound
	}
	return scanWebchat(s.db.QueryRowContext(ctx, "SELECT "+webchatColumns+",messages FROM webchat_sessions WHERE "+webchatScope, userID, keyID, id), true, nil)
}

// GetAttachment uses the same owner/key/expiry checks as reading the conversation.
func (s *WebchatStore) GetAttachment(ctx context.Context, userID, keyID int64, sessionID, attachmentID string) (*WebchatAttachment, error) {
	if !validWebchatUUID(sessionID) || !validWebchatUUID(attachmentID) {
		return nil, ErrWebchatNotFound
	}
	var a WebchatAttachment
	err := s.db.QueryRowContext(ctx, `SELECT a.id,a.name,a.mime_type,a.data
        FROM webchat_attachments a JOIN webchat_sessions s ON s.id=a.session_id
        WHERE s.user_id=$1 AND s.api_key_id=$2 AND s.id=$3 AND a.id=$4
        AND s.updated_at > statement_timestamp()-`+webchatLifetime, userID, keyID, sessionID, attachmentID).Scan(&a.ID, &a.Name, &a.MIMEType, &a.Data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrWebchatNotFound
	}
	if err != nil {
		return nil, err
	}
	a.Size = len(a.Data)
	return &a, nil
}

// ReserveStart counts both completed sessions and in-flight first turns, before billing.
// Lock only the key's short capacity check. No transaction spans a gateway request.
func (s *WebchatStore) ReserveStart(ctx context.Context, userID, keyID int64) (string, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()
	var owner int64
	err = tx.QueryRowContext(ctx, "SELECT user_id FROM api_keys WHERE user_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE", userID, keyID).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrWebchatNotFound
	}
	if err != nil {
		return "", err
	}
	var token string
	err = tx.QueryRowContext(ctx, `INSERT INTO webchat_start_leases(user_id,api_key_id)
        SELECT $1::bigint,$2::bigint WHERE
        (SELECT count(*) FROM webchat_sessions WHERE user_id=$1 AND api_key_id=$2 AND `+webchatLive+`) +
        (SELECT count(*) FROM webchat_start_leases WHERE user_id=$1 AND api_key_id=$2 AND expires_at>statement_timestamp()) < $3
        RETURNING token`, userID, keyID, WebchatMaxSessions).Scan(&token)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrWebchatLimit
	}
	if err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return token, nil
}

func (s *WebchatStore) ReleaseStart(ctx context.Context, userID, keyID int64, lease string) error {
	if !validWebchatUUID(lease) {
		return ErrWebchatConflict
	}
	_, err := s.db.ExecContext(ctx, "DELETE FROM webchat_start_leases WHERE user_id=$1 AND api_key_id=$2 AND token=$3", userID, keyID, lease)
	return err
}

// BeginTurn reserves a live session before inference. The lease can outlive a disconnected process,
// but expires automatically. Reservations and unsuccessful sends never renew the five-day lifetime.
func (s *WebchatStore) BeginTurn(ctx context.Context, userID, keyID int64, id string) (*WebchatConversation, string, error) {
	if !validWebchatUUID(id) {
		return nil, "", ErrWebchatNotFound
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = tx.Rollback() }()
	var lease string
	c, err := scanWebchat(tx.QueryRowContext(ctx, `UPDATE webchat_sessions SET
		lease_token=gen_random_uuid(),lease_until=clock_timestamp()+interval '15 minutes'
		WHERE `+webchatScope+` AND updated_at > clock_timestamp() - `+webchatLifetime+`
		AND version < $4 AND (lease_until IS NULL OR lease_until <= clock_timestamp())
		RETURNING `+webchatColumns+`,messages,lease_token`, userID, keyID, id, WebchatMaxTurns), true, &lease)
	if errors.Is(err, ErrWebchatNotFound) {
		return nil, "", s.turnRefusal(ctx, tx, userID, keyID, id, 0, 0, 0)
	}
	if err != nil {
		return nil, "", err
	}
	// Keep metadata-only reads cheap. Only inference needs the original inputs.
	if err = loadWebchatInputs(ctx, tx, c); err != nil {
		return nil, "", err
	}
	if err = tx.Commit(); err != nil {
		return nil, "", err
	}
	return c, lease, nil
}

func loadWebchatInputs(ctx context.Context, tx *sql.Tx, c *WebchatConversation) error {
	byID := make(map[string]*WebchatAttachment)
	for i := range c.Messages {
		for j := range c.Messages[i].Attachments {
			a := &c.Messages[i].Attachments[j]
			byID[a.ID] = a
		}
	}
	if len(byID) == 0 {
		return nil
	}
	// BeginTurn locked and authenticated the parent row in this transaction.
	rows, err := tx.QueryContext(ctx, `SELECT id,data,extracted_text FROM webchat_attachments WHERE session_id=$1`, c.ID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id string
		var data []byte
		var text string
		if err = rows.Scan(&id, &data, &text); err != nil {
			return err
		}
		a := byID[id]
		if a == nil || a.Size != len(data) {
			return errors.New("webchat attachment does not match message metadata")
		}
		a.Data, a.Text = data, text
		delete(byID, id)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if len(byID) != 0 {
		return errors.New("webchat stored attachment missing")
	}
	return nil
}

func (s *WebchatStore) turnRefusal(ctx context.Context, tx *sql.Tx, userID, keyID int64, id string, moreBytes, moreAttachments, moreExtracted int) error {
	var count, size, attachmentSize, extractedSize int
	err := tx.QueryRowContext(ctx, "SELECT version,content_bytes,attachment_bytes,extracted_bytes FROM webchat_sessions WHERE "+webchatScope, userID, keyID, id).Scan(&count, &size, &attachmentSize, &extractedSize)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrWebchatNotFound
	}
	if err != nil {
		return err
	}
	if count >= WebchatMaxTurns || size+moreBytes > WebchatMaxHistoryBytes || attachmentSize+moreAttachments > WebchatMaxSessionAttachmentBytes || extractedSize+moreExtracted > WebchatMaxSessionExtractedBytes {
		return ErrWebchatLimit
	}
	return ErrWebchatConflict
}

func (s *WebchatStore) ReleaseTurn(ctx context.Context, userID, keyID int64, id, lease string) error {
	if !validWebchatUUID(id) {
		return ErrWebchatNotFound
	}
	if !validWebchatUUID(lease) {
		return ErrWebchatConflict
	}
	_, err := s.db.ExecContext(ctx, `UPDATE webchat_sessions SET lease_token=NULL, lease_until=NULL
		WHERE `+webchatScope+` AND lease_token=$4`, userID, keyID, id, lease)
	return err
}

func ValidWebchatText(v string, limit int) bool {
	return len(v) <= limit && strings.TrimSpace(v) != "" && utf8.ValidString(v) && !strings.ContainsRune(v, 0)
}

func (s *WebchatStore) SaveTurn(ctx context.Context, userID, keyID int64, id string, expectedVersion int64, lease, model, userText, assistantText string, attachments ...WebchatAttachment) (*WebchatConversation, error) {
	attachmentOnly := len(attachments) > 0 && userText == ""
	if !ValidWebchatText(model, WebchatMaxModelBytes) || (!attachmentOnly && !ValidWebchatText(userText, WebchatMaxUserBytes)) || !ValidWebchatText(assistantText, WebchatMaxAssistantBytes) {
		return nil, ErrWebchatLimit
	}
	uploads, attachmentSize, extractedSize, err := prepareWebchatAttachments(attachments)
	if err != nil {
		return nil, err
	}
	pair, err := json.Marshal([]WebchatMessage{{Role: "user", Content: userText, Attachments: uploads}, {Role: "assistant", Content: assistantText}})
	if err != nil {
		return nil, err
	}
	if !validWebchatUUID(lease) {
		return nil, ErrWebchatConflict
	}
	textBytes := len(userText) + len(assistantText)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var c *WebchatConversation
	if id == "" {
		if expectedVersion != 0 {
			return nil, ErrWebchatConflict
		}
		title := userText
		if attachmentOnly {
			title = uploads[0].Name
		}
		c, err = saveFirstWebchatTurn(ctx, tx, userID, keyID, lease, model, title, string(pair), textBytes, attachmentSize, extractedSize)
		if errors.Is(err, ErrWebchatNotFound) {
			return nil, ErrWebchatConflict
		}
	} else {
		if !validWebchatUUID(id) {
			return nil, ErrWebchatNotFound
		}
		c, err = scanWebchat(tx.QueryRowContext(ctx, `UPDATE webchat_sessions SET model=$6,
            messages=messages || $7::jsonb,content_bytes=content_bytes+$8,version=version+1,
            attachment_bytes=attachment_bytes+$11,extracted_bytes=extracted_bytes+$12,
            updated_at=clock_timestamp(),lease_token=NULL,lease_until=NULL
            WHERE `+webchatScope+` AND updated_at > clock_timestamp() - `+webchatLifetime+`
            AND version=$4 AND lease_token=$5 AND lease_until>clock_timestamp()
            AND version<$9 AND content_bytes+$8<=$10 AND attachment_bytes+$11<=$13 AND extracted_bytes+$12<=$14
            RETURNING `+webchatColumns+`,messages`, userID, keyID, id, expectedVersion, lease, model, string(pair), textBytes, WebchatMaxTurns, WebchatMaxHistoryBytes, attachmentSize, extractedSize, WebchatMaxSessionAttachmentBytes, WebchatMaxSessionExtractedBytes), true, nil)
		if errors.Is(err, ErrWebchatNotFound) {
			return nil, s.turnRefusal(ctx, tx, userID, keyID, id, textBytes, attachmentSize, extractedSize)
		}
	}
	if err != nil {
		return nil, err
	}
	for _, a := range uploads {
		_, err = tx.ExecContext(ctx, `INSERT INTO webchat_attachments(id,session_id,name,mime_type,data,extracted_text)
            VALUES($1,$2,$3,$4,$5,$6)`, a.ID, c.ID, a.Name, a.MIMEType, a.Data, a.Text)
		if err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return c, nil
}

func prepareWebchatAttachments(files []WebchatAttachment) ([]WebchatAttachment, int, int, error) {
	if len(files) > WebchatMaxAttachments {
		return nil, 0, 0, ErrWebchatLimit
	}
	uploads := make([]WebchatAttachment, 0, len(files))
	var rawSize, textSize int
	for _, a := range files {
		if !ValidWebchatText(a.Name, 255) || !ValidWebchatText(a.MIMEType, 128) || len(a.Data) == 0 || len(a.Data) > WebchatMaxTurnAttachmentBytes || len(a.Text) > WebchatMaxExtractedFileBytes || !utf8.ValidString(a.Text) || strings.ContainsRune(a.Text, 0) {
			return nil, 0, 0, ErrWebchatLimit
		}
		a.Size = len(a.Data)
		rawSize += a.Size
		textSize += len(a.Text)
		if rawSize > WebchatMaxTurnAttachmentBytes {
			return nil, 0, 0, ErrWebchatLimit
		}
		id, err := uuid.NewRandom()
		if err != nil {
			return nil, 0, 0, err
		}
		a.ID = id.String()
		uploads = append(uploads, a)
	}
	return uploads, rawSize, textSize, nil
}

func saveFirstWebchatTurn(ctx context.Context, tx *sql.Tx, userID, keyID int64, lease, model, title, pair string, size, attachmentSize, extractedSize int) (*WebchatConversation, error) {
	runes := []rune(strings.Join(strings.Fields(title), " "))
	if len(runes) > 48 {
		runes = runes[:48]
	}
	// Convert the reservation atomically. Concurrent capacity checks see either the pending start or the completed conversation.
	return scanWebchat(tx.QueryRowContext(ctx, `WITH reserved AS (
        DELETE FROM webchat_start_leases WHERE user_id=$1 AND api_key_id=$2 AND token=$3 AND expires_at>clock_timestamp()
        RETURNING user_id,api_key_id
    ) INSERT INTO webchat_sessions(user_id,api_key_id,title,model,version,messages,content_bytes,attachment_bytes,extracted_bytes)
      SELECT user_id,api_key_id,$4,$5,1,$6::jsonb,$7,$8,$9 FROM reserved
      RETURNING `+webchatColumns+`,messages`, userID, keyID, lease, string(runes), model, pair, size, attachmentSize, extractedSize), true, nil)
}

func (s *WebchatStore) Delete(ctx context.Context, userID, keyID int64, id string) error {
	if !validWebchatUUID(id) {
		return ErrWebchatNotFound
	}
	var deleted string
	err := s.db.QueryRowContext(ctx, "DELETE FROM webchat_sessions WHERE "+webchatScope+" AND (lease_until IS NULL OR lease_until<=clock_timestamp()) RETURNING id", userID, keyID, id).Scan(&deleted)
	if errors.Is(err, sql.ErrNoRows) {
		var exists bool
		err = s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM webchat_sessions WHERE "+webchatScope+")", userID, keyID, id).Scan(&exists)
		if err != nil {
			return err
		}
		if exists {
			return ErrWebchatConflict
		}
		return ErrWebchatNotFound
	}
	return err
}
