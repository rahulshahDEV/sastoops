#!/usr/bin/env bash
# serverops module: apt-update — safe system updates (idempotent)
set -euo pipefail
if [ "$(id -u)" -ne 0 ]; then SUDO="sudo -n"; else SUDO=""; fi
export DEBIAN_FRONTEND=noninteractive
$SUDO apt-get update -qq
$SUDO apt-get upgrade -y -qq --no-install-recommends
$SUDO apt-get autoremove -y -qq