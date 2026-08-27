# sastoops

**SastoHost ServerOps** — lightweight, agentless server management CLI in Go.

Manage any Linux VPS from your terminal: provision, harden, deploy apps, back up to S3-compatible storage (Wasabi / Cloudflare R2 / Backblaze B2), and monitor — all over plain SSH. No agents, no daemons, no runtime dependencies. Single static binary.

## Install

```bash
go install github.com/rahulshahDEV/sastoops@latest
# or build from source
make build
```

## Quick start

```bash
# 1. Import an existing VPS (works with any provider)
sastoops server add my-vps root@1.2.3.4 --test

# 2. Harden it (idempotent, safe to re-run; --check to dry-run)
sastoops server setup my-vps

# 3. Install apps
sastoops app install n8n my-vps --domain n8n.example.com
sastoops app install minecraft my-vps --set memory=4G

# 4. Backups -> Wasabi / R2 / B2 (restic, encrypted)
sastoops backup setup my-vps --provider wasabi --bucket sastoops-backups --key-id AKIA... --secret ...

# 5. Check everything
sastoops status my-vps
```

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

© SastoHost (https://sasto.host). Open-source project under active development.# sastoops
