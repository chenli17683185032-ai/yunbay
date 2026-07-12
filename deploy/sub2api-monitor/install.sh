#!/bin/sh
set -eu

SOURCE_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
INSTALL_DIR=${INSTALL_DIR:-/opt/new-api/monitor/sub2api-pool-monitor}
CONFIG_DIR=${CONFIG_DIR:-"$HOME/.config/yunbay"}
ENV_FILE=${ENV_FILE:-"$CONFIG_DIR/sub2api-monitor.env"}
RECIPIENT=${ALERT_EMAIL_TO:-${1:-}}
CAPACITY_WEIGHTS=${ACCOUNT_CAPACITY_WEIGHTS_JSON:-${2:-}}
[ -n "$CAPACITY_WEIGHTS" ] || CAPACITY_WEIGHTS='{}'

if [ -z "$RECIPIENT" ]; then
  echo "usage: ALERT_EMAIL_TO=user@example.com $0" >&2
  exit 2
fi

mkdir -p "$INSTALL_DIR" "$CONFIG_DIR"
install -m 0750 "$SOURCE_DIR/sub2api_pool_monitor.py" "$INSTALL_DIR/sub2api_pool_monitor.py"

umask 077
cat >"$ENV_FILE" <<EOF
ALERT_EMAIL_TO=$RECIPIENT
ACCOUNT_CAPACITY_WEIGHTS_JSON='$CAPACITY_WEIGHTS'
MONITOR_STATE_FILE=$INSTALL_DIR/state.json
MONITOR_LOCK_FILE=$INSTALL_DIR/monitor.lock
EOF
chmod 0600 "$ENV_FILE"

CRON_BEGIN='# BEGIN YUNBAY SUB2API POOL MONITOR'
CRON_END='# END YUNBAY SUB2API POOL MONITOR'
CURRENT=$(crontab -l 2>/dev/null || true)
CLEANED=$(printf '%s\n' "$CURRENT" | awk -v begin="$CRON_BEGIN" -v end="$CRON_END" '
  $0 == begin {skip=1; next}
  $0 == end {skip=0; next}
  !skip {print}
')
ENTRY="*/5 * * * * set -a; . '$ENV_FILE'; set +a; '$INSTALL_DIR/sub2api_pool_monitor.py' >> '$INSTALL_DIR/monitor.log' 2>&1"
{
  printf '%s\n' "$CLEANED"
  printf '%s\n%s\n%s\n' "$CRON_BEGIN" "$ENTRY" "$CRON_END"
} | crontab -

echo "installed: $INSTALL_DIR/sub2api_pool_monitor.py"
echo "configured: $ENV_FILE"
echo "scheduled: every 5 minutes"
