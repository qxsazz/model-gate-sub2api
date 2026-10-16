# Model-Gate 0.2.17 compatibility merge

## Sources and scope

- Customized baseline: `origin/staging`, `0b86e5987f4a7452fdce9cd44a34c7fb6dfced20`.
- Upstream: `Wei-Shaw/sub2api` tag `v0.2.17`, peeled commit `060d0160417782cd962a0c7f460f6cd4565d4916`.
- Final staging candidate version: `0.2.17-mg.2`, including the subsequent reward-history and VIP group-save fixes. The upstream tag's VERSION file says 0.2.16; the actual source commit above identifies the imported release.
- The original working directory and existing running database are not upgrade targets. Development and preview use an independent worktree and an isolated synthetic database.
- This is a local candidate. It does not represent approval, CI completion, staging acceptance, or a production release. The separately requested `sever-generator` skill remains deferred by the user.

## Compatibility decisions

| Existing contract | New processing | Integration |
| --- | --- | --- |
| Strict VIP/manual request rate and peak pricing time | Extracted billing helpers and explicit Gemini storage charges | Both quotation and settlement use `APIKey.RequestRate`; strict lookup failures propagate instead of becoming a zero rate. Cache handlers capture the rate before upstream operations. |
| A request retains the price it started with | Storage token-hours surcharge and fixed-charge reservations | Storage unit price is captured before upstream creation/extension and carried into quotation and asynchronous settlement. Missing/non-finite/non-positive storage pricing is rejected. Pure storage operations do not incur a per-request model charge. |
| Model-Gate model aliases and channel pricing | New billing helper extraction | Keep the customized composite alias fallback and explicit channel/group pricing priorities. |
| Gemini thought text is separate from answer text | Capability-aware thought configuration, history filtering and signature retries | Keep one signature-aware response path. Signed Gemini history survives normal conversion; unsigned/redacted thoughts are dropped. Signature-error retries strip invalid historical state and restore the original client thinking intent. |
| Gemini 3 legacy budget-to-tier billing and buffered summaries | Upstream introduces different budget thresholds and summary defaults | Compatibility wrapper retains existing Gemini 3 thresholds (1024/minimal, 4096/low, 16384/medium, above/high) and summaries for buffered/streamed requests. Explicit disable intent and newer model families use upstream parsing. Actual model suffix remains authoritative. Direct upstream parser tests remain alongside these compatibility tests. |
| Refunds invalidate both auth and balance caches synchronously | Pending refunds hold deductions; confirmed failure restores them transactionally | Keep upstream deduction-held audit/state machine and customized `invalidateRefundBalanceCaches` at all committed balance-change boundaries, including final success and rollback. |
| Custom top-up, VIP growth, reward, currency and affiliate snapshots | Verified EasyPay notifications and order-type whitelist | New confirmation feeds the existing customized fulfillment chain. No historical amounts, reward policies or claim eligibility are recalculated by this merge. |
| Old payment UI and recovery flow | Stripe secret must not appear in URL | Use order-bound browser recovery state. Only non-visual payment/recovery/countdown changes are imported. |
| Customized long-context/group/VIP rules | GPT-6.1 Sol subscription model surcharge exception | Upstream checks actual forwarded model and OAuth-like credential. API-key pricing and explicit custom interval pricing remain separate. |
| Historical VIP/achievement migration checksums | Four upstream migrations with overlapping numeric prefixes | Keep original filenames and checksums. The runner orders and records full filenames, so both sets coexist. |
| Expired OAuth refresh work must not save credentials | Upstream cancellation boundary | Check both context cancellation and the actual deadline after provider refresh, so delayed timer scheduling under CPU contention cannot permit a late write. |

## Frontend boundary

The frontend stays on the customized staging design. Upstream account editor, group editor, i18n controls, operations layout and other UI feature changes are deliberately excluded from this backend upgrade.

Retained non-visual changes:

- `PaymentView.vue`: remove Stripe `client_secret` from both normal and mobile fallback navigation URLs.
- `StripePaymentView.vue`: read the secret only from an order-bound recovery snapshot; preserve refresh/recovery and currency behavior.
- `PaymentQRDialog.vue`, `PaymentStatusPanel.vue`, `PaymentQRCodeView.vue`: clamp negative countdown values.
- Associated payment regression tests.

New backend features that require new administration UI are not represented as newly configurable controls in the retained old interface. Existing opt-in/default behavior remains in effect.

## Overlap review

All 21 baseline/upstream overlapping paths were reviewed:

- README variants: customized documentation/branding retained.
- VERSION: explicit customized candidate version.
- Gateway/OpenAI handlers: preserve customized routes and billing; import cached-content wiring, model-availability and OAuth ambiguity guards.
- Payment handler: group localization response support retains existing checkout data.
- EasyPay provider: retain callback field whitelist and replay protections, add upstream confirmation and scalar/query compatibility.
- Antigravity compatibility: preserve thought/signature handling; import configured chat conversion.
- API-key service: add quota-used data for fixed-charge checks.
- Inflight reservations: keep request-rate snapshots and add subscription/key reservation dimensions with upstream failure/TTL semantics.
- Gateway hotpath tests: new account-model availability interface implemented by the stub.
- Gateway usage billing: strict rates, quotation/settlement and cache-price snapshot integration.
- Gemini Chat/Messages compatibility: remove duplicate thought paths, retain signed state and legacy contract, import retry handling.
- Gemini native reasoning tests: retain the Model-Gate buffered-summary expectation.
- OpenAI usage: import the narrowly scoped long-context exception without replacing VIP/manual rates.
- Payment order/fulfillment: import order-type whitelist and notification confirmation while retaining customized fulfillment.
- Refunds: new state machine plus synchronous dual-cache invalidation.
- Payment view: minimal secret-removal change.

## Data and operational limits

Added migrations retain their upstream names:

- `242_group_i18n.sql`
- `243_gemini_cached_contents.sql`
- `244_gemini_cached_contents_channel_usage.sql`
- `245_gemini_cached_contents_mutation_lease.sql`

GPT-5.5 subscription retirement is an upstream behavioral change; see `docs/OPENAI_GPT55_RETIREMENT.md`. This candidate does not rewrite production model mappings or choose a replacement model for the operator.

Local tests and synthetic preview do not establish real payment-provider acceptance, production data compatibility, or fixed-domain availability. Staging/production rollout must follow the deployment workflow and an explicitly selected source SHA/image digest after the deferred skill is supplied.
