#!/usr/bin/env bash
# serverops module: unattended-upgrades — security updates only (idempotent)
set -euo pipefail
if [ "$(id -u)" -ne 0 ]; then SUDO="sudo -n"; else SUDO=""; fi
export DEBIAN_FRONTEND=noninteractive
$SUDO apt-get install -y -qq unattended-upgrades >/dev/null
$SUDO tee /etc/apt/apt.conf.d/20auto-upgrades >/dev/null <<'EOF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Download-Upgradeable-Packages "1";
APT::Periodic::AutocleanInterval "7";
APT::Periodic::Unattended-Upgrade "1";
EOF
$SUDO tee /etc/apt/apt.conf.d/50unattended-upgrades >/dev/null <<'EOF'
Unattended-Upgrade::Allowed-Origins {
        "${distro_id}:${distro_codename}-security";
};
Unattended-Upgrade::Automatic-Reboot "false";
Unattended-Upgrade::Remove-Unused-Dependencies "true";
EOF
echo "unattended security upgrades enabled"