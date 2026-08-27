# Installing sastoops

Zero dependencies. One command. Works on any Linux VPS (and macOS/Windows for local control).

## Quick install (Linux / macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/rahulshahDEV/sastoops/main/scripts/install.sh | sh
```

This downloads the prebuilt static binary for your OS/arch from the latest GitHub
release and installs it to `/usr/local/bin` (falls back to `~/.local/bin` if you
don't have write permission). **No Go, no Python, no Node, no Docker required.**

Verify:

```bash
sastoops version
```

## Then, on your VPS (the easy part)

```bash
sastoops self                 # registers THIS machine as a server
sastoops server setup $(hostname)   # harden + Docker (idempotent)
sastoops app install n8n $(hostname) --domain n8n.example.com
sastoops backup setup $(hostname) --provider wasabi --bucket my-bucket \
  --key-id $WASABI_ACCESS_KEY_ID --secret $WASABI_SECRET_ACCESS_KEY
```

`sastoops self` auto-detects your public IP, user and SSH key — nothing else to configure.

## From any other machine (manage remote VPS)

```bash
sastoops server add my-vps root@1.2.3.4 --test   # or --key ~/.ssh/id_ed25519
sastoops server ssh my-vps
```

## Alternatives

| Method | Command | Needs |
|---|---|---|
| Release binary (recommended) | `curl ... \| sh` | nothing |
| Go toolchain | `go install github.com/rahulshahDEV/sastoops@latest` | Go 1.21+ |
| From source | `git clone … && make build` (binary in `bin/sastoops`) | Go 1.21+ |
| Cross-compile all platforms | `make release` → `dist/` | Go 1.21+ |

## Options

```bash
SASTOOPS_PREFIX=~/.local/bin sh install.sh   # custom install dir
SASTOOPS_VERSION=v0.1.0 sh install.sh         # pin a version
```

## Config file

- Linux/macOS: `~/.config/sastoops/config.yaml`
- Windows: `%AppData%\sastoops\config.yaml`
- Override: `sastoops --config /path/to/config.yaml <cmd>`

Servers are stored there (`sastoops server add` writes them). Server *state*
(installed apps, applied recipes) lives **on each server** at
`/var/lib/serverops/state.json` — your laptop can be wiped and re-added without
losing anything.

## Upgrading

```bash
# re-run the installer — it replaces the binary in place
curl -fsSL https://raw.githubusercontent.com/rahulshahDEV/sastoops/main/scripts/install.sh | sh
# or with Go
go install github.com/rahulshahDEV/sastoops@latest
```

## Uninstalling

```bash
rm "$(command -v sastoops)"
rm -rf ~/.config/sastoops        # local config (optional)
# servers keep running; nothing agent-related was ever installed on them
```

## Troubleshooting

| Symptom | Fix |
|---|---|
| `no config at …` | add a server first: `sastoops server add <name> user@host` or `sastoops self` |
| `ssh … connection refused` | wrong port/IP; check `sastoops server list`; add `--port` |
| `no auth method for …` | set `key_path` in config or add `--key`; ensure `ssh-add -L` lists your key |
| `provider hetzner not configured` | `sastoops server create` needs `providers.hetzner.token` in config or `SASTO_HETZNER_TOKEN` env |
| modules fail with `sudo -n` errors | run recipe steps as root, or make sure your user has passwordless sudo |