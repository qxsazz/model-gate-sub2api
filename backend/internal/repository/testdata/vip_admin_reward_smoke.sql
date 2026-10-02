-- Execute inside BEGIN/ROLLBACK, after migration 245.
DO $$
DECLARE uid bigint:=-9420244; paid numeric; total numeric;
BEGIN
 UPDATE vip_rules SET payload=jsonb_set(payload,'{enabled}','true') WHERE id=true;
 INSERT INTO users(id,email,password_hash,role,status,balance) VALUES(uid,'vip-admin-smoke@example.invalid','fixture','admin','active',0);
 INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) VALUES(uid,'opening','vip-admin-smoke-open',300);
 paid:=vip_claim_reward(uid,1);IF paid<>2 THEN RAISE EXCEPTION 'Admin VIP1 amount mismatch'; END IF;
 paid:=vip_claim_reward(uid,1);IF paid<>0 THEN RAISE EXCEPTION 'Duplicate admin claim paid twice'; END IF;
 paid:=vip_claim_reward(uid,2);IF paid<>6 THEN RAISE EXCEPTION 'Admin VIP2 amount mismatch'; END IF;
 BEGIN
  PERFORM vip_claim_reward(uid,3);
  RAISE EXCEPTION 'Admin bypassed threshold';
 EXCEPTION WHEN OTHERS THEN IF SQLERRM<>'Milestone not reached' THEN RAISE; END IF; END;
 UPDATE users SET status='disabled' WHERE id=uid;
 BEGIN
  PERFORM vip_claim_reward(uid,1);
  RAISE EXCEPTION 'Disabled admin accepted';
 EXCEPTION WHEN OTHERS THEN IF SQLERRM<>'User unavailable' THEN RAISE; END IF; END;
 UPDATE users SET status='active',deleted_at=now() WHERE id=uid;
 BEGIN
  PERFORM vip_claim_reward(uid,1);
  RAISE EXCEPTION 'Deleted admin accepted';
 EXCEPTION WHEN OTHERS THEN IF SQLERRM<>'User unavailable' THEN RAISE; END IF; END;
 UPDATE users SET deleted_at=NULL WHERE id=uid;
 INSERT INTO vip_recharge_ledger(user_id,source,source_id,amount) VALUES(uid,'admin_balance','vip-admin-smoke-refund',-250);
 SELECT balance INTO paid FROM users WHERE id=uid;IF paid<>0 THEN RAISE EXCEPTION 'Admin refund did not recover rewards'; END IF;
 SELECT sum(amount) INTO total FROM vip_recharge_ledger WHERE user_id=uid;IF total<>50 THEN RAISE EXCEPTION 'Admin bonus counted as recharge'; END IF;
 RAISE NOTICE 'Admin reward eligibility, repeat prevention and refund recovery passed';
END $$;
