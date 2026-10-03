CREATE FUNCTION achievement_activity_attempt(uid bigint, kind_key text, topic_key text, attempt_id uuid, idem text, submitted jsonb, grade integer, did_pass boolean) RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE previous achievement_attempts;
BEGIN
 PERFORM 1 FROM users WHERE id=uid AND status='active' AND deleted_at IS NULL FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'ACHIEVEMENT_ACCOUNT_UNAVAILABLE'; END IF;
 SELECT * INTO previous FROM achievement_attempts WHERE user_id=uid AND request_key=idem;
 IF FOUND THEN
  IF previous.kind<>kind_key OR previous.topic<>topic_key OR previous.answers<>submitted THEN RAISE EXCEPTION 'ACTIVITY_REQUEST_CONFLICT'; END IF;
  RETURN jsonb_build_object('score',previous.score,'passed',previous.passed,'replayed',true);
 END IF;
 IF (SELECT count(*) FROM achievement_attempts WHERE user_id=uid AND created_at>now()-interval '24 hours')>=30 THEN RAISE EXCEPTION 'ACTIVITY_ATTEMPT_LIMIT'; END IF;
 INSERT INTO achievement_attempts(id,user_id,kind,topic,request_key,answers,score,passed) VALUES(attempt_id,uid,kind_key,topic_key,idem,submitted,grade,did_pass);
 IF did_pass THEN
  INSERT INTO achievement_activity_passes(user_id,kind,topic) VALUES(uid,kind_key,topic_key) ON CONFLICT DO NOTHING;
  IF EXISTS(SELECT 1 FROM achievement_activity_passes WHERE user_id=uid AND kind='knowledge' AND topic='intro')
   AND (SELECT count(*) FROM achievement_activity_passes WHERE user_id=uid AND kind='practice' AND topic IN ('endpoint','request','cost'))=3 THEN
   INSERT INTO achievement_activity_passes(user_id,kind,topic) VALUES(uid,'theme','first-voyage') ON CONFLICT DO NOTHING;
  END IF;
 END IF;
 RETURN jsonb_build_object('score',grade,'passed',did_pass,'replayed',false);
END $$;
