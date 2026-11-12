-- Chat identities are independent of login/OAuth identities. A service credential
-- may only assert a configured bot/group; website sessions choose the user.
CREATE TABLE chat_bindings (
    id bigserial PRIMARY KEY,
    identity_key text NOT NULL CHECK (length(identity_key) = 64),
    identity jsonb NOT NULL CHECK (jsonb_typeof(identity) = 'object'),
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    revoked_at timestamptz
);
CREATE UNIQUE INDEX chat_bindings_active_identity ON chat_bindings(identity_key) WHERE revoked_at IS NULL;
CREATE UNIQUE INDEX chat_bindings_active_tuple ON chat_bindings
    ((identity->>'platform'), (identity->>'bot_id'), (identity->>'group_id'), (identity->>'sender_id'))
    WHERE revoked_at IS NULL;
CREATE INDEX chat_bindings_user ON chat_bindings(user_id, created_at DESC);

CREATE TABLE chat_binding_challenges (
    id uuid PRIMARY KEY,
    identity_key text NOT NULL CHECK (length(identity_key) = 64),
    identity jsonb NOT NULL,
    request_key text NOT NULL UNIQUE,
    user_id bigint REFERENCES users(id) ON DELETE CASCADE,
    code_hash text CHECK (code_hash IS NULL OR length(code_hash) = 64),
    failed_attempts integer NOT NULL DEFAULT 0 CHECK (failed_attempts BETWEEN 0 AND 5),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    expires_at timestamptz NOT NULL,
    authorized_at timestamptz,
    consumed_at timestamptz,
    superseded_at timestamptz
);
CREATE UNIQUE INDEX chat_binding_challenges_pending_identity ON chat_binding_challenges(identity_key)
    WHERE consumed_at IS NULL AND superseded_at IS NULL;
CREATE INDEX chat_binding_challenges_expiry ON chat_binding_challenges(expires_at);

