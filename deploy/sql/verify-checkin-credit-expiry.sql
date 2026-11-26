-- Touch only this transaction's disposable wallet on the staging database.
BEGIN;
DO $$
DECLARE
    uid bigint;
    today date := (statement_timestamp() AT TIME ZONE 'Asia/Shanghai')::date;
    month_start date := date_trunc('month', today::timestamp)::date;
    cutoff timestamptz := month_start::timestamp AT TIME ZONE 'Asia/Shanghai';
    next_cutoff timestamptz := (month_start + interval '1 month') AT TIME ZONE 'Asia/Shanghai';
    makeup_day date := month_start - 1;
    available numeric;
    short_cutoff timestamptz;
BEGIN
    INSERT INTO users(email, password_hash, balance)
    VALUES ('checkin-expiry-' || gen_random_uuid() || '@example.invalid', 'fixture', 10)
    RETURNING id INTO uid;

    UPDATE users SET balance = balance + 2 WHERE id = uid;
    INSERT INTO achievement_checkins(user_id, day, request_key, tier, revision, gross, net, reason, streak, created_at)
    VALUES (uid, month_start - 3, 'old-a', 0, 1, 1, 1, 'eligible', 1, cutoff - interval '2 days'),
           (uid, month_start - 2, 'old-b', 0, 1, 1, 1, 'eligible', 2, cutoff - interval '1 second');

    UPDATE users SET balance = balance + 7 WHERE id = uid;
    INSERT INTO achievement_checkins(user_id, day, request_key, tier, revision, gross, net, reason, streak, source, created_at)
    VALUES (uid, makeup_day, 'makeup', 0, 1, 4, 4, 'eligible', 3, 'card', statement_timestamp()),
           (uid, today, 'daily', 0, 1, 3, 3, 'eligible', 4, 'user', statement_timestamp());

    IF (SELECT count(*) FROM checkin_balance_credits
        WHERE user_id = uid AND day < makeup_day AND expires_at = cutoff) <> 2 THEN
        RAISE EXCEPTION 'Different awards from the same grant month did not share a deadline';
    END IF;
    IF (SELECT count(*) FROM checkin_balance_credits
        WHERE user_id = uid AND day IN (makeup_day, today) AND expires_at = next_cutoff) <> 2 THEN
        RAISE EXCEPTION 'Daily and makeup awards did not follow the month in which they were credited';
    END IF;

    SELECT balance INTO available FROM checkin_available_balance(uid);
    IF available IS DISTINCT FROM 17 OR checkin_lock_balance(uid) IS DISTINCT FROM 17
        OR checkin_lock_balance(uid) IS DISTINCT FROM 17 THEN
        RAISE EXCEPTION 'Expired monthly credit is spendable or cleanup is not idempotent';
    END IF;
    IF (SELECT count(*) FROM checkin_balance_credits
        WHERE user_id = uid AND day < makeup_day AND expired = 1 AND remaining = 0) <> 2 THEN
        RAISE EXCEPTION 'The previous month did not clear in a single expiry';
    END IF;

    SELECT balance INTO available FROM checkin_charge_balance(uid, 2);
    IF available IS DISTINCT FROM 15 OR
        (SELECT remaining FROM checkin_balance_credits WHERE user_id = uid AND day = makeup_day) IS DISTINCT FROM 2
        OR (SELECT remaining FROM checkin_balance_credits WHERE user_id = uid AND day = today) IS DISTINCT FROM 3 THEN
        RAISE EXCEPTION 'Consumption did not use check-in credit before the other balance';
    END IF;
    RAISE NOTICE 'checkin_credit_expiry fifo=passed monthly_cutover=passed makeup=passed repeated_cleanup=passed cash_preserved=passed rollback=true';

    -- The DO statement started before this deadline. Debit must use the time
    -- of the locked wallet operation after it, and expire both unused lots.
    short_cutoff := clock_timestamp() + interval '100 milliseconds';
    UPDATE checkin_balance_credits SET expires_at = short_cutoff
    WHERE user_id = uid AND day IN (makeup_day, today);
    PERFORM pg_sleep_until(short_cutoff + interval '10 milliseconds');
    SELECT balance INTO available FROM checkin_charge_balance(uid, 1);
    IF available IS DISTINCT FROM 9 OR
        (SELECT sum(expired) FROM checkin_balance_credits
         WHERE user_id = uid AND day IN (makeup_day, today)) IS DISTINCT FROM 5 THEN
        RAISE EXCEPTION 'Debit used the old statement time after the credit deadline';
    END IF;
    RAISE NOTICE 'checkin_credit_post_lock_cutoff=passed and cash was charged after expiry';
END $$;
ROLLBACK;
