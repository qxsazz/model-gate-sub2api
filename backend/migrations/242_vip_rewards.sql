-- Claims are immutable milestones; refund recovery never makes a claim reusable.
CREATE TABLE vip_reward_claims (
 user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 level integer NOT NULL CHECK(level BETWEEN 1 AND 5),
 threshold numeric(20,8) NOT NULL CHECK(threshold>0),
 amount numeric(20,8) NOT NULL CHECK(amount>0),
 claimed_at timestamptz NOT NULL DEFAULT now(),
 revoked_at timestamptz,
 PRIMARY KEY(user_id,level)
);
CREATE TABLE vip_reward_debt (
 user_id bigint PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 amount numeric(20,8) NOT NULL CHECK(amount>=0)
);
CREATE TABLE vip_reward_events (
 id bigserial PRIMARY KEY,
 user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 level integer,
 action text NOT NULL CHECK(action IN ('claim','revoke','repay')),
 amount numeric(20,8) NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX vip_reward_events_user ON vip_reward_events(user_id,id);

CREATE FUNCTION vip_claim_reward(uid bigint, target integer) RETURNS numeric LANGUAGE plpgsql AS $$
DECLARE rules jsonb; milestone jsonb; total numeric; threshold numeric; reward numeric; inserted integer;
BEGIN
 PERFORM 1 FROM users WHERE id=uid AND deleted_at IS NULL AND status='active' AND role='user' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'User unavailable'; END IF;
 SELECT payload INTO rules FROM vip_rules WHERE id=true;
 IF NOT COALESCE((rules->>'enabled')::boolean,false) THEN RAISE EXCEPTION 'VIP disabled'; END IF;
 SELECT value INTO milestone FROM jsonb_array_elements(rules->'tiers') WHERE (value->>'level')::integer=target;
 IF milestone IS NULL THEN RAISE EXCEPTION 'Invalid milestone'; END IF;
 -- A claimed/revoked milestone stays consumed even after a subsequent downgrade.
 IF EXISTS(SELECT 1 FROM vip_reward_claims WHERE user_id=uid AND level=target) THEN RETURN 0; END IF;
 threshold := (milestone->>'threshold')::numeric;
 SELECT COALESCE(sum(amount),0) INTO total FROM vip_recharge_ledger WHERE user_id=uid;
 IF total<threshold THEN RAISE EXCEPTION 'Milestone not reached'; END IF;
 reward := round(threshold*0.01,2);
 IF reward<=0 THEN RAISE EXCEPTION 'Invalid reward'; END IF;
 INSERT INTO vip_reward_claims(user_id,level,threshold,amount) VALUES(uid,target,threshold,reward) ON CONFLICT DO NOTHING;
 GET DIAGNOSTICS inserted=ROW_COUNT;
 IF inserted=0 THEN RETURN 0; END IF;
 UPDATE users SET balance=balance+reward,updated_at=now() WHERE id=uid;
 INSERT INTO vip_reward_events(user_id,level,action,amount) VALUES(uid,target,'claim',reward);
 RETURN reward;
END $$;

CREATE FUNCTION vip_recover_rewards() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE total numeric; available numeric; recovered numeric; debt numeric; claim record;
BEGIN
 IF NEW.amount>=0 THEN RETURN NEW; END IF;
 SELECT GREATEST(balance,0) INTO available FROM users WHERE id=NEW.user_id FOR UPDATE;
 SELECT COALESCE(sum(amount),0) INTO total FROM vip_recharge_ledger WHERE user_id=NEW.user_id;
 recovered:=0; debt:=0;
 FOR claim IN SELECT * FROM vip_reward_claims WHERE user_id=NEW.user_id AND revoked_at IS NULL AND threshold>total ORDER BY level FOR UPDATE LOOP
  recovered:=recovered+LEAST(available,claim.amount);
  debt:=debt+claim.amount-LEAST(available,claim.amount);
  available:=GREATEST(0,available-claim.amount);
  UPDATE vip_reward_claims SET revoked_at=now() WHERE user_id=NEW.user_id AND level=claim.level;
  INSERT INTO vip_reward_events(user_id,level,action,amount) VALUES(NEW.user_id,claim.level,'revoke',-claim.amount);
 END LOOP;
 IF recovered>0 THEN UPDATE users SET balance=balance-recovered,updated_at=now() WHERE id=NEW.user_id; END IF;
 IF debt>0 THEN INSERT INTO vip_reward_debt(user_id,amount) VALUES(NEW.user_id,debt)
  ON CONFLICT(user_id) DO UPDATE SET amount=vip_reward_debt.amount+excluded.amount; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER vip_reward_refund AFTER INSERT ON vip_recharge_ledger FOR EACH ROW EXECUTE FUNCTION vip_recover_rewards();

CREATE FUNCTION vip_repay_reward_debt() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE debt numeric; paid numeric;
BEGIN
 IF NEW.balance<=OLD.balance THEN RETURN NEW; END IF;
 SELECT amount INTO debt FROM vip_reward_debt WHERE user_id=NEW.id FOR UPDATE;
 paid:=LEAST(COALESCE(debt,0),NEW.balance-OLD.balance,GREATEST(NEW.balance,0));
 IF paid>0 THEN
  NEW.balance:=NEW.balance-paid;
  UPDATE vip_reward_debt SET amount=amount-paid WHERE user_id=NEW.id;
  INSERT INTO vip_reward_events(user_id,action,amount) VALUES(NEW.id,'repay',-paid);
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER vip_reward_debt_credit BEFORE UPDATE OF balance ON users FOR EACH ROW EXECUTE FUNCTION vip_repay_reward_debt();
