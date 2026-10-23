#!/bin/sh
set -eu

# Run inside the deployed application container. Only print these non-secret
# recovery settings; never dump the environment or application configuration.
if [ "${GATEWAY_RECOVERY_ENABLED:-}" != true ]; then
  echo 'gateway recovery must be enabled for combined staging acceptance' >&2
  exit 1
fi

printf 'gateway_recovery enabled=%s budget_seconds=%s selection_wait_seconds=%s max_attempts=%s max_waiters=%s max_waiters_per_group=%s max_waiters_per_user=%s\n' \
  "$GATEWAY_RECOVERY_ENABLED" \
  "${GATEWAY_RECOVERY_BUDGET_SECONDS:?missing recovery budget}" \
  "${GATEWAY_RECOVERY_SELECTION_WAIT_SECONDS:?missing selection wait}" \
  "${GATEWAY_RECOVERY_MAX_ATTEMPTS:?missing attempt limit}" \
  "${GATEWAY_RECOVERY_MAX_WAITERS:?missing waiter limit}" \
  "${GATEWAY_RECOVERY_MAX_WAITERS_PER_GROUP:?missing group waiter limit}" \
  "${GATEWAY_RECOVERY_MAX_WAITERS_PER_USER:?missing user waiter limit}"

binary="${1:-/app/sub2api}"
"$binary" -version
