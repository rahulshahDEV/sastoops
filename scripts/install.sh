#!/usr/bin/env sh
# sastoops — one-line installer (SastoHost ServerOps)
#
#   curl -fsSL https://raw.githubusercontent.com/rahulshahDEV/sastoops/main/scripts/install.sh | sh
#
# Environment overrides:
#   SASTOOPS_VERSION   release tag to install (default: latest)
#   SASTOOPS_PREFIX    install prefix (default: /usr/local/bin)
#   SASTOOPS_SKIP_SELF skip self-registration prompt
set -e

BIN="sastoops"
PREFIX="${SASTOOPS_PREFIX:-/usr/local/bin}"
REPO="rahulshahDEV/sastoops"

say() { printf '\033[32m%s\033[0m %s\n' '✔' "$1"; }
info() { printf '\033[34m%s\033[0m %s\n' 'ℹ' "$1"; }
warn() { printf '\033[33m%s\033[0m %s\n' '⚠' "$1"; }

# already installed?
if command -v "$BIN" >/dev/null 2>&1; then
  V=$("$BIN" version 2>/dev/null | head -1 || true)
  say "$BIN already installed: ${V:-present}"
  exit 0
fi

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) warn "unsupported arch '$ARCH' — falling back to source build"; ARCH="" ;;
esac

case "$OS" in
  linux|darwin) ;;
  *) warn "no prebuilt binary for '$OS' — falling back to source build"; OS="" ;;
esac

if [ -n "$OS" ] && [ -n "$ARCH" ]; then
  URL="https://github.com/${REPO}/releases/latest/download/${BIN}-${OS}-${ARCH}"
  info "downloading ${URL}"
  TMP="$(mktemp -d)"
  trap 'rm -rf "$TMP"' EXIT INT TERM
  if curl -fsSL -o "$TMP/$BIN" "$URL"; then
    chmod +x "$TMP/$BIN"
    if [ -w "$PREFIX" ]; then
      install -m 755 "$TMP/$BIN" "$PREFIX/$BIN"
    else
      PREFIX="$HOME/.local/bin"
      mkdir -p "$PREFIX"
      install -m 755 "$TMP/$BIN" "$PREFIX/$BIN"
    fi
    say "installed $BIN to $PREFIX/$BIN ($(ls -lh "$PREFIX/$BIN" | awk '{print $5}'))"
    "$PREFIX/$BIN" version
  else
    warn "no release binary yet — building from source (requires Go 1.21+)"
    go install "github.com/${REPO}@latest"
    BIN_DIR="$(go env GOPATH)/bin"
    say "installed $BIN to $BIN_DIR/$BIN"
    PREFIX="$BIN_DIR"
  fi
else
  warn "building from source (requires Go 1.21+)"
  go install "github.com/${REPO}@latest"
  BIN_DIR="$(go env GOPATH)/bin"
  say "installed $BIN to $BIN_DIR/$BIN"
  PREFIX="$BIN_DIR"
fi

if ! command -v "$BIN" >/dev/null 2>&1; then
  export PATH="$PATH:$PREFIX"
fi

info "next steps (no sudo, no agents, no runtime deps):"
printf '  1. %s self\n' "$BIN"
printf '  2. %s server setup <your-server>\n' "$BIN"
printf '  3. %s app install n8n <your-server> --domain n8n.example.com\n' "$BIN"
printf '  4. %s backup setup <your-server> --provider wasabi --bucket <bucket> --key-id <AK> --secret <SK>\n' "$BIN"