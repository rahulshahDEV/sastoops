# Security Policy

## Supported versions

| Version | Supported |
|---|---|
| latest (main) | ✅ |
| < latest | ⚠️ patch on request |

## Reporting a vulnerability

**Do not open a public issue for security bugs.** Email `security@sasto.host`
(or DM a maintainer) with:

- affected command/version (`sastoops version`)
- a minimal reproduction (redact all secrets)
- impact assessment, if you have one

You should get a first response within 3 business days. We will coordinate a
fix and disclosure timeline with you, and credit you in the release notes
(unless you prefer anonymity).

## Security model (summary)

- **Agentless by design.** sastoops never installs a daemon or agent on managed
  servers; every action is a one-shot SSH session. There is no open port, no
  server-side service, no database to attack.
- **SSH is the trust boundary.** The CLI is as secure as your SSH setup.
  Key-based auth is preferred; password auth is supported but discouraged.
- **Secrets:** app secrets are auto-generated (crypto/rand) and stored on the
  server at `/var/lib/serverops/env/<app>.env` and `/var/lib/serverops/secrets/`
  with mode 0600. They are never echoed to stdout; `app env` redacts
  secret-looking keys; logs strip `password=`/tokens. Local provider tokens are
  read from env vars (e.g. `SASTO_HETZNER_TOKEN`) — never commit them.
- **Hardening order matters:** the `base` recipe installs the admin SSH key,
  verifies a key-only connection, and only then disables password auth.
  Port 22 is never removed from the firewall. All destructive operations
  require confirmation (`--yes` opts out).
- **Backups are encrypted** (restic) with the password stored 0600 on the
  server; `backup setup --show-recovery` is a documented escape hatch for the
  repo password.

## Reporting other issues

Bugs and feature requests: GitHub issues. See [CONTRIBUTING.md](CONTRIBUTING.md).