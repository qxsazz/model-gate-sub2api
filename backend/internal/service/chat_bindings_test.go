package service

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type chatBindingServiceStub struct {
	UserRepository
	ChatBindingRepository
	input  BotCheckinInput
	day    string
	result *BotCheckinResult
	err    error
}

func (s *chatBindingServiceStub) ChatBotCheckin(_ context.Context, in BotCheckinInput, day, _ string) (*BotCheckinResult, error) {
	s.input = in
	s.day = day
	return s.result, s.err
}

type chatBalanceCacheStub struct {
	BillingCache
	ids []int64
}

func (s *chatBalanceCacheStub) InvalidateUserBalance(_ context.Context, id int64) error {
	s.ids = append(s.ids, id)
	return nil
}

func TestChatBotUsesOriginalReceiptAccountAndShanghaiTime(t *testing.T) {
	uid := int64(17)
	repo := &chatBindingServiceStub{result: &BotCheckinResult{Receipt: BotReceipt{Status: "checked_in", Replayed: true}, CacheUserID: &uid}}
	cache := &chatBalanceCacheStub{}
	s := NewUserService(repo, nil, nil, cache)
	input := BotCheckinInput{BotIdentityEvent: BotIdentityEvent{Identity: ChatIdentity{Platform: "qq_official", BotID: "b", GroupID: "g", SenderID: "member_openid"}, EventID: "event"}, EventAt: time.Date(2026, 11, 11, 16, 0, 0, 0, time.UTC)}
	_, err := s.ChatBotCheckin(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, "2026-11-12", repo.day)
	require.Equal(t, []int64{17}, cache.ids)
	repo.result.Receipt.Status = "ineligible"
	_, err = s.ChatBotCheckin(context.Background(), input)
	require.NoError(t, err)
	require.Len(t, cache.ids, 1)
	repo.err = errors.New("transaction rolled back")
	_, err = s.ChatBotCheckin(context.Background(), input)
	require.Error(t, err)
	require.Len(t, cache.ids, 1)
	input.EventAt = time.Time{}
	_, err = s.ChatBotCheckin(context.Background(), input)
	require.Error(t, err)
}

func TestChatIdentityKeysSeparatePlatformsAndOpaqueBoundaries(t *testing.T) {
	a := ChatIdentity{Platform: "qq_official", BotID: "a:b", GroupID: "c", SenderID: "d"}
	b := ChatIdentity{Platform: "qq_official", BotID: "a", GroupID: "b:c", SenderID: "d"}
	require.NoError(t, a.Validate())
	require.NotEqual(t, a.Key(), b.Key())
	require.Len(t, a.RequestKey("checkin", "event"), 68)
	require.NotEqual(t, a.RequestKey("confirm", "event"), a.RequestKey("checkin", "event"))
	for _, invalid := range []string{"", "\n", "has space", "hidden\u200b"} {
		b.SenderID = invalid
		require.Error(t, b.Validate())
	}
	first, err := chatCodeHash("ABCD-EFGH-IJKL-MNOP")
	require.NoError(t, err)
	second, err := chatCodeHash("abcdefghijklmnop")
	require.NoError(t, err)
	require.Equal(t, first, second)
	_, err = chatCodeHash("123456")
	require.Error(t, err)
}
