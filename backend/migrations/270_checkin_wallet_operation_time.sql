-- Pick a wallet operation's deadline after it acquires the user row lock. In a
-- statement queued before midnight, statement_timestamp() can still be yesterday.
-- Pure reads keep using their statement snapshot.
CREATE FUNCTION checkin_wallet_cutoff(uid bigint) RETURNS timestamptz LANGUAGE plpgsql AS $$
DECLARE scope text := current_setting('model_gate.checkin_wallet_operation', true);
BEGIN
    IF split_part(scope, '|', 1) = uid::text THEN
        RETURN split_part(scope, '|', 2)::timestamptz;
    END IF;
    RETURN clock_timestamp();
END $$;

-- The scoped value lets the generic balance trigger use the same instant as
-- the debit or capped refund calling it, including across statements in a
-- transaction. Bind it to the locked wallet and restore it at the operation's
-- boundary. The timestamp uses a fixed representation including the timezone.
CREATE FUNCTION checkin_prepare_wallet_operation(uid bigint)
RETURNS TABLE(wallet numeric, cutoff timestamptz) LANGUAGE plpgsql AS $$
BEGIN
    SELECT u.balance INTO wallet FROM users u WHERE u.id = uid AND u.deleted_at IS NULL FOR UPDATE;
    IF NOT FOUND THEN RETURN; END IF;
    cutoff := checkin_wallet_cutoff(uid);
    PERFORM set_config('model_gate.checkin_wallet_operation',
        uid::text || '|' || to_char(cutoff AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'), true);
    IF EXISTS (SELECT 1 FROM checkin_balance_credits
            WHERE user_id = uid AND remaining > 0 AND expires_at <= cutoff) THEN
        UPDATE users SET balance = balance, updated_at = now() WHERE id = uid RETURNING balance INTO wallet;
    END IF;
    RETURN NEXT;
END $$;

CREATE FUNCTION checkin_begin_wallet_operation(uid bigint) RETURNS text LANGUAGE plpgsql AS $$
DECLARE previous text := COALESCE(current_setting('model_gate.checkin_wallet_operation', true), '');
BEGIN
    PERFORM checkin_prepare_wallet_operation(uid);
    RETURN previous;
END $$;

CREATE FUNCTION checkin_end_wallet_operation(previous text) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
    PERFORM set_config('model_gate.checkin_wallet_operation', previous, true);
END $$;

CREATE FUNCTION checkin_allocate_credit(uid bigint, cost numeric, batch text, cutoff timestamptz)
RETURNS void LANGUAGE plpgsql AS $$
DECLARE lot record; used numeric;
BEGIN
    IF cost <= 0 THEN RETURN; END IF;
    FOR lot IN SELECT day, remaining FROM checkin_balance_credits
        WHERE user_id = uid AND remaining > 0 AND expires_at > cutoff
        ORDER BY expires_at, day
    LOOP
        used := LEAST(cost, lot.remaining);
        UPDATE checkin_balance_credits SET remaining = remaining - used WHERE user_id = uid AND day = lot.day;
        IF batch IS NOT NULL THEN
            INSERT INTO checkin_balance_holds(user_id, batch_id, day, amount) VALUES (uid, batch, lot.day, used);
        END IF;
        cost := cost - used;
        EXIT WHEN cost = 0;
    END LOOP;
END $$;

CREATE OR REPLACE FUNCTION checkin_allocate_credit(uid bigint, cost numeric, batch text DEFAULT NULL)
RETURNS void LANGUAGE plpgsql AS $$
BEGIN
    IF cost <= 0 THEN RETURN; END IF;
    PERFORM 1 FROM users WHERE id = uid FOR UPDATE;
    IF FOUND THEN PERFORM checkin_allocate_credit(uid, cost, batch, checkin_wallet_cutoff(uid)); END IF;
END $$;

CREATE OR REPLACE FUNCTION checkin_balance_debit() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE surplus numeric; removed numeric; cutoff timestamptz;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM checkin_balance_credits WHERE user_id = OLD.id AND remaining > 0) THEN
        RETURN NEW;
    END IF;
    -- A BEFORE ROW update trigger already holds the row lock.
    cutoff := checkin_wallet_cutoff(OLD.id);
    WITH due AS (
        SELECT day, remaining FROM checkin_balance_credits
        WHERE user_id = OLD.id AND remaining > 0 AND expires_at <= cutoff
    ), settled AS (
        UPDATE checkin_balance_credits c SET remaining = 0, expired = c.expired + due.remaining
        FROM due WHERE c.user_id = OLD.id AND c.day = due.day RETURNING due.remaining
    ) SELECT sum(remaining) INTO removed FROM settled;
    IF NEW.balance < OLD.balance THEN
        -- Usage already allocated its cost. A cash refund or admin correction only
        -- reduces unspent gifts if the resulting wallet cannot cover them.
        SELECT sum(remaining) - GREATEST(NEW.balance - COALESCE(removed, 0), 0) INTO surplus
        FROM checkin_balance_credits WHERE user_id = OLD.id AND remaining > 0;
        IF surplus > 0 THEN PERFORM checkin_allocate_credit(OLD.id, surplus, NULL, cutoff); END IF;
    END IF;
    NEW.balance := NEW.balance - COALESCE(removed, 0);
    RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION checkin_lock_balance(uid bigint) RETURNS numeric LANGUAGE plpgsql AS $$
DECLARE previous text := COALESCE(current_setting('model_gate.checkin_wallet_operation', true), ''); wallet numeric;
BEGIN
    SELECT operation.wallet INTO wallet FROM checkin_prepare_wallet_operation(uid) operation;
    PERFORM checkin_end_wallet_operation(previous);
    RETURN wallet;
END $$;

-- PostgreSQL restores the local setting when a function with a SET clause
-- returns or unwinds, including early failures in a hold or capture.
CREATE OR REPLACE FUNCTION checkin_charge_balance(uid bigint, cost numeric)
RETURNS TABLE (balance numeric, sufficient boolean)
LANGUAGE plpgsql SET model_gate.checkin_wallet_operation TO '' AS $$
DECLARE wallet numeric; cutoff timestamptz; updated numeric(20,8);
BEGIN
    SELECT operation.wallet, operation.cutoff INTO wallet, cutoff FROM checkin_prepare_wallet_operation(uid) operation;
    IF NOT FOUND THEN RETURN; END IF;
    updated := wallet - cost;
    IF updated < wallet THEN PERFORM checkin_allocate_credit(uid, wallet - updated, NULL, cutoff); END IF;
    RETURN QUERY UPDATE users u SET balance = updated, updated_at = now() WHERE u.id = uid RETURNING u.balance, wallet >= cost;
END $$;

CREATE OR REPLACE FUNCTION checkin_image_balance(uid bigint, batch text, held numeric, actual numeric)
RETURNS TABLE (balance numeric, frozen_balance numeric)
LANGUAGE plpgsql SET model_gate.checkin_wallet_operation TO '' AS $$
DECLARE wallet numeric; cutoff timestamptz; frozen numeric; lot record;
    change_due numeric; charge_left numeric; give_back numeric; surplus numeric;
BEGIN
    SELECT operation.wallet, operation.cutoff INTO wallet, cutoff FROM checkin_prepare_wallet_operation(uid) operation;
    IF NOT FOUND THEN RETURN; END IF;
    SELECT u.frozen_balance INTO frozen FROM users u WHERE u.id = uid;
    IF actual IS NULL THEN
        IF held <= 0 OR wallet < held THEN RETURN; END IF;
        held := round(held, 8);
        PERFORM checkin_allocate_credit(uid, held, batch, cutoff);
        RETURN QUERY UPDATE users u SET balance = u.balance - held, frozen_balance = u.frozen_balance + held,
            updated_at = now() WHERE u.id = uid RETURNING u.balance, u.frozen_balance;
        RETURN;
    END IF;
    held := round(held, 8);
    actual := round(actual, 8);
    IF frozen < held OR actual < 0 THEN RETURN; END IF;
    charge_left := actual;
    change_due := held - actual;
    FOR lot IN SELECT h.day, h.amount, c.expires_at FROM checkin_balance_holds h
        JOIN checkin_balance_credits c USING (user_id, day)
        WHERE h.user_id = uid AND h.batch_id = batch ORDER BY c.expires_at, h.day
    LOOP
        give_back := GREATEST(lot.amount - charge_left, 0);
        charge_left := GREATEST(charge_left - lot.amount, 0);
        IF give_back > 0 THEN
            IF lot.expires_at <= cutoff THEN
                change_due := change_due - give_back;
                UPDATE checkin_balance_credits SET expired = expired + give_back WHERE user_id = uid AND day = lot.day;
            ELSE
                UPDATE checkin_balance_credits SET remaining = remaining + give_back WHERE user_id = uid AND day = lot.day;
            END IF;
        END IF;
    END LOOP;
    DELETE FROM checkin_balance_holds WHERE user_id = uid AND batch_id = batch;
    UPDATE users u SET balance = u.balance + change_due, frozen_balance = u.frozen_balance - held,
        updated_at = now() WHERE u.id = uid RETURNING u.balance, u.frozen_balance INTO balance, frozen_balance;
    -- A returned hold can also clear an overdraft or VIP reward debt.
    SELECT sum(c.remaining) - GREATEST(balance, 0) INTO surplus FROM checkin_balance_credits c
        WHERE c.user_id = uid AND c.remaining > 0;
    IF surplus > 0 THEN PERFORM checkin_allocate_credit(uid, surplus, NULL, cutoff); END IF;
    RETURN NEXT;
END $$;

CREATE OR REPLACE FUNCTION achievement_admin_operation(actor bigint, uid bigint, operation text, body jsonb)
RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE previous text; result jsonb;
BEGIN
    IF operation IS DISTINCT FROM 'revoke' THEN
        RETURN achievement_admin_operation_v251(actor, uid, operation, body);
    END IF;
    -- Match the existing configuration/user lock order before inspecting recoverable cash.
    PERFORM 1 FROM achievement_config WHERE id FOR UPDATE;
    PERFORM 1 FROM users WHERE id IN (actor, uid) ORDER BY id FOR UPDATE;
    previous := checkin_begin_wallet_operation(uid);
    result := achievement_admin_operation_v251(actor, uid, operation, body);
    PERFORM checkin_end_wallet_operation(previous);
    RETURN result;
END $$;

-- A negative ledger entry still locks and settles before insertion (migration
-- 267). Each recovery trigger also pins the cutoff across its own capped wallet
-- reads and writes, or inherits an enclosing administrative balance operation.
CREATE OR REPLACE FUNCTION achievement_refund() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE rec record; available numeric; recovered numeric; remaining numeric := 0; previous text;
BEGIN
    IF NEW.amount >= 0 THEN RETURN NEW; END IF;
    previous := checkin_begin_wallet_operation(NEW.user_id);
    SELECT GREATEST(balance, 0) INTO available FROM users WHERE id = NEW.user_id FOR UPDATE;
    FOR rec IN SELECT ac.* FROM achievement_claims ac JOIN achievement_catalog cat ON cat.key = ac.key
        WHERE ac.user_id = NEW.user_id AND cat.category = 'recharge' AND ac.revoked_at IS NULL
        AND cat.target > achievement_progress(NEW.user_id, 'R01') FOR UPDATE OF ac
    LOOP
        recovered := LEAST(available, rec.net);
        IF recovered > 0 THEN UPDATE users SET balance = balance - recovered WHERE id = NEW.user_id; END IF;
        available := available - recovered;
        -- Restore debt previously paid by this reward as well as unrecovered cash.
        remaining := remaining + rec.gross - recovered;
        UPDATE achievement_claims SET revoked_at = now() WHERE user_id = NEW.user_id AND key = rec.key;
    END LOOP;
    IF remaining > 0 THEN INSERT INTO vip_reward_debt(user_id, amount) VALUES (NEW.user_id, remaining)
        ON CONFLICT(user_id) DO UPDATE SET amount = vip_reward_debt.amount + excluded.amount; END IF;
    UPDATE achievement_equipment SET key = NULL WHERE user_id = NEW.user_id AND key LIKE 'R%'
        AND NOT achievement_is_unlocked(NEW.user_id, achievement_equipment.key);
    PERFORM checkin_end_wallet_operation(previous);
    RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION vip_recover_rewards() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE total numeric; available numeric; recovered numeric := 0; debt numeric := 0; claim record; previous text;
BEGIN
    IF NEW.amount >= 0 THEN RETURN NEW; END IF;
    previous := checkin_begin_wallet_operation(NEW.user_id);
    SELECT GREATEST(balance, 0) INTO available FROM users WHERE id = NEW.user_id FOR UPDATE;
    SELECT COALESCE(sum(amount), 0) INTO total FROM vip_recharge_ledger WHERE user_id = NEW.user_id;
    FOR claim IN SELECT * FROM vip_reward_claims
        WHERE user_id = NEW.user_id AND revoked_at IS NULL AND threshold > total ORDER BY level FOR UPDATE
    LOOP
        recovered := recovered + LEAST(available, claim.amount);
        debt := debt + claim.amount - LEAST(available, claim.amount);
        available := GREATEST(0, available - claim.amount);
        UPDATE vip_reward_claims SET revoked_at = now() WHERE user_id = NEW.user_id AND level = claim.level;
        INSERT INTO vip_reward_events(user_id, level, action, amount) VALUES (NEW.user_id, claim.level, 'revoke', -claim.amount);
    END LOOP;
    IF recovered > 0 THEN UPDATE users SET balance = balance - recovered, updated_at = now() WHERE id = NEW.user_id; END IF;
    IF debt > 0 THEN INSERT INTO vip_reward_debt(user_id, amount) VALUES (NEW.user_id, debt)
        ON CONFLICT(user_id) DO UPDATE SET amount = vip_reward_debt.amount + excluded.amount; END IF;
    PERFORM checkin_end_wallet_operation(previous);
    RETURN NEW;
END $$;
