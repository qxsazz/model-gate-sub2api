-- Management operations are explicit, immediate, idempotent and audited.
ALTER TABLE achievement_checkins ADD COLUMN source text NOT NULL DEFAULT 'user';
CREATE TABLE achievement_overrides (
 user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 key text NOT NULL REFERENCES achievement_catalog(key),
 state text NOT NULL CHECK(state IN ('granted','revoked')),
 actor_id bigint NOT NULL, reason text NOT NULL, updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id,key)
);
CREATE TABLE achievement_operations (
 id bigserial PRIMARY KEY,actor_id bigint NOT NULL,user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 action text NOT NULL CHECK(action IN ('backfill','grant','revoke','restore')),key text,day date,
 reason text NOT NULL,request_key uuid NOT NULL,request jsonb NOT NULL,response jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),UNIQUE(actor_id,request_key)
);
CREATE INDEX achievement_operations_user ON achievement_operations(user_id,id DESC);
CREATE TABLE achievement_daily_history(effective_at timestamptz PRIMARY KEY,rewards jsonb NOT NULL);
INSERT INTO achievement_daily_history SELECT COALESCE((SELECT min(day)::timestamp AT TIME ZONE 'Asia/Shanghai' FROM achievement_checkins),now()),(SELECT jsonb_agg(amount ORDER BY tier) FROM achievement_daily_policy);
CREATE FUNCTION achievement_daily_history_capture() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO achievement_daily_history(effective_at,rewards) SELECT clock_timestamp(),jsonb_agg(amount ORDER BY tier) FROM achievement_daily_policy;
 RETURN NULL;
END $$;
CREATE TRIGGER achievement_daily_history_changed AFTER INSERT OR UPDATE OR DELETE ON achievement_daily_policy FOR EACH STATEMENT EXECUTE FUNCTION achievement_daily_history_capture();
CREATE TABLE achievement_vip_history(effective_at timestamptz PRIMARY KEY,rules jsonb NOT NULL);
INSERT INTO achievement_vip_history SELECT updated_at,payload FROM vip_rules WHERE id=true;
CREATE FUNCTION achievement_vip_history_capture() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO achievement_vip_history VALUES(OLD.updated_at,OLD.payload) ON CONFLICT DO NOTHING;
 INSERT INTO achievement_vip_history VALUES(NEW.updated_at,NEW.payload) ON CONFLICT(effective_at) DO UPDATE SET rules=excluded.rules;
 RETURN NEW;
END $$;
CREATE TRIGGER achievement_vip_history_changed AFTER UPDATE OF payload ON vip_rules FOR EACH ROW EXECUTE FUNCTION achievement_vip_history_capture();

UPDATE achievement_config SET payload=payload||jsonb_build_object('cash_scope',CASE WHEN jsonb_array_length(COALESCE(payload->'cash_allowlist','[]'))>0 THEN 'allowlist' ELSE 'all' END,'budget_enabled',false),revision=revision+1;

CREATE FUNCTION achievement_is_unlocked(uid bigint,medal text) RETURNS boolean LANGUAGE sql STABLE AS $$
 SELECT COALESCE((SELECT state='granted' FROM achievement_overrides WHERE user_id=uid AND key=medal),
 (SELECT NOT preview AND achievement_progress(uid,medal)>=target FROM achievement_catalog WHERE key=medal),false)
$$;

CREATE OR REPLACE FUNCTION achievement_reserve_budget(day_key date, amount numeric, cfg jsonb) RETURNS boolean LANGUAGE plpgsql AS $$
DECLARE daily numeric; monthly numeric;
BEGIN
 IF COALESCE((cfg->>'budget_enabled')::boolean,true) THEN
  SELECT COALESCE(spent,0) INTO daily FROM achievement_budget WHERE day=day_key;
  SELECT COALESCE(sum(spent),0) INTO monthly FROM achievement_budget WHERE day>=date_trunc('month',day_key)::date AND day<(date_trunc('month',day_key)+interval '1 month')::date;
  IF COALESCE(daily,0)+amount>(cfg->>'daily_budget')::numeric OR monthly+amount>(cfg->>'monthly_budget')::numeric THEN RETURN false; END IF;
 END IF;
 INSERT INTO achievement_budget(day,spent) VALUES(day_key,amount) ON CONFLICT(day) DO UPDATE SET spent=achievement_budget.spent+excluded.spent;
 RETURN true;
