//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestVIPHistoricalBackfillIsLinkedIdempotentAndDoesNotCreditBalance(t *testing.T) {
	_ = testEntClient(t)
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	var uid, actor, order int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,balance) VALUES($1,'fixture',10) RETURNING id`, fmt.Sprintf("backfill-%d@example.com", time.Now().UnixNano())).Scan(&uid))
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,role) VALUES($1,'fixture','admin') RETURNING id`, fmt.Sprintf("backfill-admin-%d@example.com", time.Now().UnixNano())).Scan(&actor))
	_, err = tx.ExecContext(ctx, `UPDATE vip_rules SET payload=jsonb_set(payload,'{enabled}','false') WHERE id=true`)
	require.NoError(t, err)
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO payment_orders(user_id,amount,pay_amount,status,expires_at) VALUES($1,105,100,'COMPLETED',now()) RETURNING id`, uid).Scan(&order))
	var audit int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO payment_audit_logs(order_id,action,detail,operator) VALUES($1,'ORDER_CREATED','{"paymentAmount":100,"payAmount":100,"creditedAmount":105}','fixture') RETURNING id`, fmt.Sprint(order)).Scan(&audit))
	var refundedOrder, refundedAudit, adminAudit int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO payment_orders(user_id,amount,pay_amount,status,refund_amount,expires_at) VALUES($1,1,1,'REFUNDED',1,now()) RETURNING id`, uid).Scan(&refundedOrder))
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO payment_audit_logs(order_id,action,detail) VALUES($1,'ORDER_CREATED','{"paymentAmount":1,"payAmount":1,"creditedAmount":1}') RETURNING id`, fmt.Sprint(refundedOrder)).Scan(&refundedAudit))
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO audit_logs(action,status_code,request_body,extra) VALUES('admin.users.balance.create',200,'{"operation":"add","balance":5}',jsonb_build_object('params',jsonb_build_object('id',$1::text))) RETURNING id`, fmt.Sprint(uid)).Scan(&adminAudit))
	manifest := map[string]any{"batch_id": "fixture-backfill", "actor_id": actor, "currency": "CNY", "fx": 1, "expected_growth": 105, "orders": []map[string]any{
		{"id": order, "user_id": uid, "status": "COMPLETED", "amount": 105, "pay_amount": 100, "fee_rate": 0, "refund_amount": 0, "principal": 100, "audit_id": audit},
		{"id": refundedOrder, "user_id": uid, "status": "REFUNDED", "amount": 1, "pay_amount": 1, "fee_rate": 0, "refund_amount": 1, "principal": 1, "audit_id": refundedAudit},
	}, "admin_credits": []map[string]any{{"audit_id": adminAudit, "user_id": uid, "amount": 5}}}
	manifest["source_sha256"] = "fixture"
	raw, err := json.Marshal(manifest)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `SELECT set_config('model_gate.vip_backfill_manifest',$1,true)`, string(raw))
	require.NoError(t, err)
	script, err := os.ReadFile("../../tools/vip/historical-backfill.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(script))
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(script))
	require.NoError(t, err)
	var growth, balance float64
	var records int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT sum(amount),count(*) FROM vip_recharge_ledger WHERE user_id=$1`, uid).Scan(&growth, &records))
	require.Equal(t, 105.0, growth)
	require.Equal(t, 4, records)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, uid).Scan(&balance))
	require.Equal(t, 10.0, balance)
	_, err = tx.ExecContext(ctx, `UPDATE payment_orders SET status='PARTIALLY_REFUNDED',refund_amount=52.5 WHERE id=$1`, order)
	require.NoError(t, err)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT sum(amount) FROM vip_recharge_ledger WHERE user_id=$1`, uid).Scan(&growth))
	require.Equal(t, 55.0, growth)
	_, err = tx.ExecContext(ctx, `SAVEPOINT stale`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(script))
	require.ErrorContains(t, err, "changed since review")
	_, err = tx.ExecContext(ctx, `ROLLBACK TO stale`)
	require.NoError(t, err)
}