-- Receipts outlive unbinding. No plaintext confirmation code or account details
-- are stored in the public receipt. Do not prune these without a replay tombstone.
CREATE TABLE chat_bot_events (
    request_key text PRIMARY KEY,
    identity_key text NOT NULL,
    operation text NOT NULL CHECK (operation IN ('confirm', 'checkin')),
    event_id text NOT NULL,
    payload_hash text NOT NULL,
    binding_id bigint REFERENCES chat_bindings(id) ON DELETE SET NULL,
    user_id bigint REFERENCES users(id) ON DELETE SET NULL,
    receipt jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX chat_bot_events_user ON chat_bot_events(user_id, created_at DESC);

CREATE FUNCTION chat_binding_lock(ikey text) RETURNS void LANGUAGE sql AS $$
    SELECT pg_advisory_xact_lock(hashtextextended('modelgate:chat:' || ikey, 0))
$$;

CREATE FUNCTION chat_binding_create(cid uuid, ikey text, ident jsonb, req text) RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE ch chat_binding_challenges;
BEGIN
    PERFORM chat_binding_lock(ikey);
    SELECT * INTO ch FROM chat_binding_challenges WHERE request_key=req;
    IF FOUND THEN
        RETURN jsonb_build_object('challenge_id',ch.id,'expires_at',ch.expires_at,'replayed',true);
    END IF;
    IF EXISTS(SELECT 1 FROM chat_bindings WHERE identity_key=ikey AND revoked_at IS NULL) THEN
        RETURN jsonb_build_object('error','BOT_IDENTITY_ALREADY_BOUND');
    END IF;
    UPDATE chat_binding_challenges SET superseded_at=clock_timestamp()
        WHERE identity_key=ikey AND consumed_at IS NULL AND superseded_at IS NULL;
    INSERT INTO chat_binding_challenges(id,identity_key,identity,request_key,expires_at)
        VALUES(cid,ikey,ident,req,clock_timestamp()+interval '10 minutes') RETURNING * INTO ch;
    RETURN jsonb_build_object('challenge_id',ch.id,'expires_at',ch.expires_at,'replayed',false);
END $$;

CREATE FUNCTION chat_binding_authorize(cid uuid, uid bigint, digest text) RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE ch chat_binding_challenges; ikey text;
BEGIN
    SELECT identity_key INTO ikey FROM chat_binding_challenges WHERE id=cid;
    IF NOT FOUND THEN RETURN jsonb_build_object('error','BOT_BINDING_CHALLENGE_NOT_FOUND'); END IF;
    PERFORM chat_binding_lock(ikey);
    SELECT * INTO ch FROM chat_binding_challenges WHERE id=cid FOR UPDATE;
    IF ch.expires_at<=clock_timestamp() OR ch.superseded_at IS NOT NULL OR ch.failed_attempts>=5 THEN
        RETURN jsonb_build_object('error','BOT_BINDING_CHALLENGE_EXPIRED');
    END IF;
    IF ch.consumed_at IS NOT NULL THEN RETURN jsonb_build_object('error','BOT_BINDING_CHALLENGE_USED'); END IF;
    IF ch.user_id IS NOT NULL AND ch.user_id<>uid THEN RETURN jsonb_build_object('error','BOT_BINDING_CHALLENGE_CLAIMED'); END IF;
    PERFORM 1 FROM users WHERE id=uid AND status='active' AND deleted_at IS NULL AND role IN ('user','admin') FOR UPDATE;
    IF NOT FOUND THEN RETURN jsonb_build_object('error','BOT_BINDING_ACCOUNT_UNAVAILABLE'); END IF;
    UPDATE chat_binding_challenges SET user_id=uid,code_hash=digest,authorized_at=clock_timestamp() WHERE id=cid;
    RETURN jsonb_build_object('expires_at',ch.expires_at);
END $$;

CREATE FUNCTION chat_binding_confirm(ikey text, req text, event text, digest text) RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE ch chat_binding_challenges; prior chat_bot_events; binding chat_bindings; result jsonb;
BEGIN
    PERFORM chat_binding_lock(ikey);
    SELECT * INTO prior FROM chat_bot_events WHERE request_key=req;
    IF FOUND THEN
        IF prior.payload_hash<>digest OR prior.identity_key<>ikey OR prior.operation<>'confirm' THEN
            RETURN jsonb_build_object('error','BOT_EVENT_CONFLICT');
        END IF;
        RETURN prior.receipt || jsonb_build_object('replayed',true);
    END IF;
    SELECT * INTO binding FROM chat_bindings WHERE identity_key=ikey AND revoked_at IS NULL;
    IF FOUND THEN
        result:=jsonb_build_object('error','BOT_IDENTITY_ALREADY_BOUND');
    ELSE
        SELECT * INTO ch FROM chat_binding_challenges
            WHERE identity_key=ikey AND consumed_at IS NULL AND superseded_at IS NULL FOR UPDATE;
        IF NOT FOUND OR ch.code_hash IS NULL THEN result:=jsonb_build_object('error','BOT_BINDING_CODE_INVALID');
        ELSIF ch.expires_at<=clock_timestamp() OR ch.failed_attempts>=5 THEN result:=jsonb_build_object('error','BOT_BINDING_CHALLENGE_EXPIRED');
        ELSIF ch.code_hash<>digest THEN
            UPDATE chat_binding_challenges SET failed_attempts=failed_attempts+1 WHERE id=ch.id;
            result:=jsonb_build_object('error','BOT_BINDING_CODE_INVALID');
        ELSE
            PERFORM 1 FROM users WHERE id=ch.user_id AND status='active' AND deleted_at IS NULL AND role IN ('user','admin') FOR UPDATE;
            IF NOT FOUND THEN result:=jsonb_build_object('error','BOT_BINDING_ACCOUNT_UNAVAILABLE');
            ELSE
                INSERT INTO chat_bindings(identity_key,identity,user_id) VALUES(ikey,ch.identity,ch.user_id) RETURNING * INTO binding;
                UPDATE chat_binding_challenges SET consumed_at=clock_timestamp(),code_hash=NULL WHERE id=ch.id;
                result:=jsonb_build_object('status','bound');
            END IF;
        END IF;
    END IF;
    result:=result||jsonb_build_object('request_key',req,'replayed',false);
    INSERT INTO chat_bot_events(request_key,identity_key,operation,event_id,payload_hash,binding_id,user_id,receipt)
        VALUES(req,ikey,'confirm',event,digest,binding.id,binding.user_id,result);
    RETURN result;
END $$;

CREATE FUNCTION chat_binding_revoke(bid bigint, uid bigint) RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE ikey text;
BEGIN
    SELECT identity_key INTO ikey FROM chat_bindings WHERE id=bid AND user_id=uid;
    IF NOT FOUND THEN RETURN jsonb_build_object('error','BOT_BINDING_NOT_FOUND'); END IF;
    PERFORM chat_binding_lock(ikey);
    UPDATE chat_bindings SET revoked_at=clock_timestamp() WHERE id=bid AND user_id=uid AND revoked_at IS NULL;
    IF NOT FOUND THEN RETURN jsonb_build_object('revoked',true); END IF;
    UPDATE chat_binding_challenges SET superseded_at=clock_timestamp()
        WHERE identity_key=ikey AND consumed_at IS NULL AND superseded_at IS NULL;
    RETURN jsonb_build_object('revoked',true);
END $$;

CREATE FUNCTION chat_bot_checkin(ikey text, req text, event text, fingerprint text, event_day date) RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE prior chat_bot_events; binding chat_bindings; result jsonb; daily jsonb; today date; state text; failure text;
BEGIN
    -- All operations for this identity serialize with unbinding. The existing
    -- achievement function then takes configuration -> VIP advisory -> user locks,
    -- exactly as website checkin and payment/refund admission already do.
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
            -- The shared write path enforces current net recharge eligibility,
            -- per-user/day uniqueness, VIP tier, cash scope/budget and reward debt.
            daily:=achievement_checkin(binding.user_id,event_day::text,req);
            state:=CASE WHEN (daily->>'replayed')::boolean THEN 'already_checked_in' ELSE 'checked_in' END;
        EXCEPTION WHEN raise_exception THEN
            GET STACKED DIAGNOSTICS failure=MESSAGE_TEXT;
            CASE failure
                WHEN 'ACHIEVEMENT_RECHARGE_REQUIRED' THEN state:='ineligible';
                WHEN 'ACHIEVEMENT_ACCOUNT_UNAVAILABLE' THEN state:='account_unavailable';
                WHEN 'ACHIEVEMENT_DATE_CHANGED' THEN state:='date_changed';
                WHEN 'ACHIEVEMENT_ACCESS_UNAVAILABLE' THEN state:='checkin_unavailable';
                WHEN 'ACHIEVEMENT_POLICY_UNAVAILABLE' THEN state:='checkin_unavailable';
                ELSE RAISE;
            END CASE;
        END;
    END IF;
    IF state='date_changed' THEN today:=(clock_timestamp() AT TIME ZONE 'Asia/Shanghai')::date; END IF;
    result:=jsonb_build_object('status',state,'checkin_date',today::text,'request_key',req,'replayed',false);
    INSERT INTO chat_bot_events(request_key,identity_key,operation,event_id,payload_hash,binding_id,user_id,receipt)
        VALUES(req,ikey,'checkin',event,fingerprint,binding.id,binding.user_id,result);
    RETURN jsonb_build_object('receipt',result,'cache_user_id',binding.user_id);
END $$;