END $$;

CREATE FUNCTION achievement_credit(uid bigint,credit numeric) RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE before_balance numeric;after_balance numeric;before_debt numeric;after_debt numeric;
BEGIN
 IF credit<0 THEN RAISE EXCEPTION 'ACHIEVEMENT_INVALID_AMOUNT'; END IF;
 SELECT balance INTO before_balance FROM users WHERE id=uid FOR UPDATE;
 SELECT amount INTO before_debt FROM vip_reward_debt WHERE user_id=uid;
 UPDATE users SET balance=balance+credit,updated_at=now() WHERE id=uid RETURNING balance INTO after_balance;
 SELECT amount INTO after_debt FROM vip_reward_debt WHERE user_id=uid;
 RETURN jsonb_build_object('gross',credit,'net',after_balance-before_balance,'offset_amount',COALESCE(before_debt,0)-COALESCE(after_debt,0));
END $$;

CREATE FUNCTION achievement_backfill_preview(uid bigint,day_key date) RETURNS jsonb LANGUAGE plpgsql STABLE AS $$
DECLARE cutoff timestamptz;rules jsonb;rewards jsonb;principal numeric;tier_no integer;reward numeric;rec achievement_checkins;registered timestamptz;today date;
BEGIN
 SELECT created_at INTO registered FROM users WHERE id=uid AND deleted_at IS NULL;
 IF NOT FOUND THEN RAISE EXCEPTION 'ACHIEVEMENT_ACCOUNT_UNAVAILABLE'; END IF;
 today:=(now() AT TIME ZONE 'Asia/Shanghai')::date;
 IF day_key>today THEN RETURN jsonb_build_object('day',day_key,'policy_known',false,'existing',false,'reason','date_invalid'); END IF;
 cutoff:=LEAST(((day_key+1)::timestamp AT TIME ZONE 'Asia/Shanghai'),clock_timestamp());
 IF registered>=cutoff THEN RETURN jsonb_build_object('day',day_key,'policy_known',false,'existing',false,'reason','account_not_created'); END IF;
 SELECT * INTO rec FROM achievement_checkins WHERE user_id=uid AND day=day_key;
 IF FOUND THEN RETURN jsonb_build_object('day',day_key,'policy_known',true,'existing',true,'tier',rec.tier,'gross',rec.gross,'reason','already_signed'); END IF;
 SELECT h.rules INTO rules FROM (
  SELECT vh.rules,vh.effective_at FROM achievement_vip_history vh
  UNION ALL SELECT detail,created_at FROM vip_audit WHERE action='rules'
 ) h WHERE effective_at<cutoff ORDER BY effective_at DESC LIMIT 1;
 SELECT h.rewards INTO rewards FROM achievement_daily_history h WHERE effective_at<cutoff ORDER BY effective_at DESC LIMIT 1;
 IF rules IS NULL OR rewards IS NULL OR jsonb_array_length(rewards)<>6 THEN RETURN jsonb_build_object('day',day_key,'policy_known',false,'existing',false,'reason','policy_unknown'); END IF;
 -- A late historical refund with no original refund timestamp cannot be
 -- reconstructed safely. Never substitute the user's current tier.
 IF EXISTS(SELECT 1 FROM vip_recharge_ledger l WHERE user_id=uid AND source='payment_refund' AND reason LIKE 'Approved historical refund:%' AND l.created_at>=cutoff) THEN RETURN jsonb_build_object('day',day_key,'policy_known',false,'existing',false,'reason','policy_unknown'); END IF;
 SELECT GREATEST(COALESCE(sum(l.amount),0),0) INTO principal FROM vip_recharge_ledger l
 LEFT JOIN payment_orders p ON l.source='payment' AND p.user_id=uid AND p.id::text=l.source_id
 LEFT JOIN audit_logs a ON l.source='admin_balance' AND l.source_id='historical-admin-audit:'||a.id::text
 WHERE l.user_id=uid AND CASE WHEN l.source='payment' THEN COALESCE(p.paid_at,p.completed_at,p.created_at,l.created_at)
 WHEN a.id IS NOT NULL THEN a.created_at ELSE l.created_at END<cutoff;
 tier_no:=0;
 IF COALESCE((rules->>'enabled')::boolean,false) THEN
  SELECT COALESCE((t->>'level')::integer,0) INTO tier_no FROM jsonb_array_elements(rules->'tiers') t WHERE (t->>'threshold')::numeric<=principal ORDER BY (t->>'threshold')::numeric DESC LIMIT 1;
  tier_no:=COALESCE(tier_no,0);
 END IF;
 reward:=(rewards->>tier_no)::numeric;
 IF reward IS NULL OR reward<0 THEN RETURN jsonb_build_object('day',day_key,'policy_known',false,'existing',false,'reason','policy_unknown'); END IF;
 RETURN jsonb_build_object('day',day_key,'policy_known',true,'existing',false,'tier',tier_no,'gross',reward,'vip_total',principal);
