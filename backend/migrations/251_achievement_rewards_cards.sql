-- Confirmed reward values; historical receipts are immutable.
UPDATE achievement_catalog SET reward=v.amount FROM (VALUES
 ('T01',.10),('T02',.50),('T03',2),('T04',8),('T05',25),('T06',80),
 ('S01',5),('S02',12),('S03',30),('R01',10),('R02',50),('R03',666)
) AS v(key,amount) WHERE achievement_catalog.key=v.key;
ALTER TABLE achievement_catalog ADD COLUMN card_reward integer NOT NULL DEFAULT 0 CHECK(card_reward>=0);
UPDATE achievement_catalog SET reward=0,card_reward=CASE right(key,2) WHEN '01' THEN 1 WHEN '02' THEN 2 ELSE 3 END WHERE category='activity';

CREATE TABLE achievement_card_claims(
 user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 key text NOT NULL REFERENCES achievement_catalog(key),
 amount integer NOT NULL CHECK(amount>0),used integer NOT NULL DEFAULT 0 CHECK(used>=0),
 reclaimed integer NOT NULL DEFAULT 0 CHECK(reclaimed>=0),created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id,key),CHECK(used+reclaimed<=amount)
);
CREATE TABLE achievement_card_ledger(
 id bigserial PRIMARY KEY,user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 key text NOT NULL REFERENCES achievement_catalog(key),delta integer NOT NULL CHECK(delta<>0),
 kind text NOT NULL CHECK(kind IN('claim','use','reclaim')),day date,created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX achievement_card_ledger_user ON achievement_card_ledger(user_id,id DESC);
CREATE TABLE achievement_card_uses(
 user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,request_key uuid NOT NULL,
 request jsonb NOT NULL,response jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id,request_key)
);
CREATE TABLE achievement_series(
 key text PRIMARY KEY,name text NOT NULL,members text[] NOT NULL CHECK(cardinality(members)=3),
 reward numeric(20,8) NOT NULL CHECK(reward>=0)
);
INSERT INTO achievement_series VALUES
 ('A-K','知识系列',ARRAY['A-K01','A-K02','A-K03'],1.40),
 ('A-X','实践系列',ARRAY['A-X01','A-X02','A-X03'],2.30),
 ('A-C','篇章系列',ARRAY['A-C01','A-C02','A-C03'],11.80);
CREATE TABLE achievement_series_claims(
 user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,key text NOT NULL REFERENCES achievement_series(key),
 gross numeric(20,8) NOT NULL,net numeric(20,8) NOT NULL,offset_amount numeric(20,8) NOT NULL,
 prior_amount numeric(20,8) NOT NULL DEFAULT 0,revision bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),revoked_at timestamptz,PRIMARY KEY(user_id,key)
);

CREATE FUNCTION achievement_card_balance(uid bigint) RETURNS integer LANGUAGE sql STABLE AS $$
 SELECT COALESCE(sum(amount-used-reclaimed),0)::integer FROM achievement_card_claims WHERE user_id=uid
$$;
CREATE FUNCTION achievement_claim_cards(uid bigint,medal text) RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE item achievement_catalog;rec achievement_card_claims;
BEGIN
 PERFORM 1 FROM achievement_config WHERE id=true FOR UPDATE;
 PERFORM 1 FROM users WHERE id=uid AND status='active' AND role IN('user','admin') AND deleted_at IS NULL FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'ACHIEVEMENT_ACCOUNT_UNAVAILABLE'; END IF;
 SELECT * INTO rec FROM achievement_card_claims WHERE user_id=uid AND key=medal;
 IF FOUND THEN RETURN jsonb_build_object('cards_awarded',rec.amount,'card_balance',achievement_card_balance(uid),'gross',0,'net',0,'offset_amount',0,'replayed',true); END IF;
 SELECT * INTO item FROM achievement_catalog WHERE key=medal;
 IF NOT FOUND OR item.preview OR item.card_reward<=0 OR NOT achievement_is_unlocked(uid,medal) THEN RAISE EXCEPTION 'ACHIEVEMENT_LOCKED'; END IF;
 INSERT INTO achievement_card_claims(user_id,key,amount) VALUES(uid,medal,item.card_reward);
 INSERT INTO achievement_card_ledger(user_id,key,delta,kind) VALUES(uid,medal,item.card_reward,'claim');
 RETURN jsonb_build_object('cards_awarded',item.card_reward,'card_balance',achievement_card_balance(uid),'gross',0,'net',0,'offset_amount',0,'replayed',false);
