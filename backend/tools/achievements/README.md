# Historical Token Progress

These are operator tools, not startup migrations. They seed only the Token
counter in `achievement_growth`. They do not debit or credit balances, pay
rewards, replay billing, modify VIP recharge, or fabricate recent-paid dates.
The small `achievement_token_history` audit table records the original source
and prevents a user from receiving the same historical seed twice.

Scope is existing stored usage only. Missing earlier history is not estimated.
The shared query counts the four recorded mutually exclusive Token buckets;
cache-creation sub-buckets are not added again. It requires a positive charge
at billing precision and matching live/archived billing dedup evidence, and
excludes known media models, image/video flags, non-Token modes, and invalid
Token fields. Ambiguous historical media requests remain outside this policy.

## Capture And Review

During the production maintenance window, pause new model billing, drain
inflight billing and usage-log queues, and verify a database backup. Keep both
achievement cash switches disabled until the reviewed backfill is complete.

Render the snapshot query:

```sh
python backend/tools/achievements/render-token-tool.py snapshot
```

Execute it in a `REPEATABLE READ READ ONLY` transaction, first setting the
transaction-local `model_gate.token_history_cutoff` to the current maximum
`usage_logs.id`. It returns a private per-user manifest with source SHA-256,
request counts, Token sums, cutoff, and `approved=false`. The default actor ID
is 1; confirm that it is the intended active administrator or explicitly
replace it during review. Do not commit real production manifests to Git.

Review the amounts, source policy and owner identities before changing the
manifest's approval to true. A pre-maintenance candidate is not an executable
frozen batch: ongoing usage requires a new capture and review at cutover.

## Preview And Apply

After deploying the achievement tables, render the apply tool:

```sh
python backend/tools/achievements/render-token-tool.py apply
```

Set `model_gate.token_history_manifest` transaction-locally to the reviewed
JSON and execute the rendered SQL in that same transaction. First preview with
`ROLLBACK`; commit is a separate authorized release action.

First execution rejects a changed usage cutoff, source hash/totals, duplicate
request keys, missing owners, enabled cash rewards, or existing nonzero Token
progress requiring reconciliation. It never overwrites progress silently.
An exact repeat is a no-op, including when new runtime increments have since
arrived. Conflicting or partially seeded batches are rejected.

Check balance, VIP recharge, reward claims and activity timestamps are unchanged
and the per-user Token counters equal the approved baseline. Only then resume
billing and separately activate the already agreed cash-reward policy. Updating
progress can unlock manual reward eligibility, but does not automatically pay it.

Integration tests use fictional transaction fixtures and cover excluded media,
free and unconfirmed usage; repeated execution; preservation of new increments;
changed sources; enabled cash; existing progress; and unchanged balances.
