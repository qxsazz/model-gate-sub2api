-- Additive VIP storage. No automatic production backfill or activation.
CREATE TABLE vip_rules (
 id boolean PRIMARY KEY DEFAULT true CHECK(id),
 payload jsonb NOT NULL,
 revision bigint NOT NULL DEFAULT 1,
 updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO vip_rules(id,payload) VALUES(true,'{"enabled":false,"currency":"USD","access_threshold":100,"tiers":[{"level":1,"threshold":100,"concurrency":8,"rpm":0,"rebate_percent":2},{"level":2,"threshold":300,"concurrency":12,"rpm":0,"rebate_percent":4},{"level":3,"threshold":600,"concurrency":16,"rpm":0,"rebate_percent":6},{"level":4,"threshold":1500,"concurrency":20,"rpm":0,"rebate_percent":8},{"level":5,"threshold":3000,"concurrency":30,"rpm":0,"rebate_percent":10}],"groups":[]}');
CREATE TABLE vip_recharge_ledger (
 id bigserial PRIMARY KEY,
 user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 source text NOT NULL,
 source_id text NOT NULL,
 amount numeric(20,8) NOT NULL,
 reason text NOT NULL DEFAULT '',
 actor_id bigint,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(source,source_id)
);
UPDATE vip_rules SET payload=payload||'{"exchange_rates":{"USD":1}}'::jsonb WHERE id=true;
CREATE INDEX vip_recharge_ledger_user ON vip_recharge_ledger(user_id,id);
CREATE TABLE vip_overrides (
 user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 benefit text NOT NULL CHECK(benefit IN ('discount','access','concurrency','rpm','rebate','badge')),
 value numeric(20,8) NOT NULL,
 expires_at timestamptz,
 reason text NOT NULL,
 actor_id bigint NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id,benefit)
);
CREATE TABLE vip_audit (
 id bigserial PRIMARY KEY,
 actor_id bigint NOT NULL,
 user_id bigint,
 action text NOT NULL,
 detail jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE OR REPLACE FUNCTION vip_record_payment() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE principal numeric; credited numeric; refunded numeric; prior_refund numeric;
BEGIN
 IF NEW.order_type <> 'balance' THEN RETURN NEW; END IF;
 IF NEW.status NOT IN ('COMPLETED','REFUNDED','PARTIALLY_REFUNDED') THEN RETURN NEW; END IF;
 IF NOT (COALESCE(NEW.provider_snapshot,'{}'::jsonb) ? 'vip_principal_usd') THEN RETURN NEW; END IF;
 principal := (NEW.provider_snapshot->>'vip_principal_usd')::numeric;
 IF principal < 0 THEN RAISE EXCEPTION 'invalid VIP principal'; END IF;
 PERFORM pg_advisory_xact_lock(9471, (NEW.user_id % 2147483647)::integer);
 INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount,reason)
 VALUES(NEW.user_id,'payment',NEW.id::text,principal,'Online recharge principal')
 ON CONFLICT(source,source_id) DO NOTHING;
 IF NEW.status IN ('REFUNDED','PARTIALLY_REFUNDED') AND NEW.amount>0 THEN
  refunded:=LEAST(principal,principal*NEW.refund_amount/NEW.amount);
  SELECT COALESCE(-SUM(amount),0) INTO prior_refund FROM vip_recharge_ledger WHERE source='payment_refund' AND source_id LIKE NEW.id::text||':%';
  IF refunded>prior_refund THEN
   INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount,reason)
   VALUES(NEW.user_id,'payment_refund',NEW.id::text||':'||NEW.refund_amount::text,-(refunded-prior_refund),'Confirmed online refund')
   ON CONFLICT(source,source_id) DO NOTHING;
  END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER vip_payment_ledger AFTER INSERT OR UPDATE OF status,refund_amount ON payment_orders FOR EACH ROW EXECUTE FUNCTION vip_record_payment();
