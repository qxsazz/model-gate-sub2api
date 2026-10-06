-- New claims use 2%; preserve existing claim amounts and once-only eligibility.
CREATE OR REPLACE FUNCTION vip_claim_reward(uid bigint, target integer) RETURNS numeric LANGUAGE plpgsql AS $$
DECLARE rules jsonb; milestone jsonb; total numeric; threshold numeric; reward numeric; inserted integer;
BEGIN
 PERFORM 1 FROM users WHERE id=uid AND deleted_at IS NULL AND status='active' AND role IN ('user','admin') FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'User unavailable'; END IF;
 SELECT payload INTO rules FROM vip_rules WHERE id=true;
 IF NOT COALESCE((rules->>'enabled')::boolean,false) THEN RAISE EXCEPTION 'VIP disabled'; END IF;
 SELECT value INTO milestone FROM jsonb_array_elements(rules->'tiers') WHERE (value->>'level')::integer=target;
 IF milestone IS NULL THEN RAISE EXCEPTION 'Invalid milestone'; END IF;
 IF EXISTS(SELECT 1 FROM vip_reward_claims WHERE user_id=uid AND level=target) THEN RETURN 0; END IF;
 threshold := (milestone->>'threshold')::numeric;
 SELECT COALESCE(sum(amount),0) INTO total FROM vip_recharge_ledger WHERE user_id=uid;
 IF total<threshold THEN RAISE EXCEPTION 'Milestone not reached'; END IF;
 reward := round(threshold*0.02,2);
 IF reward<=0 THEN RAISE EXCEPTION 'Invalid reward'; END IF;
 INSERT INTO vip_reward_claims(user_id,level,threshold,amount) VALUES(uid,target,threshold,reward) ON CONFLICT DO NOTHING;
 GET DIAGNOSTICS inserted=ROW_COUNT;
 IF inserted=0 THEN RETURN 0; END IF;
 UPDATE users SET balance=balance+reward,updated_at=now() WHERE id=uid;
 INSERT INTO vip_reward_events(user_id,level,action,amount) VALUES(uid,target,'claim',reward);
 RETURN reward;
END $$;
