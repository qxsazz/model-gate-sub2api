-- Run inside one transaction, after setting model_gate.vip_backfill_manifest.
-- This is an operator tool, never an application startup migration.
DO $$
DECLARE m jsonb := current_setting('model_gate.vip_backfill_manifest')::jsonb;
 item jsonb; rules jsonb; p payment_orders%ROWTYPE; audit payment_audit_logs%ROWTYPE;
 adjustment audit_logs%ROWTYPE; principal numeric; refunded numeric; prior numeric;
 actor bigint := (m->>'actor_id')::bigint; uid bigint; credit numeric; sid text;
 total numeric := 0; batch text := m->>'batch_id';
BEGIN
 IF NOT (m ?& ARRAY['batch_id','actor_id','currency','fx','orders','admin_credits','expected_growth','source_sha256'])
 OR COALESCE(batch,'')='' OR m->>'currency' IS DISTINCT FROM 'CNY' OR (m->>'fx')::numeric IS DISTINCT FROM 1::numeric
 OR jsonb_typeof(m->'orders') IS DISTINCT FROM 'array' OR jsonb_typeof(m->'admin_credits') IS DISTINCT FROM 'array' THEN
  RAISE EXCEPTION 'Invalid approved backfill manifest';
 END IF;
 IF EXISTS(SELECT 1 FROM vip_audit WHERE action='historical_backfill' AND detail->>'batch_id'=batch
  AND (detail->>'source_sha256' IS DISTINCT FROM m->>'source_sha256'
   OR (detail->>'growth')::numeric IS DISTINCT FROM (m->>'expected_growth')::numeric
   OR (detail->>'orders')::integer IS DISTINCT FROM jsonb_array_length(m->'orders')
   OR (detail->>'admin_credits')::integer IS DISTINCT FROM jsonb_array_length(m->'admin_credits'))) THEN
  RAISE EXCEPTION 'Reviewed batch identity conflicts';
 END IF;
 PERFORM 1 FROM users WHERE id=actor AND role='admin' AND status='active' AND deleted_at IS NULL;
 IF NOT FOUND THEN RAISE EXCEPTION 'Backfill actor must be an active administrator'; END IF;
 SELECT payload INTO rules FROM vip_rules WHERE id=true FOR UPDATE;
 IF NOT FOUND OR COALESCE((rules->>'enabled')::boolean,true) THEN
  RAISE EXCEPTION 'VIP must be installed and disabled before historical backfill';
 END IF;
 IF (SELECT count(*) FROM jsonb_array_elements(m->'orders')) <>
    (SELECT count(DISTINCT value->>'id') FROM jsonb_array_elements(m->'orders'))
 OR (SELECT count(*) FROM jsonb_array_elements(m->'admin_credits')) <>
    (SELECT count(DISTINCT value->>'audit_id') FROM jsonb_array_elements(m->'admin_credits')) THEN
  RAISE EXCEPTION 'Duplicate approved source identifiers';
 END IF;
 PERFORM 1 FROM payment_orders WHERE id IN
  (SELECT (value->>'id')::bigint FROM jsonb_array_elements(m->'orders')) ORDER BY id FOR UPDATE;
 PERFORM 1 FROM users WHERE id IN (
  SELECT (value->>'user_id')::bigint FROM jsonb_array_elements(m->'orders')
  UNION SELECT (value->>'user_id')::bigint FROM jsonb_array_elements(m->'admin_credits')) ORDER BY id FOR UPDATE;
 IF EXISTS(SELECT 1 FROM vip_recharge_ledger WHERE source='opening' AND user_id IN (
  SELECT (value->>'user_id')::bigint FROM jsonb_array_elements(m->'orders')
  UNION SELECT (value->>'user_id')::bigint FROM jsonb_array_elements(m->'admin_credits')))
 OR EXISTS(SELECT 1 FROM vip_reward_claims WHERE user_id IN (
  SELECT (value->>'user_id')::bigint FROM jsonb_array_elements(m->'orders')
  UNION SELECT (value->>'user_id')::bigint FROM jsonb_array_elements(m->'admin_credits'))) THEN
  RAISE EXCEPTION 'Existing opening credits or claims require manual reconciliation';
 END IF;
 FOR item IN SELECT value FROM jsonb_array_elements(m->'orders') ORDER BY (value->>'id')::bigint LOOP
  IF NOT (item ?& ARRAY['id','user_id','status','amount','pay_amount','fee_rate','refund_amount','principal','audit_id']) THEN
   RAISE EXCEPTION 'Incomplete approved order';
  END IF;
  SELECT * INTO p FROM payment_orders WHERE id=(item->>'id')::bigint;
  IF NOT FOUND OR p.order_type<>'balance' OR p.status NOT IN ('COMPLETED','REFUNDED','PARTIALLY_REFUNDED')
  OR p.user_id<>(item->>'user_id')::bigint OR p.status<>item->>'status'
  OR p.amount<>(item->>'amount')::numeric OR p.pay_amount<>(item->>'pay_amount')::numeric
  OR p.fee_rate<>(item->>'fee_rate')::numeric OR p.refund_amount<>(item->>'refund_amount')::numeric THEN
   RAISE EXCEPTION 'Order % changed since review',item->>'id';
  END IF;
  PERFORM 1 FROM users WHERE id=p.user_id AND deleted_at IS NULL;
  IF NOT FOUND THEN RAISE EXCEPTION 'Order owner unavailable'; END IF;
  principal:=(item->>'principal')::numeric;
  SELECT * INTO audit FROM payment_audit_logs WHERE id=(item->>'audit_id')::bigint;
  IF NOT FOUND OR audit.order_id<>p.id::text OR audit.action<>'ORDER_CREATED'
  OR (audit.detail::jsonb->>'paymentAmount')::numeric IS DISTINCT FROM principal
  OR (audit.detail::jsonb->>'payAmount')::numeric IS DISTINCT FROM p.pay_amount
  OR (audit.detail::jsonb->>'creditedAmount')::numeric IS DISTINCT FROM p.amount
  OR principal<=0 OR p.amount<=0 OR p.fee_rate<>0 OR principal<>p.pay_amount
  OR p.refund_amount<0 OR p.refund_amount>p.amount
  OR (p.status='COMPLETED' AND p.refund_amount<>0)
  OR (p.status='REFUNDED' AND p.refund_amount<>p.amount) THEN
   RAISE EXCEPTION 'Order % principal or refund evidence mismatch',p.id;
  END IF;
  IF COALESCE(p.provider_snapshot,'{}'::jsonb) ? 'vip_principal_usd' AND
   ((p.provider_snapshot->>'vip_principal_usd')::numeric IS DISTINCT FROM principal
    OR p.provider_snapshot->>'vip_currency' IS DISTINCT FROM 'CNY'
    OR (p.provider_snapshot->>'vip_fx')::numeric IS DISTINCT FROM 1::numeric) THEN
   RAISE EXCEPTION 'Existing VIP snapshot conflicts for order %',p.id;
  END IF;
  IF EXISTS(SELECT 1 FROM vip_recharge_ledger WHERE source='payment' AND source_id=p.id::text
    AND (amount<>principal OR user_id<>p.user_id)) THEN
   RAISE EXCEPTION 'Existing recharge conflicts for order %',p.id;
  END IF;
  UPDATE payment_orders SET provider_snapshot=COALESCE(provider_snapshot,'{}'::jsonb)||
   jsonb_build_object('vip_principal_usd',principal,'vip_fx',1,'vip_currency','CNY','vip_backfill_batch',batch)
   WHERE id=p.id;
  INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount,reason,actor_id)
   VALUES(p.user_id,'payment',p.id::text,principal,'Approved historical recharge: '||batch,actor)
   ON CONFLICT(source,source_id) DO NOTHING;
  refunded:=round(LEAST(principal,principal*p.refund_amount/p.amount),8);
  SELECT COALESCE(-sum(amount),0) INTO prior FROM vip_recharge_ledger
   WHERE source='payment_refund' AND source_id LIKE p.id::text||':%';
  IF prior<0 OR prior>refunded THEN RAISE EXCEPTION 'Existing refund conflicts for order %',p.id; END IF;
  IF refunded>prior THEN
   INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount,reason,actor_id)
    VALUES(p.user_id,'payment_refund',p.id::text||':'||p.refund_amount::text,-(refunded-prior),'Approved historical refund: '||batch,actor);
  END IF;
  total:=total+principal-refunded;
 END LOOP;
 FOR item IN SELECT value FROM jsonb_array_elements(m->'admin_credits') ORDER BY (value->>'audit_id')::bigint LOOP
  IF NOT (item ?& ARRAY['audit_id','user_id','amount']) THEN RAISE EXCEPTION 'Incomplete approved admin credit'; END IF;
  uid:=(item->>'user_id')::bigint; credit:=(item->>'amount')::numeric;
  SELECT * INTO adjustment FROM audit_logs WHERE id=(item->>'audit_id')::bigint;
  IF NOT FOUND OR adjustment.action<>'admin.users.balance.create'
  OR adjustment.status_code NOT BETWEEN 200 AND 299
  OR (adjustment.extra->'params'->>'id')::bigint IS DISTINCT FROM uid
  OR adjustment.request_body::jsonb->>'operation' IS DISTINCT FROM 'add'
  OR (adjustment.request_body::jsonb->>'balance')::numeric IS DISTINCT FROM credit OR credit<=0 THEN
   RAISE EXCEPTION 'Admin credit evidence mismatch: %',item->>'audit_id';
  END IF;
  PERFORM 1 FROM users WHERE id=uid AND deleted_at IS NULL;
  IF NOT FOUND THEN RAISE EXCEPTION 'Admin credit owner unavailable'; END IF;
  sid:='historical-admin-audit:'||(item->>'audit_id');
  IF EXISTS(SELECT 1 FROM vip_recharge_ledger WHERE source='admin_balance' AND source_id=sid
    AND (amount<>credit OR user_id<>uid)) THEN RAISE EXCEPTION 'Existing admin credit conflicts'; END IF;
  INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount,reason,actor_id)
   VALUES(uid,'admin_balance',sid,credit,'Approved historical admin credit: '||batch,actor)
   ON CONFLICT(source,source_id) DO NOTHING;
  total:=total+credit;
 END LOOP;
 IF total IS DISTINCT FROM (m->>'expected_growth')::numeric THEN RAISE EXCEPTION 'Reviewed growth total mismatch'; END IF;
 IF NOT EXISTS(SELECT 1 FROM vip_audit WHERE action='historical_backfill' AND detail->>'batch_id'=batch) THEN
  INSERT INTO vip_audit(actor_id,action,detail) VALUES(actor,'historical_backfill',jsonb_build_object(
   'batch_id',batch,'growth',total,'orders',jsonb_array_length(m->'orders'),
   'admin_credits',jsonb_array_length(m->'admin_credits'),'source_sha256',m->>'source_sha256'));
 END IF;
END $$;
