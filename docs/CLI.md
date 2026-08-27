# Command reference

Global flags (any command):

```
--config <path>   config file (default ~/.config/sastoops/config.yaml)
--server <name>   target server (alternative to positional arg)
--json            machine-readable JSON on stdout
--quiet, -q       errors only
--verbose         extra detail
--debug           debug output + SSH transcripts
--yes, -y         skip confirmation prompts
--check           dry-run: show what would change, change nothing
```

Exit codes: `0` ok · `1` general · `2` usage · `3` network/ssh · `4` remote exec
failed · `5` not found · `6` already exists · `7` config · `8` auth.

---

## server — ServerOps Cloud

| Command | Purpose |
|---|---|
| `server add <name> <user>@<host> [--port] [--key] [--region] [--test]` | import an existing VPS |
| `server create <name> [--provider hetzner] [--type] [--region] [--image] [--ssh-key] [--setup]` | provision a VM via provider API |
| `server list \| ls` | list configured servers |
| `server info [name]` | OS, uptime, load, disk, memory |
| `server ssh [name]` | interactive shell |
| `server run [name] -- <cmd>` | run a command, stream output |
| `server status [names...]` | one-glance stats for all/selected |
| `server reboot [name] [--provider]` | reboot via SSH (or provider API) |
| `server delete <name>` | provider teardown + config removal (confirms) |

## self — register the machine you are on

```
sastoops self [name] [--user] [--port] [--key] [--region] [--test]
```

Auto-detects public IPv4, user, and SSH key. The easy path on any fresh VPS.

## recipe — ServerOps Profiles

| Command | Purpose |
|---|---|
| `recipe list` | available recipes (base, light, production) |
| `recipe show <recipe>` | steps + modules of a recipe |
| `recipe apply <recipe> [name] [--set key=value] [--check]` | apply idempotently |

Built-in recipes:

- `base` — updates, timezone, swap, admin user, SSH hardening, UFW, fail2ban,
  unattended-upgrades, Docker
- `light` — 1G swap + low-RAM tuning (swappiness 10, journald cap, docker log
  rotation) — for small VPS
- `production` — base + Traefik reverse proxy with automatic Let's Encrypt

## secure / firewall — ServerOps Security

| Command | Purpose |
|---|---|
| `secure [name] [--skip-firewall] [--admin-key]` | apply `base` hardening |
| `firewall [name]` | show UFW status |
| `firewall [name] --action allow --port 8080` | allow a TCP port |
| `firewall [name] --action deny --port 8080` | deny a TCP port |

Safety: SSH hardening only disables password auth **after** verifying key login;
port 22 is never removed; destructive ops confirm.

## docker

| Command | Purpose |
|---|---|
| `docker install [name]` | install Docker Engine + Compose plugin (with log rotation) |
| `docker status [name]` | engine version + running containers |

## app — ServerOps Registry

| Command | Purpose |
|---|---|
| `app list` | apps available in the registry |
| `app list <name>` | apps installed on a server |
| `app search <query>` | search registry |
| `app install <app> [name] [--domain] [--version] [--port] [--set k=v]` | install via compose |
| `app uninstall <app> [name]` | remove containers, volumes, files |
| `app update <app> [name] [--version]` | re-render + recreate, rolls back on health failure |
| `app restart \| stop \| start <app> [name]` | lifecycle actions |
| `app status <app> [name]` | compose ps output |
| `app logs <app> [name] [-f] [--tail]` | container logs |
| `app env <app> [name] [list\|get KEY\|set K=V\|rm KEY]` | manage env vars (secrets redacted) |
| `app backup <app> [name]` | run the app's DB dumps |

Built-in apps: `n8n`, `minecraft`, `appwrite`, `supabase`.

## backup — ServerOps Backup

| Command | Purpose |
|---|---|
| `backup setup [name] [--engine restic\|rclone] [--provider wasabi\|r2\|b2] [--bucket] [--key-id] [--secret] [--schedule] [--apps] [--paths]` | configure engine + jobs + systemd timer |
| `backup run [name] [--job]` | DB dumps + upload now |
| `backup list [name]` | restic snapshots / rclone files |
| `backup restore [name] <snapshot\|latest> [dest]` | restore (staged, confirmed) |
| `backup verify [name]` | `restic check --read-data-subset 5%` / `rclone check` |
| `backup status [name]` | engine, remote, repo, last run |

Credentials can also come from env: `WASABI_ACCESS_KEY_ID`, `WASABI_SECRET_ACCESS_KEY`,
`R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`, `B2_APPLICATION_KEY_ID`, `B2_APPLICATION_KEY`.

## monitor / health / status — ServerOps Monitor

| Command | Purpose |
|---|---|
| `monitor [name] [--watch] [--interval]` | live CPU/RAM/disk/net/services |
| `health [name] [--disk 85] [--mem 90]` | thresholds + problems list |
| `status [name]` | overall: stats, security, apps, backups |

## dns — Cloudflare records

| Command | Purpose |
|---|---|
| `dns records <domain>` | list records |
| `dns add <domain> <type> <name> <value> [--ttl] [--proxied]` | create record |

Token: `CF_API_TOKEN` env or `providers.cloudflare.token` in config.

## version / completion

```
sastoops version
sastoops completion bash | zsh | fish    # cobra-generated shell completion
```