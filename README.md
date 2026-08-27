# sastoops

**SastoHost ServerOps** — lightweight, agentless server management CLI in Go.

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.21+-00ADD8.svg)](go.mod)

Manage any Linux VPS from your terminal: provision, harden, deploy apps, back up to S3-compatible storage (Wasabi / Cloudflare R2 / Backblaze B2), and monitor — all over plain SSH. No agents, no daemons, no runtime dependencies. Single static binary.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/rahulshahDEV/sastoops/main/scripts/install.sh | sh
# fallback (any OS with Go): go install github.com/rahulshahDEV/sastoops@latest
```

That's it — single ~9MB static binary, no Python/Node/Docker/runtime needed on your machine, nothing installed on the server (agentless, SSH only).

## Quick start

```bash
# 1. On a fresh VPS — register the machine you're on (auto-detects IP/user/key)
sastoops self

# 2. Harden it + install Docker (idempotent, safe to re-run; --check to dry-run)
sastoops server setup my-vps

# 3. Install apps
sastoops app install n8n my-vps --domain n8n.example.com
sastoops app install minecraft my-vps --set memory=1G

# 4. Backups -> Wasabi / R2 / B2 (restic, encrypted)
sastoops backup setup my-vps --provider wasabi --bucket sastoops-backups --key-id AKIA... --secret ...

# 5. Check everything
sastoops status my-vps
```

## Small VPS friendly

sastoops itself needs **nothing on the server** (it's just SSH), so it runs fine on the smallest droplets/VPS — even 512Mi / 1 vCPU / 20Gi disk. To keep such boxes healthy:

```bash
sastoops recipe apply light my-vps   # swap 1G + swappiness=10 + journald cap + docker log rotation
```

| App | Minimum RAM | Notes |
|---|---|---|
| n8n | 384Mi | add `--set mem_limit=512m` to cap it |
| minecraft | 2Gi | `--set memory=1G` for small worlds |
| appwrite | 4Gi | hard gate + warning before install |
| supabase | 4Gi | hard gate + warning before install |

Docker is installed with log rotation (`max-size 10m, max-file 3`) by default so small disks never fill up with logs.

## Command groups

| Group | Purpose |
|---|---|
| `sastoops server` | ServerOps Cloud — provision (Hetzner), import (generic), SSH, run, status, reboot, delete |
| `sastoops recipe` | ServerOps Profiles — reusable idempotent server profiles (`base`, `production`) |
| `sastoops secure` / `firewall` | ServerOps Security — SSH hardening, UFW, fail2ban, unattended upgrades |
| `sastoops docker` | Install Docker + Compose plugin, inspect engine state |
| `sastoops app` | ServerOps Registry — install/update/uninstall n8n, minecraft, appwrite, supabase (+ env, logs, health, backups) |
| `sastoops backup` | ServerOps Backup — restic/rclone to S3-compatible storage: setup, run, list, restore, verify |
| `sastoops monitor` / `health` / `status` | ServerOps Monitor — CPU/RAM/disk/net live, thresholds, one-glance overview |
| `sastoops dns` | DNS records via Cloudflare API |

## Highlights

- **Agentless**: every action is a one-shot SSH session; state lives on the server (`/var/lib/serverops/state.json`).
- **Idempotent recipes**: modules are embedded bash, versioned by content hash, marked in server state — re-runs change nothing.
- **Safe hardening**: key-only SSH is enabled *after* verifying key login works; port 22 is never dropped; destructive ops always confirm.
- **App system**: YAML-defined apps (schema in `assets/apps/*/app.yaml`) with compose rendering, secrets auto-generation, healthchecks, and backup hooks — no hard-coded Go.
- **Backups**: DB dumps via native tools (`pg_dump`/`mysqldump`), volumes + files via restic (encrypted, dedup, verify) or rclone (40+ backends).
- **Machine-readable**: `--json --quiet --verbose --debug --check --yes` on every command; exit codes 0–8.

## Global flags

```
--config <path>   config file (default ~/.config/sastoops/config.yaml)
--server <name>   target server (alternative to positional arg)
--json            JSON output on stdout
--quiet, --verbose, --debug
--yes / -y        skip confirmations
--check           dry-run, change nothing
```

## Examples

```bash
sastoops server add app-prod root@5.75.200.1 --region bangalore
sastoops server create app-prod --provider hetzner --type cpx31 --setup
sastoops recipe apply production app-prod --set traefik_email=ops@sastohost.com
sastoops app install appwrite app-prod --domain app.example.com
sastoops app env n8n app-prod set N8N_DEFAULT_BINARY_DATA_MODE=on
sastoops app update n8n app-prod --version 1.80.1
sastoops backup setup app-prod --engine restic --provider wasabi --bucket backups --key-id $WASABI_ACCESS_KEY_ID --secret $WASABI_SECRET_ACCESS_KEY --schedule "*-*-* 03:00:00"
sastoops backup run app-prod
sastoops backup verify app-prod
sastoops backup restore app-prod latest
sastoops monitor app-prod --watch
sastoops health app-prod --disk 80
sastoops dns records example.com
sastoops server ssh app-prod
```

## Development

```bash
make build      # build ./bin/sastoops
make test       # run unit tests
make lint       # go vet + gofmt check
make release    # cross-compile linux/darwin/windows × amd64/arm64
```

## Roadmap

See [docs/DESIGN.md](docs/DESIGN.md) — the full technical design (architecture, app/recipe schema, provider abstraction, backup strategy, security model, MVP phases).

Phase 1 (done): config/state/secrets, SSH executor, server mgmt, hardening recipes, docker, app system (n8n, minecraft, appwrite, supabase), backups (restic/rclone), monitor/health, Hetzner provider, Cloudflare DNS.
Phase 2: DigitalOcean/Vultr/Linode providers, scheduled backups via systemd timers, alerts, server cloning/migration, `--json` contract stabilization.
Phase 3: web UI/API, teams/RBAC, SastoHost cloud integration.

## License

© SastoHost (https://sasto.host). Open-source project under active development.

## Documentation

| Doc | Contents |
|---|---|
| [INSTALL.md](docs/INSTALL.md) | zero-dependency install, upgrade, troubleshooting |
| [CLI.md](docs/CLI.md) | full command reference + global flags + exit codes |
| [ARCHITECTURE.md](docs/ARCHITECTURE.md) | how it works, state layout, command flows |
| [DESIGN.md](docs/DESIGN.md) | the original A–N technical design |
| [APPS.md](docs/APPS.md) | authoring guide: app schema + compose templates |
| [RECIPES.md](docs/RECIPES.md) | authoring guide: recipes + idempotent modules |
| [BACKUPS.md](docs/BACKUPS.md) | restic/rclone engines, DB dumps, restore |
| [SECURITY.md](SECURITY.md) | security model + vulnerability reporting |
| [CONTRIBUTING.md](CONTRIBUTING.md) | how to contribute (apps, modules, providers) |

## License

Apache-2.0 — see [LICENSE](LICENSE). © 2026 SastoHost (https://sasto.host).# sastoops
