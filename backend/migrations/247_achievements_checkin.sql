-- Additive rollout: collection is available, all cash is OFF until configured.
CREATE TABLE achievement_config (
 id boolean PRIMARY KEY DEFAULT true CHECK(id), payload jsonb NOT NULL,
 revision bigint NOT NULL DEFAULT 1, updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO achievement_config(payload) VALUES('{"cash_enabled":false,"milestone_cash_enabled":false,"cash_allowlist":[],"daily_budget":10,"monthly_budget":100}');
CREATE TABLE achievement_daily_policy(tier integer PRIMARY KEY CHECK(tier BETWEEN 0 AND 5),amount numeric(20,8) NOT NULL CHECK(amount>=0));
INSERT INTO achievement_daily_policy VALUES(0,.01),(1,.05),(2,.10),(3,.25),(4,.50),(5,1);
CREATE TABLE achievement_catalog (
 key text PRIMARY KEY, category text NOT NULL, name text NOT NULL,
 description text NOT NULL, target bigint NOT NULL CHECK(target>0),
 reward numeric(20,8) NOT NULL CHECK(reward>=0), preview boolean NOT NULL DEFAULT false
);
INSERT INTO achievement_catalog(key,category,name,description,target,reward,preview) VALUES
 ('T01','token','星火','第一次点亮你的算力旅程。',1000000,.10,false),
 ('T02','token','星芒','让每一次思考汇成光芒。',10000000,.30,false),
 ('T03','token','星轨','持续探索，留下清晰的轨迹。',100000000,.80,false),
 ('T04','token','星河','无数次调用，汇聚成一条星河。',1000000000,2,false),
 ('T05','token','星海','在广阔的模型世界持续前行。',10000000000,5,false),
 ('T06','token','星门','开启更深远的智能探索。',100000000000,12,false),
 ('S01','sign','百日留痕','百日的坚持，刻下第一枚时光印记。',100,.20,false),
 ('S02','sign','长路同行','让坚持成为习惯，与时光同行。',199,.60,false),
 ('S03','sign','岁序全勤','日复一日，收藏完整的四季。',365,1.50,false),
 ('R01','recharge','千金铭记','以信任铸就第一枚荣誉之印。',1000,1,false),
 ('R02','recharge','恒金之印','长久的支持，沉淀为恒金印记。',5000,3,false),
 ('R03','recharge','万金典藏','将深厚的信任珍藏于荣誉册。',10000,6,false),
 ('A-K01','activity','初识星图','通过一个知识主题，点亮探索星图。',1,.10,false),
 ('A-K02','activity','协议通晓','掌握三个不同知识主题。',3,.30,false),
 ('A-K03','activity','知识典藏','完成六个主题，收藏完整知识篇章。',6,.60,false),
 ('A-X01','activity','接入初航','通过两个接入关卡，让请求顺利抵达。',2,.10,false),
 ('A-X02','activity','调用有度','通过四个实践关卡，掌握调用节奏。',4,.30,false),
 ('A-X03','activity','排障匠心','完成六个实践关卡，练就排障能力。',6,.60,false),
 ('A-C01','activity','探索留印','完成初航主题的四项任务。',1,.10,false),
 ('A-C02','activity','篇章同行','收藏三个不同活动篇章。',3,.30,true),
 ('A-C03','activity','万象收藏','收藏六个不同活动篇章。',6,.60,true);
CREATE TABLE achievement_growth (
 user_id bigint PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 tokens bigint NOT NULL DEFAULT 0 CHECK(tokens>=0),
 last_paid_at timestamptz, last_balance_paid_at timestamptz
);
CREATE TABLE achievement_checkins (
 user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 day date NOT NULL, request_key text NOT NULL,
 tier integer NOT NULL, revision bigint NOT NULL,
 gross numeric(20,8) NOT NULL DEFAULT 0, net numeric(20,8) NOT NULL DEFAULT 0,
 offset_amount numeric(20,8) NOT NULL DEFAULT 0, reason text NOT NULL,
 streak integer NOT NULL CHECK(streak>0), created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id,day), UNIQUE(user_id,request_key)
);
CREATE INDEX achievement_checkin_date ON achievement_checkins(day,user_id);
CREATE TABLE achievement_activity_passes (
 user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 kind text NOT NULL CHECK(kind IN ('knowledge','practice','theme')),
 topic text NOT NULL, passed_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id,kind,topic)
);
CREATE TABLE achievement_attempts (
 id uuid PRIMARY KEY, user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 kind text NOT NULL, topic text NOT NULL, request_key text NOT NULL,
 answers jsonb NOT NULL, score integer NOT NULL, passed boolean NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(user_id,request_key)
);
CREATE TABLE achievement_claims (
 user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 key text NOT NULL REFERENCES achievement_catalog(key),
 gross numeric(20,8) NOT NULL, net numeric(20,8) NOT NULL, offset_amount numeric(20,8) NOT NULL,
 revision bigint NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), revoked_at timestamptz,
 PRIMARY KEY(user_id,key)
);
CREATE TABLE achievement_equipment (
 user_id bigint PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 key text REFERENCES achievement_catalog(key), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE achievement_budget (
 day date PRIMARY KEY, spent numeric(20,8) NOT NULL DEFAULT 0 CHECK(spent>=0)
);
CREATE TABLE achievement_admin_audit (
 id bigserial PRIMARY KEY, actor_id bigint NOT NULL, previous jsonb NOT NULL,
 current jsonb NOT NULL, revision bigint NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);

CREATE FUNCTION achievement_progress(uid bigint, medal text) RETURNS numeric LANGUAGE sql STABLE AS $$
 SELECT CASE
 WHEN medal LIKE 'T%' THEN COALESCE((SELECT tokens FROM achievement_growth WHERE user_id=uid),0)
 WHEN medal LIKE 'S%' THEN COALESCE((SELECT max(streak) FROM achievement_checkins WHERE user_id=uid),0)
 WHEN medal LIKE 'R%' THEN GREATEST(COALESCE((SELECT sum(amount) FROM vip_recharge_ledger WHERE user_id=uid),0),0)
 WHEN medal LIKE 'A-K%' THEN (SELECT count(*) FROM achievement_activity_passes WHERE user_id=uid AND kind='knowledge')
 WHEN medal LIKE 'A-X%' THEN (SELECT count(*) FROM achievement_activity_passes WHERE user_id=uid AND kind='practice')
 WHEN medal LIKE 'A-C%' THEN (SELECT count(*) FROM achievement_activity_passes WHERE user_id=uid AND kind='theme')
 ELSE 0 END
$$;
CREATE FUNCTION achievement_tier(uid bigint) RETURNS integer LANGUAGE sql STABLE AS $$
 SELECT CASE WHEN COALESCE((SELECT (payload->>'enabled')::boolean FROM vip_rules WHERE id=true),false)
 THEN COALESCE((SELECT (tier->>'level')::integer FROM vip_rules,jsonb_array_elements(payload->'tiers') tier
 WHERE id=true AND (tier->>'threshold')::numeric<=achievement_progress(uid,'R01')
 ORDER BY (tier->>'threshold')::numeric DESC LIMIT 1),0) ELSE 0 END
$$;
-- Initial cash rollout is explicit allowlist only. No implicit unverified signup grants.
CREATE FUNCTION achievement_cash_reason(uid bigint, cfg jsonb) RETURNS text LANGUAGE sql STABLE AS $$
 SELECT CASE
 WHEN NOT COALESCE((cfg->>'cash_enabled')::boolean,false) THEN 'cash_disabled'
 WHEN NOT (cfg->'cash_allowlist' @> jsonb_build_array(uid)) THEN 'not_in_cash_pilot'
 WHEN NOT ((achievement_progress(uid,'R01')>0 AND COALESCE((SELECT sum(l.amount) FROM vip_recharge_ledger l WHERE l.user_id=uid AND l.created_at>now()-interval '30 days'
 AND (l.amount<0 OR (l.source='admin_balance' AND l.source_id NOT LIKE 'historical-admin-audit:%')
 OR (l.source='payment' AND EXISTS(SELECT 1 FROM payment_orders p WHERE p.user_id=uid AND p.id::text=l.source_id
 AND p.order_type='balance' AND p.status IN ('COMPLETED','PARTIALLY_REFUNDED')
 AND COALESCE(p.paid_at,p.completed_at,p.created_at)>now()-interval '30 days'
 AND NOT (COALESCE(p.provider_snapshot,'{}'::jsonb) ? 'vip_backfill_batch'))))),0)>0)
 OR EXISTS(SELECT 1 FROM achievement_growth WHERE user_id=uid AND last_balance_paid_at>now()-interval '30 days')) THEN 'recent_activity_required'
 ELSE 'eligible' END
$$;
CREATE FUNCTION achievement_reserve_budget(day_key date, amount numeric, cfg jsonb) RETURNS boolean LANGUAGE plpgsql AS $$
DECLARE daily numeric; monthly numeric;
BEGIN
 SELECT COALESCE(spent,0) INTO daily FROM achievement_budget WHERE day=day_key;
 SELECT COALESCE(sum(spent),0) INTO monthly FROM achievement_budget WHERE day>=date_trunc('month',day_key)::date AND day<(date_trunc('month',day_key)+interval '1 month')::date;
 IF COALESCE(daily,0)+amount>(cfg->>'daily_budget')::numeric OR monthly+amount>(cfg->>'monthly_budget')::numeric THEN RETURN false; END IF;
 INSERT INTO achievement_budget(day,spent) VALUES(day_key,amount) ON CONFLICT(day) DO UPDATE SET spent=achievement_budget.spent+excluded.spent;
 RETURN true;
END $$;

CREATE FUNCTION achievement_checkin(uid bigint, expected_day text, idem text) RETURNS jsonb LANGUAGE plpgsql AS $$
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
 reason_code:=achievement_cash_reason(uid,cfg);
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

CREATE FUNCTION achievement_claim(uid bigint, medal text) RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE cfg jsonb;rev bigint;item achievement_catalog;rec achievement_claims;before_balance numeric;after_balance numeric;
 debt_before numeric;debt_after numeric;today date;
BEGIN
 SELECT payload,revision INTO cfg,rev FROM achievement_config WHERE id=true FOR UPDATE;
 SELECT balance INTO before_balance FROM users WHERE id=uid AND deleted_at IS NULL AND status='active' AND role IN ('user','admin') FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'ACHIEVEMENT_ACCOUNT_UNAVAILABLE'; END IF;
 SELECT * INTO rec FROM achievement_claims WHERE user_id=uid AND key=medal;
 IF FOUND THEN RETURN to_jsonb(rec)||jsonb_build_object('replayed',true); END IF;
 SELECT * INTO item FROM achievement_catalog WHERE key=medal;
 IF NOT FOUND OR item.preview OR achievement_progress(uid,medal)<item.target THEN RAISE EXCEPTION 'ACHIEVEMENT_LOCKED'; END IF;
 IF achievement_cash_reason(uid,cfg)<>'eligible' OR NOT COALESCE((cfg->>'milestone_cash_enabled')::boolean,false) THEN RAISE EXCEPTION 'ACHIEVEMENT_CASH_UNAVAILABLE'; END IF;
 today:=(clock_timestamp() AT TIME ZONE 'Asia/Shanghai')::date;
 IF NOT achievement_reserve_budget(today,item.reward,cfg) THEN RAISE EXCEPTION 'ACHIEVEMENT_BUDGET_EXHAUSTED'; END IF;
 SELECT COALESCE(d.amount,0) INTO debt_before FROM vip_reward_debt d WHERE d.user_id=uid;
 UPDATE users SET balance=balance+item.reward,updated_at=now() WHERE id=uid RETURNING balance INTO after_balance;
 SELECT COALESCE(d.amount,0) INTO debt_after FROM vip_reward_debt d WHERE d.user_id=uid;
 INSERT INTO achievement_claims(user_id,key,gross,net,offset_amount,revision) VALUES(uid,medal,item.reward,after_balance-before_balance,COALESCE(debt_before,0)-COALESCE(debt_after,0),rev) RETURNING * INTO rec;
 RETURN to_jsonb(rec)||jsonb_build_object('replayed',false);
END $$;

CREATE FUNCTION achievement_equip(uid bigint, medal text) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
 PERFORM 1 FROM users WHERE id=uid AND deleted_at IS NULL AND status='active' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'ACHIEVEMENT_ACCOUNT_UNAVAILABLE'; END IF;
 IF medal<>'' AND NOT EXISTS(SELECT 1 FROM achievement_catalog WHERE key=medal AND NOT preview AND achievement_progress(uid,medal)>=target) THEN RAISE EXCEPTION 'ACHIEVEMENT_LOCKED'; END IF;
 INSERT INTO achievement_equipment(user_id,key) VALUES(uid,NULLIF(medal,'')) ON CONFLICT(user_id) DO UPDATE SET key=excluded.key,updated_at=now();
END $$;

-- A recharge reversal removes the honor and recovers each payout once. Claims
-- stay consumed forever, including after another recharge reaches the threshold.
CREATE FUNCTION achievement_refund() RETURNS trigger LANGUAGE plpgsql AS $$
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
 UPDATE achievement_equipment SET key=NULL WHERE user_id=NEW.user_id AND key LIKE 'R%' AND EXISTS(SELECT 1 FROM achievement_catalog WHERE key=achievement_equipment.key AND target>achievement_progress(NEW.user_id,'R01'));
 RETURN NEW;
END $$;
CREATE TRIGGER achievement_recharge_refund AFTER INSERT ON vip_recharge_ledger FOR EACH ROW EXECUTE FUNCTION achievement_refund();
