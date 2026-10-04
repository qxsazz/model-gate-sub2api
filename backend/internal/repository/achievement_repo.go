package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

var _ service.AchievementRepository = (*userRepository)(nil)

func (r *userRepository) SaveAchievementZodiac(ctx context.Context, id int64, zodiac string) error {
	result, err := r.sql.ExecContext(ctx, `INSERT INTO achievement_preferences(user_id,zodiac)
 SELECT id,$2 FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL AND role IN ('user','admin')
 ON CONFLICT(user_id) DO UPDATE SET zodiac=EXCLUDED.zodiac,updated_at=clock_timestamp()`, id, zodiac)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return infraerrors.Forbidden("ACHIEVEMENT_ACCOUNT_UNAVAILABLE", "当前账户不可用")
	}
	return nil
}

func achievementError(err error) error {
	var pg *pq.Error
	if errors.As(err, &pg) {
		switch pg.Message {
		case "ACHIEVEMENT_CARD_DATE_INVALID":
			return infraerrors.BadRequest(pg.Message, "只能补签注册后、最近 30 天内的漏签日期，不含今天")
		case "ACHIEVEMENT_CARD_INSUFFICIENT":
			return infraerrors.BadRequest(pg.Message, "补签卡不足，完成活动成就后可领取")
		case "ACHIEVEMENT_CARD_CONFLICT":
			return infraerrors.Conflict(pg.Message, "请求编号已用于其他补签，请重新核验日期")
		case "ACHIEVEMENT_CHECKIN_CASH_UNAVAILABLE":
			return infraerrors.BadRequest(pg.Message, "当前签到金额奖励未对该账户开放，未扣除补签卡")
		case "ACHIEVEMENT_ADMIN_FORBIDDEN":
			return infraerrors.Forbidden(pg.Message, "需要有效管理员权限")
		case "ACHIEVEMENT_ADMIN_CONFLICT":
			return infraerrors.Conflict(pg.Message, "该请求编号已用于不同的管理操作")
		case "ACHIEVEMENT_HISTORY_CHANGED":
			return infraerrors.Conflict(pg.Message, "历史权益已变化，请重新核验金额后确认补签")
		case "ACHIEVEMENT_HISTORY_UNAVAILABLE":
			return infraerrors.BadRequest(pg.Message, "历史权益无法完整核验，或所选日期不可补签，请核对资料")
		case "ACHIEVEMENT_ADMIN_REASON":
			return infraerrors.BadRequest(pg.Message, "请填写操作原因")
		case "ACTIVITY_REQUEST_CONFLICT":
			return infraerrors.Conflict(pg.Message, "请求编号已用于不同的答题提交")
		case "ACTIVITY_ATTEMPT_LIMIT":
			return infraerrors.TooManyRequests(pg.Message, "24 小时内最多提交 30 次，请稍后再试")
		case "ACHIEVEMENT_DATE_CHANGED":
			return infraerrors.Conflict(pg.Message, "日期已切换，请刷新后重新签到")
		case "ACHIEVEMENT_LOCKED":
			return infraerrors.BadRequest(pg.Message, "尚未达到该成就的条件")
		case "ACHIEVEMENT_CASH_UNAVAILABLE":
			return infraerrors.BadRequest(pg.Message, "当前账户未开放成就现金奖励")
		case "ACHIEVEMENT_BUDGET_EXHAUSTED":
			return infraerrors.Conflict(pg.Message, "本期奖励预算已用完")
		case "ACHIEVEMENT_ACCOUNT_UNAVAILABLE":
			return infraerrors.Forbidden(pg.Message, "当前账户不可用")
		}
	}
	return err
}

