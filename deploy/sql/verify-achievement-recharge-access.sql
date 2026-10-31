-- Synthetic staging-only acceptance. No payment gateway calls or persistent user/fund changes.
BEGIN;
SET LOCAL statement_timeout = '15s';
DO $$
DECLARE uid bigint;order_id bigint;minimum numeric;nearby numeric;access jsonb;receipt jsonb;counted integer;remaining_balance numeric;
BEGIN
 SELECT (payload->>'minimum_recharge_cny')::numeric INTO minimum FROM achievement_config WHERE id=true FOR UPDATE;
 IF minimum IS NULL OR minimum<=0 THEN RAISE EXCEPTION 'achievement minimum recharge is invalid';END IF;
 INSERT INTO users(email,password_hash,balance,created_at) VALUES('achievement-access-verify-'||gen_random_uuid()::text||'@example.invalid','synthetic-unusable',minimum,now()-interval '2 days') RETURNING id INTO uid;
 nearby:=GREATEST(minimum-.01,0);
 IF nearby>0 THEN
  INSERT INTO payment_orders(user_id,amount,pay_amount,expires_at,status,payment_type,provider_snapshot,out_trade_no,paid_at,completed_at)
  VALUES(uid,nearby,nearby,now(),'COMPLETED','alipay',jsonb_build_object('currency','CNY','recharge_principal',nearby),gen_random_uuid()::text,now(),now());
 END IF;
 access:=achievement_access_snapshot(uid);
 IF (access->>'access_unlocked')::boolean OR (access->>'qualified_recharge_cny')::numeric<>nearby THEN RAISE EXCEPTION 'below-threshold access mismatch';END IF;
 BEGIN
  PERFORM achievement_checkin(uid,(clock_timestamp() AT TIME ZONE 'Asia/Shanghai')::date::text,gen_random_uuid()::text);
  RAISE EXCEPTION 'below-threshold checkin was allowed';
 EXCEPTION WHEN raise_exception THEN IF SQLERRM<>'ACHIEVEMENT_RECHARGE_REQUIRED' THEN RAISE;END IF;END;
 INSERT INTO payment_orders(user_id,amount,pay_amount,expires_at,status,payment_type,provider_snapshot,out_trade_no,paid_at,completed_at)
 VALUES(uid,.01,.01,now(),'COMPLETED','alipay','{"currency":"CNY","recharge_principal":0.01}'::jsonb,gen_random_uuid()::text,now(),now()) RETURNING id INTO order_id;
 IF NOT (achievement_access_snapshot(uid)->>'access_unlocked')::boolean THEN RAISE EXCEPTION 'split recharge did not unlock';END IF;
 receipt:=achievement_checkin(uid,(clock_timestamp() AT TIME ZONE 'Asia/Shanghai')::date::text,gen_random_uuid()::text);
 UPDATE users SET balance=0 WHERE id=uid;
 IF NOT (achievement_access_snapshot(uid)->>'access_unlocked')::boolean THEN RAISE EXCEPTION 'consumption removed admission';END IF;
 UPDATE payment_orders SET status='REFUNDED',refund_amount=.01,refund_at=now() WHERE id=order_id;
 IF (achievement_access_snapshot(uid)->>'access_unlocked')::boolean THEN RAISE EXCEPTION 'confirmed refund did not relock';END IF;
 BEGIN
  PERFORM achievement_require_access(uid,false);RAISE EXCEPTION 'refund relock bypass';
 EXCEPTION WHEN raise_exception THEN IF SQLERRM<>'ACHIEVEMENT_RECHARGE_REQUIRED' THEN RAISE;END IF;END;
 INSERT INTO payment_orders(user_id,amount,pay_amount,expires_at,status,payment_type,provider_snapshot,out_trade_no,paid_at,completed_at)
 VALUES(uid,.01,.01,now(),'COMPLETED','alipay','{"currency":"CNY","recharge_principal":0.01}'::jsonb,gen_random_uuid()::text,now(),now());
 receipt:=achievement_checkin(uid,(clock_timestamp() AT TIME ZONE 'Asia/Shanghai')::date::text,gen_random_uuid()::text);
 SELECT count(*) INTO counted FROM achievement_checkins WHERE user_id=uid;
 SELECT balance INTO remaining_balance FROM users WHERE id=uid;
 IF counted<>1 OR remaining_balance<>0 OR NOT (receipt->>'replayed')::boolean THEN RAISE EXCEPTION 'readmission duplicated historical reward';END IF;
 UPDATE users SET status='disabled' WHERE id=uid;
 BEGIN
  PERFORM achievement_require_access(uid,false);RAISE EXCEPTION 'disabled account bypass';
 EXCEPTION WHEN raise_exception THEN IF SQLERRM<>'ACHIEVEMENT_ACCOUNT_UNAVAILABLE' THEN RAISE;END IF;END;
 RAISE NOTICE 'achievement_recharge_access minimum_cny=% synthetic_flow=passed rollback=true',minimum;
END $$;
ROLLBACK;
