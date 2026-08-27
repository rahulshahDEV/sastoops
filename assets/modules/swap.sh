#!/usr/bin/env bash
# serverops module: swap — add swapfile if missing (idempotent)
set -euo pipefail
SIZE="${SERVEROPS_SWAP_SIZE:-2G}"
if [ "$(id -u)" -ne 0 ]; then SUDO="sudo -n"; else SUDO=""; fi
if [ -f /swapfile ] && grep -q '/swapfile' /etc/fstab; then
  echo "swapfile already present"
  exit 0
fi
$SUDO fallocate -l "$SIZE" /swapfile 2>/dev/null || $SUDO dd if=/dev/zero of=/swapfile bs=1M count=2048 status=none
$SUDO chmod 600 /swapfile
$SUDO mkswap /swapfile >/dev/null
$SUDO swapon /swapfile 2>/dev/null || true
if ! grep -q '/swapfile' /etc/fstab; then
  echo '/swapfile none swap sw 0 0' | $SUDO tee -a /etc/fstab >/dev/null
fi
echo "swap enabled ($SIZE)"