package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

type webchatTickets struct{ redis *redis.Client }

func NewWebchatTickets(client *redis.Client) service.WebchatTicketCache {
	if client == nil {
		return nil
	}
	return &webchatTickets{redis: client}
}

const webchatTicketPrefix = "webchat:turn:"

func (s *webchatTickets) Issue(ctx context.Context, id string, record []byte, ttl time.Duration) (bool, error) {
	return s.redis.SetNX(ctx, webchatTicketPrefix+id, record, ttl).Result()
}

func (s *webchatTickets) Take(ctx context.Context, id string) ([]byte, error) {
	record, err := s.redis.GetDel(ctx, webchatTicketPrefix+id).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	return record, err
}

func (s *webchatTickets) Pending(ctx context.Context, id string) (bool, error) {
	found, err := s.redis.Exists(ctx, webchatTicketPrefix+id).Result()
	return found != 0, err
}

func (s *webchatTickets) Drop(ctx context.Context, id string) error {
	return s.redis.Del(ctx, webchatTicketPrefix+id).Err()
}
