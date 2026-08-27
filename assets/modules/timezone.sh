#!/usr/bin/env bash
# serverops module: timezone — set system timezone (idempotent)
set -euo pipefail
TZ="${SERVEROPS_TZ:-UTC}"
if [ "$(id -u)" -ne 0 ]; then SUDO="sudo -n"; else SUDO=""; fi
CUR="$(cat /etc/timezone 2>/dev/null || echo "")"
if [ "$CUR" = "$TZ" ]; then exit 0; fi
$SUDO timedatectl set-timezone "$TZ" 2>/dev/null || {
  $SUDO ln -sf "/usr/share/zoneinfo/$TZ" /etc/localtime
  echo "$TZ" | $SUDO tee /etc/timezone >/dev/null
}
echo "timezone set to $TZ"