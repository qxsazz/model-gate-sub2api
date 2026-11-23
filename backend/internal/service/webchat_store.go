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
	WebchatMaxSessions       = 50
	WebchatMaxTurns          = 32
	WebchatMaxHistoryBytes   = 512 << 10
	WebchatMaxUserBytes      = 32 << 10
	WebchatMaxAssistantBytes = 128 << 10
	WebchatMaxModelBytes     = 256

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

type WebchatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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
	var lease string
	c, err := scanWebchat(s.db.QueryRowContext(ctx, `UPDATE webchat_sessions SET
		lease_token=gen_random_uuid(),lease_until=clock_timestamp()+interval '15 minutes'
		WHERE `+webchatScope+` AND updated_at > clock_timestamp() - `+webchatLifetime+`
		AND version < $4 AND (lease_until IS NULL OR lease_until <= clock_timestamp())
		RETURNING `+webchatColumns+`,messages,lease_token`, userID, keyID, id, WebchatMaxTurns), true, &lease)
	if errors.Is(err, ErrWebchatNotFound) {
		err = s.turnRefusal(ctx, userID, keyID, id, 0)
	}
	return c, lease, err
}

func (s *WebchatStore) turnRefusal(ctx context.Context, userID, keyID int64, id string, moreBytes int) error {
	var count, size int
	err := s.db.QueryRowContext(ctx, "SELECT version,content_bytes FROM webchat_sessions WHERE "+webchatScope, userID, keyID, id).Scan(&count, &size)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrWebchatNotFound
	}
	if err != nil {
		return err
	}
	if count >= WebchatMaxTurns || size+moreBytes > WebchatMaxHistoryBytes {
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

func (s *WebchatStore) SaveTurn(ctx context.Context, userID, keyID int64, id string, expectedVersion int64, lease, model, userText, assistantText string) (*WebchatConversation, error) {
	if !ValidWebchatText(model, WebchatMaxModelBytes) || !ValidWebchatText(userText, WebchatMaxUserBytes) || !ValidWebchatText(assistantText, WebchatMaxAssistantBytes) {
		return nil, ErrWebchatLimit
	}
	pair, err := json.Marshal([]WebchatMessage{{Role: "user", Content: userText}, {Role: "assistant", Content: assistantText}})
	if err != nil {
		return nil, err
	}
	size := len(userText) + len(assistantText)
	if id == "" {
		if expectedVersion != 0 || !validWebchatUUID(lease) {
			return nil, ErrWebchatConflict
		}
		return s.saveFirstTurn(ctx, userID, keyID, lease, model, userText, string(pair), size)
	}
	if !validWebchatUUID(id) {
		return nil, ErrWebchatNotFound
	}
	if !validWebchatUUID(lease) {
		return nil, ErrWebchatConflict
	}
	c, err := scanWebchat(s.db.QueryRowContext(ctx, `UPDATE webchat_sessions SET model=$6,
		messages=messages || $7::jsonb,content_bytes=content_bytes+$8,version=version+1,
		updated_at=clock_timestamp(),lease_token=NULL,lease_until=NULL
		WHERE `+webchatScope+` AND updated_at > clock_timestamp() - `+webchatLifetime+`
		AND version=$4 AND lease_token=$5 AND lease_until>clock_timestamp()
		AND version<$9 AND content_bytes+$8<=$10
		RETURNING `+webchatColumns+`,messages`, userID, keyID, id, expectedVersion, lease, model, string(pair), size, WebchatMaxTurns, WebchatMaxHistoryBytes), true, nil)
	if errors.Is(err, ErrWebchatNotFound) {
		err = s.turnRefusal(ctx, userID, keyID, id, size)
	}
	return c, err
}

func (s *WebchatStore) saveFirstTurn(ctx context.Context, userID, keyID int64, lease, model, prompt, pair string, size int) (*WebchatConversation, error) {
	title := []rune(strings.Join(strings.Fields(prompt), " "))
	if len(title) > 48 {
		title = title[:48]
	}
	// Convert the reservation atomically: a quota check in another transaction
	// sees either the pending start or the completed conversation, never a gap.
	c, err := scanWebchat(s.db.QueryRowContext(ctx, `WITH reserved AS (
        DELETE FROM webchat_start_leases WHERE user_id=$1 AND api_key_id=$2 AND token=$3 AND expires_at>clock_timestamp()
        RETURNING user_id,api_key_id
    ) INSERT INTO webchat_sessions(user_id,api_key_id,title,model,version,messages,content_bytes)
      SELECT user_id,api_key_id,$4,$5,1,$6::jsonb,$7 FROM reserved
      RETURNING `+webchatColumns+`,messages`, userID, keyID, lease, string(title), model, pair, size), true, nil)
	if errors.Is(err, ErrWebchatNotFound) {
		return nil, ErrWebchatConflict
	}
	return c, err
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