END $$;

CREATE FUNCTION achievement_admin_operation(actor bigint,uid bigint,operation text,body jsonb) RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE cfg jsonb;rev bigint;previous achievement_operations;medal text;day_key date;item achievement_catalog;rec achievement_claims;
 idem uuid;details jsonb;preview jsonb;result jsonb;cash jsonb;recovered numeric:=0;debt_added numeric:=0;
BEGIN
 SELECT payload,revision INTO cfg,rev FROM achievement_config WHERE id=true FOR UPDATE;
 PERFORM 1 FROM users WHERE id IN(actor,uid) ORDER BY id FOR UPDATE;
 IF NOT EXISTS(SELECT 1 FROM users WHERE id=actor AND role='admin' AND status='active' AND deleted_at IS NULL) THEN RAISE EXCEPTION 'ACHIEVEMENT_ADMIN_FORBIDDEN'; END IF;
 IF NOT EXISTS(SELECT 1 FROM users WHERE id=uid AND role IN('user','admin') AND status='active' AND deleted_at IS NULL) THEN RAISE EXCEPTION 'ACHIEVEMENT_ACCOUNT_UNAVAILABLE'; END IF;
 IF COALESCE(trim(body->>'reason'),'')='' THEN RAISE EXCEPTION 'ACHIEVEMENT_ADMIN_REASON'; END IF;
 idem:=(body->>'request_key')::uuid;
 details:=body||jsonb_build_object('action',operation,'user_id',uid);
 SELECT * INTO previous FROM achievement_operations WHERE actor_id=actor AND request_key=idem;
 IF FOUND THEN
  IF previous.request<>details THEN RAISE EXCEPTION 'ACHIEVEMENT_ADMIN_CONFLICT'; END IF;
  RETURN previous.response||jsonb_build_object('replayed',true);
 END IF;
 cash:=jsonb_build_object('gross',0,'net',0,'offset_amount',0);
 IF operation='backfill' THEN
  day_key:=(body->>'date')::date;preview:=achievement_backfill_preview(uid,day_key);
  IF COALESCE((preview->>'existing')::boolean,false) THEN result:=cash||jsonb_build_object('saved',false,'existing',true,'day',day_key);
  ELSE
   IF NOT COALESCE((preview->>'policy_known')::boolean,false) THEN RAISE EXCEPTION 'ACHIEVEMENT_HISTORY_UNAVAILABLE'; END IF;
   IF body->>'expected_gross' IS NULL OR body->>'expected_tier' IS NULL OR (body->>'expected_gross')::numeric<>(preview->>'gross')::numeric OR (body->>'expected_tier')::integer<>(preview->>'tier')::integer THEN RAISE EXCEPTION 'ACHIEVEMENT_HISTORY_CHANGED'; END IF;
   IF NOT achievement_reserve_budget((clock_timestamp() AT TIME ZONE 'Asia/Shanghai')::date,(preview->>'gross')::numeric,cfg) THEN RAISE EXCEPTION 'ACHIEVEMENT_BUDGET_EXHAUSTED'; END IF;
   cash:=achievement_credit(uid,(preview->>'gross')::numeric);
   INSERT INTO achievement_checkins(user_id,day,request_key,tier,revision,gross,net,offset_amount,reason,streak,source)
   VALUES(uid,day_key,'admin:'||idem::text,(preview->>'tier')::integer,rev,(cash->>'gross')::numeric,(cash->>'net')::numeric,(cash->>'offset_amount')::numeric,'admin_backfill',1,'admin');
   WITH islands AS(SELECT day,day-(row_number() OVER(ORDER BY day))::integer AS bucket FROM achievement_checkins WHERE user_id=uid),ranked AS(SELECT day,row_number() OVER(PARTITION BY bucket ORDER BY day)::integer AS chain FROM islands)
   UPDATE achievement_checkins c SET streak=r.chain FROM ranked r WHERE c.user_id=uid AND c.day=r.day;
   result:=cash||jsonb_build_object('saved',true,'day',day_key,'tier',(preview->>'tier')::integer);
  END IF;
 ELSIF operation IN('grant','revoke','restore') THEN
  medal:=body->>'key';SELECT * INTO item FROM achievement_catalog WHERE key=medal;
  IF NOT FOUND THEN RAISE EXCEPTION 'ACHIEVEMENT_LOCKED'; END IF;
  IF operation='restore' THEN DELETE FROM achievement_overrides WHERE user_id=uid AND key=medal;
  ELSE INSERT INTO achievement_overrides(user_id,key,state,actor_id,reason) VALUES(uid,medal,CASE WHEN operation='grant' THEN 'granted' ELSE 'revoked' END,actor,body->>'reason') ON CONFLICT(user_id,key) DO UPDATE SET state=excluded.state,actor_id=excluded.actor_id,reason=excluded.reason,updated_at=now(); END IF;
  IF operation='grant' AND COALESCE((body->>'grant_reward')::boolean,false) AND NOT EXISTS(SELECT 1 FROM achievement_claims WHERE user_id=uid AND key=medal) THEN
   IF NOT achievement_reserve_budget((clock_timestamp() AT TIME ZONE 'Asia/Shanghai')::date,item.reward,cfg) THEN RAISE EXCEPTION 'ACHIEVEMENT_BUDGET_EXHAUSTED'; END IF;
   cash:=achievement_credit(uid,item.reward);
   INSERT INTO achievement_claims(user_id,key,gross,net,offset_amount,revision) VALUES(uid,medal,item.reward,(cash->>'net')::numeric,(cash->>'offset_amount')::numeric,rev);
  ELSIF operation='revoke' AND COALESCE((body->>'reclaim_reward')::boolean,false) THEN
   SELECT * INTO rec FROM achievement_claims WHERE user_id=uid AND key=medal AND revoked_at IS NULL FOR UPDATE;
   IF FOUND THEN
    SELECT LEAST(GREATEST(balance,0),rec.net) INTO recovered FROM users WHERE id=uid;
    IF recovered>0 THEN UPDATE users SET balance=balance-recovered,updated_at=now() WHERE id=uid; END IF;
    debt_added:=rec.gross-recovered;
    IF debt_added>0 THEN INSERT INTO vip_reward_debt(user_id,amount) VALUES(uid,debt_added) ON CONFLICT(user_id) DO UPDATE SET amount=vip_reward_debt.amount+excluded.amount; END IF;
    UPDATE achievement_claims SET revoked_at=now() WHERE user_id=uid AND key=medal;
    cash:=jsonb_build_object('gross',rec.gross,'net',-recovered,'offset_amount',0,'recovered',recovered,'debt_added',debt_added);
   END IF;
  END IF;
  IF NOT achievement_is_unlocked(uid,medal) THEN UPDATE achievement_equipment SET key=NULL,updated_at=now() WHERE user_id=uid AND key=medal; END IF;
  result:=cash||jsonb_build_object('saved',true,'key',medal,'unlocked',achievement_is_unlocked(uid,medal));
 ELSE RAISE EXCEPTION 'ACHIEVEMENT_ADMIN_ACTION'; END IF;
 result:=result||jsonb_build_object('action',operation,'replayed',false);
 INSERT INTO achievement_operations(actor_id,user_id,action,key,day,reason,request_key,request,response) VALUES(actor,uid,operation,medal,day_key,body->>'reason',idem,details,result);
 RETURN result;