END $$;

-- Preserve all original cash semantics while routing activity rewards to cards.
ALTER FUNCTION achievement_claim(bigint,text) RENAME TO achievement_claim_cash_v249;
CREATE FUNCTION achievement_claim(uid bigint,medal text) RETURNS jsonb LANGUAGE plpgsql AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM achievement_catalog WHERE key=medal AND card_reward>0) THEN RETURN achievement_claim_cards(uid,medal); END IF;
 RETURN achievement_claim_cash_v249(uid,medal);
END $$;

CREATE FUNCTION achievement_claim_series(uid bigint,series_key text) RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE cfg jsonb;rev bigint;item achievement_series;rec achievement_series_claims;cash jsonb;prior numeric;amount numeric;
BEGIN
 SELECT payload,revision INTO cfg,rev FROM achievement_config WHERE id=true FOR UPDATE;
 PERFORM 1 FROM users WHERE id=uid AND status='active' AND role IN('user','admin') AND deleted_at IS NULL FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'ACHIEVEMENT_ACCOUNT_UNAVAILABLE'; END IF;
 SELECT * INTO rec FROM achievement_series_claims WHERE user_id=uid AND key=series_key;
 IF FOUND THEN RETURN to_jsonb(rec)||jsonb_build_object('replayed',true); END IF;
 SELECT * INTO item FROM achievement_series WHERE key=series_key;
 IF NOT FOUND OR EXISTS(SELECT 1 FROM unnest(item.members) AS m(key) LEFT JOIN achievement_catalog c USING(key) WHERE c.key IS NULL OR c.preview OR NOT achievement_is_unlocked(uid,m.key)) THEN RAISE EXCEPTION 'ACHIEVEMENT_LOCKED'; END IF;
 IF achievement_cash_reason(uid,jsonb_set(cfg,'{cash_enabled}','true'))<>'eligible' OR NOT COALESCE((cfg->>'milestone_cash_enabled')::boolean,false) THEN RAISE EXCEPTION 'ACHIEVEMENT_CASH_UNAVAILABLE'; END IF;
 SELECT COALESCE(sum(gross),0) INTO prior FROM achievement_claims WHERE user_id=uid AND key=ANY(item.members);
 amount:=GREATEST(item.reward-prior,0);
 IF amount>0 AND NOT achievement_reserve_budget((clock_timestamp() AT TIME ZONE 'Asia/Shanghai')::date,amount,cfg) THEN RAISE EXCEPTION 'ACHIEVEMENT_BUDGET_EXHAUSTED'; END IF;
 cash:=achievement_credit(uid,amount);
 INSERT INTO achievement_series_claims(user_id,key,gross,net,offset_amount,prior_amount,revision)
 VALUES(uid,series_key,amount,(cash->>'net')::numeric,(cash->>'offset_amount')::numeric,prior,rev) RETURNING * INTO rec;
 RETURN to_jsonb(rec)||jsonb_build_object('replayed',false);
END $$;

CREATE FUNCTION achievement_card_preview(uid bigint,day_key date) RETURNS jsonb LANGUAGE plpgsql STABLE AS $$
DECLARE today date;preview jsonb;cfg jsonb;reason text;
BEGIN
 today:=(now() AT TIME ZONE 'Asia/Shanghai')::date;
 IF day_key<today-30 OR day_key>=today THEN RAISE EXCEPTION 'ACHIEVEMENT_CARD_DATE_INVALID'; END IF;
 IF NOT EXISTS(SELECT 1 FROM users WHERE id=uid AND status='active' AND deleted_at IS NULL AND role IN('user','admin')) THEN RAISE EXCEPTION 'ACHIEVEMENT_ACCOUNT_UNAVAILABLE'; END IF;
 preview:=achievement_backfill_preview(uid,day_key);
 SELECT payload INTO cfg FROM achievement_config WHERE id=true;
 reason:=achievement_checkin_cash_reason(uid,cfg);
 RETURN preview||jsonb_build_object('card_balance',achievement_card_balance(uid),'cash_reason',reason,'available',COALESCE((preview->>'policy_known')::boolean,false) AND NOT COALESCE((preview->>'existing')::boolean,false) AND reason='eligible' AND achievement_card_balance(uid)>0);
END $$;

