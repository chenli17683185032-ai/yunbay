# LDXP browser worker auto top-up runbook

Last updated: 2026-07-15

This runbook records the public, non-secret operational contract for the Yunbay LDXP browser-worker top-up flow. Do not put SSH keys, worker tokens, mailbox passwords, cookies, payment credentials, or full private server coordinates in this file.

## Production business rules

- `LDXP_REQUIRE_MAIL_MATCH=false` is the intended production mode for the current gray test: once the worker observes a trusted LinkPay/Alipay paid result page, the backend can immediately create a successful `TopUp`, credit quota, and mark the LDXP session as `success`.
- QQ IMAP mail is an audit signal only. Missing, delayed, or temporarily unconfigured mail must not block successful user crediting in the direct top-up flow.
- VIP auto-upgrade is based on accumulated successful `top_ups.money` (actual paid money), not `top_ups.amount` (credited/displayed amount). The current threshold in code is `30.0` actual paid money.
- LDXP top-ups create successful `top_ups` rows with `payment_method=ldxp` and `payment_provider=ldxp`.
- Successful paid top-ups may create affiliate commission rows. The inviter is resolved from the existing new-api invite relationship (`users.inviter_id`); no separate invite-code system is used for LDXP.

## Production LDXP product links

The intended production LDXP product configuration has exactly six tiers. Do not add a `200` tier unless product/business requirements change.

```json
[
  {"amount":10,"money":10,"product_url":"https://pay.ldxp.cn/item/nzkyrt","product_name":"LDXP 10"},
  {"amount":20,"money":20,"product_url":"https://pay.ldxp.cn/item/ka4pg7","product_name":"LDXP 20"},
  {"amount":30,"money":30,"product_url":"https://pay.ldxp.cn/item/n8schm","product_name":"LDXP 30"},
  {"amount":50,"money":50,"product_url":"https://pay.ldxp.cn/item/5c4yft","product_name":"LDXP 50"},
  {"amount":100,"money":100,"product_url":"https://pay.ldxp.cn/item/sb48mz","product_name":"LDXP 100"},
  {"amount":500,"money":500,"product_url":"https://pay.ldxp.cn/item/y8t52c","product_name":"LDXP 500"}
]
```

Operational notes:

- Code defaults are in `service/ldxp_config.go`.
- If production sets `LDXP_TOPUP_PRODUCTS_JSON`, that environment value overrides the code default. Back up `/opt/new-api/secrets/prod.env` before changing it and do not print secrets while editing.
- Frontend amount options must stay aligned with the backend allowed tiers: `10, 20, 30, 50, 100, 500`.
- LinkPay/card-network service fees are paid by the user on the cashier page. For example, the 10 CNY product may show `10.30` on Alipay, but Yunbay business money, `ldxp_topup_sessions.money`, `top_ups.money`, VIP accumulation, and affiliate commission base remain `10.00`.

## Payment-browser proxy

`LDXP_BROWSER_PROXY_SERVER` optionally routes only Playwright Chromium traffic through a proxy. Backend polling, worker callbacks, IMAP, the main API, and other containers continue using their normal network path.

Production constraints:

- Accepted URLs are `socks5://`, `http://`, or `https://` with an explicit host and port. Credentials, paths, query strings, and fragments are rejected at worker startup so proxy secrets cannot be embedded in process arguments or error logs.
- The production sidecar should share its network namespace with the worker and listen only on loopback. The current production value is `socks5://127.0.0.1:7891`; no proxy port should be published on the host or application bridge network.
- Keep the Mihomo subscription URL, token, selected node credentials, and provider cache under `/opt/new-api/secrets` or `/opt/new-api/data` with restrictive permissions. Never add them to this repository, `prod.env`, screenshots, or shared logs.
- Select only an exit that has passed both checks: zero ESA challenge elements and a visible LDXP contact input. HTTP 200 by itself is not sufficient because the ESA challenge also returns 200.
- If ESA returns after an exit change or subscription expiry, the worker classifies the page as `waf_challenge` immediately instead of consuming the full product-load timeout.

Before recreating the production worker, validate the private Mihomo configuration and run three consecutive read-only product-page probes through the candidate exit. After recreation, verify the proxy exit, `new-api` health, worker claim polling, and the absence of new `waf_challenge` failures.

## Affiliate commission behavior for paid top-ups

The affiliate reward feature reuses the built-in new-api invite relationship:

```text
invite link/code: users.aff_code, /api/user/aff
invite relationship: users.inviter_id
legacy quota transfer: users.aff_quota, /api/user/aff_transfer
monetary commission ledger: affiliate_commissions
withdrawal ledger: affiliate_withdrawals
```

Commission rules:

- Only successful top-ups (`top_ups.status='success'`) with `top_ups.money > 0` are eligible.
- Commission base is actual paid money: `top_ups.money`, not credited amount/quota.
- Commission rate is `15%`, rounded to cents.
- Each `topup_id` can create at most one `affiliate_commissions` row.
- No commission is created when the invitee has no inviter, self-invites, the inviter no longer exists, or the top-up is not successful.
- LDXP direct top-up creates the commission in the same transaction as the successful `TopUp` and quota credit. If an existing already-successful LDXP session is rechecked, commission creation is idempotent and should not credit quota twice.

