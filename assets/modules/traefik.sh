#!/usr/bin/env bash
# serverops module: traefik — reverse proxy with Let's Encrypt via Docker socket (idempotent)
set -euo pipefail
EMAIL="${SERVEROPS_TRAEFIK_EMAIL:-}"
if [ "$(id -u)" -ne 0 ]; then SUDO="sudo -n"; else SUDO=""; fi
DIR=/var/lib/serverops/traefik
$SUDO mkdir -p "$DIR/data"
$SUDO tee "$DIR/traefik.yml" >/dev/null <<EOF
global:
  sendAnonymousUsage: false
api:
  dashboard: true
  insecure: false
entryPoints:
  web:
    address: ":80"
    http:
      redirections:
        entryPoint:
          to: websecure
          scheme: https
  websecure:
    address: ":443"
providers:
  docker:
    endpoint: "unix:///var/run/docker.sock"
    exposedByDefault: false
    network: traefik
certificatesResolvers:
  letsencrypt:
    acme:
      email: ${EMAIL:-admin@example.com}
      storage: /data/acme.json
      httpChallenge:
        entryPoint: web
EOF
$SUDO tee "$DIR/docker-compose.yml" >/dev/null <<'EOF'
services:
  traefik:
    image: traefik:v3.3
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./traefik.yml:/etc/traefik/traefik.yml:ro
      - ./data:/data
    networks:
      - traefik
networks:
  traefik:
    name: traefik
EOF
cd "$DIR"
if ! docker network inspect traefik >/dev/null 2>&1; then
  $SUDO docker network create traefik >/dev/null
fi
$SUDO docker compose up -d --wait >/dev/null
echo "traefik running (dashboard via 127.0.0.1:8080)"