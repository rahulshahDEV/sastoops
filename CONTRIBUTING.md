# Contributing to sastoops

sastoops is fully open source (Apache-2.0) — issues, pull requests, app recipes,
and docs are all welcome. SastoHost maintains it, but the codebase is yours to
use, fork, and improve.

## Ground rules

- **Keep it agentless.** No daemons on managed servers, ever.
- **Keep it light.** The local binary stays a single static binary ~10MB; the
  managed server gets nothing but standard tools (docker, restic/rclone).
- **Idempotent everything.** Recipes and app lifecycles must be safe to re-run.
- **No secrets in plaintext config.** Auto-generated secrets, env refs, or
  age-encryption only.

## Ways to contribute

1. **App recipes** — add `assets/apps/<name>/app.yaml` + `compose.yaml`
   (see [docs/APPS.md](docs/APPS.md)). This is the fastest way to help: pick an
   app, write a recipe, open a PR.
2. **Recipe modules** — new hardening/tuning modules in `assets/modules/*.sh`
   (see [docs/RECIPES.md](docs/RECIPES.md)).
3. **Providers** — implement `internal/provider.Provider` for DigitalOcean,
   Vultr, Linode, AWS, GCP, OCI…
4. **Bug reports** — include: command, `--debug` output (redact secrets), OS,
   and what you expected.
5. **Docs** — INSTALL / CLI / ARCHITECTURE / BACKUPS are in `docs/`.

## Development workflow

```bash
git clone https://github.com/rahulshahDEV/sastoops.git
cd sastoops
make test        # unit tests
make lint        # go vet + gofmt
make build       # bin/sastoops
make release     # cross-compile all platforms into dist/
```

Test with a throwaway VPS:

```bash
sastoops server add dev root@<vps-ip> --test
sastoops recipe apply base dev --check     # dry-run first, always
sastoops recipe apply base dev
```

## Commit conventions

- One logical change per commit; message style matches existing history
  (`scope: summary`).
- Never commit secrets, `.env` files, or real tokens — even in tests.
- `gofmt` + `go vet` clean before pushing.

## PR checklist

- [ ] `make test` passes
- [ ] `make lint` clean
- [ ] New apps/modules include at least one unit test
- [ ] Docs updated if behavior changed
- [ ] No secrets added

## Code of conduct

By participating you agree to the [Code of Conduct](CODE_OF_CONDUCT.md):
be respectful, constructive, and assume good faith. Reports go to
`opensource@sasto.host`.

## License

Apache-2.0 — see [LICENSE](LICENSE).