## Current gray-test status on 2026-06-29

The 2026-06-29 UI gray test changed only the user-facing waiting state before a QR code is ready:

- Statuses `created` and `worker_claimed` show a prominent pop-in spinner panel.
- The amount chip keeps a 30-second progress animation.
- The user-facing hint says that the payment QR code usually appears in about 20 seconds.
- No worker image or proxy configuration change is part of this UI gray test.

Production verification from the gray test:

```text
yunbay-new-api: healthy
ldxp-browser-worker: running
new-api image: sha256:6632a1ce50ede30f84c897820678c080f32899c9883e5c81e2d177ea3938a036
worker image: sha256:d0596df45239b943f45b9de7881b2ddd96d26f62c7386cbeae2475409f62f55c
served css marker: ldxp-qr-creation-pop / ldxp-qr-creation-pulse / ldxp-qr-creation-spinner
```

Rollback artifacts from that gray test:

```text
backup dir: /opt/new-api/backups/ldxp-ui-popup-spinner-20260629164951
rollback image: yunbay-new-api:pre-ui-popup-spinner-20260629164951
```

## 2026-06-29 formal product QR fix

Production initially failed to create QR codes after switching from the `0.1` test product to the formal LDXP product links. The decisive evidence was a formal 10 CNY session on `nzkyrt` where the worker had claimed the session but logged `qr=not_called`; the debug snapshot was still on the LinkPay item/transition flow rather than a ready Alipay cashier page.

Root cause and fix:

- Formal products pass through a LinkPay `payApi` transition URL before reaching `excashier.alipay.com`.
- The worker must not treat the `payApi` transition URL as QR/cashier readiness. It now waits for real cashier text containing an order number plus amount/payment markers before extracting the QR.
- The Alipay cashier amount may include a user-paid card-network service fee. Backend and worker amount validation now allow `actual = configured money + reasonable service fee`, while still rejecting underpayment or large mismatches.
- Business accounting remains based on configured money: formal tiers are `10/20/30/50/100/500`; the extra service fee is not added to Yunbay top-up money or affiliate commission base.

Production validation:

```text
formal 10 CNY probe product: https://pay.ldxp.cn/item/nzkyrt
probe result: QR reached on excashier.alipay.com
cashier amount: 10.30
configured Yunbay money: 10.00
probe elapsed: about 24.4s
new-api image: sha256:e7427b2921cfcc9ee4ad31a7efdbb05448991931dab66249b6332b3b0abb99ba
worker image: sha256:86ac7d873aa1ae7afc596e5cde83733e74180aa60cccd4f36d2262623ad51c97
backup dir before formal env switch: /opt/new-api/backups/ldxp-formal-products-payapi-fee-fix-20260629234617
```

Current production product env summary after the fix:

```text
amounts=10,20,30,50,100,500
money=10,20,30,50,100,500
slugs=nzkyrt,ka4pg7,n8schm,5c4yft,sb48mz,y8t52c
```

If QR creation fails again, check `wait_cashier_ready`, `click_purchase_to_cashier`, `extract_qr`, and `qr=called/not_called` timing fields before changing product money or product URLs.

### 2026-06-30 payment-result regression and queue-slot follow-up

Production evidence showed two separate failure modes after the formal QR fix. Keep them separate when diagnosing future incidents.

First, queue-slot pressure can happen when several users open QR sessions and leave them unpaid:

```text
first two sessions: worker_claimed -> qr_ready
third session: stayed created with no worker_id / no worker_order_no / no qr_code
worker logs: no further /api/ldxp/worker/sessions/claim during that window
```

A temporary mitigation changed `LDXP_RELEASE_SLOT_AFTER_QR=true`, which made the main browser flow release its slot immediately after posting QR and then tried to use the paid-watch path to re-open existing `qr_ready` sessions. That did improve claim rotation for queued QR generation, but it broke the proven paid-result path.

Decisive regression evidence:

```text
paid user order LD260630C62RUK:
session status stayed qr_ready, topup_id=0, worker_detected_time=0
worker kept claiming paid-watch but never posted a paid result
re-opening qr_page_url in the worker redirected from excashier.alipay.com/standard/auth.htm to /home/error.htm
```

Correct production contract:

- `LDXP_RELEASE_SLOT_AFTER_QR=false` is the recommended production setting and the worker default.
- The worker must keep the same live Alipay cashier page open after QR extraction, because that page is the reliable place where Alipay transitions to the paid/success state after the user scans and pays.
- The paid-watch path is not a reliable primary confirmation path for current Alipay QR URLs, because re-opening `qr_page_url` can show an Alipay error page instead of the paid success page. Keep it disabled unless a future cashier URL is proven re-openable.
- QQ IMAP mail remains an audit signal only; missing mail is not a success-confirmation condition for direct LDXP top-up crediting.

Queue-slot behavior is now handled by cancellation abort rather than releasing the cashier page after QR:

