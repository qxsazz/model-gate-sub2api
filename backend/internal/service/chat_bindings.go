package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

type ChatIdentity struct {
	Platform string `json:"platform"`
	BotID    string `json:"bot_id"`
	GroupID  string `json:"group_id"`
	SenderID string `json:"sender_id"`
}

func validChatID(value string) bool {
	return value != "" && len(value) <= 200 && utf8.ValidString(value) && !strings.ContainsFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r) || unicode.In(r, unicode.Cf)
	})
}

func (i ChatIdentity) Validate() error {
	if (i.Platform != "qq_official" && i.Platform != "wechat_native") || !validChatID(i.BotID) || !validChatID(i.GroupID) || !validChatID(i.SenderID) {
		return infraerrors.BadRequest("BOT_IDENTITY_INVALID", "请提供平台群事件的完整身份信息")
	}
	return nil
}

// JSON arrays make boundaries unambiguous even when opaque IDs contain separators.
func chatDigest(values ...string) string {
	encoded, _ := json.Marshal(values)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func (i ChatIdentity) Key() string { return chatDigest(i.Platform, i.BotID, i.GroupID, i.SenderID) }

func (i ChatIdentity) RequestKey(operation, eventID string) string {
	return "bot:" + chatDigest(operation, i.Platform, i.BotID, i.GroupID, i.SenderID, eventID)
}

type BotIdentityEvent struct {
	Identity ChatIdentity `json:"identity"`
	EventID  string       `json:"event_id"`
}

func (e BotIdentityEvent) Validate() error {
	if err := e.Identity.Validate(); err != nil {
		return err
	}
	if !validChatID(e.EventID) {
		return infraerrors.BadRequest("BOT_EVENT_INVALID", "请提供平台消息事件编号")
	}
	return nil
}

type BotConfirmInput struct {
	BotIdentityEvent
	Code string `json:"code"`
}

type BotCheckinInput struct {
	BotIdentityEvent
	EventAt time.Time `json:"event_at"`
}

type ChatBindingChallenge struct {
	ChallengeID string       `json:"challenge_id"`
	Identity    ChatIdentity `json:"identity,omitempty"`
	ExpiresAt   time.Time    `json:"expires_at"`
	State       string       `json:"state,omitempty"`
	Replayed    bool         `json:"replayed"`
}

type ChatBindingAuthorization struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

type ChatBinding struct {
	ID        int64        `json:"id"`
	Identity  ChatIdentity `json:"identity"`
	CreatedAt time.Time    `json:"created_at"`
}

type BotReceipt struct {
	Status      string `json:"status"`
	RequestKey  string `json:"request_key"`
	Replayed    bool   `json:"replayed"`
	CheckinDate string `json:"checkin_date,omitempty"`
}

type BotCheckinResult struct {
	Receipt     BotReceipt `json:"receipt"`
	CacheUserID *int64     `json:"cache_user_id"`
}

type ChatBindingRepository interface {
	CreateChatBindingChallenge(context.Context, string, BotIdentityEvent) (*ChatBindingChallenge, error)
	GetChatBindingChallenge(context.Context, int64, string) (*ChatBindingChallenge, error)
	AuthorizeChatBindingChallenge(context.Context, int64, string, string) (*ChatBindingAuthorization, error)
	ConfirmChatBinding(context.Context, BotIdentityEvent, string) (*BotReceipt, error)
	ListChatBindings(context.Context, int64) ([]ChatBinding, error)
	RevokeChatBinding(context.Context, int64, int64) error
	ChatBotCheckin(context.Context, BotCheckinInput, string, string) (*BotCheckinResult, error)
}

func (s *UserService) chatBindingRepo() (ChatBindingRepository, error) {
	r, ok := s.userRepo.(ChatBindingRepository)
	if !ok {
		return nil, infraerrors.New(503, "BOT_INTEGRATION_UNAVAILABLE", "群聊接入暂不可用")
	}
	return r, nil
}

func (s *UserService) CreateChatBindingChallenge(ctx context.Context, event BotIdentityEvent) (*ChatBindingChallenge, error) {
	if err := event.Validate(); err != nil {
		return nil, err
	}
	r, err := s.chatBindingRepo()
	if err != nil {
		return nil, err
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	return r.CreateChatBindingChallenge(ctx, id.String(), event)
}

func validateChatChallengeID(id string) error {
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.String() != id {
		return infraerrors.BadRequest("BOT_BINDING_CHALLENGE_INVALID", "绑定链接无效")
	}
	return nil
}

func (s *UserService) GetChatBindingChallenge(ctx context.Context, userID int64, id string) (*ChatBindingChallenge, error) {
	if err := validateChatChallengeID(id); err != nil {
		return nil, err
	}
	r, err := s.chatBindingRepo()
	if err != nil {
		return nil, err
	}
	return r.GetChatBindingChallenge(ctx, userID, id)
}

func chatCodeHash(code string) (string, error) {
	code = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))
	if len(code) != 16 || strings.ContainsFunc(code, func(r rune) bool { return !(r >= 'A' && r <= 'Z' || r >= '2' && r <= '7') }) {
		return "", infraerrors.BadRequest("BOT_BINDING_CODE_INVALID", "确认码无效，请从网站重新获取")
	}
	digest := sha256.Sum256([]byte(code))
	return hex.EncodeToString(digest[:]), nil
}

