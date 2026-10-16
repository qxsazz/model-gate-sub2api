# Reward sources in administrator balance history

Version: 0.2.17-mg.2, based on the completed 0.2.17-mg.1 compatibility merge.

The administrator balance history previously combined redeemed/admin codes and affiliate transfers. Daily check-in, achievement and series cash receipts existed in their own tables but were absent from this response.

The history endpoint now projects those receipts read-only and merges them with existing records by original credit time, before pagination. It does not execute payout functions, create redeemable codes, update balances, or alter migration files. Historical receipts appear automatically; future committed receipts use the same path. No data backfill job or extra payment is needed. Records without a stored positive cash reward are not invented from attendance or unlock state.

| Type | Reference | Amount |
| --- | --- | --- |
| checkin_balance | SIGN-{user_id}-{YYYYMMDD} | achievement_checkins.net |
| achievement_balance | ACH-{user_id}-{key} | achievement_claims.net |
| achievement_series_balance | SERIES-{user_id}-{key} | achievement_series_claims.net |

Only receipts with gross > 0 are included. A fully debt-offset reward therefore appears with zero net and an explanation. Notes identify ordinary/admin/card check-in, achievement name/key, gross reward, net credited and any debt offset. Revoked cash claims retain the original credit and are marked as later revoked; this change does not fabricate reversal amounts where the old receipt does not store actual recovered balance. It is a source-history enhancement, not a complete balance reconciliation ledger.

These types are display-only and do not change the paid-recharge total, VIP recharge growth, referral rules, cash eligibility or claim idempotency. Amounts come from historical receipts, not current reward policy. The reference code is the authoritative history identity; negative safe-integer IDs are compatibility hashes, not redeem-code table IDs. The frontend keys by source/code and displays full reward references, up to eight decimal places, plus source filters in Chinese and English.

Validation includes frontend component/i18n tests and production build, stable mixed-source pagination tests, and PostgreSQL integration coverage of historical reward visibility, existing payment records, source filtering, empty pages, cross-user isolation, debt offsets, revoked receipts, actual new check-in/claim retries and unchanged balances on repeated history reads.

Production data was not accessed or changed. The Docker preview uses synthetic records only.
