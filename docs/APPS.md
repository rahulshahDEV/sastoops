# App recipes — authoring guide

Apps are pure data: YAML definition + a compose template. No Go code required.
Embedded apps live in `assets/apps/<name>/` (bundled into the binary); you can
also add your own under `~/.config/sastoops/apps/<name>/` — the overlay takes
precedence and requires no recompile.

## Anatomy

```
assets/apps/<name>/
├── app.yaml         # definition: requirements, env, lifecycle, backup
└── compose.yaml     # docker compose template (Go text/template syntax)
```

## app.yaml schema

```yaml
name: myapp
version: "1.0"              # recipe-schema version, not app version
category: web
description: What it does
homepage: https://example.com

requirements:
  os: [ubuntu, debian]      # /etc/os-release $ID values
  arch: [amd64, arm64]
  min_ram: 512Mi            # gate: install refuses below this
  min_disk: 2Gi
  ports_free: [8080]        # checked during install

params:                     # user-tunable knobs (--set name=value)
  - name: memory
    default: 2G

env:                        # rendered into compose + persisted .env
  - name: APP_PASSWORD
    secret: true            # auto-generated (crypto/rand), survives re-install
  - name: APP_DOMAIN
    from: domain            # injected from --domain
  - name: APP_TZ
    default: UTC

ports: [8080]               # host ports
volumes: [data]             # named volumes (informational)

proxy:                      # traefik integration when --domain is used
  enabled: true
  port: 8080                # container port traefik routes to
  tls: true                 # Let's Encrypt via the production recipe

healthcheck:
  type: http                # http | tcp
  url: http://localhost:8080/healthz
  interval: 5s
  retries: 30

lifecycle:
  pre_update: [backup]      # auto-backup before update

backup:                     # used by `sastoops app backup` / backup run
  databases:
    - type: postgres        # postgres | mariadb | mysql
      container: db         # compose service name
      user: "{{Env "DB_USER"}}"
      password_ref: DB_PASSWORD   # key in the app's .env
      db: "{{Env "DB_NAME"}}"
      name: app             # dump filename suffix
  resources:
    - type: volume
      name: data
    - type: file
      path: /var/lib/serverops/compose/myapp/.env
```

## compose.yaml template

Rendered with Go `text/template`. Available values and functions:

| Token | Meaning |
|---|---|
| `{{.Domain}}` | `--domain` value ("" if unset) |
| `{{.Version}}` | `--version` value (default "latest") |
| `{{.Port}}` | host port (first of `ports`, or `--port`) |
| `{{.ProxyPort}}` | container port traefik targets |
| `{{if .Proxy}}…{{end}}` | true when `--domain` was given |
| `Env "NAME"` | resolved env var (incl. secrets) |
| `Param "name"` | resolved param (default or `--set`) |

Example:

```yaml
services:
  app:
    image: org/app:{{.Version}}
    restart: unless-stopped
    ports: ["{{.Port}}:8080"]
    environment:
      DOMAIN: "{{.Domain}}"
      PASSWORD: "{{Env "APP_PASSWORD"}}"
{{if .Proxy}}
    networks: [traefik]
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.app.rule=Host(`{{.Domain}}`)"
      - "traefik.http.routers.app.entrypoints=websecure"
      - "traefik.http.routers.app.tls.certresolver=letsencrypt"
      - "traefik.http.services.app.loadbalancer.server.port=8080"
{{end}}
volumes:
  data:
```

## Lifecycle & safety

- `install`: requirements check → render → upload (0600) → `up -d --wait` →
  healthcheck → state update. Re-running with a changed `--domain`/`--set`
  re-renders in place; secrets persist.
- `update`: re-renders from current `.env`, `--force-recreate`, rolls back to
  the previous image tag if the health check fails.
- `uninstall`: `compose down -v` (data volumes removed) + files removed —
  always confirms.

## Testing a new app

```bash
sastoops recipe apply base dev          # docker present
sastoops app install myapp dev --domain app.example.com
sastoops app status myapp dev
sastoops app backup myapp dev
sastoops backup run dev                 # includes dumps + volumes
```

Add a unit test asserting the rendered compose (see `internal/app/app_test.go`).