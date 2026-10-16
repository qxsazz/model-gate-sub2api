# Group-specific VIP discounts and zero base rates

Release remains 0.2.17-mg.2 as requested by the operator. This follows the staging candidate in PR #41.

The former fixed 0.075 ceiling existed in both VIP policy validation and the effective-rate calculator (and the legacy discount override API). Merely increasing the UI input limit would leave actual billing capped. Remove that fixed ceiling from all three paths; accept finite non-negative discounts with a non-decreasing five-grade ladder. Effective rate is max(group floor, base rate minus discount), with the floor clamped to [0, base] for existing rules after a base-price reduction. Nominal discounts may exceed the base-to-floor gap: the result is clamped, preserving existing small-group rules instead of rejecting previously saved configurations.

Examples: base .35, floor .2, discount .15 gives .2; base .35, floor 0, discount .35 gives 0. A zero base stays zero. Display precise group/grade validation errors in the editor and remove the false global maximum from user-facing rules/docs.

Group creation/update accept finite rates >= 0. The editor minimum is 0. The HTTP create handler defaults omitted rates to 1, while retaining an explicitly submitted zero; no omitted field silently creates a free group. Simple-mode commercial-field sanitization remains unchanged. VIP floors already supported zero in storage and validation and are retained verbatim.

Independent image/video pricing and explicit user/group manual rate priority are unchanged. Zero applies to costs using the selected base/effective multiplier; it does not force separately configured pricing components or manual overrides to zero. Existing legacy discount overrides above .075, if any, now use their stored discount up to each group's floor for future requests.

No historical migration is edited or added. Existing configurations, balances, reward claims and usage logs are not rewritten or recalculated; future requests use the new policy. Historical migration 246 remains immutable and its installation defaults remain unchanged.

Regression coverage: group creation/editing and HTTP default-versus-zero semantics; invalid negative/non-finite rates; .35-.15=.2 and zero-floor/full-discount calculations; actual PostgreSQL group persistence, saved VIP rules and authentication snapshots; existing zero-multiplier billing tests; editor requests retain explicit zero and report non-monotonic ladders before submission.
