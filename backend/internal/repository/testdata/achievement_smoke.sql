\set ON_ERROR_STOP on
BEGIN;
INSERT INTO users(email,password_hash,role,status,balance) VALUES('achievement-smoke@example.com','fixture','user','active',0) RETURNING id AS uid \gset
INSERT INTO vip_rules(id,payload) VALUES(true,'{"enabled":true,"tiers":[{"level":1,"threshold":100},{"level":2,"threshold":300},{"level":3,"threshold":600},{"level":4,"threshold":1500},{"level":5,"threshold":3000}]}') ON CONFLICT(id) DO UPDATE SET payload=excluded.payload;
INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) VALUES(:uid,'admin_balance','achievement-smoke',300);
SELECT achievement_checkin(:uid, (clock_timestamp() AT TIME ZONE 'Asia/Shanghai')::date::text, 'smoke-disabled') AS receipt \gset
SELECT set_config('test.uid', :'uid',true);
SELECT set_config('test.receipt', :'receipt',true);
DO $$ BEGIN
 IF (current_setting('test.receipt')::jsonb->>'gross')::numeric<>0 THEN RAISE EXCEPTION 'default cash on'; END IF;
 IF (SELECT count(*) FROM achievement_checkins WHERE user_id=current_setting('test.uid')::bigint)<>1 THEN RAISE EXCEPTION 'missing checkin'; END IF;
END $$;
SELECT achievement_checkin(:uid, (clock_timestamp() AT TIME ZONE 'Asia/Shanghai')::date::text, 'smoke-disabled');
DO $$ BEGIN
 IF (SELECT count(*) FROM achievement_checkins WHERE user_id=current_setting('test.uid')::bigint)<>1 THEN RAISE EXCEPTION 'duplicate'; END IF;
END $$;
-- Use a second account to test enabled payout and the existing debt trigger.
INSERT INTO users(email,password_hash,role,status,balance) VALUES('achievement-cash@example.com','fixture','user','active',0) RETURNING id AS cash_uid \gset
INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) VALUES(:cash_uid,'admin_balance','achievement-cash',300);
UPDATE achievement_config SET payload=jsonb_set(jsonb_set(payload,'{cash_enabled}','true'),'{cash_allowlist}',jsonb_build_array(:cash_uid));
INSERT INTO vip_reward_debt(user_id,amount) VALUES(:cash_uid,.04);
SELECT achievement_checkin(:cash_uid,(clock_timestamp() AT TIME ZONE 'Asia/Shanghai')::date::text,'smoke-cash') AS receipt \gset
SELECT set_config('test.receipt', :'receipt',true);
SELECT set_config('test.uid', :'cash_uid',true);
DO $$ BEGIN
 IF (current_setting('test.receipt')::jsonb->>'gross')::numeric<>.10 THEN RAISE EXCEPTION 'wrong VIP2 reward'; END IF;
 IF (current_setting('test.receipt')::jsonb->>'net')::numeric<>.06 THEN RAISE EXCEPTION 'wrong debt offset'; END IF;
 IF (SELECT count(*) FROM vip_recharge_ledger WHERE user_id=current_setting('test.uid')::bigint)<>1 THEN RAISE EXCEPTION 'reward counted as recharge'; END IF;
END $$;
SELECT achievement_checkin(:cash_uid,(clock_timestamp() AT TIME ZONE 'Asia/Shanghai')::date::text,'another-key');
DO $$ BEGIN
 IF (SELECT balance FROM users WHERE id=current_setting('test.uid')::bigint)<>.06 THEN RAISE EXCEPTION 'paid twice'; END IF;
END $$;
-- Existing request replay ignores stale expected date; a new stale request fails.
SELECT achievement_checkin(:cash_uid,'2000-01-01','smoke-cash');
DO $$ BEGIN
 BEGIN
  PERFORM achievement_checkin(current_setting('test.uid')::bigint,'2000-01-01','stale-new');
  RAISE EXCEPTION 'stale date accepted';
 EXCEPTION WHEN SQLSTATE 'P0001' THEN
  IF SQLERRM<>'ACHIEVEMENT_DATE_CHANGED' THEN RAISE; END IF;
 END;
END $$;
ROLLBACK;
