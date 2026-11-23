package repository

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestWebchatTicketCanOnlyBeConsumedOnceAcrossReplicas(t *testing.T) {
	s := miniredis.RunT(t)
	a := redis.NewClient(&redis.Options{Addr: s.Addr()})
	b := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { _ = a.Close(); _ = b.Close() })
	sender, receiver := NewWebchatTickets(a), NewWebchatTickets(b)
	ctx := context.Background()
	ok, err := sender.Issue(ctx, "turn", []byte("bound-turn"), time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = receiver.Issue(ctx, "turn", []byte("new-turn"), time.Minute)
	require.NoError(t, err)
	require.False(t, ok)
	var winners atomic.Int32
	var racers sync.WaitGroup
	for i := 0; i < 10; i++ {
		racers.Go(func() {
			v, err := receiver.Take(ctx, "turn")
			if err != nil {
				t.Error(err)
				return
			}
			if string(v) == "bound-turn" {
				winners.Add(1)
			}
		})
	}
	racers.Wait()
	require.EqualValues(t, 1, winners.Load())
	ok, err = sender.Issue(ctx, "stale", []byte("turn"), time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	s.FastForward(2 * time.Minute)
	v, err := receiver.Take(ctx, "stale")
	require.NoError(t, err)
	require.Nil(t, v)
}