func (r *userRepository) SaveActivityAttempt(ctx context.Context, id int64, kind, topic, attempt, idem string, answers json.RawMessage, grade service.ActivityGrade) (json.RawMessage, error) {
	return r.achievementJSON(ctx, `SELECT achievement_activity_attempt($1,$2,$3,$4::uuid,$5,$6::jsonb,$7,$8)`, id, kind, topic, attempt, idem, string(answers), grade.Score, grade.Passed)
}
func (r *userRepository) achievementJSON(ctx context.Context, query string, args ...any) (json.RawMessage, error) {
	rows, e := r.sql.QueryContext(ctx, query, args...)
	if e != nil {
		return nil, achievementError(e)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, achievementError(err)
		}
		return nil, service.ErrUserNotFound
	}
	var raw []byte
	if e = rows.Scan(&raw); e != nil {
		return nil, e
	}
	return json.RawMessage(raw), rows.Err()
}
func (r *userRepository) AchievementSnapshot(ctx context.Context, id int64) (json.RawMessage, error) {
	return r.achievementJSON(ctx, `WITH d AS (SELECT (clock_timestamp() AT TIME ZONE 'Asia/Shanghai')::date AS today),c AS (SELECT * FROM achievement_config WHERE id=true)
 SELECT jsonb_build_object(
 'date',d.today,'timezone','Asia/Shanghai','tier',achievement_tier($1),
 'joined_date',(u.created_at AT TIME ZONE 'Asia/Shanghai')::date,
 'companionship_days',GREATEST(d.today-(u.created_at AT TIME ZONE 'Asia/Shanghai')::date+1,1),
 'zodiac',COALESCE((SELECT zodiac FROM achievement_preferences WHERE user_id=$1),''),
 'daily_amount',(SELECT amount FROM achievement_daily_policy WHERE tier=achievement_tier($1)),
 'daily_rewards',(SELECT jsonb_agg(amount ORDER BY tier) FROM achievement_daily_policy),
 'card_balance',achievement_card_balance($1),
 'card_min_date',GREATEST((u.created_at AT TIME ZONE 'Asia/Shanghai')::date,d.today-30),'card_max_date',d.today-1,
 'card_history',COALESCE((SELECT jsonb_agg(to_jsonb(l) ORDER BY id DESC) FROM (SELECT * FROM achievement_card_ledger WHERE user_id=$1 ORDER BY id DESC LIMIT 20)l),'[]'::jsonb),
 'series',(SELECT jsonb_agg(to_jsonb(s)||jsonb_build_object('total',cardinality(s.members),'preview',EXISTS(SELECT 1 FROM achievement_catalog WHERE key=ANY(s.members) AND preview),'prior_amount',(SELECT COALESCE(sum(gross),0) FROM achievement_claims WHERE user_id=$1 AND key=ANY(s.members)),'claimable_amount',GREATEST(s.reward-(SELECT COALESCE(sum(gross),0) FROM achievement_claims WHERE user_id=$1 AND key=ANY(s.members)),0),'collected',(SELECT count(*) FROM unnest(s.members) AS m(key) JOIN achievement_catalog cat USING(key) WHERE NOT cat.preview AND achievement_is_unlocked($1,m.key)), 'unlocked',NOT EXISTS(SELECT 1 FROM unnest(s.members) AS m(key) LEFT JOIN achievement_catalog cat USING(key) WHERE cat.key IS NULL OR cat.preview OR NOT achievement_is_unlocked($1,m.key)),'claim',(SELECT to_jsonb(cl) FROM achievement_series_claims cl WHERE cl.user_id=$1 AND cl.key=s.key)) ORDER BY s.key) FROM achievement_series s),
 'cash_reason',achievement_checkin_cash_reason($1,c.payload),'milestone_cash_enabled',COALESCE((c.payload->>'milestone_cash_enabled')::boolean,false),
 'milestone_cash_reason',achievement_cash_reason($1,jsonb_set(c.payload,'{cash_enabled}',COALESCE(c.payload->'milestone_cash_enabled','false'))),
 'today',(SELECT to_jsonb(ch) FROM achievement_checkins ch WHERE user_id=$1 AND day=d.today),
 'total_days',(SELECT count(*) FROM achievement_checkins WHERE user_id=$1),
 'streak',COALESCE((SELECT streak FROM achievement_checkins WHERE user_id=$1 AND day>=d.today-1 ORDER BY day DESC LIMIT 1),0),
 'longest',achievement_progress($1,'S01'),
 'calendar',COALESCE((SELECT jsonb_agg(day ORDER BY day) FROM achievement_checkins WHERE user_id=$1 AND day>=LEAST(date_trunc('month',d.today)::date,d.today-30)),'[]'::jsonb),
 'history',COALESCE((SELECT jsonb_agg(to_jsonb(h) ORDER BY day DESC) FROM (SELECT * FROM achievement_checkins WHERE user_id=$1 ORDER BY day DESC LIMIT 10) h),'[]'::jsonb),
 'equipment',(SELECT eq.key FROM achievement_equipment eq WHERE eq.user_id=$1 AND achievement_is_unlocked($1,eq.key)),
 'passes',COALESCE((SELECT jsonb_agg(jsonb_build_object('kind',kind,'topic',topic)) FROM achievement_activity_passes WHERE user_id=$1),'[]'::jsonb),
 'medals',(SELECT jsonb_agg(to_jsonb(cat)||jsonb_build_object('progress',achievement_progress($1,cat.key),'unlocked',achievement_is_unlocked($1,cat.key),'manual',(SELECT ov.state FROM achievement_overrides ov WHERE ov.user_id=$1 AND ov.key=cat.key),'claim',(SELECT to_jsonb(cl) FROM achievement_claims cl WHERE cl.user_id=$1 AND cl.key=cat.key),'card_claim',(SELECT to_jsonb(cc) FROM achievement_card_claims cc WHERE cc.user_id=$1 AND cc.key=cat.key)) ORDER BY cat.key) FROM achievement_catalog cat)
 ) FROM d,c,users u WHERE u.id=$1 AND u.deleted_at IS NULL`, id)
}

