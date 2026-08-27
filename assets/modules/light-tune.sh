#!/usr/bin/env bash
# serverops module: light-tune — optimize a small/low-RAM VPS (idempotent)
#  - swapiness 10 (RAM first, swap only when needed)
#  - journald capped at 100M
#  - docker log rotation (max 10m x 3) so small disks don't fill up
set -euo pipefail
if [ "$(id -u)" -ne 0 ]; then SUDO="sudo -n"; else SUDO=""; fi

# kernel tuning
$SUDO sysctl -w vm.swappiness=10 >/dev/null 2>&1 || true
$SUDO sysctl -w vm.vfs_cache_pressure=50 >/dev/null 2>&1 || true
if ! grep -q 'vm.swappiness' /etc/sysctl.d/99-serverops.conf 2>/dev/null; then
  $SUDO mkdir -p /etc/sysctl.d
  printf 'vm.swappiness=10\nvm.vfs_cache_pressure=50\n' | $SUDO tee -a /etc/sysctl.d/99-serverops.conf >/dev/null
fi

# journald: cap log disk usage
$SUDO mkdir -p /etc/systemd/journald.conf.d
$SUDO tee /etc/systemd/journald.conf.d/99-serverops.conf >/dev/null <<'EOF'
[Journal]
SystemMaxUse=100M
SystemMaxFileSize=10M
MaxRetentionSec=7d
EOF
$SUDO systemctl restart systemd-journald >/dev/null 2>&1 || true

# docker: log rotation + no ipv6 (small footprint)
if command -v docker >/dev/null 2>&1; then
  $SUDO mkdir -p /etc/docker
  if [ ! -f /etc/docker/daemon.json ]; then
    $SUDO tee /etc/docker/daemon.json >/dev/null <<'EOF'
{
  "log-driver": "json-file",
  "log-opts": {"max-size": "10m", "max-file": "3"},
  "ipv6": false
}
EOF
    $SUDO systemctl restart docker >/dev/null 2>&1 || true
  fi
fi
echo "light-VPS tuning applied (swappiness=10, journald 100M cap, docker logs rotated)"