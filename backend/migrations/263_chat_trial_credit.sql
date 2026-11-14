CREATE TABLE chat_trial_credit_policy (
    id boolean PRIMARY KEY CHECK (id),
    enabled boolean NOT NULL DEFAULT false,
    amount numeric(20,8) NOT NULL DEFAULT 0
        CHECK (amount BETWEEN 0 AND 999999999999.99999999),
    eligible_after timestamptz,
    CHECK (NOT enabled OR (amount > 0 AND eligible_after IS NOT NULL))
);
INSERT INTO chat_trial_credit_policy(id) VALUES (true);

CREATE TABLE chat_trial_credit_claims (
    user_id bigint PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    request_key text NOT NULL UNIQUE,
    amount numeric(20,8) NOT NULL CHECK (amount > 0),
    credited_amount numeric(20,8) NOT NULL CHECK (credited_amount BETWEEN 0 AND amount),
    claimed_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

ALTER TABLE chat_bot_events DROP CONSTRAINT chat_bot_events_operation_check;
ALTER TABLE chat_bot_events ADD CONSTRAINT chat_bot_events_operation_check
    CHECK (operation IN ('confirm', 'checkin', 'trial_credit'));

CREATE FUNCTION chat_configure_trial_credit(is_enabled boolean, grant_amount numeric)
RETURNS void LANGUAGE plpgsql AS $$
BEGIN
    IF is_enabled IS NULL OR grant_amount IS NULL
        OR grant_amount NOT BETWEEN 0 AND 999999999999.99999999
        OR grant_amount<>round(grant_amount,8) OR (is_enabled AND grant_amount=0) THEN
        RAISE EXCEPTION 'CHAT_TRIAL_CREDIT_POLICY_INVALID';
    END IF;

    UPDATE chat_trial_credit_policy
    SET enabled=is_enabled, amount=grant_amount,
        eligible_after=COALESCE(eligible_after, CASE WHEN is_enabled THEN clock_timestamp() END)
    WHERE id=true;
END $$;

CREATE FUNCTION chat_bot_claim_trial_credit(ikey text, req text, event text)
RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE
    prior chat_bot_events;
    policy chat_trial_credit_policy;
    binding chat_bindings;
    registered_at timestamptz;
    previous_balance numeric;
    updated_balance numeric;
    credited numeric;
    state text;
    result jsonb;
BEGIN
    PERFORM chat_binding_lock(ikey);
    SELECT * INTO prior FROM chat_bot_events WHERE request_key=req;
    IF FOUND THEN
        IF prior.identity_key<>ikey OR prior.operation<>'trial_credit' OR prior.event_id<>event THEN
            RETURN jsonb_build_object('error','BOT_EVENT_CONFLICT');
        END IF;
        RETURN jsonb_build_object(
            'receipt',prior.receipt||jsonb_build_object('replayed',true),'cache_user_id',prior.user_id);
    END IF;

    SELECT * INTO policy FROM chat_trial_credit_policy WHERE id=true FOR SHARE;
    IF NOT FOUND OR NOT policy.enabled THEN
        RETURN jsonb_build_object('receipt',jsonb_build_object(
            'status','disabled','request_key',req,'replayed',false));
    END IF;

    SELECT * INTO binding FROM chat_bindings WHERE identity_key=ikey AND revoked_at IS NULL;
    IF NOT FOUND THEN
        state:='not_bound';
    ELSE
        -- Both chat platforms take the same user row lock before checking the lifetime grant.
        SELECT created_at,balance INTO registered_at,previous_balance
        FROM users WHERE id=binding.user_id AND status='active' AND deleted_at IS NULL
        AND role IN ('user','admin') FOR UPDATE;
        IF NOT FOUND THEN
            state:='account_unavailable';
        ELSIF registered_at<=policy.eligible_after THEN
            state:='not_new';
        ELSIF EXISTS(SELECT 1 FROM chat_trial_credit_claims WHERE user_id=binding.user_id) THEN
            state:='already_claimed';
        ELSE
            UPDATE users SET balance=balance+policy.amount,updated_at=now()
            WHERE id=binding.user_id RETURNING balance INTO updated_balance;
            credited:=updated_balance-previous_balance;
            INSERT INTO chat_trial_credit_claims(user_id,request_key,amount,credited_amount)
            VALUES(binding.user_id,req,policy.amount,credited);
            -- The history row records actual credited balance after existing reward-debt handling.
            INSERT INTO redeem_codes(code,type,value,status,used_by,used_at)
            VALUES(replace(gen_random_uuid()::text,'-',''),'trial_credit',credited,'used',binding.user_id,now());
            state:='claimed';
        END IF;
    END IF;

    result:=jsonb_build_object('status',state,'request_key',req,'replayed',false);
    INSERT INTO chat_bot_events(request_key,identity_key,operation,event_id,payload_hash,binding_id,user_id,receipt)
    VALUES(req,ikey,'trial_credit',event,'',binding.id,binding.user_id,result);
    RETURN jsonb_build_object('receipt',result,'cache_user_id',binding.user_id);
END $$;
