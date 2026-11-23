-- Historical balance mixed cash and rewards. Only a new check-in receipt creates a lot.
CREATE TABLE checkin_balance_credits (
    user_id bigint NOT NULL,
    day date NOT NULL,
    expires_at timestamptz NOT NULL,
    remaining numeric(20,8) NOT NULL CHECK (remaining >= 0),
    expired numeric(20,8) NOT NULL DEFAULT 0 CHECK (expired >= 0),
    PRIMARY KEY (user_id, day),
    FOREIGN KEY (user_id, day) REFERENCES achievement_checkins(user_id, day) ON DELETE CASCADE
);
CREATE INDEX checkin_balance_fifo ON checkin_balance_credits(user_id, expires_at, day) WHERE remaining > 0;
CREATE INDEX checkin_balance_due ON checkin_balance_credits(expires_at, user_id) WHERE remaining > 0;

-- A job can reserve parts of several rewards. A cancellation returns each part to its original deadline.
CREATE TABLE checkin_balance_holds (
    user_id bigint NOT NULL,
    batch_id text NOT NULL,
    day date NOT NULL,
    amount numeric(20,8) NOT NULL CHECK (amount > 0),
    PRIMARY KEY (user_id, batch_id, day),
    FOREIGN KEY (user_id, day) REFERENCES checkin_balance_credits(user_id, day) ON DELETE CASCADE
);

CREATE FUNCTION checkin_record_credit() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE available numeric;
BEGIN
    -- All grant paths have already credited and locked this wallet. Balance used to
    -- repay an overdraft or reward debt is no longer available to expire later.
    SELECT u.balance - COALESCE((SELECT sum(c.remaining) FROM checkin_balance_credits c
        WHERE c.user_id = NEW.user_id AND c.remaining > 0), 0)
    INTO available FROM users u WHERE u.id = NEW.user_id FOR UPDATE;
    INSERT INTO checkin_balance_credits(user_id, day, expires_at, remaining)
    VALUES (NEW.user_id, NEW.day,
        ((NEW.created_at AT TIME ZONE 'Asia/Shanghai') + interval '1 month') AT TIME ZONE 'Asia/Shanghai',
        LEAST(NEW.net, GREATEST(available, 0)));
    RETURN NEW;
END $$;
CREATE TRIGGER checkin_record_credit AFTER INSERT ON achievement_checkins
    FOR EACH ROW WHEN (NEW.net > 0) EXECUTE FUNCTION checkin_record_credit();

-- All ledger mutations take the users row lock first. The receipt is never
-- rewritten and billing retries retain their existing request-level deduplication.
CREATE FUNCTION checkin_allocate_credit(uid bigint, cost numeric, batch text DEFAULT NULL)
RETURNS void LANGUAGE plpgsql AS $$
DECLARE lot record; used numeric;
BEGIN
    IF cost <= 0 THEN RETURN; END IF;
    FOR lot IN SELECT day, remaining FROM checkin_balance_credits
        WHERE user_id = uid AND remaining > 0 AND expires_at > statement_timestamp()
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

-- A same-value UPDATE explicitly settles idle wallets. Positive deposits keep
-- their original delta for VIP debt, check-in and other credit receipts.
CREATE FUNCTION checkin_balance_debit() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE surplus numeric; removed numeric;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM checkin_balance_credits WHERE user_id = OLD.id AND remaining > 0) THEN
        RETURN NEW;
    END IF;
    WITH due AS (
        SELECT day, remaining FROM checkin_balance_credits
        WHERE user_id = OLD.id AND remaining > 0 AND expires_at <= statement_timestamp()
    ), settled AS (
        UPDATE checkin_balance_credits c SET remaining = 0, expired = c.expired + due.remaining
        FROM due WHERE c.user_id = OLD.id AND c.day = due.day RETURNING due.remaining
    ) SELECT sum(remaining) INTO removed FROM settled;
    IF NEW.balance < OLD.balance THEN
        -- Usage already allocated its cost. A cash refund or admin correction only
        -- reduces unspent gifts if the resulting wallet cannot cover them.
        SELECT sum(remaining) - GREATEST(NEW.balance - COALESCE(removed, 0), 0) INTO surplus
        FROM checkin_balance_credits WHERE user_id = OLD.id AND remaining > 0;
        IF surplus > 0 THEN PERFORM checkin_allocate_credit(OLD.id, surplus); END IF;
    END IF;
    NEW.balance := NEW.balance - COALESCE(removed, 0);
    RETURN NEW;
END $$;
CREATE TRIGGER checkin_balance_debit BEFORE UPDATE OF balance ON users FOR EACH ROW
    WHEN (NEW.balance <= OLD.balance AND NEW.frozen_balance = OLD.frozen_balance)
    EXECUTE FUNCTION checkin_balance_debit();