- The frontend cancel button calls `/api/user/ldxp/topup/session/:session_id/cancel`, which marks `created`, `worker_claimed`, or `qr_ready` sessions as `canceled`.
- Worker flows poll `/api/ldxp/worker/sessions/:session_id/active` while a browser flow is running.
- If the backend reports `active=false` because the user canceled, the worker aborts the in-flight browser flow, closes the browser context, skips late QR/result/error callbacks, and frees the slot.
- Temporary failures of the active-check request are fail-open: the worker logs a warning and keeps the payment flow alive rather than canceling a live order because of a transient backend/network issue.

Current recommended production setting:

```text
LDXP_RELEASE_SLOT_AFTER_QR=false
LDXP_WORKER_CONCURRENCY=2
LDXP_WORKER_POLL_INTERVAL_MS=2000
```

If many unpaid QR sessions are left open, ask users/testers to click cancel for abandoned payments, or raise worker concurrency after validating server capacity. Do not switch the primary production path back to `LDXP_RELEASE_SLOT_AFTER_QR=true` unless paid success detection has been re-proven end-to-end with a re-openable cashier state page.

## Safe production checks

Run from the production app directory after loading the production env file. Keep command output redacted before sharing publicly.

```bash
cd /opt/new-api/app
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml ps new-api ldxp-browser-worker
curl -fsS http://127.0.0.1:3000/api/status >/tmp/yunbay-status.json
```

Check that the deployed frontend still contains the QR creation animation markers:

```bash
curl -fsS http://127.0.0.1:3000/ -o /tmp/yunbay-index.html
grep -oE '/static/css/[^" ]+\.css' /tmp/yunbay-index.html | sort -u > /tmp/yunbay-css-assets.txt
while IFS= read -r css; do
  curl -fsS "http://127.0.0.1:3000$css" -o /tmp/yunbay-asset.css
  grep -q 'ldxp-qr-creation-pop\|ldxp-qr-creation-spinner' /tmp/yunbay-asset.css && echo "marker_found_in=$css"
done < /tmp/yunbay-css-assets.txt
```

## QR speed and payment result diagnostics

```sql
select
  id,
  left(session_id, 14) || '...' as session,
  status,
  qr_ready_time - created_time as sec_create_to_qr,
  worker_detected_time - qr_ready_time as sec_qr_to_paid,
  verified_time - worker_detected_time as sec_paid_to_verified,
  redeemed_time - verified_time as sec_verified_to_redeemed,
  topup_id,
  redemption_id,
  left(coalesce(error_code, ''), 80) as error_code
from ldxp_topup_sessions
order by id desc
limit 20;
```

Worker phase timing logs:

```bash
docker logs --since 30m yunbay-ldxp-browser-worker \
  | grep -E 'result posted|qr posted|timings|paid-watch|session processing failed' \
  | tail -100
```

## VIP upgrade diagnostics

Use this when a user asks why a successful LDXP top-up did not auto-upgrade to VIP:

```sql
select id, username, "group"
from users
where username = '<username>';

select coalesce(sum(money), 0) as success_money,
       coalesce(sum(amount), 0) as success_amount
from top_ups
where user_id = (select id from users where username = '<username>')
  and status = 'success';

select id, amount, money, trade_no, payment_method, payment_provider, status
from top_ups
where user_id = (select id from users where username = '<username>')
  and status = 'success'
order by id desc
limit 20;
```

Interpretation:

- `success_money >= 30.0` is required for automatic VIP upgrade.
- `success_amount` may be much larger than actual paid money during discounted gray tests and must not be used as the VIP threshold.

## Affiliate commission diagnostics

Use this when validating whether an invited user’s successful LDXP or redemption/payment top-up generated the inviter’s 15% monetary reward:

```sql
select id, username, aff_code, inviter_id
from users
where username in ('<invitee_username>', '<inviter_username>');

select id, user_id, amount, money, trade_no, payment_method, payment_provider, status, created_time, completed_time
from top_ups
where user_id = (select id from users where username = '<invitee_username>')
order by id desc
limit 20;

select id, commission_id, inviter_user_id, invitee_user_id, topup_id, trade_no,
       base_money, rate, commission_money, status, created_time
from affiliate_commissions
where invitee_user_id = (select id from users where username = '<invitee_username>')
   or inviter_user_id = (select id from users where username = '<inviter_username>')
order by id desc
limit 20;
```

Interpretation:

- `base_money` should match successful `top_ups.money`.
- `commission_money` should be `base_money * 0.15`, rounded to cents.
- Missing commission is expected if `users.inviter_id` was empty/invalid at the time the top-up was processed.
- Duplicate rewards for the same top-up should be prevented by the unique `topup_id` ledger constraint.

## Rollback

For the 2026-06-29 UI waiting animation gray test, rollback only `new-api` if the UI change causes issues. Do not recreate the worker unless a worker change was deployed separately.

```bash
cd /opt/new-api/app
docker tag yunbay-new-api:pre-ui-popup-spinner-20260629164951 yunbay-new-api:prod
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d --force-recreate --no-deps new-api
```

If the whole LDXP entry must be hidden while investigating, disable the feature in the production env file and recreate `new-api`. Keep an env backup before editing.
