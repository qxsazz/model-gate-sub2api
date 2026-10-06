WITH source AS (
/*TOKEN_SOURCE_SQL*/
), per_user AS (
 SELECT user_id,sum(tokens)::bigint tokens,count(*) requests,
 encode(sha256(convert_to(string_agg(id::text||':'||row_hash,E'\n' ORDER BY id),'UTF8')),'hex') source_sha256
 FROM source GROUP BY user_id
), reviewed AS (
 SELECT COALESCE(jsonb_agg(jsonb_build_object('user_id',user_id,'tokens',tokens,'requests',requests,
 'source_sha256',source_sha256) ORDER BY user_id),'[]'::jsonb) users,
 COALESCE(sum(tokens),0) total,COALESCE(sum(requests),0) requests,count(*) user_count FROM per_user
)
SELECT jsonb_build_object('policy','charged-text-v1','approved',false,'actor_id',1,
 'batch_id','token-history-'||to_char(current_timestamp AT TIME ZONE 'UTC','YYYYMMDD-HH24MISS'),
 'captured_at',current_timestamp,'cutoff_usage_id',current_setting('model_gate.token_history_cutoff')::bigint,
 'expected_tokens',total,'expected_requests',requests,'expected_users',user_count,'users',users)
FROM reviewed;
