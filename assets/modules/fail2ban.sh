#!/usr/bin/env bash
# serverops module: fail2ban — sshd jail (idempotent)
set -euo pipefail
if [ "$(id -u)" -ne 0 ]; then SUDO="sudo -n"; else SUDO=""; fi
export DEBIAN_FRONTEND=noninteractive
$SUDO apt-get install -y -qq fail2ban >/dev/null
if ! $SUDO systemctl is-enabled fail2ban >/dev/null 2>&1; then
  $SUDO tee /etc/fail2ban/jail.local >/dev/null <<'EOF'
[sshd]
enabled = true
port = ssh
maxretry = 5
bantime = 1h
findtime = 10m
EOF
  $SUDO systemctl enable --now fail2ban >/dev/null
fi
echo "fail2ban active (sshd jail)"