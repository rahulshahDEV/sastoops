#!/usr/bin/env bash
# serverops module: ssh-hardening — key-only auth, safe defaults (idempotent)
# SAFETY: this module must run AFTER admin-user; the engine verifies a
# key-only connection before disabling password auth.
set -euo pipefail
if [ "$(id -u)" -ne 0 ]; then SUDO="sudo -n"; else SUDO=""; fi
DIR=/etc/ssh/sshd_config.d
$SUDO mkdir -p "$DIR"
$SUDO tee "$DIR/99-serverops.conf" >/dev/null <<'EOF'
PasswordAuthentication no
PermitRootLogin prohibit-password
PubkeyAuthentication yes
MaxAuthTries 3
X11Forwarding no
EOF
$SUDO chmod 644 "$DIR/99-serverops.conf"
$SUDO systemctl reload ssh 2>/dev/null || $SUDO systemctl reload sshd 2>/dev/null || true
echo "sshd hardened (key-only)"