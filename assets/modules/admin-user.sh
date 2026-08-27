#!/usr/bin/env bash
# serverops module: admin-user — create non-root admin + install SSH key (idempotent)
set -euo pipefail
ADMIN="${SERVEROPS_ADMIN_USER:-admin}"
KEY="${SERVEROPS_ADMIN_PUBKEY:-}"
if [ -z "$KEY" ]; then
  echo "no public key provided; skipping admin user"
  exit 0
fi
if [ "$(id -u)" -ne 0 ]; then SUDO="sudo -n"; else SUDO=""; fi
if ! id "$ADMIN" >/dev/null 2>&1; then
  $SUDO useradd -m -s /bin/bash "$ADMIN"
  echo "created user $ADMIN"
fi
$SUDO mkdir -p "/home/$ADMIN/.ssh"
echo "$KEY" | $SUDO tee "/home/$ADMIN/.ssh/authorized_keys" >/dev/null
$SUDO chmod 700 "/home/$ADMIN/.ssh"
$SUDO chmod 600 "/home/$ADMIN/.ssh/authorized_keys"
$SUDO chown -R "$ADMIN:$ADMIN" "/home/$ADMIN/.ssh"
if getent group sudo >/dev/null 2>&1; then
  $SUDO usermod -aG sudo "$ADMIN"
fi
echo "admin user $ADMIN ready"