CREATE OR REPLACE FUNCTION vip_rebate_rate(uid bigint) RETURNS numeric LANGUAGE sql STABLE AS $$
 SELECT COALESCE(
 (SELECT aff_rebate_rate_percent FROM user_affiliates WHERE user_id=uid),
 CASE WHEN (SELECT (payload->>'enabled')::boolean FROM vip_rules WHERE id=true) THEN
 COALESCE(
 (SELECT value FROM vip_overrides WHERE user_id=uid AND benefit='rebate' AND (expires_at IS NULL OR expires_at>now())),
 (SELECT (tier->>'rebate_percent')::numeric FROM vip_rules, jsonb_array_elements(payload->'tiers') tier
 WHERE id=true AND (tier->>'threshold')::numeric <= GREATEST(COALESCE((SELECT sum(amount) FROM vip_recharge_ledger WHERE user_id=uid),0),0)
 ORDER BY (tier->>'threshold')::numeric DESC LIMIT 1),0)
 ELSE NULL END)
$$;
CREATE OR REPLACE FUNCTION vip_payment_rebate_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE inviter bigint; rate numeric;
BEGIN
 IF NEW.status='PAID' AND OLD.status IS DISTINCT FROM 'PAID'
 AND (SELECT (payload->>'enabled')::boolean FROM vip_rules WHERE id=true) THEN
  SELECT inviter_id INTO inviter FROM user_affiliates WHERE user_id=NEW.user_id;
  IF inviter IS NOT NULL THEN rate:=COALESCE(vip_rebate_rate(inviter),0); ELSE rate:=0; END IF;
  NEW.provider_snapshot:=COALESCE(NEW.provider_snapshot,'{}'::jsonb)||jsonb_build_object('vip_rebate_percent',rate,'vip_inviter_id',inviter);
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER vip_payment_rebate_before BEFORE UPDATE OF status ON payment_orders FOR EACH ROW EXECUTE FUNCTION vip_payment_rebate_snapshot();

CREATE TABLE vip_rebate_refunds (
 source_order_id bigint PRIMARY KEY,
 inviter_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 amount numeric(20,8) NOT NULL DEFAULT 0
);
CREATE TABLE vip_rebate_debt (
 user_id bigint PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 amount numeric(20,8) NOT NULL CHECK(amount>=0)
);
CREATE OR REPLACE FUNCTION vip_reverse_rebate() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE original record; previous numeric; target numeric; delta numeric; available numeric; recovered numeric;
BEGIN
 IF NEW.status NOT IN ('REFUNDED','PARTIALLY_REFUNDED') OR NEW.amount<=0
 OR NOT (COALESCE(NEW.provider_snapshot,'{}'::jsonb) ? 'vip_rebate_percent') THEN RETURN NEW; END IF;
 SELECT * INTO original FROM user_affiliate_ledger WHERE source_order_id=NEW.id AND action='accrue' ORDER BY id LIMIT 1;
 IF original.id IS NULL THEN RETURN NEW; END IF;
 PERFORM 1 FROM user_affiliates WHERE user_id=original.user_id FOR UPDATE;
 INSERT INTO vip_rebate_refunds(source_order_id,inviter_id) VALUES(NEW.id,original.user_id) ON CONFLICT DO NOTHING;
 SELECT amount INTO previous FROM vip_rebate_refunds WHERE source_order_id=NEW.id FOR UPDATE;
 target:=LEAST(original.amount,original.amount*NEW.refund_amount/NEW.amount);delta:=target-previous;
 IF delta<=0 THEN RETURN NEW; END IF;
 IF original.frozen_until IS NOT NULL THEN
  SELECT aff_frozen_quota INTO available FROM user_affiliates WHERE user_id=original.user_id;
  recovered:=LEAST(delta,GREATEST(available,0));
  UPDATE user_affiliates SET aff_frozen_quota=aff_frozen_quota-recovered WHERE user_id=original.user_id;
  -- The matching negative frozen record offsets future thaw of this order.
  INSERT INTO user_affiliate_ledger(user_id,action,amount,source_user_id,source_order_id,frozen_until)
  VALUES(original.user_id,'vip_refund',-recovered,NEW.user_id,NEW.id,original.frozen_until);
 ELSE
  SELECT aff_quota INTO available FROM user_affiliates WHERE user_id=original.user_id;
  recovered:=LEAST(delta,GREATEST(available,0));
  UPDATE user_affiliates SET aff_quota=aff_quota-recovered WHERE user_id=original.user_id;
  INSERT INTO user_affiliate_ledger(user_id,action,amount,source_user_id,source_order_id)
  VALUES(original.user_id,'vip_refund',-recovered,NEW.user_id,NEW.id);
 END IF;
 IF delta>recovered THEN
  INSERT INTO vip_rebate_debt(user_id,amount) VALUES(original.user_id,delta-recovered)
  ON CONFLICT(user_id) DO UPDATE SET amount=vip_rebate_debt.amount+excluded.amount;
 END IF;
 UPDATE vip_rebate_refunds SET amount=target WHERE source_order_id=NEW.id;
 RETURN NEW;
