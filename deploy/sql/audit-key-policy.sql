BEGIN READ ONLY;
SELECT json_build_object(
  'vip_enabled', v.payload->'enabled',
  'access_threshold', v.payload->'access_threshold',
  'groups', (SELECT json_agg(json_build_object(
    'id', g.id, 'name', g.name, 'platform', g.platform,
    'exclusive', g.is_exclusive, 'status', g.status,
    'subscription_type', g.subscription_type,
    'allow_image_generation', g.allow_image_generation,
    'model_allowlist', g.model_allowlist,
    'vip_access', COALESCE((rule->>'access')::boolean, false),
    'vip_private', COALESCE((rule->>'private')::boolean, false)
  ) ORDER BY g.id)
  FROM groups g
  LEFT JOIN LATERAL (SELECT item AS rule FROM jsonb_array_elements(COALESCE(v.payload->'groups', '[]'::jsonb)) item WHERE (item->>'group_id')::bigint=g.id) rules ON true
  WHERE g.deleted_at IS NULL)
) FROM vip_rules v WHERE v.id=true;
COMMIT;