END $$;

CREATE OR REPLACE FUNCTION achievement_cash_reason(uid bigint, cfg jsonb) RETURNS text LANGUAGE sql STABLE AS $$
 SELECT CASE
 WHEN NOT COALESCE((cfg->>'cash_enabled')::boolean,false) THEN 'cash_disabled'
 WHEN COALESCE(NULLIF(cfg->>'cash_scope',''),'allowlist')='allowlist' AND NOT (cfg->'cash_allowlist' @> jsonb_build_array(uid)) THEN 'not_in_cash_pilot'
 WHEN NOT ((achievement_progress(uid,'R01')>0 AND COALESCE((SELECT sum(l.amount) FROM vip_recharge_ledger l WHERE l.user_id=uid AND l.created_at>now()-interval '30 days'
 AND (l.amount<0 OR (l.source='admin_balance' AND l.source_id NOT LIKE 'historical-admin-audit:%')
 OR (l.source='payment' AND EXISTS(SELECT 1 FROM payment_orders p WHERE p.user_id=uid AND p.id::text=l.source_id
 AND p.order_type='balance' AND p.status IN ('COMPLETED','PARTIALLY_REFUNDED')
 AND COALESCE(p.paid_at,p.completed_at,p.created_at)>now()-interval '30 days'
 AND NOT (COALESCE(p.provider_snapshot,'{}'::jsonb) ? 'vip_backfill_batch'))))),0)>0)
 OR EXISTS(SELECT 1 FROM achievement_growth WHERE user_id=uid AND last_balance_paid_at>now()-interval '30 days')) THEN 'recent_activity_required'
 ELSE 'eligible' END
