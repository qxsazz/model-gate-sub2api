package repository

import (
	"context"
	"errors"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func (r *userRepository) VIPSetLevel(ctx context.Context, actor, id int64, c *service.VIPLevelCommand, reason string) error {
	var level, expiry any
	if c != nil {
		level = c.Level
		expiry = c.ExpiresAt
	}
	_, err := r.sql.ExecContext(ctx, `SELECT vip_set_user_level($1,$2,$3,$4,$5)`, actor, id, level, expiry, reason)
	var db *pq.Error
	if errors.As(err, &db) {
		switch db.Message {
		case "VIP_LEVEL_ADMIN_FORBIDDEN":
			return infraerrors.Forbidden(db.Message, "需要有效管理员权限")
		case "VIP_LEVEL_ACCOUNT_UNAVAILABLE":
			return infraerrors.BadRequest(db.Message, "目标用户不可用")
		case "VIP_LEVEL_INVALID":
			return infraerrors.BadRequest(db.Message, "等级、到期时间或操作原因无效")
		}
	}
	return err
}
