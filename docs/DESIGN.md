# SastoHost ServerOps — Technical Design

> Status: Draft v1 — author: SastoHost
> Scope: Design-first blueprint for the `sasto` CLI. No code beyond this document reflects final decisions.

---

## 0. Opinionated decisions (the short version)

These are the calls that shape everything else. Each is argued in its section.

1. **Rename away from `sops`.** Mozilla's `sops` (Secrets OPerationS) is ubiquitous in infra. `sops` will collide in shells, tab-completion, docs, and Google. **Recommended binary: `sasto`.** (kept trivially aliased in code, see §C).
2. **Go + Cobra + YAML — agreed.** But only for *authored* content (config, apps, recipes). **Machine-written state is JSON**, written atomically, on the *server* (§I).
3. **No Ansible, no OpenTofu in the MVP.** Ansible breaks the single-binary/no-runtime promise and adds an inventory/control-node model you don't need for opinionated single-server ops. **Recipes are idempotent bash modules embedded in the binary via `go:embed`** and executed over SSH. OpenTofu is an optional later path, not the core.
4. **Do NOT use a Go Docker client. Shell out to the *remote* `docker` / `docker compose` CLI over SSH.** Docker is server-side; the remote already has the battle-tested CLI. A Go Docker client would force a remote TLS API on every managed server — a security and complexity tax with zero benefit for an orchestrator.
5. **One reverse proxy for everything (Traefik), not per-app nginx.** Apps register with labels; SSL is solved once, for all apps (§H, §L).
6. **Authoritative state lives on the server** (`/var/lib/serverops/`). The laptop is a cache. This survives laptop loss, multi-admin teams, and matches how servers actually are.
7. **Backup = restic by default, rclone for raw sync.** restic gives encryption + dedup + integrity checks + `verify`. rclone gives 40+ backends and cheap mirroring. Both run *on the server* (the data is there); the CLI only orchestrates (§G).
8. **Database backups are always native dumps** (`pg_dump`/`mysqldump`/`mongodump`) via pre/post hooks — never a blind copy of live data files.
9. **Secrets: env vars + optional `age`-encrypted config. Never plaintext secrets in YAML.** No OS-keyring dependency (it's a cross-platform nightmare for a CLI) (§I).
10. **Providers: official Go SDKs where they exist** (Hetzner, DigitalOcean, Vultr, Linode), thin hand-rolled REST only if an SDK is missing. Generic SSH "import" is first-class (§F).
11. **Single static binary, `CGO_ENABLED=0`**, cross-compiled for linux/mac/windows; `curl | sh` installer.
12. **Every command supports `--json --quiet --verbose --debug`** from day one; stdout stays machine-readable, all human chatter goes to stderr (§C).

---

## A. Product definition

**SastoHost ServerOps (`sasto`)** is a lightweight, agentless, cross-platform CLI that turns a Linux VPS into a production server — and keeps it that way — with a handful of idempotent commands. It is an **orchestrator**, not a replacement for existing DevOps tools.

It solves the 80% repeated workflow every sysadmin/agency/host does by hand:

```
VPS/cloud provider
  → provision (provider API)
  → import existing (generic SSH)
  → base setup + security hardening
  → Docker / Docker Compose
  → applications (n8n, appwrite, supabase, minecraft, coolify, dokploy…)
  → reverse proxy / SSL / DNS
  → backups (restic/rclone → S3-compatible)
  → monitoring / health
  → ongoing management
```

### What it is NOT (and never should become)

| Tool | What it does | ServerOps must NOT become |
|---|---|---|
| Ansible | general config mgmt, inventories, arbitrary playbooks | a generic config-mgmt language with an inventory. Recipes are a small, opinionated, auditable set. |
| Terraform/OpenTofu | declarative infra as code, resource graph | a state engine for infra resources. `sasto` provisions quickly; it doesn't manage terraform state graphs. |
| Coolify / Dokploy / EasyPanel | self-hosted **PaaS dashboards** (web UI + DB) | a web UI with a server-side database. `sasto` is a CLI, agentless, with **no server-side daemon** in MVP. |
| Portainer | container GUI | scope is bigger (VM lifecycle, security, backup, providers). |
| Provider CLIs (hcloud, doctl…) | single-provider VM control | multi-provider, plus every layer above the VM. |

### Positioning statement

> One binary, zero agents, zero runtime deps, zero learning curve beyond `server → recipe → app → backup`. For anyone who manages more than one Linux box, `sasto` is the fastest path from "empty VPS" to "hardened server running appwrite + n8n with encrypted offsite backups".

---

## B. Architecture

```
┌────────────────────────────────────────────────────────────┐
│  sasto CLI  (local, single static Go binary, CGO=0)        │
│                                                            │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ cmd/sasto            cobra commands, orchestrator    │  │
│  │   · server/recipe/app/backup/dns/ssl/monitor…        │  │
│  │   · global flags --json --quiet --verbose --debug    │  │
│  └───────┬───────────────────────────────┬──────────────┘  │
│          │ SSH executor                  │ providers       │
│   ┌──────▼────────┐              ┌───────▼──────────────┐  │
│   │ internal/ssh  │              │ internal/provider    │  │
│   │ ssh + sftp    │              │ hetzner·do·vultr·…   │  │
│   │ keepalive,    │              │ + generic (imported) │  │
│   │ streams, tty  │              └───────┬──────────────┘  │
│   └──────┬────────┘                      │                 │
└──────────┼───────────────────────────────┼─────────────────┘
     SSH 22│                          HTTPS API
┌──────────▼───────────────────────────────▼─────────────────┐
│  Remote VPS  —  AGENTLESS, nothing installed by us         │
│                                                             │
│  docker / docker compose     (compose files we render)      │
│  systemd                    (units we generate)             │
│  restic / rclone            (repos → S3/R2/B2/Wasabi/GDrive)│
│  traefik                    (reverse proxy + ACME)          │
│  state: /var/lib/serverops/{state.json, env, compose, …}    │
└─────────────────────────────────────────────────────────────┘
```

**Key principles**
- **Agentless.** We never install a daemon. Every action is a one-shot SSH session. State is written as files, not by a running service.
- **Local is thin, remote is authoritative.** All heavy lifting happens on the server.
- **Streaming over slow links.** Long ops poll a remote status file rather than holding one giant SSH session open (§N).
- **Parallelism.** Multi-server commands (status, health, backup) fan out over goroutines with a worker pool, one SSH session per server, bounded concurrency.

---

## C. CLI design

### Naming

`sops` collides with Mozilla's SOPS. **Final binary: `sasto`.** It is short, brandable (`sastohost`), and greppable. Internally the module is `github.com/sastohost/serverops`; the binary is `sasto`. Zero risk for users who already have `sops` installed.

### Global flags (every command)

```
--config <dir>     config dir override (default: XDG, see §I)
--server <name>    target server (alternative to positional)
--json             machine-readable output on stdout
--quiet            only errors
--verbose          extra detail
--debug            full stack traces, SSH transcripts
--yes / -y         skip confirmation prompts (CI)
--check            dry-run: report what WOULD change, change nothing
```

Exit codes: `0` ok · `1` general · `2` usage · `3` network/ssh · `4` remote exec failed · `5` not found · `6` already exists · `7` config/validation · `8` auth.

### Command tree (final)

```
sasto
├── server
│   ├── create <name>                 # provider-backed provisioning
│   │     --provider hetzner --type cx22 --region nbg1 --image ubuntu-24.04 --ssh-key id
│   ├── add <name> --host 1.2.3.4 --user root   # import existing / generic VPS
│   ├── list | ls
│   ├── info <name>
│   ├── setup <name> [--recipe base]
│   ├── ssh <name>                    # interactive shell  (alias: shell)
│   ├── run <name> -- <cmd>           # one-shot remote command
│   ├── reboot <name>
│   └── delete <name>                 # confirm-required, provider teardown
│
├── secure <name> [--skip-firewall]   # hardening (idempotent) = recipe base
├── firewall <name> [list|allow <p>|deny <p>]
├── docker install <name> | docker status <name>
│
├── recipe list | recipe show <r> | recipe apply <r> <name> [--check]
│
├── app
│   ├── list [name] | app search <q>
│   ├── install <app> <name> [--domain d] [--version v] [--set K=V]…
│   ├── uninstall <app> <name>
│   ├── update <app> <name>           # with --version pinning, pre-backup
│   ├── restart|stop|start|status <app> <name>
│   ├── logs <app> <name> [-f]
│   ├── env <app> <name> [get|set|rm KEY]
│   └── backup <app> <name>           # run that app's backup hooks
│
├── backup
│   ├── setup <name> [--engine restic|rclone] [--remote s3:…] [--provider wasabi|r2|b2]
│   ├── run [--app <a>] [--job <j>] [name]
│   ├── list [--snapshots]
│   ├── restore <snapshot> <dest>     # confirm-required
│   ├── verify [name]                 # restic check --read-data / rclone check
│   └── status [name]
│
├── dns records <domain> | dns add <domain> <type> <name> <value>   # Cloudflare
├── ssl <name> --domain d             # wires ACME via traefik/caddy
├── monitor <name> [--watch]          # CPU/RAM/disk/net live
├── health [name]                     # run app healthchecks + server probes
├── status [name]                     # one-glance overview
├── completion <bash|zsh|fish>        # cobra completion
└── version
```

**Deliberate changes vs the brief:**
- Dropped separate `sops shell` — `sasto ssh` *is* the shell; `sasto server run` covers one-shots.
- `sasto server create` (provision) vs `sasto server add` (import) are distinct — two very different operations.
- `sasto app backup` exists so app-specific backup hooks are runnable on demand, separate from the global schedule.
- `--server` global flag means the target can come from flag or positional; same UX.

### Example UX

```
$ sasto app install n8n srv01
✓ Connected to srv01
✓ OS ubuntu 24.04 (ok)
✓ Docker 26.1.4 (ok)
→ Creating /var/lib/serverops/compose/n8n
→ Writing env + secrets (0600)
→ docker compose up -d --wait
→ Health check http://localhost:5678/healthz ........ ok (2.1s)
✓ n8n installed (v1.80.1)

  URL:      https://n8n.example.com
  Status:   healthy
  Volumes:  n8n_data
  Next:     sasto backup setup srv01
```

Colored only when stdout is a TTY; `--json` emits a stable schema; all progress/step lines go to **stderr**.

---

## D. Go architecture

### Package layout (see §K for full tree)

```
cmd/sasto/            cobra commands (thin, orchestration only)
internal/
  cli/                global flags, output/UX (spinner, table, colors, json)
  config/             config.yaml, servers.yaml loading + validation
  secrets/            env-file + age encryption helpers
  state/              server-side state.json read/write, schema versioning
  ssh/                SSH/SFTP client: dial, run, runStream, put, tunnel, keepalive
  executor/           step runner: ssh/shell module/docker compose/restic… 
  provider/           Provider interface + registry + generic
  server/             server lifecycle orchestration (setup, status, …)
  security/           hardening modules (ssh, firewall, fail2ban, …)
  docker/             install probe, compose renderer, ps/inspect helpers
  app/                app model, schema validation, lifecycle, env, healthcheck
  recipe/             recipe model + engine (module sequence, markers)
  backup/             engines (restic, rclone), jobs, verify, restore
  dns/                cloudflare client
  ssl/                traefik/caddy ACME wiring
  monitoring/         /proc collectors (remote)
  version/            version + build info
providers/            hetzner/, digitalocean/, vultr/, linode/  (import only)
apps/                 embedded app definitions (go:embed)
recipes/              embedded recipes + modules (go:embed)
```

### Core interfaces

```go
// A server is something we can run commands on. Keeps providers abstract.
type Server interface {
    ID() string
    Name() string
    Run(ctx context.Context, cmd string) (Result, error)      // one-shot
    RunStream(ctx context.Context, cmd string, w io.Writer) error
    Put(ctx context.Context, local, remote string, mode os.FileMode) error
    Shell(ctx context.Context) error                          // interactive
    Close() error
}

// Provider provisions/tears down VMs. A generic "imported" server
// implements a read-only Provider that never talks to an API.
type Provider interface {
    Name() string
    Create(ctx context.Context, req CreateRequest) (*Machine, error)
    List(ctx context.Context) ([]*Machine, error)
    Get(ctx context.Context, id string) (*Machine, error)
    Delete(ctx context.Context, id string) error
    Reboot(ctx context.Context, id string) error
    Rescue(ctx context.Context, id string, keys []string) (*RescueInfo, error) // lockout escape
    Snapshot(ctx context.Context, id string) (string, error)
}

type Machine struct {
    ID, Name, IP, Region, Type, Image, Status string
    Labels map[string]string
}

// One step in a recipe/app lifecycle. Modules are embedded bash, idempotent.
type Step struct {
    ID       string
    Module   string            // embedded bash module name
    Params   map[string]string // rendered into the script
    Requires []string          // step ids that must run first
}

// Backup engine contract.
type BackupEngine interface {
    Setup(ctx, cfg EngineConfig) error
    Backup(ctx, job Job) (SnapshotID, error)
    List(ctx, repo string) ([]Snapshot, error)
    Restore(ctx, snap, dest string) error
    Verify(ctx, repo string) error
    Check(ctx) (Health, error)
}
```

---

## E. App / recipe system

**Two related concepts sharing one engine:**
- **Recipe** = server-level profile → an ordered sequence of **modules** (e.g. `base`, `production`).
- **App** = Recipe + lifecycle (install/update/uninstall), env, volumes, ports, healthcheck, proxy labels, backup hooks, migrations.

Apps are NOT hard-coded in Go. They are YAML + an embedded `compose.yaml`, stored in `apps/<name>/` inside the binary via `go:embed`, with a **user overlay** at `~/.config/sasto/serverops/apps/<name>/` that overrides or adds apps (extensibility, §I).

### App schema (authoritative)

```yaml
name: n8n
version: "1.x"            # recipe schema version this definition targets
category: automation
description: Workflow automation
homepage: https://n8n.io
maintainer: sastohost

requirements:
  os: [ubuntu, debian]
  arch: [amd64, arm64]
  docker: ">= 24"
  min_ram: 512Mi
  min_disk: 2Gi
  ports_free: [5678]

compose:
  source: compose.yaml    # embedded, may use {{env}}, {{var}} templating
  version_pin: false      # if true, images tagged with exact version

env:
  - name: N8N_HOST
    from: domain          # injected from --domain
  - name: N8N_PORT
    default: "5678"
  - name: DB_PASSWORD
    secret: true          # auto-generated, stored in secrets env (0600)

ports: [5678]
volumes: [n8n_data]

proxy:
  enabled: true
  host: "{{domain}}"      # traefik router Host()
  port: 5678
  tls: true               # ACME via traefik

healthcheck:
  type: http
  url: http://localhost:5678/healthz
  interval: 5s
  retries: 30

lifecycle:
  install:
    - step: compose-up     # built-in step
      params: {wait: true}
  update:
    - step: compose-up
      params: {recreate: true, wait: true}
  uninstall:
    - step: compose-down
  pre_update:
    - step: backup         # auto-backup before update

backup:
  databases:
    - type: postgres
      container: n8n-db
      user: n8n
      db: n8n
      dump: /var/lib/serverops/backup/dumps/n8n.sql
  resources:               # paths/volumes sent to the engine
    - type: volume
      name: n8n_data
    - type: file
      path: /var/lib/serverops/compose/n8n/.env
  pre_hooks: [ ]
  post_hooks: [ ]
  exclude: [ ]
```

### Built-in steps (the only "magic" in the engine)

`compose-up · compose-down · compose-pull · restart · wait-health · backup · restore-env · systemd-unit · clone-repo · copy-file · run-module`

Everything else is an embedded bash module. If it can't be expressed as one of these + a module, it doesn't get a new Go feature — it gets a new recipe.

### Lifecycle & versioning
- `install` always renders compose + env from pinned templates; stores rendered artifacts under `/var/lib/serverops/compose/<app>/`.
- `update` runs `pre_update` (auto-backup), diffs rendered compose, pulls new images, `up -d`, healthcheck; rolls back to previous tag on failure.
- Apps register `serverops.app.<name>.version` in state; `app list` shows installed vs available.

### MVP app set (by complexity)

| App | Complexity | Notes |
|---|---|---|
| n8n | low | 1–2 containers, single volume, http healthcheck → **ship first** |
| minecraft | low | itzg/minecraft-server, env-driven, no DB → **ship first** |
| appwrite | medium | official compose stack; heavy; resource precheck + warning |
| supabase | high | many containers (kong, auth, postgres, storage, realtime…); resource precheck; own DB dump hooks |
| coolify / dokploy | medium | each is itself a PaaS installer; thin compose recipe + update hooks |

All heavy apps get an explicit RAM/disk gate with a clear `⚠ requires ≥ 4Gi RAM` warning before install.

---

## F. Provider system

```go
// providers/ packages each implement provider.Provider.
// internal/provider holds the registry + generic provider.
```

- **Hetzner first.** Official SDK `github.com/hetznercloud/hcloud-go/v2` — reliable, typed, paginated. Create/list/get/delete/reboot/rescue/snapshot all map 1:1.
- **DigitalOcean, Vultr, Linode** next via their official Go SDKs (`godo`, `govultr`, `linodego`).
- **Generic** = "imported server": a `Server` record in `servers.yaml`, no API, `Provider` methods return `ErrUnsupported`. This is how you manage any VPS you didn't create. **First-class, not an afterthought.**
- **Creds** come from `providers.yaml` (token references) + env (`SASTO_HETZNER_TOKEN` etc.), or `age`-encrypted (§I). Never in git.

Provider config example:

```yaml
# providers.yaml
hetzner:
  token_ref: env:SASTO_HETZNER_TOKEN     # or age:HETZNER_TOKEN
  default_type: cx22
  default_region: nbg1
  default_image: ubuntu-24.04
  ssh_key: 45:aa:…
digitalocean:
  token_ref: env:SASTO_DO_TOKEN
  default_region: blr1
```

`server create` flow: validate name → provider API create → poll for IP (with timeout + progress) → wait SSH up → write server record → optional auto-run `setup`/`recipe base`.

---

## G. Backup architecture

**Runs on the server.** The data lives there; uploading from the laptop is wrong. `sasto` orchestrates; restic/rclone execute remotely via SSH.

### Engines
- **restic (default):** encrypted, deduplicated, snapshot-based. Repo on any S3-compatible backend (Wasabi, Cloudflare R2, Backblaze B2), local disk, or rest-server later. Password stored in the server's secrets env (0600) + recoverable via `backup setup --show-recovery`.
- **rclone:** raw sync/copy to 40+ backends (incl. Google Drive). Optional `crypt` remote for encryption. Used for offsite mirrors and for app data where snapshot semantics aren't needed.
- `sasto backup setup` generates the restic/rclone remote config, tests connectivity (`rclone lsd` / `restic snapshots`), writes the job file, and (optional) installs a systemd timer for scheduled runs.

### Data-type strategy (never blind-copy)

| Data | Strategy |
|---|---|
| PostgreSQL/MySQL/MariaDB/Mongo | native dump (`pg_dump`, `mysqldump`, `mongodump`) inside the container → file → engine. `pre` hook flushes/pauses writes where safe; `post` resumes. |
| Docker volumes | engine snapshots the volume directory (respecting excludes) or `docker run --volumes-from` tar for consistency. |
| Config / env | always included (that's the restore key). |
| App data dirs | per-app `resources` in the app schema. |

### Commands
`setup → run → list → restore → verify → status`
- `backup verify` = `restic check --read-data` (sample) or `rclone check` — scheduled, not just manual.
- `restore` restores to a staging dir first, validates, then asks before overwriting live data.
- App-level: `sasto app backup n8n srv01` runs that app's hooks + resources.

### Retention
Per-job: `keep-last 7`, `keep-daily 30`, `keep-monthly 12` (restic native). Default job set at setup with sane `7d/30d/12m`; `--retention` to override.

---

## H. Security architecture

### Default baseline (recipe `base` → `secure`)
1. apt update/upgrade + unattended-upgrades (security channel only)
2. non-root **admin user** + your SSH key installed
3. **test key-login before disabling anything** (order matters, see safety)
4. SSH hardening: `PasswordAuthentication no`, `PermitRootLogin prohibit-password`, `PubkeyAuthentication yes`, `MaxAuthTries 3`, keep **port 22** by default (do NOT change port — security by obscurity, breaks tooling)
5. UFW: allow `22/tcp`, `80`, `443`, established/related; deny default
6. fail2ban (sshd jail, moderate bantime)
7. timezone, hostname, swap (if low RAM), sysctl hardening (net.ipv4.* sane defaults)
8. Docker install + docker daemon config (no exposed TCP socket; rootless optional flag)
9. App secrets owned by root/admin with 0600

### Automatic vs confirm

| Class | Action | Behavior |
|---|---|---|
| **Automatic** (safe, idempotent) | updates, timezone, swap, admin user, key install, UFW allow 22/80/443, fail2ban, unattended-upgrades, docker | runs on `setup`/`secure` |
| **Confirm** (`--yes` to skip) | disabling password auth, disabling root login, firewall `deny`, nonstandard firewall rules, `server delete`, `app uninstall`, `restore` overwriting data, `reboot` | prompts with clear impact; `--check` to dry-run |

### Lockout safety (the critical design point)
- Hardening is **ordered + test-after-mutate**: install key → verify a *key-only* connection → only then flip `PasswordAuthentication no`. If verification fails, it aborts and prints rescue steps.
- Port 22 is never removed from the firewall; custom ports are opt-in.
- Every destructive action prints a **rescue path** (provider console, rescue mode — `Provider.Rescue` exists in the interface for exactly this).
- `--check` shows exactly what would change, changed nothing.

### Secret hygiene
- Auto-generated passwords/keys (DB passwords, restic password) never echo to stdout; stored in server secrets env (0600).
- Logs redact `password=`, tokens, and `.env` contents.
- Optional `age` encryption of local `secrets.env` and `providers.yaml` (§I).

---

## I. State, config, secrets

### Directories (OS-aware via `os.UserConfigDir`/`os.UserDataDir`)

```
Linux/macOS:  ~/.config/sasto/serverops/          # config (0700)
              ~/.local/share/sasto/serverops/     # cache/state mirror
Windows:      %AppData%\sasto\serverops\          # config
              %LocalAppData%\sasto\serverops\     # cache/state
Override:     --config <dir> or $SASTO_CONFIG_DIR
```

```
~/.config/sasto/serverops/
├── config.yaml            # global defaults (ssh timeout, concurrency, proxy…)
├── servers.yaml           # server records (+ provider refs, tags)  0700
├── providers.yaml         # provider tokens (refs, not plaintext)   0600
├── secrets.env            # local secrets (optional age-encrypted)  0600
└── apps/                  # user overlay recipes (extends embedded)
```

### Server-side state (authoritative)

```
/var/lib/serverops/
├── state.json            # schema_version, recipes applied, apps+versions, backup jobs
├── env/<app>.env         # 0600, written by us
├── compose/<app>/        # rendered compose.yaml + .env + applied diff
├── secrets/              # auto-generated secrets, 0600
├── backup/
│   ├── jobs.yaml
│   ├── restic/           # restic repo (or s3: remote URL)
│   └── dumps/            # staged DB dumps before upload
└── hooks/                # generated pre/post scripts
```

- `state.json` has `schema_version`; a migration runner bumps versions cleanly (handles future recipe-schema changes).
- **Local mirror** (`~/.local/share/sasto/serverops/cache/`) caches remote state for fast `status`; always re-syncs on writes. Laptop loss = zero loss; re-add the server and re-sync.

### Secrets strategy (ranked)
1. **Env vars** (`SASTO_HETZNER_TOKEN`, `WASABI_ACCESS_KEY`) — CI/automation friendly.
2. **`age`-encrypted `secrets.env` / `providers.yaml`** — default for local, encrypted-at-rest with a key at `~/.config/sasto/serverops/key.txt` (0600) or `SASTO_AGE_KEY`. Uses `filippo.io/age` (pure Go, audited, tiny). **This is the default we ship** — `sasto config init` scaffolds it.
3. Plaintext YAML — **rejected** for anything secret; the schema validator refuses secret-looking fields in plaintext.

No OS-keyring dependency: the keyring package adds platform-specific native libs and a terrible cross-platform story for a tool that must run everywhere and in CI. age + env covers 100% of cases.

---

## J. MVP roadmap

### Phase 1 — Core foundation
`scaffold → config/state/secrets → ssh executor → server add/list/ssh/run/status → generic provider → recipe engine → recipe base (updates, timezone, swap, admin, ssh-hardening, firewall, fail2ban, unattended) → docker install/status → status/health → version/completion`
**Exit:** manage an imported VPS end-to-end and harden it, idempotently, with `--check`.

### Phase 2 — Applications + Backups
`app model + compose renderer → app lifecycle (install/uninstall/update/restart/logs/env) → apps: n8n, minecraft → backup engine (restic + rclone) → backup setup/run/list/restore/verify → DB dumps + app backup hooks → app: appwrite, supabase (heavy-gated)`
**Exit:** install n8n + minecraft on a hardened server, with encrypted, verified, restorable backups to Wasabi/R2/B2.

### Phase 3 — Cloud + Production
`provider package + hetzner → server create/delete/reboot/info → provider: digitalocean → recipe production (traefik reverse proxy + ACME, monitoring, backup client) → dns (cloudflare) → ssl → monitor --watch → health checks`
**Exit:** `sasto server create → setup → recipe production → app install` = a fresh VPS running SSL apps with backups, fully automated.

### Phase 4 — Scale (post-MVP)
Providers: vultr, linode, (aws/gcp/oci later) · multi-server recipes · `--json` contract stabilization · alerts (health → webhook) · server cloning/migration · teams/RBAC · web UI/API · SastoHost cloud integration · Tailscale · Kubernetes (value-add, not core).

---

## K. Repository structure (final)

```
serverops/
├── cmd/
│   └── sasto/
│       ├── main.go
│       ├── root.go                 # global flags, completion, version
│       ├── server.go  recipe.go  app.go  backup.go
│       ├── dns.go  ssl.go  monitor.go  health.go  status.go
│       └── secure.go  firewall.go  docker.go
├── internal/
│   ├── cli/        # output, spinner, table, json, confirm, flags
│   ├── config/     # config.yaml, servers.yaml, providers.yaml
│   ├── secrets/    # env-file + age
│   ├── state/      # server state.json (+ local mirror, migrations)
│   ├── ssh/        # client: dial/run/stream/put/shell/tunnel/keepalive
│   ├── executor/   # step runner (compose, module, systemd, restic…)
│   ├── provider/   # Provider iface + registry + generic
│   ├── server/     # lifecycle orchestration
│   ├── security/   # hardening modules
│   ├── docker/     # probe + compose render
│   ├── app/        # schema, validate, lifecycle, env, healthcheck
│   ├── recipe/     # recipe engine + module runner + markers
│   ├── backup/     # restic, rclone engines; jobs; verify; restore
│   ├── dns/        # cloudflare
│   ├── ssl/        # traefik/caddy ACME wiring
│   ├── monitoring/ # /proc collectors
│   └── version/
├── providers/
│   ├── hetzner/    digitalocean/  vultr/  linode/
├── apps/           # go:embed app definitions
│   ├── n8n/  minecraft/  appwrite/  supabase/  coolify/  dokploy/
├── recipes/        # go:embed recipes + modules (base, production, …)
│   └── modules/    # *.sh idempotent modules
├── scripts/        # installer (install.sh), release helpers
├── docs/           # DESIGN.md, SECURITY.md, docs/*.md
├── test/
│   ├── fixtures/   # golden compose/env output
│   └── e2e/        # tagged live-VPS tests (CI opt-in)
├── .github/workflows/  # ci.yml, release.yml (GoReleaser)
├── go.mod
└── README.md
```

**Changes vs the brief's structure:** removed the redundant `internal/cli` + `cmd/sops` split into a single thin `cmd/sasto` (cobra is the CLI layer); kept `providers/` out of `internal/` only conceptually — actually keep `internal/provider` for the interface and `providers/` for implementations to force a clean import boundary; `apps/`/`recipes/` as embedded data (not Go) so they're diffable and user-overlayable; added `internal/executor` because "step runner" is the real heart of the engine.

---

## L. Example workflows

### 1. New Hetzner VPS
```
$ sasto server create app-prod --provider hetzner --type cpx31 --region nbg1 \
    --image ubuntu-24.04 --ssh-key sastohost
✓ Creating cpx31 in nbg1 … (ID 1234567)
→ Waiting for IP … 5.75.x.x
→ Waiting for SSH … ok
✓ server app-prod (5.75.x.x)
Next: sasto server setup app-prod
```

### 2. Configure server
```
$ sasto server setup app-prod
✓ Connected
✓ apt update + security upgrades (idempotent)
✓ admin user + SSH key
✓ SSH hardened (key-only verified)
✓ UFW (22,80,443)
✓ fail2ban
✓ Docker 26.1.4
✓ server app-prod ready
Next: sasto app install n8n app-prod
```

### 3. Install Appwrite
```
$ sasto app install appwrite app-prod --domain app.example.com
⚠ appwrite requires ≥ 4Gi RAM, ~10Gi disk — proceed? [y/N] y
✓ Docker ok
→ Cloning official compose (pinned tag)
→ Writing env + secrets (0600)
→ docker compose up -d --wait
→ Health check http://localhost/health … ok
✓ appwrite installed
  URL: https://app.example.com    Status: healthy    Next: sasto backup setup
```

### 4. Install n8n
```
$ sasto app install n8n app-prod --domain n8n.example.com
✓ Docker ok
→ Writing env + secrets
→ docker compose up -d --wait
→ Health check http://localhost:5678/healthz … ok
✓ n8n installed (v1.80.1)  URL: https://n8n.example.com
```

### 5. Configure backup
```
$ sasto backup setup app-prod --remote s3:wasabi --bucket ops-app-prod
→ Generating restic repo on s3:wasabi/ops-app-prod
→ Connectivity test … ok
→ Writing jobs (appwrite: nightly 7d/30d/12m, n8n: hourly)
→ Installing systemd timers
✓ backups scheduled; recovery key saved to your secrets file
```

### 6. Check everything
```
$ sasto status
app-prod   ubuntu 24.04 · 4vCPU/8Gi · load 0.42 · disk 34% · mem 41%
  apps: appwrite ✓ healthy · n8n ✓ healthy
  backups: last run 12h ago · verify: ok · retention: 7d/30d/12m
  security: ssh-hardened ✓ firewall ✓ fail2ban ✓ unattended ✓
```

---

## M. Development roadmap (implementation order)

1. **Skeleton + UX contract** — go.mod, cobra, global flags, spinner/table/json/confirm, exit codes. This is the API of the tool; get it right first.
2. **config + secrets (age) + state** packages with unit tests + schema validation.
3. **SSH executor** (dial, run, stream, put, shell, keepalive) + `server add/list/ssh/run/status`.
4. **Recipe engine + module runner + markers**; write `base` modules (the hardest correctness work: idempotency).
5. **security** (SSH/firewall/fail2ban) with test-after-mutate + lockout safety.
6. **docker** probe + compose renderer.
7. **app** model + lifecycle; **n8n, minecraft** recipes end-to-end (validates the whole schema before heavy apps).
8. **backup** engines + setup/run/list/restore/verify + DB dumps + app hooks.
9. **appwrite, supabase** (heavy, gated).
10. **provider** + **hetzner** + `server create/delete/reboot`.
11. **recipe production** (traefik+ACME) + **dns** + **ssl**.
12. **monitor/health/status** polish, `--json` contract, completion, docs.
13. **Tests**: unit (schema, render, validation, golden files) → e2e on a throwaway VPS (CI opt-in, `SASTO_E2E=1`).
14. **Release**: GoReleaser (darwin/linux/windows × amd64/arm64, `CGO_ENABLED=0`), `install.sh` (curl|sh), Homebrew tap, versioned release notes, upgrade self-check.

---

## N. Technical risks (hardest problems + how we avoid mistakes)

1. **SSH reliability over slow links.** Giant single-session ops die on flaky networks. → keepalives, `ServerAliveInterval`-style pings, streaming output, and **long ops run a remote script writing progress to a status file the CLI polls** in short sessions. `--wait` + timeouts everywhere.
2. **Idempotency is the #1 correctness risk.** A recipe that isn't idempotent corrupts servers. → markers (`serverops:module:<id>@ver` in state), `--check` dry-run, and integration tests re-running recipes twice and asserting byte-identical state.
3. **SSH lockout from hardening.** → test-after-mutate ordering, never drop port 22, `Provider.Rescue` path, and destructive ops always print escape-hatch instructions.
4. **Docker Compose drift.** `up -d` is not a deployment. → pin images, render + store compose, `diff` before update, rollback to previous tag on health failure.
5. **Secret leakage** (the "let me just print the env file" trap). → redaction in all logs, 0600, age-encryption default, schema validator refuses plaintext secrets, auto-generated secrets never echoed.
6. **Heavy apps (supabase) on small VPS.** → RAM/disk gates + explicit warnings + documented compose overrides, never silent installs.
7. **Binary bloat from many SDKs.** → provider SDKs live behind `internal/provider` and are lazily compiled; a provider is a small, isolated package — prune with `//go:build` tags if needed. Keep the core (~ssh + cobra + age) tiny (~12MB).
8. **Windows cross-platform traps.** → all *local* code uses `os.UserConfigDir/DataDir` (no hardcoded `~/.`), path handling via `filepath`, and the remote surface is Linux-only by design (VPS target), clearly documented.
9. **Recipe/schema versioning.** Recipes evolve; servers are long-lived. → `schema_version` in state + migration runner + recipes carry `version` and `requires_docker`-style contracts.
10. **"Do everything" scope creep** (K8s, web UI, teams in v1). → the roadmap gates every one of these behind an interface boundary (Provider, Engine, App, Recipe) so they can arrive without a rewrite — and none of them are in Phase 1–3.

---

## Appendix — Competitive positioning (summary)

- **vs Ansible/Terraform:** they're languages and state engines; `sasto` is an opinionated product with a fixed, safe workflow. You can use them *with* `sasto` (e.g. tofu for infra you declare), but `sasto` never requires them.
- **vs Coolify/Dokploy/EasyPanel:** those are web dashboards you self-host (a server-side service + DB). `sasto` is a zero-server-side footprint CLI: no daemon, no database, no open port, works over pure SSH. Their strength is a GUI for one host; ours is fleet-scale scripting, provider-native provisioning, and backups.
- **vs provider CLIs:** one tool, many providers, plus everything above the VM layer.
- **What `sasto` uniquely is:** the fastest agentless path from "empty VPS" to "hardened server running multiple SSL apps with encrypted verified backups" — scriptable, idempotent, and reversible.