CREATE FUNCTION achievement_use_card(uid bigint,body jsonb) RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE cfg jsonb;rev bigint;day_key date;today date;idem uuid;previous achievement_card_uses;
 preview jsonb;cash jsonb;lot achievement_card_claims;result jsonb;rec achievement_checkins;
BEGIN
 SELECT payload,revision INTO cfg,rev FROM achievement_config WHERE id=true FOR UPDATE;
 PERFORM 1 FROM users WHERE id=uid AND status='active' AND role IN('user','admin') AND deleted_at IS NULL FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'ACHIEVEMENT_ACCOUNT_UNAVAILABLE'; END IF;
 idem:=(body->>'request_key')::uuid;
 SELECT * INTO previous FROM achievement_card_uses WHERE user_id=uid AND request_key=idem;
 IF FOUND THEN
  IF previous.request<>body THEN RAISE EXCEPTION 'ACHIEVEMENT_CARD_CONFLICT'; END IF;
  RETURN previous.response||jsonb_build_object('replayed',true);
 END IF;
 day_key:=(body->>'date')::date;today:=(clock_timestamp() AT TIME ZONE 'Asia/Shanghai')::date;
 IF day_key<today-30 OR day_key>=today THEN RAISE EXCEPTION 'ACHIEVEMENT_CARD_DATE_INVALID'; END IF;
 SELECT * INTO rec FROM achievement_checkins WHERE user_id=uid AND day=day_key;
 IF FOUND THEN
  result:=to_jsonb(rec)||jsonb_build_object('cards_spent',0,'card_balance',achievement_card_balance(uid),'existing',true,'saved',false);
 ELSE
  preview:=achievement_backfill_preview(uid,day_key);
  IF NOT COALESCE((preview->>'policy_known')::boolean,false) THEN RAISE EXCEPTION 'ACHIEVEMENT_HISTORY_UNAVAILABLE'; END IF;
  IF body->>'expected_gross' IS NULL OR body->>'expected_tier' IS NULL OR (body->>'expected_gross')::numeric<>(preview->>'gross')::numeric OR (body->>'expected_tier')::integer<>(preview->>'tier')::integer THEN RAISE EXCEPTION 'ACHIEVEMENT_HISTORY_CHANGED'; END IF;
  IF achievement_checkin_cash_reason(uid,cfg)<>'eligible' THEN RAISE EXCEPTION 'ACHIEVEMENT_CHECKIN_CASH_UNAVAILABLE'; END IF;
  SELECT * INTO lot FROM achievement_card_claims WHERE user_id=uid AND amount>used+reclaimed ORDER BY created_at,key LIMIT 1 FOR UPDATE;
  IF NOT FOUND THEN RAISE EXCEPTION 'ACHIEVEMENT_CARD_INSUFFICIENT'; END IF;
  IF NOT achievement_reserve_budget(today,(preview->>'gross')::numeric,cfg) THEN RAISE EXCEPTION 'ACHIEVEMENT_BUDGET_EXHAUSTED'; END IF;
  cash:=achievement_credit(uid,(preview->>'gross')::numeric);
  UPDATE achievement_card_claims SET used=used+1 WHERE user_id=uid AND key=lot.key;
  INSERT INTO achievement_card_ledger(user_id,key,delta,kind,day) VALUES(uid,lot.key,-1,'use',day_key);
  INSERT INTO achievement_checkins(user_id,day,request_key,tier,revision,gross,net,offset_amount,reason,streak,source)
  VALUES(uid,day_key,'card:'||idem::text,(preview->>'tier')::integer,rev,(cash->>'gross')::numeric,(cash->>'net')::numeric,(cash->>'offset_amount')::numeric,'card_backfill',1,'card');
  WITH islands AS(SELECT day,day-(row_number() OVER(ORDER BY day))::integer AS bucket FROM achievement_checkins WHERE user_id=uid),ranked AS(SELECT day,row_number() OVER(PARTITION BY bucket ORDER BY day)::integer AS chain FROM islands)
  UPDATE achievement_checkins c SET streak=r.chain FROM ranked r WHERE c.user_id=uid AND c.day=r.day;
  SELECT * INTO rec FROM achievement_checkins WHERE user_id=uid AND day=day_key;
  result:=to_jsonb(rec)||jsonb_build_object('cards_spent',1,'card_balance',achievement_card_balance(uid),'existing',false,'saved',true);
 END IF;
 INSERT INTO achievement_card_uses(user_id,request_key,request,response) VALUES(uid,idem,body,result);
 RETURN result||jsonb_build_object('replayed',false);