$$;

CREATE OR REPLACE FUNCTION achievement_claim(uid bigint, medal text) RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE cfg jsonb;rev bigint;item achievement_catalog;rec achievement_claims;before_balance numeric;after_balance numeric;
 debt_before numeric;debt_after numeric;today date;
BEGIN
 SELECT payload,revision INTO cfg,rev FROM achievement_config WHERE id=true FOR UPDATE;
 SELECT balance INTO before_balance FROM users WHERE id=uid AND deleted_at IS NULL AND status='active' AND role IN ('user','admin') FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'ACHIEVEMENT_ACCOUNT_UNAVAILABLE'; END IF;
 SELECT * INTO rec FROM achievement_claims WHERE user_id=uid AND key=medal;
 IF FOUND THEN RETURN to_jsonb(rec)||jsonb_build_object('replayed',true); END IF;
 SELECT * INTO item FROM achievement_catalog WHERE key=medal;
 IF NOT FOUND OR NOT achievement_is_unlocked(uid,medal) THEN RAISE EXCEPTION 'ACHIEVEMENT_LOCKED'; END IF;
 IF achievement_cash_reason(uid,jsonb_set(cfg,'{cash_enabled}','true'))<>'eligible' OR NOT COALESCE((cfg->>'milestone_cash_enabled')::boolean,false) THEN RAISE EXCEPTION 'ACHIEVEMENT_CASH_UNAVAILABLE'; END IF;
 today:=(clock_timestamp() AT TIME ZONE 'Asia/Shanghai')::date;
 IF NOT achievement_reserve_budget(today,item.reward,cfg) THEN RAISE EXCEPTION 'ACHIEVEMENT_BUDGET_EXHAUSTED'; END IF;
 SELECT COALESCE(d.amount,0) INTO debt_before FROM vip_reward_debt d WHERE d.user_id=uid;
 UPDATE users SET balance=balance+item.reward,updated_at=now() WHERE id=uid RETURNING balance INTO after_balance;
 SELECT COALESCE(d.amount,0) INTO debt_after FROM vip_reward_debt d WHERE d.user_id=uid;
 INSERT INTO achievement_claims(user_id,key,gross,net,offset_amount,revision) VALUES(uid,medal,item.reward,after_balance-before_balance,COALESCE(debt_before,0)-COALESCE(debt_after,0),rev) RETURNING * INTO rec;
 RETURN to_jsonb(rec)||jsonb_build_object('replayed',false);
