# Historical VIP recharge backfill

This operator tool is not a startup migration or a payment replay. Apply the
application migrations first, keep VIP disabled, and take a verified database
backup before any committed production execution.

Provide a reviewed JSON manifest through the transaction-local
`model_gate.vip_backfill_manifest` setting, then execute
`historical-backfill.sql` in the same transaction. Preview with `ROLLBACK`;
committed execution is a separate deployment step. The private production
manifest must not be checked into Git or exposed through staging pages.

The manifest includes an active administrator ID, unique batch ID, source
checksum, CNY growth conversion of 1, reviewed order fields, order-creation
audit IDs, positive admin credit audit IDs, and an expected net growth total.
Historical admin deductions and unverified legacy totals are omitted.

The tool verifies live source evidence, rejects changed order/refund amounts,
conflicting snapshots or ledger amounts, and refuses accounts with existing
opening credits or reward claims pending reconciliation. It writes order-linked
principal/refund growth and audit-linked admin growth, without crediting account
balances, replaying payments, changing payment status, enabling VIP, granting
overrides, or paying historical referral rewards.

The ledger uniqueness constraints prevent repeated growth. The existing payment
trigger handles future refunds using the backfilled principal snapshot. Repeat
execution while VIP is disabled is tested; a subsequent refund requires a
refreshed review, not reuse of the stale manifest.

Integration tests use fictional users and roll back all fixtures. They cover
completed recharge, full historical refund, admin credit, duplicate execution,
unchanged account balance, future partial refund and stale-plan rejection.
