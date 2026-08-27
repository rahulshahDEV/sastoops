#!/usr/bin/env bash
# serverops module: firewall — UFW default-deny with allow list (idempotent)
set -euo pipefail
ALLOW="${SERVEROPS_FIREWALL_ALLOW:-22,80,443}"
if [ "$(id -u)" -ne 0 ]; then SUDO="sudo -n"; else SUDO=""; fi
export DEBIAN_FRONTEND=noninteractive
$SUDO apt-get install -y -qq ufw >/dev/null
$SUDO ufw --force default deny incoming >/dev/null
$SUDO ufw --force default allow outgoing >/dev/null
for p in $(echo "$ALLOW" | tr ',' ' '); do
  case "$p" in
    */*) $SUDO ufw --force allow "$p" >/dev/null ;;
    *)   $SUDO ufw --force allow "$p/tcp" >/dev/null ;;
  esac
done
$SUDO ufw --force enable >/dev/null
echo "firewall active (allow: $ALLOW)"