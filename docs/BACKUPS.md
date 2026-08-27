# Backups

Backups run **on the server** (the data lives there; the laptop only
orchestrates). Two engines:

| | restic (default) | rclone |
|---|---|---|
| Encryption | ✅ built-in (password 0600 on server) | optional via `crypt` remote |
| Dedup / snapshots | ✅ | ❌ (mirror) |
| Integrity check | `restic check --read-data-subset 5%` | `rclone check` |
| Backends | any S3-compatible | 40+ (incl. Google Drive) |

## Setup

```bash
sastoops backup setup my-vps \
  --engine restic \
  --provider wasabi \            # wasabi | r2 | b2 | s3
  --bucket my-bucket \
  --key-id $WASABI_ACCESS_KEY_ID \
  --secret $WASABI_SECRET_ACCESS_KEY \
  --schedule "*-*-* 03:00:00"    # optional systemd timer
```

What it does on the server:

1. installs restic (or rclone)
2. writes `RESTIC_PASSWORD` + S3 credentials to
   `/var/lib/serverops/secrets/backup.env` (0600)
3. initializes the repository (`s3:<endpoint>/<bucket>/<server>`)
4. saves jobs (apps + extra paths, retention 7d/30d/12m)
5. installs a systemd timer if `--schedule` given

Credentials can come from env instead of flags:
`WASABI_ACCESS_KEY_ID`, `R2_ACCESS_KEY_ID`, `B2_APPLICATION_KEY_ID`, or
`--provider s3` + `S3_ACCESS_KEY_ID`.

## What gets backed up

For each job, `backup run`:

1. **Database dumps first** — never blind-copies live data files:
   - postgres: `pg_dump` in-container
   - mariadb/mysql: `mysqldump` in-container
   - per-app definitions in `assets/apps/*/app.yaml` (`backup.databases`)
   - dumps staged in `/var/lib/serverops/backup/dumps/`
2. **Files** — dumps dir + per-app paths/volumes (`backup.resources`)
3. restic: snapshot with tags `daily,<server>`; then `forget --prune` with
   retention (7 last / 30 daily / 12 monthly by default)
   rclone: `sync` to `remote:<server>/`

## Daily operations

```bash
sastoops backup run my-vps             # run now (also app-specific: app backup n8n my-vps)
sastoops backup list my-vps            # snapshots / files
sastoops backup status my-vps          # engine, repo, last run, jobs
sastoops backup verify my-vps          # integrity check (sample read-back)
sastoops backup restore my-vps latest  # staged restore to /var/lib/serverops/restore
sastoops backup restore my-vps a1b2c3d4 /srv/restored
```

## Restore

`backup restore` always writes to a staging directory first and requires
confirmation before anything could overwrite live data. Restoring an app:

```bash
sastoops backup restore my-vps latest /var/lib/serverops/restore
# then copy data back into the app's volume and restart the app:
sastoops app restart n8n my-vps
```

## Disaster-recovery note

The restic repo password lives in the server's `backup.env` (0600). If the
server is destroyed, you need it to unlock the repository — keep a copy in your
password manager (`cat /var/lib/serverops/secrets/backup.env` on the server
before decommissioning it).

## App-defined backup hooks

Each app declares what to dump and which resources to include
(`assets/apps/*/app.yaml` → `backup:`). To extend an app's backup without
forking: add `--paths /srv/custom` at `backup setup`, or add a new job:

```bash
sastoops backup setup my-vps --paths /srv/data --apps n8n,postgres-data
```