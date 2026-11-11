package repository

import (
	"context"
	"encoding/json"
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.ChatBindingRepository = (*userRepository)(nil)

func (r *userRepository) chatBindingJSON(ctx context.Context, target any, query string, args ...any) error {
	raw, err := r.achievementJSON(ctx, query, args...)
	if err != nil {
		return err
	}
	var failure struct {
		Error      string `json:"error"`
		Replayed   bool   `json:"replayed"`
		RequestKey string `json:"request_key"`
	}
	if len(raw) > 0 && raw[0] == '{' {
		if err = json.Unmarshal(raw, &failure); err != nil {
			return err
		}
		if failure.Error != "" {
			code, message := 400, "绑定确认码无效，请重新获取"
			switch failure.Error {
			case "BOT_BINDING_CHALLENGE_NOT_FOUND", "BOT_BINDING_NOT_FOUND":
				code, message = 404, "绑定记录不存在"
			case "BOT_IDENTITY_ALREADY_BOUND":
				code, message = 409, "该群聊身份已绑定，请先在网站解除原绑定"
			case "BOT_BINDING_CHALLENGE_CLAIMED":
				code, message = 409, "该绑定请求已由其他账号确认，请重新发起"
			case "BOT_BINDING_CHALLENGE_USED":
				code, message = 409, "该绑定请求已使用"
			case "BOT_EVENT_CONFLICT":
				code, message = 409, "同一平台事件的内容发生变化，请使用原请求重试"
			case "BOT_BINDING_CHALLENGE_EXPIRED":
				code, message = 410, "绑定请求已过期或失效，请回到原群重新发送 /绑定"
			case "BOT_BINDING_ACCOUNT_UNAVAILABLE":
				code, message = 403, "当前网站账号不可用"
			case "BOT_BINDING_ACCOUNT_MISMATCH":
				code, message = 403, "请用发起绑定时填写的注册邮箱登录，然后重新打开绑定链接"
			}
			return infraerrors.New(code, failure.Error, message).WithMetadata(map[string]string{"request_key": failure.RequestKey, "replayed": strconv.FormatBool(failure.Replayed)})
		}
	}
	return json.Unmarshal(raw, target)
}

func (r *userRepository) CreateChatBindingChallenge(ctx context.Context, id string, input service.BotBindingChallengeInput) (*service.ChatBindingChallenge, error) {
	identity, err := json.Marshal(input.Identity)
	if err != nil {
		return nil, err
	}
	var result service.ChatBindingChallenge
	err = r.chatBindingJSON(ctx, &result, `SELECT chat_binding_create($1::uuid,$2,$3::jsonb,$4,$5::text)`,
		id, input.Identity.Key(), string(identity), input.Identity.RequestKey("challenge", input.EventID), input.RequestedEmail)
	return &result, err
}

func (r *userRepository) GetChatBindingChallenge(ctx context.Context, userID int64, id string) (*service.ChatBindingChallenge, error) {
	var result service.ChatBindingChallenge
	err := r.chatBindingJSON(ctx, &result, `SELECT COALESCE((SELECT CASE
        WHEN requested_email_hash IS NOT NULL AND requested_email_hash IS DISTINCT FROM
            (SELECT chat_binding_email_hash(email) FROM users WHERE id=$2 AND status='active' AND deleted_at IS NULL)
            THEN jsonb_build_object('error','BOT_BINDING_ACCOUNT_MISMATCH')
        WHEN user_id IS NOT NULL AND user_id<>$2 THEN jsonb_build_object('error','BOT_BINDING_CHALLENGE_CLAIMED')
        ELSE jsonb_build_object('challenge_id',id,'identity',identity,'expires_at',expires_at,
        'binding_mode',CASE WHEN requested_email_hash IS NOT NULL THEN 'email' ELSE 'code' END,'state',
        CASE WHEN consumed_at IS NOT NULL THEN 'completed' WHEN superseded_at IS NOT NULL OR expires_at<=clock_timestamp() OR failed_attempts>=5 THEN 'expired'
        WHEN authorized_at IS NOT NULL THEN 'authorized' ELSE 'pending' END) END
        FROM chat_binding_challenges WHERE id=$1::uuid),jsonb_build_object('error','BOT_BINDING_CHALLENGE_NOT_FOUND'))`, id, userID)
	return &result, err
}

func (r *userRepository) AuthorizeChatBindingChallenge(ctx context.Context, userID int64, id, digest string) (*service.ChatBindingAuthorization, error) {
	var result service.ChatBindingAuthorization
	err := r.chatBindingJSON(ctx, &result, `SELECT chat_binding_authorize($1::uuid,$2,$3)`, id, userID, digest)
	return &result, err
}

func (r *userRepository) ConfirmChatBinding(ctx context.Context, event service.BotIdentityEvent, digest string) (*service.BotReceipt, error) {
	var result service.BotReceipt
	err := r.chatBindingJSON(ctx, &result, `SELECT chat_binding_confirm($1,$2,$3,$4)`, event.Identity.Key(), event.Identity.RequestKey("confirm", event.EventID), event.EventID, digest)
	return &result, err
}

func (r *userRepository) ListChatBindings(ctx context.Context, userID int64) ([]service.ChatBinding, error) {
	var result []service.ChatBinding
	err := r.chatBindingJSON(ctx, &result, `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'identity',identity,'created_at',created_at) ORDER BY id DESC),'[]'::jsonb)
        FROM chat_bindings WHERE user_id=$1 AND revoked_at IS NULL`, userID)
	return result, err
}

func (r *userRepository) RevokeChatBinding(ctx context.Context, userID, bindingID int64) error {
	var result struct {
		Revoked bool `json:"revoked"`
	}
	return r.chatBindingJSON(ctx, &result, `SELECT chat_binding_revoke($1,$2)`, bindingID, userID)
}

func (r *userRepository) ChatBotCheckin(ctx context.Context, input service.BotCheckinInput, day, fingerprint string) (*service.BotCheckinResult, error) {
	var result service.BotCheckinResult
	err := r.chatBindingJSON(ctx, &result, `SELECT chat_bot_checkin($1,$2,$3,$4,$5::date)`, input.Identity.Key(), input.Identity.RequestKey("checkin", input.EventID), input.EventID, fingerprint, day)
	return &result, err
}