END $$;
CREATE TRIGGER vip_payment_rebate_refund AFTER UPDATE OF status,refund_amount ON payment_orders FOR EACH ROW EXECUTE FUNCTION vip_reverse_rebate();
CREATE OR REPLACE FUNCTION vip_repay_rebate_debt() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE owed numeric; recovered numeric;
BEGIN
 IF NEW.action<>'accrue' OR NEW.amount<=0 THEN RETURN NEW; END IF;
 SELECT amount INTO owed FROM vip_rebate_debt WHERE user_id=NEW.user_id FOR UPDATE;
 IF owed IS NULL OR owed<=0 THEN RETURN NEW; END IF;
 recovered:=LEAST(owed,NEW.amount);
 UPDATE vip_rebate_debt SET amount=amount-recovered WHERE user_id=NEW.user_id;
 IF NEW.frozen_until IS NOT NULL THEN
  UPDATE user_affiliates SET aff_frozen_quota=aff_frozen_quota-recovered WHERE user_id=NEW.user_id;
 ELSE
  UPDATE user_affiliates SET aff_quota=aff_quota-recovered WHERE user_id=NEW.user_id;
 END IF;
 INSERT INTO user_affiliate_ledger(user_id,action,amount,source_user_id,source_order_id,frozen_until)
 VALUES(NEW.user_id,'vip_debt_repayment',-recovered,NEW.source_user_id,NEW.source_order_id,NEW.frozen_until);
 RETURN NEW;
