-- Touch only this transaction's disposable wallet on the staging database.
BEGIN;
DO $$
DECLARE uid bigint; first_day date := current_date - 1; final_balance numeric;
    second_remaining numeric; expired_amount numeric;
BEGIN
    INSERT INTO users(email,password_hash,balance) VALUES
        ('checkin-expiry-' || gen_random_uuid() || '@example.invalid','fixture',10) RETURNING id INTO uid;
    UPDATE users SET balance = balance + 3 WHERE id = uid;
    INSERT INTO achievement_checkins(user_id,day,request_key,tier,revision,gross,net,reason,streak,created_at)
      VALUES (uid,first_day,'first',0,1,3,3,'eligible',1,now() - interval '1 day');
    UPDATE users SET balance = balance + 4 WHERE id = uid;
    INSERT INTO achievement_checkins(user_id,day,request_key,tier,revision,gross,net,reason,streak)
      VALUES (uid,current_date,'second',0,1,4,4,'eligible',2);
    PERFORM checkin_charge_balance(uid, 2);
    IF (SELECT remaining FROM checkin_balance_credits WHERE user_id=uid AND day=first_day) IS DISTINCT FROM 1 THEN
        RAISE EXCEPTION 'Oldest check-in credit was not spent first';
    END IF;
    UPDATE checkin_balance_credits SET expires_at=now() WHERE user_id=uid AND day=first_day;
    SELECT balance INTO final_balance FROM checkin_available_balance(uid);
    IF final_balance IS DISTINCT FROM 14 THEN RAISE EXCEPTION 'Expired credit is still spendable'; END IF;
    PERFORM checkin_lock_balance(uid);
    final_balance := checkin_lock_balance(uid);
    SELECT expired INTO expired_amount FROM checkin_balance_credits WHERE user_id=uid AND day=first_day;
    SELECT remaining INTO second_remaining FROM checkin_balance_credits WHERE user_id=uid AND day=current_date;
    IF final_balance IS DISTINCT FROM 14 OR expired_amount IS DISTINCT FROM 1 OR second_remaining IS DISTINCT FROM 4 THEN
        RAISE EXCEPTION 'Check-in expiry did not preserve cash and the later award';
    END IF;
    RAISE NOTICE 'checkin_credit_expiry fifo=passed partial_expiry=passed repeated_cleanup=passed cash_preserved=passed rollback=true';
END $$;
ROLLBACK;
