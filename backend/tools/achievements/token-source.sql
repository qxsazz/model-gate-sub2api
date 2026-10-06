SELECT l.id,l.user_id,l.api_key_id,l.request_id,
 (l.input_tokens::bigint+l.output_tokens+l.cache_creation_tokens+l.cache_read_tokens) AS tokens,
 encode(sha256(convert_to(jsonb_build_array(l.id,l.user_id,l.api_key_id,l.request_id,l.model,
 l.actual_cost,l.input_tokens,l.output_tokens,l.cache_creation_tokens,l.cache_read_tokens,
 COALESCE(d.request_fingerprint,a.request_fingerprint))::text,'UTF8')),'hex') AS row_hash
FROM usage_logs l
LEFT JOIN usage_billing_dedup d ON d.request_id=l.request_id AND d.api_key_id=l.api_key_id
LEFT JOIN usage_billing_dedup_archive a ON a.request_id=l.request_id AND a.api_key_id=l.api_key_id
WHERE l.id<=current_setting('model_gate.token_history_cutoff')::bigint
 AND round(l.actual_cost,8)>0 AND l.actual_cost<'Infinity'::numeric
 AND l.billing_mode='token'
 AND COALESCE(l.image_count,0)=0 AND COALESCE(l.video_count,0)=0
 AND COALESCE(l.image_input_tokens,0)=0 AND COALESCE(l.image_output_tokens,0)=0
 AND l.input_tokens>=0 AND l.output_tokens>=0 AND l.cache_creation_tokens>=0 AND l.cache_read_tokens>=0
 AND (l.input_tokens::bigint+l.output_tokens+l.cache_creation_tokens+l.cache_read_tokens)>0
 AND l.model !~* '(audio|image|video|tts|realtime|embed)'
 AND l.request_id IS NOT NULL AND btrim(l.request_id)<>''
 AND (d.request_id IS NOT NULL OR a.request_id IS NOT NULL)
 AND (d.request_id IS NULL OR a.request_id IS NULL OR d.request_fingerprint=a.request_fingerprint)
