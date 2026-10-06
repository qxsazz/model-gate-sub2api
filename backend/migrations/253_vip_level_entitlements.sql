-- A member grade is one entitlement source; it never creates recharge principal.
ALTER TABLE vip_overrides DROP CONSTRAINT vip_overrides_benefit_check;
ALTER TABLE vip_overrides ADD CONSTRAINT vip_overrides_benefit_check CHECK(benefit IN ('discount','access','concurrency','rpm','rebate','badge','tier'));
ALTER TABLE vip_overrides ADD CONSTRAINT vip_override_tier_valid CHECK(benefit<>'tier' OR (value BETWEEN 0 AND 5 AND trunc(value)=value));
CREATE TABLE vip_user_level_history (
 id bigserial PRIMARY KEY,user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 level integer CHECK(level BETWEEN 0 AND 5),expires_at timestamptz,
 effective_at timestamptz NOT NULL DEFAULT clock_timestamp(),actor_id bigint NOT NULL,reason text NOT NULL
);
CREATE INDEX vip_user_level_history_user ON vip_user_level_history(user_id,effective_at DESC,id DESC);

CREATE FUNCTION vip_effective_level(uid bigint) RETURNS integer LANGUAGE sql STABLE AS $$
 SELECT CASE WHEN COALESCE((SELECT (payload->>'enabled')::boolean FROM vip_rules WHERE id=true),false)
 THEN COALESCE((SELECT value::integer FROM vip_overrides WHERE user_id=uid AND benefit='tier' AND (expires_at IS NULL OR expires_at>now())),
 (SELECT (t->>'level')::integer FROM vip_rules,jsonb_array_elements(payload->'tiers') t WHERE id=true AND (t->>'threshold')::numeric<=GREATEST(COALESCE((SELECT sum(amount) FROM vip_recharge_ledger WHERE user_id=uid),0),0) ORDER BY (t->>'threshold')::numeric DESC LIMIT 1),0) ELSE 0 END
$$;
CREATE FUNCTION vip_set_user_level(actor bigint,uid bigint,grade integer,expiry timestamptz,note text) RETURNS void LANGUAGE plpgsql AS $$
DECLARE previous jsonb; event_at timestamptz;
BEGIN
 PERFORM 1 FROM users WHERE id IN(actor,uid) ORDER BY id FOR UPDATE;
 IF NOT EXISTS(SELECT 1 FROM users WHERE id=actor AND role='admin' AND status='active' AND deleted_at IS NULL) THEN RAISE EXCEPTION 'VIP_LEVEL_ADMIN_FORBIDDEN'; END IF;
 IF NOT EXISTS(SELECT 1 FROM users WHERE id=uid AND status='active' AND deleted_at IS NULL AND role IN('user','admin')) THEN RAISE EXCEPTION 'VIP_LEVEL_ACCOUNT_UNAVAILABLE'; END IF;
 IF grade IS NOT NULL AND (grade<0 OR grade>5) THEN RAISE EXCEPTION 'VIP_LEVEL_INVALID'; END IF;
 IF length(btrim(note))<3 OR length(note)>500 OR (expiry IS NOT NULL AND expiry<=clock_timestamp()) THEN RAISE EXCEPTION 'VIP_LEVEL_INVALID'; END IF;
 SELECT COALESCE(jsonb_agg(to_jsonb(o)),'[]'::jsonb) INTO previous FROM vip_overrides o WHERE user_id=uid;
 DELETE FROM vip_overrides WHERE user_id=uid;
 event_at:=clock_timestamp();
 IF grade IS NOT NULL THEN
  INSERT INTO vip_overrides(user_id,benefit,value,expires_at,reason,actor_id,updated_at) VALUES(uid,'tier',grade,expiry,note,actor,event_at);
 END IF;
 INSERT INTO vip_user_level_history(user_id,level,expires_at,effective_at,actor_id,reason) VALUES(uid,grade,expiry,event_at,actor,note);
 INSERT INTO vip_audit(actor_id,user_id,action,detail,created_at) VALUES(actor,uid,CASE WHEN grade IS NULL THEN 'restore_level' ELSE 'set_level' END,jsonb_build_object('level',grade,'expires_at',expiry,'reason',note,'previous_overrides',previous),event_at);
END $$;

CREATE OR REPLACE FUNCTION achievement_tier(uid bigint) RETURNS integer LANGUAGE sql STABLE AS $$ SELECT vip_effective_level(uid) $$;
CREATE OR REPLACE FUNCTION vip_rebate_rate(uid bigint) RETURNS numeric LANGUAGE sql STABLE AS $$
 SELECT CASE WHEN COALESCE((SELECT (payload->>'enabled')::boolean FROM vip_rules WHERE id=true),false) THEN
 CASE WHEN EXISTS(SELECT 1 FROM vip_user_level_history WHERE user_id=uid)
 THEN COALESCE((SELECT (t->>'rebate_percent')::numeric FROM vip_rules,jsonb_array_elements(payload->'tiers') t WHERE id=true AND (t->>'level')::integer=vip_effective_level(uid)),0)
 ELSE COALESCE((SELECT value FROM vip_overrides WHERE user_id=uid AND benefit='rebate' AND (expires_at IS NULL OR expires_at>now())),(SELECT (t->>'rebate_percent')::numeric FROM vip_rules,jsonb_array_elements(payload->'tiers') t WHERE id=true AND (t->>'level')::integer=vip_effective_level(uid)),0) END ELSE NULL END
