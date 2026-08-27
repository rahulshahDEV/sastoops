# Architecture

> Full design rationale lives in [DESIGN.md](DESIGN.md). This page is the practical
> architecture: how sastoops works, where state lives, and how to reason about it.

## One picture

```
+--------------------------------------------------------------+
|  sastoops CLI (local) - single ~9MB static Go binary          |
|  cobra commands -> orchestrator                               |
|  +--------------+  +--------------+  +-------------------+    |
|  | ssh executor |  | providers    |  | cloudflare API     |   |
|  +------+-------+  | hetzner/generic | +--------+---------+   |
|         | SSH :22  +------+-------+            |               |
+---------+----------------+---------------------+---------------+
          |                | HTTPS               |
+---------v----------------v---------------------v---------------+
|  Remote Linux VPS - AGENTLESS (nothing installed)              |
|  . docker / docker compose   (files we render)                 |
|  . restic / rclone           (backups -> S3/R2/B2/Wasabi)      |
|  . state: /var/lib/serverops/state.json                        |
+---------------------------------------------------------------+
```

## Principles

1. **Agentless.** Every action is a one-shot SSH session. We never install a
   daemon or agent on the server. sastoops works against *any* Linux box you can
   SSH into.
2. **Local is thin, server is the source of truth.** Server state (installed
   apps, applied recipes, backup jobs) lives at `/var/lib/serverops/state.json`
   on the server. Your laptop only caches a copy for fast `status`.
3. **Idempotency everywhere.** Recipes/apps are re-runnable. Modules are marked
   in state by content-hash; nothing re-runs unless the module content changed.
4. **No runtime deps on the managed box.** We only use tools the server already
   has (bash, curl, apt) or installs once (docker, restic/rclone) — all standard,
   all ours.

## Server-side layout

```
/var/lib/serverops/
+-- state.json              # schema-versioned: modules, recipes, apps, backups
+-- env/<app>.env           # app env vars + secrets (0600)
+-- compose/<app>/          # rendered compose.yaml + .env
|   +-- compose.yaml
+-- secrets/                # backup.env, rclone.conf (0600)
+-- backup/
    +-- dumps/              # staged DB dumps before upload
```

## Flow of a command

`app install n8n srv01 --domain n8n.example.com`:

```
dial SSH -> requirements check (OS/arch/docker/RAM)
-> load remote state (or create)
-> render compose.yaml + .env from embedded template
   (secrets auto-generated, persisted on re-install)
-> upload both files (0600)
-> docker compose up -d --wait
-> health check (http/tcp poll on the server)
-> update state.json -> cache locally -> print URL
```

`recipe apply base srv01`:

```
load recipe YAML (embedded, user-overlayable)
for each step:
    if state.modules[step.id].version == hash(module.sh): skip
    else: export SERVEROPS_* params -> run module bash via SSH -> mark in state
```

## What uses what

| Capability | Mechanism |
|---|---|
| Server ops | SSH (`golang.org/x/crypto/ssh`), streams + interactive PTY |
| File transfer | base64-over-SSH `Put` (no SFTP dependency) |
| Provisioning | provider REST APIs (Hetzner thin client; generic = config record) |
| Apps | remote `docker compose` CLI (rendered compose files) |
| Recipes | embedded bash modules (`go:embed`), content-hashed markers |
| Backups | remote restic (default) or rclone -> S3-compatible endpoints |
| DNS | Cloudflare API (Bearer token) |

## Configuration (local)

- `~/.config/sastoops/config.yaml` — servers, providers (tokens via env refs),
  backup defaults. Written 0600.
- Server creds are resolved per-command: key file > agent > password.

## Exit codes

`0` ok · `1` general · `2` usage · `3` network/ssh · `4` remote exec failed ·
`5` not found · `6` already exists · `7` config/validation · `8` auth.

## Concurrency

Multi-server commands iterate servers sequentially today; the design reserves a
bounded worker pool for fan-out (see DESIGN.md §B).

## Extending

- New app: `apps/<name>/app.yaml` + `compose.yaml` (embedded at build time) or
  user overlay `~/.config/sastoops/apps/<name>/` — see [APPS.md](APPS.md).
- New recipe/module: `recipes/<name>.yaml` + `modules/<module>.sh` — see
  [RECIPES.md](RECIPES.md).
- New provider: implement `internal/provider.Provider` and register it in
  `provider.NewRegistry`.