-- Settle before operations that compare against, cap, or replace the current
-- wallet balance. A caller holding this lock can use the returned balance directly.
CREATE FUNCTION checkin_lock_balance(uid bigint) RETURNS numeric LANGUAGE plpgsql AS $$
DECLARE wallet numeric;
BEGIN
    SELECT balance INTO wallet FROM users WHERE id = uid AND deleted_at IS NULL FOR UPDATE;
    IF FOUND AND EXISTS (SELECT 1 FROM checkin_balance_credits
            WHERE user_id = uid AND remaining > 0 AND expires_at <= statement_timestamp()) THEN
        UPDATE users SET balance = balance, updated_at = now() WHERE id = uid RETURNING balance INTO wallet;
    END IF;
    RETURN wallet;
END $$;

-- Allocate usage under the wallet lock. Preserve postpaid overdrafts and the
-- existing billing precision: round the resulting wallet balance once.
CREATE FUNCTION checkin_charge_balance(uid bigint, cost numeric) RETURNS TABLE (balance numeric, sufficient boolean) LANGUAGE plpgsql AS $$
DECLARE wallet numeric; updated numeric(20,8);
BEGIN
    wallet := checkin_lock_balance(uid);
    IF wallet IS NULL THEN RETURN; END IF;
    updated := wallet - cost;
    IF updated < wallet THEN PERFORM checkin_allocate_credit(uid, wallet - updated); END IF;
    RETURN QUERY UPDATE users u SET balance = updated, updated_at = now() WHERE u.id = uid RETURNING u.balance, wallet >= cost;
END $$;

-- The same statement snapshot supplies both the spendable amount and cache deadline.
CREATE FUNCTION checkin_available_balance(uid bigint)
RETURNS TABLE (balance numeric, next_expiry timestamptz) LANGUAGE sql STABLE AS $$
    SELECT u.balance - COALESCE(sum(c.remaining) FILTER (WHERE c.expires_at <= statement_timestamp()), 0),
           min(c.expires_at) FILTER (WHERE c.expires_at > statement_timestamp())
    FROM users u LEFT JOIN checkin_balance_credits c ON c.user_id = u.id AND c.remaining > 0
    WHERE u.id = uid AND u.deleted_at IS NULL GROUP BY u.id
$$;

CREATE FUNCTION checkin_image_balance(uid bigint, batch text, held numeric, actual numeric)
RETURNS TABLE (balance numeric, frozen_balance numeric) LANGUAGE plpgsql AS $$
DECLARE wallet numeric; frozen numeric; lot record; change_due numeric;
    charge_left numeric; give_back numeric; surplus numeric;
BEGIN
    wallet := checkin_lock_balance(uid);
    IF wallet IS NULL THEN RETURN; END IF;
    SELECT u.frozen_balance INTO frozen FROM users u WHERE u.id = uid;
    IF actual IS NULL THEN
        IF held <= 0 OR wallet < held THEN RETURN; END IF;
        held := round(held, 8);
        PERFORM checkin_allocate_credit(uid, held, batch);
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
            IF lot.expires_at <= statement_timestamp() THEN
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
    IF surplus > 0 THEN PERFORM checkin_allocate_credit(uid, surplus); END IF;
    RETURN NEXT;
END $$;

-- Process small batches on every instance without waiting behind a live billing transaction.
CREATE FUNCTION checkin_expire_balances(batch_size integer) RETURNS SETOF bigint LANGUAGE sql AS $$
    WITH due AS (
        SELECT c.user_id, min(c.expires_at) AS deadline FROM checkin_balance_credits c
        WHERE c.remaining > 0 AND c.expires_at <= statement_timestamp() GROUP BY c.user_id
    ), wallets AS (
        SELECT u.id FROM due JOIN users u ON u.id = due.user_id ORDER BY due.deadline, u.id
        LIMIT batch_size FOR UPDATE OF u SKIP LOCKED
    )
    UPDATE users u SET balance = u.balance, updated_at = now() FROM wallets
    WHERE u.id = wallets.id RETURNING u.id
$$;

-- Refund clawbacks and medal revocation cap recoveries to available funds.
CREATE FUNCTION checkin_settle_recharge_refund() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM checkin_lock_balance(NEW.user_id);
    RETURN NEW;
END $$;
CREATE TRIGGER checkin_settle_recharge_refund BEFORE INSERT ON vip_recharge_ledger
    FOR EACH ROW WHEN (NEW.amount < 0) EXECUTE FUNCTION checkin_settle_recharge_refund();

ALTER FUNCTION achievement_admin_operation(bigint,bigint,text,jsonb) RENAME TO achievement_admin_operation_v251;
CREATE FUNCTION achievement_admin_operation(actor bigint, uid bigint, operation text, body jsonb)
RETURNS jsonb LANGUAGE plpgsql AS $$
BEGIN
    IF operation = 'revoke' THEN
        -- Match the existing configuration/user lock order before inspecting recoverable cash.
        PERFORM 1 FROM achievement_config WHERE id FOR UPDATE;
        PERFORM 1 FROM users WHERE id IN (actor, uid) ORDER BY id FOR UPDATE;
        PERFORM checkin_lock_balance(uid);
    END IF;
    RETURN achievement_admin_operation_v251(actor, uid, operation, body);
END $$;
