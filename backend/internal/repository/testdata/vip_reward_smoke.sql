-- Run after migration 242 in a surrounding BEGIN/ROLLBACK transaction.
DO $$
DECLARE uid bigint := -9420242; reward numeric; actual numeric;
BEGIN
 UPDATE vip_rules SET payload=jsonb_set(payload,'{enabled}','true') WHERE id=true;
 INSERT INTO users(id,email,password_hash,balance,role,status) VALUES(uid,'reward-smoke@example.invalid','fixture',0,'user','active');
 BEGIN
  PERFORM vip_claim_reward(uid,1);
  RAISE EXCEPTION 'below-threshold claim accepted';
 EXCEPTION WHEN OTHERS THEN
  IF SQLERRM <> 'Milestone not reached' THEN RAISE; END IF;
 END;
 INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) VALUES(uid,'opening','reward-smoke-open',300);
 reward:=vip_claim_reward(uid,1); IF reward<>1 THEN RAISE EXCEPTION 'VIP1 reward mismatch'; END IF;
 reward:=vip_claim_reward(uid,1); IF reward<>0 THEN RAISE EXCEPTION 'duplicate reward'; END IF;
 reward:=vip_claim_reward(uid,2); IF reward<>3 THEN RAISE EXCEPTION 'VIP2 reward mismatch'; END IF;
 SELECT balance INTO actual FROM users WHERE id=uid; IF actual<>4 THEN RAISE EXCEPTION 'balance mismatch'; END IF;
 UPDATE users SET balance=1 WHERE id=uid;
 INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) VALUES(uid,'admin_balance','reward-smoke-refund',-250);
 SELECT balance INTO actual FROM users WHERE id=uid; IF actual<>0 THEN RAISE EXCEPTION 'recovery balance mismatch'; END IF;
 SELECT amount INTO actual FROM vip_reward_debt WHERE user_id=uid; IF actual<>3 THEN RAISE EXCEPTION 'debt mismatch'; END IF;
 UPDATE users SET balance=balance+2 WHERE id=uid;
 SELECT balance INTO actual FROM users WHERE id=uid; IF actual<>0 THEN RAISE EXCEPTION 'partial repay mismatch'; END IF;
 UPDATE users SET balance=balance+5 WHERE id=uid;
 SELECT balance INTO actual FROM users WHERE id=uid; IF actual<>4 THEN RAISE EXCEPTION 'full repay mismatch'; END IF;
 INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) VALUES(uid,'opening','reward-smoke-restore',250);
 reward:=vip_claim_reward(uid,2); IF reward<>0 THEN RAISE EXCEPTION 'revoked milestone reused'; END IF;
 SELECT sum(amount) INTO actual FROM vip_recharge_ledger WHERE user_id=uid; IF actual<>300 THEN RAISE EXCEPTION 'bonus included in recharge'; END IF;
 -- Exercise the existing admin CTE: a ledger trigger updates the same user again.
 INSERT INTO users(id,email,password_hash,balance,role,status) VALUES(uid-1,'reward-admin-smoke@example.invalid','fixture',100,'user','active');
 INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) VALUES(uid-1,'opening','reward-admin-open',100);
 PERFORM vip_claim_reward(uid-1,1);
 WITH changed AS (UPDATE users SET balance=balance-1 WHERE id=uid-1 RETURNING id), recorded AS (
 INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) SELECT id,'admin_balance','reward-admin-refund',-1 FROM changed RETURNING id)
 SELECT count(*) INTO actual FROM recorded;
 SELECT balance INTO actual FROM users WHERE id=uid-1; IF actual<>99 THEN RAISE EXCEPTION 'admin CTE recovery mismatch: %',actual; END IF;
 RAISE NOTICE 'VIP reward smoke passed';
END $$;
