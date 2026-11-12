ALTER TABLE chat_binding_challenges ADD COLUMN declined_at timestamptz;

CREATE FUNCTION chat_binding_decline(cid uuid, uid bigint) RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE ch chat_binding_challenges; ikey text; current_email text;
BEGIN
    SELECT identity_key INTO ikey FROM chat_binding_challenges WHERE id=cid;
    IF NOT FOUND THEN RETURN jsonb_build_object('error','BOT_BINDING_CHALLENGE_NOT_FOUND'); END IF;
    PERFORM chat_binding_lock(ikey);
    SELECT * INTO ch FROM chat_binding_challenges WHERE id=cid FOR UPDATE;
    IF NOT FOUND THEN RETURN jsonb_build_object('error','BOT_BINDING_CHALLENGE_NOT_FOUND'); END IF;
    IF ch.requested_email_hash IS NULL THEN RETURN jsonb_build_object('error','BOT_BINDING_DECLINE_UNAVAILABLE'); END IF;
    SELECT email INTO current_email FROM users
        WHERE id=uid AND status='active' AND deleted_at IS NULL AND role IN ('user','admin') FOR UPDATE;
    IF NOT FOUND THEN RETURN jsonb_build_object('error','BOT_BINDING_ACCOUNT_UNAVAILABLE'); END IF;
    IF chat_binding_email_hash(current_email) IS DISTINCT FROM ch.requested_email_hash OR
        (ch.user_id IS NOT NULL AND ch.user_id<>uid) THEN
        RETURN jsonb_build_object('error','BOT_BINDING_ACCOUNT_MISMATCH');
    END IF;
    IF ch.declined_at IS NOT NULL THEN
        RETURN jsonb_build_object('status','declined','expires_at',ch.expires_at);
    END IF;
    IF ch.consumed_at IS NOT NULL THEN RETURN jsonb_build_object('error','BOT_BINDING_CHALLENGE_USED'); END IF;
    IF ch.expires_at<=clock_timestamp() OR ch.superseded_at IS NOT NULL OR ch.failed_attempts>=5 THEN
        RETURN jsonb_build_object('error','BOT_BINDING_CHALLENGE_EXPIRED');
    END IF;
    UPDATE chat_binding_challenges SET user_id=uid,declined_at=clock_timestamp(),consumed_at=clock_timestamp(),code_hash=NULL WHERE id=cid;
    RETURN jsonb_build_object('status','declined','expires_at',ch.expires_at);
END $$;
