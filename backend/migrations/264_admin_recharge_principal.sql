-- Only the release's explicit admin deposit/refund writer supplies this signed CNY amount.
-- Historical entries, absolute balance edits, promotional credits and legacy writers leave it NULL.
ALTER TABLE vip_recharge_ledger ADD COLUMN admin_principal_delta_cny numeric(20,8);
CREATE INDEX vip_recharge_ledger_admin_principal ON vip_recharge_ledger (user_id, id)
 WHERE source='admin_balance' AND admin_principal_delta_cny IS NOT NULL;

CREATE OR REPLACE FUNCTION achievement_access_snapshot(uid bigint) RETURNS jsonb LANGUAGE plpgsql STABLE AS $$
DECLARE minimum numeric;qualified numeric:=0;principal numeric;refunded numeric;confirmed numeric;p payment_orders;unknown boolean:=false;had_refund boolean:=false;reason text;pending boolean;
BEGIN
 SELECT achievement_access_number(COALESCE(payload->>'minimum_recharge_cny','10')) INTO minimum FROM achievement_config WHERE id=true;
 IF NOT EXISTS(SELECT 1 FROM users WHERE id=uid AND status='active' AND deleted_at IS NULL AND role IN('user','admin')) THEN
  RETURN jsonb_build_object('access_unlocked',false,'minimum_recharge_cny',minimum,'qualified_recharge_cny',NULL,'remaining_recharge_cny',NULL,'access_reason','account_unavailable');
 END IF;
 IF minimum IS NULL OR minimum<=0 OR minimum<>round(minimum,2) THEN unknown:=true; END IF;

 -- Admin adjustments are serialized by the user row lock before their ledger IDs are allocated.
 -- Ignore the excess of each refund over the admin principal available at that time.
 -- Subtracting the lowest negative running total removes those excesses without charging
 -- later deposits or online orders. For example, +5, -8, +2 leaves 2 of admin principal.
 WITH movements AS (
   SELECT admin_principal_delta_cny AS delta,
          sum(admin_principal_delta_cny) OVER (ORDER BY id ROWS UNBOUNDED PRECEDING) AS running_net
   FROM vip_recharge_ledger
   WHERE user_id=uid AND source='admin_balance' AND admin_principal_delta_cny IS NOT NULL
 ), totals AS (
   SELECT coalesce(sum(delta), 0) - least(0, coalesce(min(running_net), 0)) AS net,
          coalesce(sum(delta) FILTER (WHERE delta > 0), 0) AS deposits
   FROM movements
 )
 SELECT trunc(net, 2), deposits > net INTO qualified, had_refund FROM totals;

 FOR p IN SELECT o.* FROM payment_orders o
 WHERE o.user_id=uid AND o.order_type='balance' AND achievement_access_online(o.payment_type)
 AND (o.completed_at IS NOT NULL OR o.status IN('COMPLETED','REFUNDED','PARTIALLY_REFUNDED','REFUND_REQUESTED','REFUNDING','REFUND_PENDING','REFUND_FAILED')) LOOP
  principal:=achievement_access_principal(p);
  IF principal IS NULL THEN unknown:=true;CONTINUE; END IF;
  IF principal=0 THEN CONTINUE; END IF;
  SELECT COALESCE((SELECT confirmed_amount FROM achievement_access_refunds WHERE order_id=p.id),0) INTO confirmed;
  IF p.amount<=0 OR p.amount::text IN('NaN','Infinity','-Infinity') OR (p.status='PARTIALLY_REFUNDED' AND confirmed<=0) THEN unknown:=true;CONTINUE; END IF;
  refunded:=CASE WHEN p.status='REFUNDED' THEN principal ELSE LEAST(principal,round(principal*confirmed/p.amount,2)) END;
  qualified:=qualified+GREATEST(principal-refunded,0);
  had_refund:=had_refund OR refunded>0;
 END LOOP;
 IF unknown THEN
  RETURN jsonb_build_object('access_unlocked',false,'minimum_recharge_cny',minimum,'qualified_recharge_cny',NULL,'remaining_recharge_cny',NULL,'access_reason','verification_unavailable');
 END IF;
 SELECT EXISTS(SELECT 1 FROM payment_orders WHERE user_id=uid AND order_type='balance' AND achievement_access_online(payment_type) AND paid_at IS NOT NULL AND completed_at IS NULL AND status IN('PAID','RECHARGING','FAILED')) INTO pending;
 reason:=CASE WHEN qualified>=minimum THEN 'eligible' WHEN pending THEN 'payment_pending' WHEN had_refund THEN 'recharge_refunded' ELSE 'recharge_required' END;
 RETURN jsonb_build_object('access_unlocked',qualified>=minimum,'minimum_recharge_cny',minimum,'qualified_recharge_cny',qualified,'remaining_recharge_cny',GREATEST(minimum-qualified,0),'access_reason',reason);
END $$;
