# VIP staging acceptance

This feature is opt-in. The additive migration creates VIP rules, recharge events,
manual overrides and audit records with automatic benefits disabled.

User page: `/vip`. Admin page: `/admin/vip`.

Default tiers use confirmed recharge USD 100/300/600/1500/3000,
concurrency 8/12/16/20/30, and referral percentages 2/4/6/8/10.
Group discounts and exclusive grants are configured per environment by group ID.
Private groups zth-plus/zth-pro, testing groups, and subscription groups must not
be automatically granted. Existing manual group and price overrides are preserved.

Online payments snapshot explicitly configured currency conversion and referral
percentages. Unsupported currencies block new orders only when VIP is enabled;
configure the actual USD conversion before enabling payment tests. No production
conversion rate is guessed. Historical payments are not automatically backfilled.
An audited opening record is available once per user after reviewing historical
records. It does not change the balance. Stage demo users can use these records.

Admin balance adjustments atomically append recharge changes. Payment completion
and confirmed refund triggers append idempotent principal changes. Confirmed
refunds reverse order referral credits; unrecovered commissions become debt and
offset future referral accrual. Admin recharge commissions respect the existing
affiliate switch, freeze period, duration, and cap.

VIP authentication and rates read current effective rights rather than mutating
the original auth cache or stored manual rates. This favors prompt refund
revocation; query/cache performance must be measured before a large production
rollout. RPM targets do not replace existing unlimited defaults or hard channel
limits. The interface distinguishes RPM and concurrent requests.

Acceptance: tier boundaries, duplicate callbacks, manual overrides, private-group
exclusion, partial/full refunds, referral recovery, existing Key grants, media
pricing exclusion, desktop/mobile, dark mode and precise small multipliers.

The reference site's authenticated page was not accessible during design. The
badge implements the approved Model-Gate dark/champagne style, not copied assets.

Production remains disabled until an explicit production release and configuration
review. This PR targets staging; merging to main is a separate action.