func (r *userRepository) AdminAchievementSnapshot(ctx context.Context, id int64) (json.RawMessage, error) {
	snapshot, e := r.AchievementSnapshot(ctx, id)
	if e != nil {
		return nil, e
	}
	return r.achievementJSON(ctx, `SELECT $2::jsonb||jsonb_build_object('user',jsonb_build_object('id',u.id,'email',u.email,'username',u.username,'balance',u.balance,'status',u.status,'created_at',u.created_at),'operations',COALESCE((SELECT jsonb_agg(to_jsonb(o) - 'request' ORDER BY id DESC) FROM (SELECT * FROM achievement_operations WHERE user_id=$1 ORDER BY id DESC LIMIT 50)o),'[]'::jsonb)) FROM users u WHERE u.id=$1 AND u.deleted_at IS NULL`, id, string(snapshot))
}
func (r *userRepository) AchievementBackfillPreview(ctx context.Context, id int64, date string) (json.RawMessage, error) {
	return r.achievementJSON(ctx, `SELECT achievement_backfill_preview($1,$2::date)`, id, date)
}
func (r *userRepository) AdminAchievementMutation(ctx context.Context, actor, id int64, action string, c service.AchievementAdminCommand) (json.RawMessage, error) {
	raw, e := json.Marshal(c)
	if e != nil {
		return nil, e
	}
	return r.achievementJSON(ctx, `SELECT achievement_admin_operation($1,$2,$3,$4::jsonb)`, actor, id, action, string(raw))
}
func (r *userRepository) AchievementAudit(ctx context.Context) (json.RawMessage, error) {
	return r.achievementJSON(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(o)-'request' ORDER BY id DESC),'[]'::jsonb) FROM (SELECT * FROM achievement_operations ORDER BY id DESC LIMIT 100)o`)
}
func (r *userRepository) AchievementMutation(ctx context.Context, id int64, action, key, date, idem string) (json.RawMessage, error) {
	switch action {
	case "checkin":
		return r.achievementJSON(ctx, `SELECT achievement_checkin($1,$2,$3)`, id, date, idem)
	case "claim":
		return r.achievementJSON(ctx, `SELECT achievement_claim($1,$2)`, id, key)
	case "claim_series":
		return r.achievementJSON(ctx, `SELECT achievement_claim_series($1,$2)`, id, key)
	case "equip":
		return r.achievementJSON(ctx, `SELECT jsonb_build_object('saved',true) FROM achievement_equip($1,$2)`, id, key)
	default:
		return nil, fmt.Errorf("unknown achievement action")
	}
}
func (r *userRepository) AchievementCardPreview(ctx context.Context, id int64, date string) (json.RawMessage, error) {
	return r.achievementJSON(ctx, `SELECT achievement_card_preview($1,$2::date)`, id, date)
}
func (r *userRepository) UseAchievementCard(ctx context.Context, id int64, c service.AchievementCardCommand) (json.RawMessage, error) {
	raw, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	return r.achievementJSON(ctx, `SELECT achievement_use_card($1,$2::jsonb)`, id, string(raw))
}
func (r *userRepository) AchievementConfig(ctx context.Context) (*service.AchievementConfig, error) {
	raw, e := r.achievementJSON(ctx, `SELECT payload||jsonb_build_object('daily_rewards',(SELECT jsonb_agg(amount ORDER BY tier) FROM achievement_daily_policy)) FROM achievement_config WHERE id=true`)
	if e != nil {
		return nil, e
	}
	var c service.AchievementConfig
	e = json.Unmarshal(raw, &c)
	return &c, e
}
func (r *userRepository) SaveAchievementConfig(ctx context.Context, actor int64, c service.AchievementConfig) error {
	raw, e := json.Marshal(c)
	if e != nil {
		return e
	}
	_, e = r.sql.ExecContext(ctx, `WITH old AS MATERIALIZED (SELECT payload,revision FROM achievement_config WHERE id=true FOR UPDATE), changed AS (UPDATE achievement_config SET payload=$1::jsonb,revision=achievement_config.revision+1,updated_at=now() FROM old WHERE id=true RETURNING achievement_config.payload,achievement_config.revision) INSERT INTO achievement_admin_audit(actor_id,previous,current,revision) SELECT $2,old.payload,changed.payload,changed.revision FROM old,changed`, string(raw), actor)
	return e
}
