-- Change only future order bonuses and the VIP auto-discount policy.
-- Existing orders, balances, growth, claims and small-discount rules are unchanged.
UPDATE vip_rules
SET payload=jsonb_set(jsonb_set(jsonb_set(payload,
 '{recharge_bonus_enabled}',COALESCE(payload->'recharge_bonus_enabled','true'::jsonb),true),
 '{tiers}',COALESCE((SELECT jsonb_agg(t.value||jsonb_build_object(
   'recharge_bonus_percent',COALESCE(t.value->'recharge_bonus_percent',t.value->'level')) ORDER BY t.ordinality)
   FROM jsonb_array_elements(payload->'tiers') WITH ORDINALITY t),'[]'::jsonb),true),
 '{groups}',COALESCE((SELECT jsonb_agg(
   CASE WHEN NOT COALESCE((g.value->>'private')::boolean,false)
     AND NOT COALESCE((g.value->>'access')::boolean,false)
     AND (SELECT max(d.value::numeric) FROM jsonb_array_elements_text(g.value->'discounts') d)>0.075
   THEN jsonb_set(g.value,'{discounts}',
     (SELECT jsonb_agg(to_jsonb(CASE WHEN
       (SELECT max(d.value::numeric) FROM jsonb_array_elements_text(g.value->'discounts') d)=0.1
       THEN round(c.value::numeric*0.75,6) ELSE LEAST(c.value::numeric,0.075) END) ORDER BY c.ordinality)
       FROM jsonb_array_elements_text(g.value->'discounts') WITH ORDINALITY c),true)
   ELSE g.value END ORDER BY g.ordinality)
   FROM jsonb_array_elements(payload->'groups') WITH ORDINALITY g),'[]'::jsonb),true),
 revision=revision+1,updated_at=now()
WHERE id=true;
