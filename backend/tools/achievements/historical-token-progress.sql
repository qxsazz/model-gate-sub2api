-- Operator tool, never a startup migration. Render TOKEN_SOURCE_SQL first.
-- Use one transaction, maintenance window, verified backup, and ROLLBACK preview.
DO $$
DECLARE m jsonb:=current_setting('model_gate.token_history_manifest')::jsonb;
 item jsonb; cfg jsonb; actor bigint; cutoff bigint; batch text; matched bigint;
BEGIN
 IF m->>'policy' IS DISTINCT FROM 'charged-text-v1' OR m->'approved' IS DISTINCT FROM 'true'::jsonb
 OR NOT(m ?& ARRAY['actor_id','batch_id','cutoff_usage_id','expected_tokens','expected_requests','expected_users','users'])
 OR jsonb_typeof(m->'users') IS DISTINCT FROM 'array' OR jsonb_array_length(m->'users')=0 THEN
  RAISE EXCEPTION 'Reviewed token manifest required';
 END IF;
 actor:=(m->>'actor_id')::bigint;cutoff:=(m->>'cutoff_usage_id')::bigint;batch:=m->>'batch_id';
 IF cutoff<=0 OR COALESCE(btrim(batch),'')='' THEN RAISE EXCEPTION 'Invalid token cutoff or batch'; END IF;
 SELECT payload INTO cfg FROM achievement_config WHERE id=true FOR UPDATE;
 IF NOT FOUND OR COALESCE((cfg->>'cash_enabled')::boolean,true)
 OR COALESCE((cfg->>'milestone_cash_enabled')::boolean,true) THEN
  RAISE EXCEPTION 'Achievement cash must be disabled before token backfill';
 END IF;
 IF NOT EXISTS(SELECT 1 FROM users WHERE id=actor AND role='admin' AND status='active' AND deleted_at IS NULL) THEN
  RAISE EXCEPTION 'Active backfill administrator required';
 END IF;
 CREATE TABLE IF NOT EXISTS achievement_token_history (
  user_id bigint PRIMARY KEY,tokens bigint NOT NULL CHECK(tokens>0),requests bigint NOT NULL CHECK(requests>0),
  source_sha256 text NOT NULL,cutoff_usage_id bigint NOT NULL,batch_id text NOT NULL,
  actor_id bigint NOT NULL,applied_at timestamptz NOT NULL DEFAULT now()
 );
 CREATE TEMP TABLE token_history_approved(user_id bigint PRIMARY KEY,tokens bigint,requests bigint,source_sha256 text) ON COMMIT DROP;
 FOR item IN SELECT value FROM jsonb_array_elements(m->'users') LOOP
  IF NOT(item ?& ARRAY['user_id','tokens','requests','source_sha256'])
   OR (item->>'tokens')::bigint<=0 OR (item->>'requests')::bigint<=0
   OR item->>'source_sha256' !~ '^[a-f0-9]{64}$' THEN RAISE EXCEPTION 'Invalid token source review'; END IF;
  INSERT INTO token_history_approved VALUES((item->>'user_id')::bigint,(item->>'tokens')::bigint,
   (item->>'requests')::bigint,item->>'source_sha256');
 END LOOP;
 IF (SELECT count(*) FROM token_history_approved)<>(m->>'expected_users')::bigint
 OR (SELECT sum(tokens) FROM token_history_approved)<>(m->>'expected_tokens')::bigint
 OR (SELECT sum(requests) FROM token_history_approved)<>(m->>'expected_requests')::bigint THEN
  RAISE EXCEPTION 'Reviewed token totals mismatch';
 END IF;
 PERFORM 1 FROM users WHERE id IN(SELECT user_id FROM token_history_approved) ORDER BY id FOR UPDATE;
 IF EXISTS(SELECT 1 FROM token_history_approved r LEFT JOIN users u ON u.id=r.user_id WHERE u.id IS NULL OR u.deleted_at IS NOT NULL) THEN
  RAISE EXCEPTION 'Historical token owner unavailable';
 END IF;
 SELECT count(*) INTO matched FROM achievement_token_history h JOIN token_history_approved r ON r.user_id=h.user_id;
 IF matched>0 THEN
  IF matched<>(m->>'expected_users')::bigint OR EXISTS(
   SELECT 1 FROM achievement_token_history h JOIN token_history_approved r ON r.user_id=h.user_id
   WHERE h.tokens<>r.tokens OR h.requests<>r.requests OR h.source_sha256<>r.source_sha256
    OR h.cutoff_usage_id<>cutoff OR h.batch_id<>batch) THEN
   RAISE EXCEPTION 'Existing historical batch conflicts';
  END IF;
  DROP TABLE token_history_approved;
  RETURN;
 END IF;
 IF (SELECT COALESCE(max(id),0) FROM usage_logs)<>cutoff THEN
  RAISE EXCEPTION 'Usage changed after snapshot; quiesce billing and recapture';
 END IF;
 PERFORM set_config('model_gate.token_history_cutoff',cutoff::text,true);
 CREATE TEMP TABLE token_history_source ON COMMIT DROP AS
 /*TOKEN_SOURCE_SQL*/;
 IF EXISTS(SELECT 1 FROM token_history_source GROUP BY request_id,api_key_id HAVING count(*)<>1) THEN
  RAISE EXCEPTION 'Duplicate historical usage key';
 END IF;
 CREATE TEMP TABLE token_history_actual ON COMMIT DROP AS
 SELECT user_id,sum(tokens)::bigint tokens,count(*) requests,
 encode(sha256(convert_to(string_agg(id::text||':'||row_hash,E'\n' ORDER BY id),'UTF8')),'hex') source_sha256
 FROM token_history_source GROUP BY user_id;
 IF EXISTS(SELECT 1 FROM token_history_actual a FULL JOIN token_history_approved r ON a.user_id=r.user_id
  WHERE a.user_id IS NULL OR r.user_id IS NULL OR a.tokens<>r.tokens OR a.requests<>r.requests OR a.source_sha256<>r.source_sha256) THEN
  RAISE EXCEPTION 'Historical token source changed since review';
 END IF;
 PERFORM 1 FROM achievement_growth WHERE user_id IN(SELECT user_id FROM token_history_approved) ORDER BY user_id FOR UPDATE;
 IF EXISTS(SELECT 1 FROM achievement_growth g JOIN token_history_approved r ON r.user_id=g.user_id WHERE g.tokens<>0) THEN
  RAISE EXCEPTION 'Existing token progress needs separate reconciliation';
 END IF;
 INSERT INTO achievement_token_history(user_id,tokens,requests,source_sha256,cutoff_usage_id,batch_id,actor_id)
 SELECT user_id,tokens,requests,source_sha256,cutoff,batch,actor FROM token_history_approved;
 INSERT INTO achievement_growth(user_id,tokens) SELECT user_id,tokens FROM token_history_approved
 ON CONFLICT(user_id) DO UPDATE SET tokens=achievement_growth.tokens+excluded.tokens;
 DROP TABLE token_history_actual,token_history_source,token_history_approved;
END $$;