func (s *UserService) AuthorizeChatBindingChallenge(ctx context.Context, userID int64, id string) (*ChatBindingAuthorization, error) {
	if err := validateChatChallengeID(id); err != nil {
		return nil, err
	}
	r, err := s.chatBindingRepo()
	if err != nil {
		return nil, err
	}
	var entropy [10]byte
	if _, err = rand.Read(entropy[:]); err != nil {
		return nil, err
	}
	code := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(entropy[:])
	digest, err := chatCodeHash(code)
	if err != nil {
		return nil, err
	}
	result, err := r.AuthorizeChatBindingChallenge(ctx, userID, id, digest)
	if err != nil {
		return nil, err
	}
	result.Code = code[:4] + "-" + code[4:8] + "-" + code[8:12] + "-" + code[12:]
	return result, nil
}

func (s *UserService) ConfirmChatBinding(ctx context.Context, input BotConfirmInput) (*BotReceipt, error) {
	if err := input.BotIdentityEvent.Validate(); err != nil {
		return nil, err
	}
	digest, err := chatCodeHash(input.Code)
	if err != nil {
		return nil, err
	}
	r, err := s.chatBindingRepo()
	if err != nil {
		return nil, err
	}
	return r.ConfirmChatBinding(ctx, input.BotIdentityEvent, digest)
}

func (s *UserService) ListChatBindings(ctx context.Context, userID int64) ([]ChatBinding, error) {
	r, err := s.chatBindingRepo()
	if err != nil {
		return nil, err
	}
	return r.ListChatBindings(ctx, userID)
}

func (s *UserService) RevokeChatBinding(ctx context.Context, userID, bindingID int64) error {
	r, err := s.chatBindingRepo()
	if err != nil {
		return err
	}
	return r.RevokeChatBinding(ctx, userID, bindingID)
}

func (s *UserService) ChatBotCheckin(ctx context.Context, input BotCheckinInput) (*BotReceipt, error) {
	if err := input.BotIdentityEvent.Validate(); err != nil {
		return nil, err
	}
	if input.EventAt.IsZero() {
		return nil, infraerrors.BadRequest("BOT_EVENT_INVALID", "请提供平台消息的 event_at 时间")
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return nil, err
	}
	day := input.EventAt.In(location).Format("2006-01-02")
	fingerprint := chatDigest(input.EventAt.UTC().Format(time.RFC3339Nano))
	r, err := s.chatBindingRepo()
	if err != nil {
		return nil, err
	}
	result, err := r.ChatBotCheckin(ctx, input, day, fingerprint)
	if err != nil {
		return nil, err
	}
	// The SQL function invokes the same achievement_checkin write path as the
	// website. Only invalidate after the receipt and debit transaction committed.
	if result.CacheUserID != nil && (result.Receipt.Status == "checked_in" || result.Receipt.Status == "already_checked_in") {
		s.invalidateAchievementBalanceCaches(ctx, *result.CacheUserID)
	}
	return &result.Receipt, nil
}
