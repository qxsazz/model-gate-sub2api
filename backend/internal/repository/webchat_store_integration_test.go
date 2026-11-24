//go:build integration

package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func webchatTestKey(t *testing.T, uid int64) int64 {
	t.Helper()
	var id int64
	require.NoError(t, integrationDB.QueryRow("INSERT INTO api_keys(user_id,key,name) VALUES($1,$2,'webchat test') RETURNING id", uid, uuid.NewString()).Scan(&id))
	return id
}

func saveWebchatFixture(ctx context.Context, s *service.WebchatStore, uid, key int64, model, prompt, answer string) (*service.WebchatConversation, error) {
	lease, err := s.ReserveStart(ctx, uid, key)
	if err != nil {
		return nil, err
	}
	defer func() { _ = s.ReleaseStart(ctx, uid, key, lease) }()
	return s.SaveTurn(ctx, uid, key, "", 0, lease, model, prompt, answer)
}

func TestWebchatStoreIsolationAndTurnLease(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	db := integrationDB
	s := service.NewWebchatStore(db)
	uid := achievementAccessUser(t, db)
	other := achievementAccessUser(t, db)
	key := webchatTestKey(t, uid)
	secondKey := webchatTestKey(t, uid)
	otherKey := webchatTestKey(t, other)

	_, err := s.ReserveStart(ctx, other, key)
	require.ErrorIs(t, err, service.ErrWebchatNotFound)
	c, err := saveWebchatFixture(ctx, s, uid, key, "model", "  第一行\n第二行 ❤️ ", "hello")
	require.NoError(t, err)
	require.Equal(t, "第一行 第二行 ❤️", c.Title)
	require.Equal(t, 120*time.Hour, c.ExpiresAt.Sub(c.UpdatedAt))
	require.Equal(t, time.UTC, c.ExpiresAt.Location())
	require.Equal(t, "  第一行\n第二行 ❤️ ", c.Messages[0].Content)
	require.Len(t, c.Messages, 2)
	list, err := s.List(ctx, uid, key)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Empty(t, list[0].Messages)
	for _, pair := range [][2]int64{{uid, secondKey}, {other, otherKey}, {other, key}} {
		list, err := s.List(ctx, pair[0], pair[1])
		require.NoError(t, err)
		require.Empty(t, list)
		_, err = s.Get(ctx, pair[0], pair[1], c.ID)
		require.ErrorIs(t, err, service.ErrWebchatNotFound)
		_, _, err = s.BeginTurn(ctx, pair[0], pair[1], c.ID)
		require.ErrorIs(t, err, service.ErrWebchatNotFound)
		require.ErrorIs(t, s.Delete(ctx, pair[0], pair[1], c.ID), service.ErrWebchatNotFound)
	}

	type attempt struct {
		lease string
		err   error
	}
	results := make(chan attempt, 2)
	start := make(chan struct{})
	for range 2 {
		go func() {
			<-start
			_, lease, err := s.BeginTurn(ctx, uid, key, c.ID)
			results <- attempt{lease, err}
		}()
	}
	close(start)
	var winner string
	conflicts := 0
	for range 2 {
		r := <-results
		if errors.Is(r.err, service.ErrWebchatConflict) {
			conflicts++
			continue
		}
		require.NoError(t, r.err)
		winner = r.lease
	}
	require.Equal(t, 1, conflicts)
	require.NotEmpty(t, winner)
	locked, err := s.Get(ctx, uid, key, c.ID)
	require.NoError(t, err)
	require.Equal(t, c.ExpiresAt, locked.ExpiresAt)
	require.ErrorIs(t, s.Delete(ctx, uid, key, c.ID), service.ErrWebchatConflict)
	_, err = s.SaveTurn(ctx, other, key, c.ID, c.Version, winner, "model", "wrong", "owner")
	require.ErrorIs(t, err, service.ErrWebchatNotFound)
	_, err = s.SaveTurn(ctx, uid, key, c.ID, c.Version+1, winner, "model", "stale", "version")
	require.ErrorIs(t, err, service.ErrWebchatConflict)
	_, err = db.Exec("UPDATE webchat_sessions SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1", c.ID)
	require.NoError(t, err)
	reserved, next, err := s.BeginTurn(ctx, uid, key, c.ID)
	require.NoError(t, err)
	require.Equal(t, c.Version, reserved.Version)
	require.NotEqual(t, winner, next)
	require.NoError(t, s.ReleaseTurn(ctx, uid, key, c.ID, winner))
	_, err = s.SaveTurn(ctx, uid, key, c.ID, c.Version, winner, "model", "late", "reply")
	require.ErrorIs(t, err, service.ErrWebchatConflict)
	updated, err := s.SaveTurn(ctx, uid, key, c.ID, c.Version, next, "model2", "next", "second")
	require.NoError(t, err)
	require.Len(t, updated.Messages, 4)
	require.Equal(t, int64(2), updated.Version)
	require.Equal(t, c.CreatedAt, updated.CreatedAt)
	require.True(t, updated.UpdatedAt.After(c.UpdatedAt))
	require.Equal(t, 120*time.Hour, updated.ExpiresAt.Sub(updated.UpdatedAt))
	_, err = s.SaveTurn(ctx, uid, key, c.ID, c.Version, next, "model2", "next", "second")
	require.ErrorIs(t, err, service.ErrWebchatConflict)
	_, lease, err := s.BeginTurn(ctx, uid, key, c.ID)
	require.NoError(t, err)
	require.NoError(t, s.ReleaseTurn(ctx, uid, key, c.ID, lease))
	require.NoError(t, s.Delete(ctx, uid, key, c.ID))
	_, err = s.Get(ctx, uid, key, c.ID)
	require.ErrorIs(t, err, service.ErrWebchatNotFound)
}

