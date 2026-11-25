-- The web console supplies an optional, deployment-configured test account.
-- A blank configuration leaves the existing recharge rule in force.
CREATE FUNCTION achievement_console_checkin_exempt(uid bigint, configured_email text)
RETURNS boolean LANGUAGE sql STABLE AS $$
 SELECT COALESCE(configured_email, '') <> '' AND EXISTS (
   SELECT 1 FROM users
   WHERE id = uid AND email = configured_email
     AND role = 'admin' AND status = 'active' AND deleted_at IS NULL
 );
$$;

CREATE FUNCTION achievement_require_console_checkin_access(uid bigint, configured_email text, writing boolean)
RETURNS void LANGUAGE plpgsql AS $$
BEGIN
 IF COALESCE(configured_email, '') = '' THEN
  PERFORM achievement_require_access(uid, writing);
  RETURN;
 END IF;

 IF writing THEN
  -- Match achievement_require_access: configuration, VIP advisory lock, user.
  PERFORM 1 FROM achievement_config WHERE id = true FOR UPDATE;
  PERFORM pg_advisory_xact_lock(9471, (uid % 2147483647)::integer);
  PERFORM 1 FROM users WHERE id = uid FOR UPDATE;
 END IF;

 IF NOT achievement_console_checkin_exempt(uid, configured_email) THEN
  PERFORM achievement_require_access(uid, false);
 END IF;
END $$;

CREATE FUNCTION achievement_console_checkin(uid bigint, expected_day text, idem text, configured_email text)
RETURNS jsonb LANGUAGE plpgsql AS $$
BEGIN
 PERFORM achievement_require_console_checkin_access(uid, configured_email, true);
 RETURN achievement_checkin_before_access(uid, expected_day, idem);
END $$;