$$;

CREATE FUNCTION achievement_apply_daily_policy(rewards jsonb) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
 IF rewards IS NULL THEN RETURN; END IF;
 IF jsonb_typeof(rewards)<>'array' OR jsonb_array_length(rewards)<>6 THEN RAISE EXCEPTION 'VIP_DAILY_POLICY_INVALID'; END IF;
 IF EXISTS(SELECT 1 FROM jsonb_array_elements(rewards) r WHERE jsonb_typeof(r)<>'number' OR r::numeric<0 OR r::numeric>1000) THEN RAISE EXCEPTION 'VIP_DAILY_POLICY_INVALID'; END IF;
 IF rewards IS DISTINCT FROM (SELECT jsonb_agg(amount ORDER BY tier) FROM achievement_daily_policy) THEN
  UPDATE achievement_daily_policy p SET amount=e.value::numeric FROM jsonb_array_elements(rewards) WITH ORDINALITY e(value,n) WHERE p.tier=e.n-1;
 END IF;
END $$;
CREATE FUNCTION vip_save_rules_with_daily(actor bigint,rules jsonb,rewards jsonb) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
 -- Same config lock order as checkin before any policy updates.
 PERFORM 1 FROM achievement_config WHERE id=true FOR UPDATE;
 UPDATE vip_rules SET payload=rules-'daily_rewards',revision=revision+1,updated_at=now() WHERE id=true;
 PERFORM achievement_apply_daily_policy(rewards);
 IF rewards IS NOT NULL THEN UPDATE achievement_config SET revision=revision+1,updated_at=now() WHERE id=true; END IF;
 INSERT INTO vip_audit(actor_id,action,detail) VALUES(actor,'rules',rules);
END $$;

CREATE FUNCTION achievement_save_config_with_policy(actor bigint,settings jsonb,rewards jsonb) RETURNS void LANGUAGE plpgsql AS $$
DECLARE previous jsonb;current_payload jsonb;rev bigint;
BEGIN
 SELECT payload INTO previous FROM achievement_config WHERE id=true FOR UPDATE;
 PERFORM achievement_apply_daily_policy(rewards);
 UPDATE achievement_config SET payload=settings-'daily_rewards',revision=revision+1,updated_at=now() WHERE id=true RETURNING payload,revision INTO current_payload,rev;
 INSERT INTO achievement_admin_audit(actor_id,previous,current,revision) VALUES(actor,previous,current_payload,rev);
END $$;
-- Once managed by a full grade, a member cannot acquire new split VIP overrides.
CREATE FUNCTION vip_set_legacy_override(uid bigint,kind text,val numeric,expiry timestamptz,note text,actor bigint,details jsonb) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
 PERFORM 1 FROM users WHERE id=uid FOR UPDATE;
 IF EXISTS(SELECT 1 FROM vip_user_level_history WHERE user_id=uid) THEN RAISE EXCEPTION 'VIP_LEVEL_MANAGED'; END IF;
 INSERT INTO vip_overrides(user_id,benefit,value,expires_at,reason,actor_id) VALUES(uid,kind,val,expiry,note,actor)
 ON CONFLICT(user_id,benefit) DO UPDATE SET value=excluded.value,expires_at=excluded.expires_at,reason=excluded.reason,actor_id=excluded.actor_id,updated_at=now();
 INSERT INTO vip_audit(actor_id,user_id,action,detail) VALUES(actor,uid,'override',details);
END $$;

CREATE OR REPLACE FUNCTION achievement_backfill_preview(uid bigint,day_key date) RETURNS jsonb LANGUAGE plpgsql STABLE AS $$
DECLARE cutoff timestamptz;rules jsonb;rewards jsonb;principal numeric;tier_no integer;reward numeric;rec achievement_checkins;registered timestamptz;today date;assignment vip_user_level_history;
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
 IF COALESCE((rules->>'enabled')::boolean,false) THEN
  SELECT * INTO assignment FROM vip_user_level_history WHERE user_id=uid AND effective_at<cutoff ORDER BY effective_at DESC,id DESC LIMIT 1;
  IF FOUND AND assignment.level IS NOT NULL AND (assignment.expires_at IS NULL OR assignment.expires_at>=cutoff) THEN tier_no:=assignment.level; END IF;
 END IF;
 reward:=(rewards->>tier_no)::numeric;
 IF reward IS NULL OR reward<0 THEN RETURN jsonb_build_object('day',day_key,'policy_known',false,'existing',false,'reason','policy_unknown'); END IF;
 RETURN jsonb_build_object('day',day_key,'policy_known',true,'existing',false,'tier',tier_no,'gross',reward,'vip_total',principal);
END $$;
