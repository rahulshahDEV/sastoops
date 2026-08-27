#!/usr/bin/env bash
# serverops module: docker — install Docker Engine + Compose plugin (idempotent)
set -euo pipefail
if [ "$(id -u)" -ne 0 ]; then SUDO="sudo -n"; else SUDO=""; fi
if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
  echo "docker already installed: $(docker --version)"
  exit 0
fi
export DEBIAN_FRONTEND=noninteractive
$SUDO apt-get install -y -qq curl ca-certificates >/dev/null
$SUDO curl -fsSL https://get.docker.com | $SUDO sh >/dev/null 2>&1
# small-disk friendly defaults: log rotation, no ipv6
$SUDO mkdir -p /etc/docker
if [ ! -f /etc/docker/daemon.json ]; then
  $SUDO tee /etc/docker/daemon.json >/dev/null <<'EOF'
{
  "log-driver": "json-file",
  "log-opts": {"max-size": "10m", "max-file": "3"},
  "ipv6": false
}
EOF
fi
$SUDO systemctl enable --now docker >/dev/null 2>&1 || true
docker --version
echo "docker installed"