END $$;

ALTER FUNCTION achievement_admin_operation(bigint,bigint,text,jsonb) RENAME TO achievement_admin_operation_v249;
CREATE FUNCTION achievement_admin_operation(actor bigint,uid bigint,operation text,body jsonb) RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE cfg jsonb;previous achievement_operations;details jsonb;modified jsonb;result jsonb;cards jsonb;
 remaining integer;series_rec achievement_series_claims;recovered numeric;debt_added numeric;
BEGIN
 SELECT payload INTO cfg FROM achievement_config WHERE id=true FOR UPDATE;
 PERFORM 1 FROM users WHERE id IN(actor,uid) ORDER BY id FOR UPDATE;
 IF NOT EXISTS(SELECT 1 FROM users WHERE id=actor AND role='admin' AND status='active' AND deleted_at IS NULL) THEN RAISE EXCEPTION 'ACHIEVEMENT_ADMIN_FORBIDDEN'; END IF;
 IF NOT EXISTS(SELECT 1 FROM users WHERE id=uid AND role IN('user','admin') AND status='active' AND deleted_at IS NULL) THEN RAISE EXCEPTION 'ACHIEVEMENT_ACCOUNT_UNAVAILABLE'; END IF;
 details:=body||jsonb_build_object('action',operation,'user_id',uid);
 SELECT * INTO previous FROM achievement_operations WHERE actor_id=actor AND request_key=(body->>'request_key')::uuid;
 IF FOUND THEN
  IF previous.request<>details THEN RAISE EXCEPTION 'ACHIEVEMENT_ADMIN_CONFLICT'; END IF;
  RETURN previous.response||jsonb_build_object('replayed',true);
 END IF;
 modified:=body;
 IF operation='grant' AND COALESCE((body->>'grant_reward')::boolean,false) AND EXISTS(SELECT 1 FROM achievement_catalog WHERE key=body->>'key' AND card_reward>0) THEN modified:=jsonb_set(body,'{grant_reward}','false'); END IF;
 result:=achievement_admin_operation_v249(actor,uid,operation,modified);
 IF operation='grant' AND modified<>body THEN
  cards:=achievement_claim_cards(uid,body->>'key');
  result:=result||jsonb_build_object('cards_awarded',CASE WHEN COALESCE((cards->>'replayed')::boolean,false) THEN 0 ELSE (cards->>'cards_awarded')::integer END,'card_balance',achievement_card_balance(uid));
 ELSIF operation='revoke' AND COALESCE((body->>'reclaim_reward')::boolean,false) THEN
  SELECT amount-used-reclaimed INTO remaining FROM achievement_card_claims WHERE user_id=uid AND key=body->>'key';
  IF COALESCE(remaining,0)>0 THEN
   UPDATE achievement_card_claims SET reclaimed=reclaimed+remaining WHERE user_id=uid AND key=body->>'key';
   INSERT INTO achievement_card_ledger(user_id,key,delta,kind) VALUES(uid,body->>'key',-remaining,'reclaim');
  END IF;
  result:=result||jsonb_build_object('cards_reclaimed',COALESCE(remaining,0),'card_balance',achievement_card_balance(uid));
  FOR series_rec IN SELECT cl.* FROM achievement_series_claims cl JOIN achievement_series s ON s.key=cl.key WHERE cl.user_id=uid AND cl.revoked_at IS NULL AND body->>'key'=ANY(s.members) LOOP
   SELECT LEAST(GREATEST(balance,0),series_rec.net) INTO recovered FROM users WHERE id=uid;
   IF recovered>0 THEN UPDATE users SET balance=balance-recovered,updated_at=now() WHERE id=uid; END IF;
   debt_added:=series_rec.gross-recovered;
   IF debt_added>0 THEN INSERT INTO vip_reward_debt(user_id,amount) VALUES(uid,debt_added) ON CONFLICT(user_id) DO UPDATE SET amount=vip_reward_debt.amount+excluded.amount; END IF;
   UPDATE achievement_series_claims SET revoked_at=now() WHERE user_id=uid AND key=series_rec.key;
   result:=result||jsonb_build_object('series_recovered',recovered,'series_debt_added',debt_added);
  END LOOP;
 END IF;
 UPDATE achievement_operations SET request=details,response=result WHERE actor_id=actor AND request_key=(body->>'request_key')::uuid;
 RETURN result;
END $$;
