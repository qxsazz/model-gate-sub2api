-- The explicitly chat-only Grok group previously advertised every media model.
-- Keep public availability and existing user grants; restrict only the intended group.
UPDATE groups
SET allow_image_generation = false,
    model_allowlist = jsonb_build_object(
      'enabled', true,
      'models', (
        SELECT jsonb_agg(DISTINCT model ORDER BY model)
        FROM (
          SELECT model FROM jsonb_array_elements_text(COALESCE(groups.model_allowlist->'models', '[]'::jsonb)) model
          WHERE model !~* 'grok-imagine' AND model NOT LIKE '%*%'
          UNION
          SELECT value FROM jsonb_array_elements_text('["grok", "grok-latest", "grok-4.7", "grok-4.7-latest", "grok-4.6", "grok-4.6-latest", "grok-4.5", "grok-4.5-latest", "grok-4.3", "grok-4.3-latest", "grok-3-mini", "grok-3-mini-fast", "grok-build", "grok-build-latest", "grok-build-0.1", "grok-composer-2.5-fast", "grok-composer", "composer-2.5", "grok-4.20-reasoning", "grok-4.20-0309-reasoning", "grok-4.20-non-reasoning", "grok-4.20-0309-non-reasoning", "grok-4.20-multi-agent", "grok-4.20-multi-agent-latest", "grok-4.20-multi-agent-0309"]'::jsonb)
        ) text_models(model)
      )
    ),
    updated_at = now()
WHERE platform='grok' AND deleted_at IS NULL
  AND name IN ('grok分组（只可聊天使用）', 'grok分组(只可聊天使用)');
