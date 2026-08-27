# Recipes & modules — authoring guide

Recipes are server-level profiles (base hardening, light tuning, production
proxy). A recipe is an ordered list of steps; each step runs an **idempotent
bash module**. Everything is embedded via `go:embed` and content-hashed, so:

- re-running a recipe is a no-op (markers in server state)
- editing a module's script changes its hash → only that step re-runs on the
  next apply

## Files

```
assets/recipes/<name>.yaml     # recipe definition
assets/modules/<module>.sh     # bash module, idempotent
```

## Recipe schema

```yaml
name: light
version: "1.0"
description: Optimize for small/low-RAM VPS
extends: base                  # optional: inherit all steps of another recipe
params:                        # recipe-level defaults, overridable via --set
  traefik_email: ops@sastohost.com
steps:
  - id: swap                   # unique id; also the state marker key
    module: swap               # embedded script name
    description: 1G swap
    params: {size: "1G"}       # passed to the module as SERVEROPS_SIZE
```

## Module contract

- `#!/usr/bin/env bash`, `set -euo pipefail`.
- Idempotent: detect already-done state and exit 0 without side effects.
- Root-safe: use `if [ "$(id -u)" -ne 0 ]; then SUDO="sudo -n"; else SUDO=""; fi`
  and prefix privileged commands with `$SUDO`.
- Params arrive as environment variables: `size: "1G"` → `SERVEROPS_SIZE=1G`
  (key uppercased, `-`/`.` → `_`).

Example module:

```bash
#!/usr/bin/env bash
# serverops module: example (idempotent)
set -euo pipefail
SIZE="${SERVEROPS_SIZE:-1G}"
if [ "$(id -u)" -ne 0 ]; then SUDO="sudo -n"; else SUDO=""; fi
if [ -f /marker ] && grep -q "$SIZE" /marker; then exit 0; fi
$SUDO mkdir -p /opt/example
echo "$SIZE" | $SUDO tee /marker >/dev/null
echo "example configured"
```

## Built-in modules

| Module | What it does |
|---|---|
| `apt-update` | apt update + upgrade + autoremove |
| `timezone` | set system timezone (`tz`, default UTC) |
| `swap` | create/enable swapfile (`size`, default 2G) |
| `admin-user` | create non-root admin + install SSH key (`user`, `pubkey`) |
| `ssh-hardening` | key-only SSH, MaxAuthTries 3, no X11 forwarding |
| `firewall` | UFW default-deny, allow list (`allow`, default 22,80,443) |
| `fail2ban` | sshd jail (5 retries, 1h ban) |
| `unattended-upgrades` | security-channel only, no auto-reboot |
| `docker` | Docker Engine + Compose plugin + log rotation daemon.json |
| `light-tune` | swappiness 10, journald 100M cap, docker log rotation |
| `traefik` | reverse proxy + Let's Encrypt on the traefik docker network |

## Built-in recipes

| Recipe | Steps |
|---|---|
| `base` | updates, timezone, swap, admin-user, ssh-hardening, firewall, fail2ban, unattended, docker |
| `light` | swap 1G, light-tune |
| `production` | extends base + traefik |

## Safety rules

- Never drop SSH port 22. ssh-hardening runs after admin-user and only after
  the engine verified key login works.
- Destructive steps belong in `app uninstall`/`backup restore` (confirmed), not
  in recipes.
- Keep modules small and composable — one concern per module.

## Testing a new module

```bash
# locally (if it's safe) or on a throwaway VPS:
sastoops recipe apply base dev --check     # see what would run
sastoops recipe apply base dev             # apply
sastoops recipe apply base dev             # re-run: should be a no-op
```

Unit tests: `internal/recipe/recipe_test.go` (recipe loading, param resolution,
marker logic in `engine_test.go`).