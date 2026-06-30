# LDXP browser worker auto top-up runbook

Last updated: 2026-06-29

This runbook records the public, non-secret operational contract for the Yunbay LDXP browser-worker top-up flow. Do not put SSH keys, worker tokens, mailbox passwords, cookies, payment credentials, or full private server coordinates in this file.

## Production business rules

- `LDXP_REQUIRE_MAIL_MATCH=false` is the intended production mode for the current gray test: once the worker observes a trusted LinkPay/Alipay paid result page, the backend can immediately create a successful `TopUp`, credit quota, and mark the LDXP session as `success`.
- QQ IMAP mail is an audit signal only. Missing, delayed, or temporarily unconfigured mail must not block successful user crediting in the direct top-up flow.
- VIP auto-upgrade is based on accumulated successful `top_ups.money` (actual paid money), not `top_ups.amount` (credited/displayed amount). The current threshold in code is `30.0` actual paid money.
- LDXP top-ups create successful `top_ups` rows with `payment_method=ldxp` and `payment_provider=ldxp`.

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

## Rollback

For the 2026-06-29 UI waiting animation gray test, rollback only `new-api` if the UI change causes issues. Do not recreate the worker unless a worker change was deployed separately.

```bash
cd /opt/new-api/app
docker tag yunbay-new-api:pre-ui-popup-spinner-20260629164951 yunbay-new-api:prod
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d --force-recreate --no-deps new-api
```

If the whole LDXP entry must be hidden while investigating, disable the feature in the production env file and recreate `new-api`. Keep an env backup before editing.
