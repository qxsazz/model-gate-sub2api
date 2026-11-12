ALTER TABLE chat_binding_challenges ADD COLUMN requested_email_hash text
    CHECK (requested_email_hash ~ '^[0-9a-f]{64}$');

CREATE FUNCTION chat_binding_email_hash(email text) RETURNS text LANGUAGE sql IMMUTABLE STRICT AS $$
    SELECT encode(sha256(convert_to(lower(btrim(email)), 'UTF8')), 'hex')
$$;

CREATE FUNCTION chat_binding_create(cid uuid, ikey text, ident jsonb, req text, requested_email text)
RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE result jsonb; wanted_hash text; prior_hash text;
BEGIN
    -- Reuse the existing identity lock, expiry, and supersession behavior.
    result:=chat_binding_create(cid,ikey,ident,req);
    IF result ? 'error' THEN RETURN result; END IF;

    wanted_hash:=chat_binding_email_hash(requested_email);
    IF (result->>'replayed')::boolean THEN
        SELECT requested_email_hash INTO prior_hash FROM chat_binding_challenges WHERE request_key=req;
        IF prior_hash IS DISTINCT FROM wanted_hash THEN
            RETURN jsonb_build_object('error','BOT_EVENT_CONFLICT');
        END IF;
    ELSIF wanted_hash IS NOT NULL THEN
        UPDATE chat_binding_challenges SET requested_email_hash=wanted_hash WHERE id=cid;
    END IF;
    RETURN result;
END $$;

CREATE OR REPLACE FUNCTION chat_binding_authorize(cid uuid, uid bigint, digest text) RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE ch chat_binding_challenges; ikey text; current_email text;
BEGIN
    SELECT identity_key INTO ikey FROM chat_binding_challenges WHERE id=cid;
    IF NOT FOUND THEN RETURN jsonb_build_object('error','BOT_BINDING_CHALLENGE_NOT_FOUND'); END IF;
    PERFORM chat_binding_lock(ikey);
    SELECT * INTO ch FROM chat_binding_challenges WHERE id=cid FOR UPDATE;
    IF NOT FOUND THEN RETURN jsonb_build_object('error','BOT_BINDING_CHALLENGE_NOT_FOUND'); END IF;
    IF ch.expires_at<=clock_timestamp() OR ch.superseded_at IS NOT NULL OR ch.failed_attempts>=5 THEN
        RETURN jsonb_build_object('error','BOT_BINDING_CHALLENGE_EXPIRED');
    END IF;
    IF ch.consumed_at IS NOT NULL THEN RETURN jsonb_build_object('error','BOT_BINDING_CHALLENGE_USED'); END IF;
    IF ch.user_id IS NOT NULL AND ch.user_id<>uid THEN RETURN jsonb_build_object('error','BOT_BINDING_CHALLENGE_CLAIMED'); END IF;
    SELECT email INTO current_email FROM users
        WHERE id=uid AND status='active' AND deleted_at IS NULL AND role IN ('user','admin') FOR UPDATE;
    IF NOT FOUND THEN RETURN jsonb_build_object('error','BOT_BINDING_ACCOUNT_UNAVAILABLE'); END IF;

    IF ch.requested_email_hash IS NOT NULL THEN
        IF chat_binding_email_hash(current_email) IS DISTINCT FROM ch.requested_email_hash THEN
            RETURN jsonb_build_object('error','BOT_BINDING_ACCOUNT_MISMATCH');
        END IF;
        IF EXISTS(SELECT 1 FROM chat_bindings WHERE identity_key=ikey AND revoked_at IS NULL) THEN
            RETURN jsonb_build_object('error','BOT_IDENTITY_ALREADY_BOUND');
        END IF;
        INSERT INTO chat_bindings(identity_key,identity,user_id) VALUES(ikey,ch.identity,uid);
        UPDATE chat_binding_challenges
            SET user_id=uid,authorized_at=clock_timestamp(),consumed_at=clock_timestamp()
            WHERE id=cid;
        RETURN jsonb_build_object('status','bound','expires_at',ch.expires_at);
    END IF;

    UPDATE chat_binding_challenges SET user_id=uid,code_hash=digest,authorized_at=clock_timestamp() WHERE id=cid;
    RETURN jsonb_build_object('expires_at',ch.expires_at);
