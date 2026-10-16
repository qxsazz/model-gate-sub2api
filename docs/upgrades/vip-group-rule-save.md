# VIP group rule save after group deletion

Release version: 0.2.17-mg.2, including both the reward-history and VIP group rule fixes.

## Reproduction and cause

A saved VIP config keeps group rules by group_id in its JSON payload. Deleting a group does not remove its historical VIP rule. The VIP editor previously added newly listed groups to that payload but never removed missing IDs. Saving submits the entire rules object, and VIPSaveRules compares the supplied IDs to non-deleted groups; any missing reference produces `unknown group`, blocking both new-group configuration and edits to existing groups.

## Fix

The editor now requests all non-deleted groups with include_inactive=true. It keeps rules for inactive groups, drops only IDs absent from that complete list from the draft, and appends default rules for new groups. A notice identifies removed IDs and states that cleanup takes effect on save. Successful save clears the notice. Inactive groups are explicitly labelled.

The server continues to reject references that disappear between loading and saving, but returns the missing IDs and instructions to refresh instead of the unhelpful `unknown group` message. Rejected saves do not update the configuration.

This change does not alter VIP discount caps, monotonic tier requirements, floor constraints, access eligibility, user overrides, balances or growth records. No database migration is needed; old JSON is changed only when an administrator saves the corrected draft. No global cleanup job or production configuration write was performed.

## Validation

- VIP editor component tests cover deleted-rule removal, inclusion and preservation of inactive groups, unchanged existing discounts, editing an existing rule and configuring a new group in the same save, and the cleanup notice lifecycle.
- PostgreSQL integration test starts with three saved group rules, soft-deletes one group, verifies a stale save is rejected without modifying config, then saves a reconciled draft with existing/inactive/new groups and checks persisted rules and unchanged balance.
- Existing VIP repository regressions and a full frontend production build are run for this candidate.
