//go:build integration

package repository

import (
	"context"
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