END $$;

CREATE OR REPLACE FUNCTION achievement_equip(uid bigint, medal text) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
 PERFORM 1 FROM users WHERE id=uid AND deleted_at IS NULL AND status='active' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'ACHIEVEMENT_ACCOUNT_UNAVAILABLE'; END IF;
 IF medal<>'' AND NOT EXISTS(SELECT 1 FROM achievement_catalog WHERE key=medal AND achievement_is_unlocked(uid,medal)) THEN RAISE EXCEPTION 'ACHIEVEMENT_LOCKED'; END IF;
 INSERT INTO achievement_equipment(user_id,key) VALUES(uid,NULLIF(medal,'')) ON CONFLICT(user_id) DO UPDATE SET key=excluded.key,updated_at=now();
END $$;


CREATE OR REPLACE FUNCTION achievement_refund() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE rec record; available numeric; recovered numeric; remaining numeric:=0;
BEGIN
 IF NEW.amount>=0 THEN RETURN NEW; END IF;
 SELECT GREATEST(balance,0) INTO available FROM users WHERE id=NEW.user_id FOR UPDATE;
 FOR rec IN SELECT ac.* FROM achievement_claims ac JOIN achievement_catalog cat ON cat.key=ac.key
 WHERE ac.user_id=NEW.user_id AND cat.category='recharge' AND ac.revoked_at IS NULL AND cat.target>achievement_progress(NEW.user_id,'R01') FOR UPDATE OF ac LOOP
  recovered:=LEAST(available,rec.net);
  IF recovered>0 THEN UPDATE users SET balance=balance-recovered WHERE id=NEW.user_id; END IF;
  available:=available-recovered;
  -- Restore debt previously paid by this reward as well as unrecovered cash.
  remaining:=remaining+rec.gross-recovered;
  UPDATE achievement_claims SET revoked_at=now() WHERE user_id=NEW.user_id AND key=rec.key;
 END LOOP;
 IF remaining>0 THEN INSERT INTO vip_reward_debt(user_id,amount) VALUES(NEW.user_id,remaining) ON CONFLICT(user_id) DO UPDATE SET amount=vip_reward_debt.amount+excluded.amount; END IF;
 UPDATE achievement_equipment SET key=NULL WHERE user_id=NEW.user_id AND key LIKE 'R%' AND NOT achievement_is_unlocked(NEW.user_id,achievement_equipment.key);
 RETURN NEW;
END $$;