func TestWebchatStoreLimitsAndConcurrentFirstTurns(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	db := integrationDB
	s := service.NewWebchatStore(db)
	uid := achievementAccessUser(t, db)
	key := webchatTestKey(t, uid)
	_, err := saveWebchatFixture(ctx, s, uid, key, "model", "\x00", "answer")
	require.ErrorIs(t, err, service.ErrWebchatLimit)
	c, err := saveWebchatFixture(ctx, s, uid, key, "model", "Hi", "answer")
	require.NoError(t, err)
	for i := 1; i < service.WebchatMaxTurns; i++ {
		reserved, lease, err := s.BeginTurn(ctx, uid, key, c.ID)
		require.NoError(t, err)
		_, err = s.SaveTurn(ctx, uid, key, c.ID, reserved.Version, lease, "m", "q", "a")
		require.NoError(t, err)
	}
	_, _, err = s.BeginTurn(ctx, uid, key, c.ID)
	require.ErrorIs(t, err, service.ErrWebchatLimit)

	key2 := webchatTestKey(t, uid)
	prompt := strings.Repeat("u", service.WebchatMaxUserBytes)
	answer := strings.Repeat("a", service.WebchatMaxAssistantBytes)
	c2, err := saveWebchatFixture(ctx, s, uid, key2, "m", prompt, answer)
	require.NoError(t, err)
	for i := 1; i <= 3; i++ {
		version, lease, err := s.BeginTurn(ctx, uid, key2, c2.ID)
		require.NoError(t, err)
		_, err = s.SaveTurn(ctx, uid, key2, c2.ID, version.Version, lease, "m", prompt, answer)
		if i == 3 {
			require.ErrorIs(t, err, service.ErrWebchatLimit)
			break
		}
		require.NoError(t, err)
	}

	_, err = db.Exec(`INSERT INTO webchat_sessions(user_id,api_key_id,title,model,version,messages,content_bytes)
	SELECT $1,$2,'q','m',1,'[{"role":"user","content":"q"},{"role":"assistant","content":"a"}]',2 FROM generate_series(1,$3)`, uid, key, service.WebchatMaxSessions-2)
	require.NoError(t, err)
	lease, err := s.ReserveStart(ctx, uid, key)
	require.NoError(t, err)
	_, err = s.ReserveStart(ctx, uid, key)
	require.ErrorIs(t, err, service.ErrWebchatLimit)
	require.NoError(t, s.ReleaseStart(ctx, uid, key, lease))
	results := make(chan error, 2)
	start := make(chan struct{})
	for range 2 {
		go func() { <-start; _, err := saveWebchatFixture(ctx, s, uid, key, "m", "q", "a"); results <- err }()
	}
	close(start)
	failures := 0
	for range 2 {
		err := <-results
		if errors.Is(err, service.ErrWebchatLimit) {
			failures++
			continue
		}
		require.NoError(t, err)
	}
	require.Equal(t, 1, failures)
	_, err = s.ReserveStart(ctx, uid, key)
	require.ErrorIs(t, err, service.ErrWebchatLimit)
}

