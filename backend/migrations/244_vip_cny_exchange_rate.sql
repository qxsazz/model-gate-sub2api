-- Approved platform VIP growth conversion: 1 CNY = 1 USD of growth.
-- Preserve explicit rates, activation state, historical orders and balances.
UPDATE vip_rules
SET payload = jsonb_set(
      payload,
      '{exchange_rates}',
      COALESCE(NULLIF(payload->'exchange_rates', 'null'::jsonb), '{}'::jsonb)
        || '{"CNY":1}'::jsonb,
      true
    ),
    revision = revision + 1,
    updated_at = now()
WHERE id = true
  AND NOT COALESCE((payload->'exchange_rates') ? 'CNY', false);
