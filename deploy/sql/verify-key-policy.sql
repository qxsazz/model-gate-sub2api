BEGIN READ ONLY;
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM groups WHERE deleted_at IS NULL AND platform='grok'
      AND name IN ('grok分组（只可聊天使用）','grok分组(只可聊天使用)')
      AND (allow_image_generation OR NOT COALESCE((model_allowlist->>'enabled')::boolean,false)
        OR EXISTS (SELECT 1 FROM jsonb_array_elements_text(model_allowlist->'models') m WHERE m ~* 'grok-imagine'))) THEN
    RAISE EXCEPTION 'chat-only Grok group still permits media models';
  END IF;
  IF EXISTS (SELECT 1 FROM groups WHERE deleted_at IS NULL
      AND name IN ('专属生图分组','gemini专属生图') AND NOT is_exclusive) THEN
    RAISE EXCEPTION 'dedicated image group must be exclusive';
  END IF;
END $$;
COMMIT;
