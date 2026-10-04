-- Daily cash rewards follow the configured switch and scope without requiring spending.
-- Existing checkins remain immutable; this migration never backfills historical cash.
CREATE FUNCTION achievement_checkin_cash_reason(uid bigint, cfg jsonb) RETURNS text LANGUAGE sql STABLE AS $$
 SELECT CASE
 WHEN NOT EXISTS(SELECT 1 FROM users WHERE id=uid AND deleted_at IS NULL AND status='active' AND role IN('user','admin')) THEN 'account_unavailable'
 WHEN NOT COALESCE((cfg->>'cash_enabled')::boolean,false) THEN 'cash_disabled'
 WHEN COALESCE(NULLIF(cfg->>'cash_scope',''),'allowlist')='allowlist' AND NOT (COALESCE(cfg->'cash_allowlist','[]'::jsonb) @> jsonb_build_array(uid)) THEN 'not_in_cash_pilot'
 ELSE 'eligible' END
$$;

CREATE OR REPLACE FUNCTION achievement_checkin(uid bigint, expected_day text, idem text) RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE cfg jsonb; rev bigint; today date; rec achievement_checkins; tier_no integer;
 amount numeric:=0; before_balance numeric; after_balance numeric; debt_before numeric; debt_after numeric;
 reason_code text; chain integer;
BEGIN
 SELECT payload,revision INTO cfg,rev FROM achievement_config WHERE id=true FOR UPDATE;
 SELECT balance INTO before_balance FROM users WHERE id=uid AND deleted_at IS NULL AND status='active' AND role IN ('user','admin') FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'ACHIEVEMENT_ACCOUNT_UNAVAILABLE'; END IF;
 SELECT * INTO rec FROM achievement_checkins WHERE user_id=uid AND request_key=idem;
 IF FOUND THEN RETURN to_jsonb(rec)||jsonb_build_object('replayed',true); END IF;
 today:=(clock_timestamp() AT TIME ZONE 'Asia/Shanghai')::date;
 IF expected_day<>today::text THEN RAISE EXCEPTION 'ACHIEVEMENT_DATE_CHANGED'; END IF;
 SELECT * INTO rec FROM achievement_checkins WHERE user_id=uid AND day=today;
 IF FOUND THEN RETURN to_jsonb(rec)||jsonb_build_object('replayed',true); END IF;
 tier_no:=achievement_tier(uid);
 reason_code:=achievement_checkin_cash_reason(uid,cfg);
 IF reason_code='eligible' THEN
  SELECT p.amount INTO amount FROM achievement_daily_policy p WHERE p.tier=tier_no;
  IF amount IS NULL THEN RAISE EXCEPTION 'ACHIEVEMENT_POLICY_UNAVAILABLE'; END IF;
  IF NOT achievement_reserve_budget(today,amount,cfg) THEN amount:=0;reason_code:='budget_exhausted'; END IF;
 END IF;
 SELECT COALESCE(streak,0)+1 INTO chain FROM achievement_checkins WHERE user_id=uid AND day=today-1;
 chain:=COALESCE(chain,1);
 SELECT COALESCE(d.amount,0) INTO debt_before FROM vip_reward_debt d WHERE d.user_id=uid;
 IF amount>0 THEN UPDATE users SET balance=balance+amount,updated_at=now() WHERE id=uid RETURNING balance INTO after_balance;
 ELSE after_balance:=before_balance; END IF;
 SELECT COALESCE(d.amount,0) INTO debt_after FROM vip_reward_debt d WHERE d.user_id=uid;
 INSERT INTO achievement_checkins(user_id,day,request_key,tier,revision,gross,net,offset_amount,reason,streak)
 VALUES(uid,today,idem,tier_no,rev,amount,after_balance-before_balance,COALESCE(debt_before,0)-COALESCE(debt_after,0),reason_code,chain) RETURNING * INTO rec;
 RETURN to_jsonb(rec)||jsonb_build_object('replayed',false);
END $$;
