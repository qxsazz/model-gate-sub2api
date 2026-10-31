-- RMB admission is separate from VIP growth, balances and reward accounting.
CREATE FUNCTION achievement_access_number(value text) RETURNS numeric LANGUAGE plpgsql IMMUTABLE AS $$
BEGIN
 IF value IS NULL OR value !~ '^[0-9]+([.][0-9]+)?$' THEN RETURN NULL; END IF;
 RETURN value::numeric;
END $$;
CREATE FUNCTION achievement_access_audit(value text) RETURNS jsonb LANGUAGE plpgsql IMMUTABLE AS $$
BEGIN RETURN value::jsonb; EXCEPTION WHEN invalid_text_representation THEN RETURN '{}'::jsonb; END $$;
CREATE FUNCTION achievement_access_online(method text) RETURNS boolean LANGUAGE sql IMMUTABLE AS $$
 SELECT method_name LIKE 'alipay%' OR method_name LIKE 'wxpay%' OR method_name IN('stripe','card','link','easypay','airwallex') FROM (SELECT lower(trim(method)) AS method_name) m
$$;

-- refund_amount also holds pending requests. Preserve only confirmed units,
-- independently of subsequent status changes, without changing refund execution.
CREATE TABLE achievement_access_refunds(
 order_id bigint PRIMARY KEY REFERENCES payment_orders(id) ON DELETE CASCADE,
 confirmed_amount numeric(20,2) NOT NULL CHECK(confirmed_amount>=0),
 updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO achievement_access_refunds(order_id,confirmed_amount)
SELECT p.id,LEAST(p.amount,GREATEST(
 CASE WHEN p.status='REFUNDED' THEN p.amount WHEN p.status='PARTIALLY_REFUNDED' THEN p.refund_amount ELSE 0 END,
 COALESCE((SELECT max(achievement_access_number(achievement_access_audit(a.detail)->>'refundAmount'))
 FROM payment_audit_logs a WHERE a.order_id=p.id::text AND a.action='REFUND_SUCCESS'),0)))
FROM payment_orders p WHERE p.order_type='balance' AND p.amount>0;

-- Order writers already hold their order row. Admission never locks orders:
-- configuration -> existing VIP user advisory lock -> user -> plain fact reads.
-- Writers use advisory lock -> user before their existing VIP/refund triggers.
CREATE FUNCTION achievement_access_order_sync() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE owner bigint;owners bigint[];
BEGIN
 IF TG_OP='DELETE' THEN
  IF OLD.order_type<>'balance' THEN RETURN OLD; END IF;
 ELSIF NEW.order_type<>'balance' AND (TG_OP='INSERT' OR OLD.order_type<>'balance') THEN RETURN NEW;
 END IF;
 owners:=CASE WHEN TG_OP='INSERT' THEN ARRAY[NEW.user_id] WHEN TG_OP='DELETE' THEN ARRAY[OLD.user_id] ELSE ARRAY[OLD.user_id,NEW.user_id] END;
 FOR owner IN SELECT DISTINCT x FROM unnest(owners) AS t(x) ORDER BY x LOOP
  PERFORM pg_advisory_xact_lock(9471,(owner % 2147483647)::integer);
  PERFORM 1 FROM users WHERE id=owner FOR UPDATE;
 END LOOP;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER achievement_access_order_sync BEFORE INSERT OR DELETE OR UPDATE OF user_id,order_type,payment_type,status,refund_amount,refund_at,amount,pay_amount,fee_rate,provider_snapshot ON payment_orders
FOR EACH ROW EXECUTE FUNCTION achievement_access_order_sync();

CREATE FUNCTION achievement_access_record_refund() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE confirmed numeric:=0;
BEGIN
 IF NEW.order_type<>'balance' OR NEW.amount<=0 THEN RETURN NEW; END IF;
 IF NEW.status='REFUNDED' THEN confirmed:=NEW.amount;
 ELSIF NEW.status='PARTIALLY_REFUNDED' THEN confirmed:=LEAST(NEW.amount,GREATEST(NEW.refund_amount,0)); END IF;
 IF confirmed>0 THEN
  INSERT INTO achievement_access_refunds(order_id,confirmed_amount) VALUES(NEW.id,confirmed)
  ON CONFLICT(order_id) DO UPDATE SET confirmed_amount=GREATEST(achievement_access_refunds.confirmed_amount,EXCLUDED.confirmed_amount),updated_at=now();
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER achievement_access_refund_record AFTER INSERT OR UPDATE OF status,refund_amount,refund_at ON payment_orders
FOR EACH ROW EXECUTE FUNCTION achievement_access_record_refund();

CREATE FUNCTION achievement_access_principal(p payment_orders) RETURNS numeric LANGUAGE plpgsql STABLE AS $$
DECLARE snapshot jsonb:=COALESCE(p.provider_snapshot,'{}'::jsonb);currency text;principal numeric;candidate numeric;rate numeric;
BEGIN
 IF p.pay_amount<=0 OR p.pay_amount::text IN('NaN','Infinity','-Infinity') OR p.fee_rate::text IN('NaN','Infinity','-Infinity') THEN RETURN NULL; END IF;
 currency:=COALESCE(NULLIF(upper(trim(snapshot->>'currency')),''),'CNY');
 IF currency<>'CNY' THEN
  IF NOT (snapshot ? 'recharge_principal_cny') THEN RETURN 0; END IF;
  principal:=achievement_access_number(snapshot->>'recharge_principal_cny');
 ELSIF snapshot ? 'recharge_principal' THEN
  principal:=achievement_access_number(snapshot->>'recharge_principal');
 ELSE
  -- Legacy CNY fee was ceil(principal * fee_rate / 100) in fen.
  -- Invert that monotone integer function, then verify instead of guessing FX.
  rate:=GREATEST(p.fee_rate,0)/100;
  candidate:=floor(p.pay_amount*100/(1+rate));
  IF candidate+ceil(candidate*rate)<>p.pay_amount*100 THEN RETURN NULL; END IF;
  principal:=candidate/100;
 END IF;
 IF principal IS NULL OR principal<=0 OR principal<>round(principal,2) THEN RETURN NULL; END IF;
 IF currency='CNY' AND snapshot ? 'recharge_principal' THEN
  rate:=GREATEST(p.fee_rate,0)/100;
  IF abs(principal+ceil(principal*100*rate)/100-p.pay_amount)>.01 THEN RETURN NULL; END IF;
  -- Payment confirmation tolerates a cent; an underpayment must not cross admission.
  principal:=LEAST(principal,GREATEST(p.pay_amount-ceil(principal*100*rate)/100,0));
 END IF;
 RETURN principal;
END $$;

UPDATE achievement_config SET payload=payload||jsonb_build_object('minimum_recharge_cny',10),revision=revision+1,updated_at=now() WHERE id=true;
CREATE FUNCTION achievement_access_snapshot(uid bigint) RETURNS jsonb LANGUAGE plpgsql STABLE AS $$
DECLARE minimum numeric;qualified numeric:=0;principal numeric;refunded numeric;confirmed numeric;p payment_orders;unknown boolean:=false;had_refund boolean:=false;reason text;pending boolean;
BEGIN
 SELECT achievement_access_number(COALESCE(payload->>'minimum_recharge_cny','10')) INTO minimum FROM achievement_config WHERE id=true;
 IF NOT EXISTS(SELECT 1 FROM users WHERE id=uid AND status='active' AND deleted_at IS NULL AND role IN('user','admin')) THEN
  RETURN jsonb_build_object('access_unlocked',false,'minimum_recharge_cny',minimum,'qualified_recharge_cny',NULL,'remaining_recharge_cny',NULL,'access_reason','account_unavailable');
 END IF;
 IF minimum IS NULL OR minimum<=0 OR minimum<>round(minimum,2) THEN unknown:=true; END IF;
 FOR p IN SELECT o.* FROM payment_orders o
 WHERE o.user_id=uid AND o.order_type='balance' AND achievement_access_online(o.payment_type)
 AND (o.completed_at IS NOT NULL OR o.status IN('COMPLETED','REFUNDED','PARTIALLY_REFUNDED','REFUND_REQUESTED','REFUNDING','REFUND_PENDING','REFUND_FAILED')) LOOP
  principal:=achievement_access_principal(p);
  IF principal IS NULL THEN unknown:=true;CONTINUE; END IF;
  IF principal=0 THEN CONTINUE; END IF;
  SELECT COALESCE((SELECT confirmed_amount FROM achievement_access_refunds WHERE order_id=p.id),0) INTO confirmed;
  IF p.amount<=0 OR p.amount::text IN('NaN','Infinity','-Infinity') OR (p.status='PARTIALLY_REFUNDED' AND confirmed<=0) THEN unknown:=true;CONTINUE; END IF;
  refunded:=CASE WHEN p.status='REFUNDED' THEN principal ELSE LEAST(principal,round(principal*confirmed/p.amount,2)) END;
  qualified:=qualified+GREATEST(principal-refunded,0);
  had_refund:=had_refund OR refunded>0;
 END LOOP;
 IF unknown THEN
  RETURN jsonb_build_object('access_unlocked',false,'minimum_recharge_cny',minimum,'qualified_recharge_cny',NULL,'remaining_recharge_cny',NULL,'access_reason','verification_unavailable');
 END IF;
 SELECT EXISTS(SELECT 1 FROM payment_orders WHERE user_id=uid AND order_type='balance' AND achievement_access_online(payment_type) AND paid_at IS NOT NULL AND completed_at IS NULL AND status IN('PAID','RECHARGING','FAILED')) INTO pending;
 reason:=CASE WHEN qualified>=minimum THEN 'eligible' WHEN pending THEN 'payment_pending' WHEN had_refund THEN 'recharge_refunded' ELSE 'recharge_required' END;
 RETURN jsonb_build_object('access_unlocked',qualified>=minimum,'minimum_recharge_cny',minimum,'qualified_recharge_cny',qualified,'remaining_recharge_cny',GREATEST(minimum-qualified,0),'access_reason',reason);
END $$;

CREATE FUNCTION achievement_require_access(uid bigint,writing boolean DEFAULT true) RETURNS void LANGUAGE plpgsql AS $$
DECLARE access jsonb;
BEGIN
 IF writing THEN
  PERFORM 1 FROM achievement_config WHERE id=true FOR UPDATE;
  PERFORM pg_advisory_xact_lock(9471,(uid % 2147483647)::integer);
  PERFORM 1 FROM users WHERE id=uid FOR UPDATE;
 END IF;
 -- This is a separate statement after the locks: observe committed facts after waiting.
 access:=achievement_access_snapshot(uid);
 IF access->>'access_reason'='account_unavailable' THEN RAISE EXCEPTION 'ACHIEVEMENT_ACCOUNT_UNAVAILABLE'; END IF;
 IF access->>'access_reason'='verification_unavailable' THEN RAISE EXCEPTION 'ACHIEVEMENT_ACCESS_UNAVAILABLE' USING DETAIL=access::text; END IF;
 IF NOT (access->>'access_unlocked')::boolean THEN RAISE EXCEPTION 'ACHIEVEMENT_RECHARGE_REQUIRED' USING DETAIL=access::text; END IF;
END $$;

ALTER FUNCTION achievement_checkin(bigint,text,text) RENAME TO achievement_checkin_before_access;
CREATE FUNCTION achievement_checkin(uid bigint,expected_day text,idem text) RETURNS jsonb LANGUAGE plpgsql AS $$
BEGIN PERFORM achievement_require_access(uid);RETURN achievement_checkin_before_access(uid,expected_day,idem);END $$;
ALTER FUNCTION achievement_claim(bigint,text) RENAME TO achievement_claim_before_access;
CREATE FUNCTION achievement_claim(uid bigint,medal text) RETURNS jsonb LANGUAGE plpgsql AS $$
BEGIN PERFORM achievement_require_access(uid);RETURN achievement_claim_before_access(uid,medal);END $$;
ALTER FUNCTION achievement_claim_cards(bigint,text) RENAME TO achievement_claim_cards_before_access;
CREATE FUNCTION achievement_claim_cards(uid bigint,medal text) RETURNS jsonb LANGUAGE plpgsql AS $$
BEGIN PERFORM achievement_require_access(uid);RETURN achievement_claim_cards_before_access(uid,medal);END $$;
ALTER FUNCTION achievement_claim_series(bigint,text) RENAME TO achievement_claim_series_before_access;
CREATE FUNCTION achievement_claim_series(uid bigint,series_key text) RETURNS jsonb LANGUAGE plpgsql AS $$
BEGIN PERFORM achievement_require_access(uid);RETURN achievement_claim_series_before_access(uid,series_key);END $$;
ALTER FUNCTION achievement_use_card(bigint,jsonb) RENAME TO achievement_use_card_before_access;
CREATE FUNCTION achievement_use_card(uid bigint,body jsonb) RETURNS jsonb LANGUAGE plpgsql AS $$
BEGIN PERFORM achievement_require_access(uid);RETURN achievement_use_card_before_access(uid,body);END $$;
ALTER FUNCTION achievement_activity_attempt(bigint,text,text,uuid,text,jsonb,integer,boolean) RENAME TO achievement_activity_attempt_before_access;
CREATE FUNCTION achievement_activity_attempt(uid bigint,kind_key text,topic_key text,attempt_id uuid,idem text,submitted jsonb,grade integer,did_pass boolean) RETURNS jsonb LANGUAGE plpgsql AS $$
BEGIN PERFORM achievement_require_access(uid);RETURN achievement_activity_attempt_before_access(uid,kind_key,topic_key,attempt_id,idem,submitted,grade,did_pass);END $$;
ALTER FUNCTION achievement_card_preview(bigint,date) RENAME TO achievement_card_preview_before_access;
CREATE FUNCTION achievement_card_preview(uid bigint,day_key date) RETURNS jsonb LANGUAGE plpgsql AS $$
BEGIN PERFORM achievement_require_access(uid,false);RETURN achievement_card_preview_before_access(uid,day_key);END $$;

CREATE OR REPLACE FUNCTION achievement_save_config_with_policy(actor bigint,settings jsonb,rewards jsonb) RETURNS void LANGUAGE plpgsql AS $$
DECLARE previous jsonb;current_payload jsonb;rev bigint;minimum numeric;
BEGIN
 SELECT payload INTO previous FROM achievement_config WHERE id=true FOR UPDATE;
 IF NOT(settings ? 'minimum_recharge_cny') OR settings->'minimum_recharge_cny'='null'::jsonb THEN
  settings:=settings||jsonb_build_object('minimum_recharge_cny',COALESCE(previous->'minimum_recharge_cny','10'::jsonb));
 END IF;
 minimum:=achievement_access_number(settings->>'minimum_recharge_cny');
 IF minimum IS NULL OR minimum<=0 OR minimum>1000000 OR minimum<>round(minimum,2) THEN RAISE EXCEPTION 'ACHIEVEMENT_CONFIG_INVALID'; END IF;
 PERFORM achievement_apply_daily_policy(rewards);
 UPDATE achievement_config SET payload=settings-'daily_rewards',revision=revision+1,updated_at=now() WHERE id=true RETURNING payload,revision INTO current_payload,rev;
 INSERT INTO achievement_admin_audit(actor_id,previous,current,revision) VALUES(actor,previous,current_payload,rev);
END $$;