END $$;
CREATE TRIGGER vip_affiliate_debt AFTER INSERT ON user_affiliate_ledger FOR EACH ROW EXECUTE FUNCTION vip_repay_rebate_debt();
CREATE TABLE vip_admin_rebates (
 recharge_id bigint PRIMARY KEY REFERENCES vip_recharge_ledger(id) ON DELETE CASCADE,
 invitee_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 inviter_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 principal numeric(20,8) NOT NULL,
 rebate numeric(20,8) NOT NULL,
 refunded_principal numeric(20,8) NOT NULL DEFAULT 0,
 frozen_until timestamptz
);
CREATE OR REPLACE FUNCTION vip_admin_rebate() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE inviter bigint; rate numeric; earned numeric; hours integer; days integer; cap numeric; used numeric; expiry timestamptz; remaining numeric; portion numeric; reversal numeric; available numeric; recovered numeric; old record;
BEGIN
 IF NEW.source<>'admin_balance' OR NOT (SELECT (payload->>'enabled')::boolean FROM vip_rules WHERE id=true) THEN RETURN NEW; END IF;
 IF NEW.amount>0 THEN
  IF COALESCE((SELECT value FROM settings WHERE key='affiliate_enabled'),'false')<>'true'
  OR COALESCE((SELECT value FROM settings WHERE key='affiliate_admin_recharge_enabled'),'false')<>'true' THEN RETURN NEW; END IF;
  SELECT inviter_id,created_at INTO inviter,expiry FROM user_affiliates WHERE user_id=NEW.user_id;
  IF inviter IS NULL OR inviter=NEW.user_id THEN RETURN NEW; END IF;
  days:=COALESCE((SELECT value::integer FROM settings WHERE key='affiliate_rebate_duration_days'),0);
  IF days>0 AND expiry+make_interval(days=>days)<now() THEN RETURN NEW; END IF;
  PERFORM 1 FROM user_affiliates WHERE user_id=inviter FOR UPDATE;
  rate:=COALESCE(vip_rebate_rate(inviter),0);earned:=round(NEW.amount*rate/100,8);
  cap:=COALESCE((SELECT value::numeric FROM settings WHERE key='affiliate_rebate_per_invitee_cap'),0);
  IF cap>0 THEN
   SELECT COALESCE(sum(amount),0) INTO used FROM user_affiliate_ledger WHERE user_id=inviter AND source_user_id=NEW.user_id AND action='accrue';
   earned:=LEAST(earned,GREATEST(cap-used,0));
  END IF;
  IF earned<=0 THEN RETURN NEW; END IF;
  hours:=COALESCE((SELECT value::integer FROM settings WHERE key='affiliate_rebate_freeze_hours'),0);
  IF hours>0 THEN expiry:=now()+make_interval(hours=>hours); ELSE expiry:=NULL; END IF;
  INSERT INTO vip_admin_rebates(recharge_id,invitee_id,inviter_id,principal,rebate,frozen_until) VALUES(NEW.id,NEW.user_id,inviter,NEW.amount,earned,expiry);
  UPDATE user_affiliates SET aff_quota=aff_quota+CASE WHEN hours=0 THEN earned ELSE 0 END,
  aff_frozen_quota=aff_frozen_quota+CASE WHEN hours>0 THEN earned ELSE 0 END,aff_history_quota=aff_history_quota+earned,updated_at=now() WHERE user_id=inviter;
  INSERT INTO user_affiliate_ledger(user_id,action,amount,source_user_id,operation_id,frozen_until)
  VALUES(inviter,'accrue',earned,NEW.user_id,'vipadmin:'||NEW.id::text,expiry);
 ELSE
  remaining:=-NEW.amount;
  FOR old IN SELECT * FROM vip_admin_rebates WHERE invitee_id=NEW.user_id AND refunded_principal<principal ORDER BY recharge_id FOR UPDATE LOOP
   EXIT WHEN remaining<=0;
   portion:=LEAST(remaining,old.principal-old.refunded_principal);reversal:=round(old.rebate*portion/old.principal,8);
   PERFORM 1 FROM user_affiliates WHERE user_id=old.inviter_id FOR UPDATE;
   IF old.frozen_until IS NOT NULL AND EXISTS(SELECT 1 FROM user_affiliate_ledger WHERE operation_id='vipadmin:'||old.recharge_id::text AND frozen_until IS NOT NULL) THEN
    SELECT aff_frozen_quota INTO available FROM user_affiliates WHERE user_id=old.inviter_id;recovered:=LEAST(reversal,GREATEST(available,0));
    UPDATE user_affiliates SET aff_frozen_quota=aff_frozen_quota-recovered WHERE user_id=old.inviter_id;
    expiry:=old.frozen_until;
   ELSE
    SELECT aff_quota INTO available FROM user_affiliates WHERE user_id=old.inviter_id;recovered:=LEAST(reversal,GREATEST(available,0));
    UPDATE user_affiliates SET aff_quota=aff_quota-recovered WHERE user_id=old.inviter_id;expiry:=NULL;
   END IF;
   INSERT INTO user_affiliate_ledger(user_id,action,amount,source_user_id,operation_id,frozen_until)
   VALUES(old.inviter_id,'vip_refund',-recovered,NEW.user_id,'vipadminrefund:'||NEW.id::text||':'||old.recharge_id::text,expiry);
   IF reversal>recovered THEN INSERT INTO vip_rebate_debt(user_id,amount) VALUES(old.inviter_id,reversal-recovered)
    ON CONFLICT(user_id) DO UPDATE SET amount=vip_rebate_debt.amount+excluded.amount; END IF;
   UPDATE vip_admin_rebates SET refunded_principal=refunded_principal+portion WHERE recharge_id=old.recharge_id;
   remaining:=remaining-portion;
  END LOOP;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER vip_admin_rebate_ledger AFTER INSERT ON vip_recharge_ledger FOR EACH ROW EXECUTE FUNCTION vip_admin_rebate();
