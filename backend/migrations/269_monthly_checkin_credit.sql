-- Keep previously granted credit and holds on their original deadlines.
-- NULL identifies these legacy lots while they are still available.
ALTER TABLE checkin_balance_credits ADD COLUMN credit_month date;

-- A payout can cross midnight after waiting on the wallet lock. Use its actual
-- receipt time, not the start of the transaction or the historical signed day.
ALTER TABLE achievement_checkins ALTER COLUMN created_at SET DEFAULT clock_timestamp();

CREATE OR REPLACE FUNCTION checkin_record_credit() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    available numeric;
    grant_month timestamp := date_trunc('month', NEW.created_at AT TIME ZONE 'Asia/Shanghai');
BEGIN
    -- The existing receipt trigger handles daily sign-ins and both makeup paths.
    -- Record the spendable net increase, after overdraft or reward debt.
    SELECT u.balance - COALESCE((SELECT sum(c.remaining) FROM checkin_balance_credits c
        WHERE c.user_id = NEW.user_id AND c.remaining > 0), 0)
    INTO available FROM users u WHERE u.id = NEW.user_id FOR UPDATE;

    INSERT INTO checkin_balance_credits(user_id, day, credit_month, expires_at, remaining)
    VALUES (NEW.user_id, NEW.day, grant_month::date,
        (grant_month + interval '1 month') AT TIME ZONE 'Asia/Shanghai',
        LEAST(NEW.net, GREATEST(available, 0)));
    RETURN NEW;
END $$;