func TestWebchatStartLeaseOwnershipAndExpiry(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	s := service.NewWebchatStore(integrationDB)
	uid := achievementAccessUser(t, integrationDB)
	key := webchatTestKey(t, uid)
	otherKey := webchatTestKey(t, uid)
	lease, err := s.ReserveStart(ctx, uid, key)
	require.NoError(t, err)
	list, err := s.List(ctx, uid, key)
	require.NoError(t, err)
	require.Empty(t, list)
	require.NoError(t, s.ReleaseStart(ctx, uid, otherKey, lease))
	_, err = s.SaveTurn(ctx, uid, otherKey, "", 0, lease, "m", "q", "a")
	require.ErrorIs(t, err, service.ErrWebchatConflict)
	_, err = integrationDB.Exec("UPDATE webchat_start_leases SET expires_at=clock_timestamp()-interval '1 second' WHERE token=$1", lease)
	require.NoError(t, err)
	_, err = s.SaveTurn(ctx, uid, key, "", 0, lease, "m", "q", "a")
	require.ErrorIs(t, err, service.ErrWebchatConflict)
	n, err := s.CleanupExpired(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
	lease, err = s.ReserveStart(ctx, uid, key)
	require.NoError(t, err)
	_, err = s.SaveTurn(ctx, uid, key, "", 0, lease, "m", "q", "a")
	require.NoError(t, err)
	_, err = s.SaveTurn(ctx, uid, key, "", 0, lease, "m", "q", "a")
	require.ErrorIs(t, err, service.ErrWebchatConflict)
	list, err = s.List(ctx, uid, key)
	require.NoError(t, err)
	require.Len(t, list, 1)
}

func TestWebchatStoreExpiryAndBatchedCleanup(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	db := integrationDB
	s := service.NewWebchatStore(db)
	uid := achievementAccessUser(t, db)
	key := webchatTestKey(t, uid)
	c, err := saveWebchatFixture(ctx, s, uid, key, "model", "keep?", "expire")
	require.NoError(t, err)
	_, lease, err := s.BeginTurn(ctx, uid, key, c.ID)
	require.NoError(t, err)
	_, err = db.Exec("UPDATE webchat_sessions SET updated_at=clock_timestamp()-interval '120 hours' WHERE id=$1", c.ID)
	require.NoError(t, err)
	_, err = s.Get(ctx, uid, key, c.ID)
	require.ErrorIs(t, err, service.ErrWebchatNotFound)
	_, _, err = s.BeginTurn(ctx, uid, key, c.ID)
	require.ErrorIs(t, err, service.ErrWebchatNotFound)
	_, err = s.SaveTurn(ctx, uid, key, c.ID, c.Version, lease, "model", "resurrect", "?")
	require.ErrorIs(t, err, service.ErrWebchatNotFound)
	require.ErrorIs(t, s.Delete(ctx, uid, key, c.ID), service.ErrWebchatNotFound)
	list, err := s.List(ctx, uid, key)
	require.NoError(t, err)
	require.Empty(t, list)
	fresh, err := saveWebchatFixture(ctx, s, uid, key, "model", "retain", "me")
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO webchat_sessions(user_id,api_key_id,title,model,version,messages,content_bytes,updated_at)
	SELECT user_id,api_key_id,title,model,version,messages,content_bytes,updated_at FROM webchat_sessions CROSS JOIN generate_series(1,260) WHERE id=$1`, c.ID)
	require.NoError(t, err)
	lock, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = lock.Rollback() }()
	_, err = lock.Exec("SELECT id FROM webchat_sessions WHERE id=$1 FOR UPDATE", c.ID)
	require.NoError(t, err)
	var wg sync.WaitGroup
	type batch struct {
		n   int64
		err error
	}
	results := make(chan batch, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); n, err := s.CleanupExpired(ctx); results <- batch{n, err} }()
	}
	wg.Wait()
	close(results)
	var total int64
	for b := range results {
		require.NoError(t, b.err)
		require.LessOrEqual(t, b.n, int64(256))
		total += b.n
	}
	require.Equal(t, int64(260), total)
	require.NoError(t, lock.Commit())
	s.Start()
	s.Start()
	t.Cleanup(s.Stop)
	require.Eventually(t, func() bool {
		var exists bool
		return db.QueryRow("SELECT EXISTS(SELECT 1 FROM webchat_sessions WHERE id=$1)", c.ID).Scan(&exists) == nil && !exists
	}, 5*time.Second, 10*time.Millisecond)
	s.Stop()
	s.Stop()
	_, err = s.Get(ctx, uid, key, fresh.ID)
	require.NoError(t, err)
}

func TestWebchatAttachmentsAtomicAndIsolated(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	db := integrationDB
	s := service.NewWebchatStore(db)
	uid := achievementAccessUser(t, db)
	other := achievementAccessUser(t, db)
	key := webchatTestKey(t, uid)
	otherKey := webchatTestKey(t, uid)
	reservation, err := s.ReserveStart(ctx, uid, key)
	require.NoError(t, err)
	document := service.WebchatAttachment{Name: "提纲.txt", MIMEType: "text/plain", Data: []byte("PRIVATE_RAW_FIXTURE"), Text: "PRIVATE_EXTRACTED_FIXTURE", ID: uuid.NewString()}
	photo := service.WebchatAttachment{Name: "会议照片.png", MIMEType: "image/png", Data: []byte{137, 80, 78, 71}}
	// Verify a failed second file rolls back the first attachment, the history and the lease conversion.
	const forbidden = "webchat-store-rollback-fixture.txt"
	_, err = db.Exec(`ALTER TABLE webchat_attachments ADD CONSTRAINT webchat_attachment_rollback_fixture CHECK(name <> 'webchat-store-rollback-fixture.txt') NOT VALID`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec("ALTER TABLE webchat_attachments DROP CONSTRAINT IF EXISTS webchat_attachment_rollback_fixture")
	})
	invalid := photo
	invalid.Name = forbidden
	_, err = s.SaveTurn(ctx, uid, key, "", 0, reservation, "gpt", "", "answer", document, invalid)
	require.Error(t, err)
	list, err := s.List(ctx, uid, key)
	require.NoError(t, err)
	require.Empty(t, list)
	c, err := s.SaveTurn(ctx, uid, key, "", 0, reservation, "gpt", "", "answer", document, photo)
	require.NoError(t, err)
	require.Equal(t, document.Name, c.Title)
	metas := c.Messages[0].Attachments
	require.Len(t, metas, 2)
	require.NotEqual(t, document.ID, metas[0].ID)
	require.NotEqual(t, metas[0].ID, metas[1].ID)
	require.Equal(t, len(document.Data), metas[0].Size)
	require.Empty(t, metas[0].Data)
	require.Empty(t, metas[0].Text)
	read, err := s.Get(ctx, uid, key, c.ID)
	require.NoError(t, err)
	require.Equal(t, metas, read.Messages[0].Attachments)
	raw, err := json.Marshal(read)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "PRIVATE_")
	require.NotContains(t, string(raw), `"data":`)
	require.NotContains(t, string(raw), `"text":`)
	wrongSession, err := saveWebchatFixture(ctx, s, uid, key, "m", "old", "text only")
	require.NoError(t, err)
	require.Empty(t, wrongSession.Messages[0].Attachments)
	for _, scope := range []struct {
		uid, key int64
		session  string
	}{{uid, otherKey, c.ID}, {other, key, c.ID}, {uid, key, wrongSession.ID}} {
		_, err = s.GetAttachment(ctx, scope.uid, scope.key, scope.session, metas[0].ID)
		require.ErrorIs(t, err, service.ErrWebchatNotFound)
	}
	download, err := s.GetAttachment(ctx, uid, key, c.ID, metas[0].ID)
	require.NoError(t, err)
	require.Equal(t, document.Data, download.Data)
	history, lease, err := s.BeginTurn(ctx, uid, key, c.ID)
	require.NoError(t, err)
	require.Equal(t, document.Text, history.Messages[0].Attachments[0].Text)
	require.Equal(t, photo.Data, history.Messages[0].Attachments[1].Data)
	raw, err = json.Marshal(history)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "PRIVATE_")
	_, err = s.SaveTurn(ctx, uid, key, c.ID, history.Version, lease, "m", "another", "ans", photo, invalid)
	require.Error(t, err)
	read, err = s.Get(ctx, uid, key, c.ID)
	require.NoError(t, err)
	require.Equal(t, c.UpdatedAt, read.UpdatedAt)
	require.Equal(t, c.Version, read.Version)
	var count int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM webchat_attachments WHERE session_id=$1", c.ID).Scan(&count))
	require.Equal(t, 2, count)
	// An old plain-text session can accept a new attachment on its next turn.
	history2, lease2, err := s.BeginTurn(ctx, uid, key, wrongSession.ID)
	require.NoError(t, err)
	got, err := s.SaveTurn(ctx, uid, key, wrongSession.ID, history2.Version, lease2, "m", "look", "ok", photo)
	require.NoError(t, err)
	attached := got.Messages[2].Attachments[0].ID
	require.NoError(t, s.Delete(ctx, uid, key, wrongSession.ID))
	require.NoError(t, db.QueryRow("SELECT count(*) FROM webchat_attachments WHERE id=$1", attached).Scan(&count))
	require.Zero(t, count)
	_, err = db.Exec("UPDATE webchat_sessions SET updated_at=clock_timestamp()-interval '120 hours' WHERE id=$1", c.ID)
	require.NoError(t, err)
	_, err = s.GetAttachment(ctx, uid, key, c.ID, metas[0].ID)
	require.ErrorIs(t, err, service.ErrWebchatNotFound)
	_, err = s.CleanupExpired(ctx)
	require.NoError(t, err)
	require.NoError(t, db.QueryRow("SELECT count(*) FROM webchat_attachments WHERE session_id=$1", c.ID).Scan(&count))
	require.Zero(t, count)
}

func TestWebchatAttachmentLimits(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	db := integrationDB
	s := service.NewWebchatStore(db)
	uid := achievementAccessUser(t, db)
	key := webchatTestKey(t, uid)
	lease, err := s.ReserveStart(ctx, uid, key)
	require.NoError(t, err)
	file := service.WebchatAttachment{Name: "image.webp", MIMEType: "image/webp", Data: bytes.Repeat([]byte("F"), service.WebchatMaxTurnAttachmentBytes)}
	small := service.WebchatAttachment{Name: "brief.txt", MIMEType: "text/plain", Data: []byte("a")}
	for _, files := range [][]service.WebchatAttachment{{file, small}, {small, small, small, small}} {
		_, err = s.SaveTurn(ctx, uid, key, "", 0, lease, "m", "", "ok", files...)
		require.ErrorIs(t, err, service.ErrWebchatLimit)
	}
	c, err := s.SaveTurn(ctx, uid, key, "", 0, lease, "m", "", "a", file)
	require.NoError(t, err)
	for range 2 {
		c, lease, err = s.BeginTurn(ctx, uid, key, c.ID)
		require.NoError(t, err)
		c, err = s.SaveTurn(ctx, uid, key, c.ID, c.Version, lease, "m", "", "a", file)
		require.NoError(t, err)
	}
	c, lease, err = s.BeginTurn(ctx, uid, key, c.ID)
	require.NoError(t, err)
	_, err = s.SaveTurn(ctx, uid, key, c.ID, c.Version, lease, "m", "", "a", small)
	require.ErrorIs(t, err, service.ErrWebchatLimit)
	// A text-only turn still works when binary retention has reached 12 MiB.
	_, err = s.SaveTurn(ctx, uid, key, c.ID, c.Version, lease, "m", "text only", "a")
	require.NoError(t, err)
	lease, err = s.ReserveStart(ctx, uid, key)
	require.NoError(t, err)
	small.Text = strings.Repeat("e", service.WebchatMaxExtractedFileBytes+1)
	_, err = s.SaveTurn(ctx, uid, key, "", 0, lease, "m", "", "ok", small)
	require.ErrorIs(t, err, service.ErrWebchatLimit)
	small.Text = small.Text[1:]
	c, err = s.SaveTurn(ctx, uid, key, "", 0, lease, "m", "", "ok", small, small, small)
	require.NoError(t, err)
	c, lease, err = s.BeginTurn(ctx, uid, key, c.ID)
	require.NoError(t, err)
	_, err = s.SaveTurn(ctx, uid, key, c.ID, c.Version, lease, "m", "", "ok", small, small)
	require.ErrorIs(t, err, service.ErrWebchatLimit)
	_, err = s.SaveTurn(ctx, uid, key, c.ID, c.Version, lease, "m", "", "ok", small)
	require.NoError(t, err)
}