END $$;

CREATE OR REPLACE FUNCTION chat_bot_checkin(ikey text, req text, event text, fingerprint text, event_day date)
RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE
    prior chat_bot_events;
    binding chat_bindings;
    result jsonb;
    daily jsonb;
    today date;
    state text;
    failure text;
    failure_detail text;
    access jsonb;
    public_access jsonb:='{}'::jsonb;
BEGIN
    -- Keep eligibility and the Bot receipt in the same transaction.
    PERFORM chat_binding_lock(ikey);
    SELECT * INTO prior FROM chat_bot_events WHERE request_key=req;
    IF FOUND THEN
        IF prior.payload_hash<>fingerprint OR prior.identity_key<>ikey OR prior.operation<>'checkin' THEN
            RETURN jsonb_build_object('error','BOT_EVENT_CONFLICT');
        END IF;
        RETURN jsonb_build_object('receipt',prior.receipt||jsonb_build_object('replayed',true),'cache_user_id',prior.user_id);
    END IF;
    today:=(clock_timestamp() AT TIME ZONE 'Asia/Shanghai')::date;
    SELECT * INTO binding FROM chat_bindings WHERE identity_key=ikey AND revoked_at IS NULL;
    IF event_day<>today THEN state:='date_changed';
    ELSIF NOT FOUND THEN state:='not_bound';
    ELSE
        BEGIN
            daily:=achievement_checkin(binding.user_id,event_day::text,req);
            state:=CASE WHEN (daily->>'replayed')::boolean THEN 'already_checked_in' ELSE 'checked_in' END;
        EXCEPTION WHEN raise_exception THEN
            GET STACKED DIAGNOSTICS failure=MESSAGE_TEXT, failure_detail=PG_EXCEPTION_DETAIL;
            CASE failure
                WHEN 'ACHIEVEMENT_RECHARGE_REQUIRED' THEN
                    state:='ineligible';
                    access:=NULLIF(failure_detail,'')::jsonb;
                    -- Only copy the public threshold and a recognized reason from the original decision.
                    IF access->>'access_reason' IN ('recharge_required','payment_pending','recharge_refunded') THEN
                        public_access:=jsonb_build_object(
                            'access_reason',access->>'access_reason',
                            'minimum_recharge_cny',access->'minimum_recharge_cny'
                        );
                    END IF;
                WHEN 'ACHIEVEMENT_ACCOUNT_UNAVAILABLE' THEN state:='account_unavailable';
                WHEN 'ACHIEVEMENT_DATE_CHANGED' THEN state:='date_changed';
                WHEN 'ACHIEVEMENT_ACCESS_UNAVAILABLE' THEN
                    state:='checkin_unavailable';
                    public_access:=jsonb_build_object('access_reason','verification_unavailable');
                WHEN 'ACHIEVEMENT_POLICY_UNAVAILABLE' THEN state:='checkin_unavailable';
                ELSE RAISE;
            END CASE;
        END;
    END IF;
    IF state='date_changed' THEN today:=(clock_timestamp() AT TIME ZONE 'Asia/Shanghai')::date; END IF;
    result:=jsonb_build_object('status',state,'checkin_date',today::text,'request_key',req,'replayed',false)||public_access;
    INSERT INTO chat_bot_events(request_key,identity_key,operation,event_id,payload_hash,binding_id,user_id,receipt)
        VALUES(req,ikey,'checkin',event,fingerprint,binding.id,binding.user_id,result);
    RETURN jsonb_build_object('receipt',result,'cache_user_id',binding.user_id);
END $$;
