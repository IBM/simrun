# Configuration

Every deploy-time setting SimRun reads.

## Overview

Deploy-time configuration is environment-only. The `SR_*` environment variables below are the only settings SimRun reads at startup. Everything else — connectors, secrets, packs, schedules, assessments, and app-level operational defaults — lives in the database and is managed through the UI.

## Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `SR_DATABASE_URL` | yes | — | PostgreSQL connection string |
| `SR_WEB_PORT` | no | `8080` | HTTP listen port |
| `SR_DATA_DIR` | no | `~/.simrun` | Local data dir (encryption key, SSH logs) |
| `SR_ENCRYPTION_KEY_FILE` | no | `$SR_DATA_DIR/encryption.key` | Key file for encrypting stored secrets |
| `SR_DEBUG` | no | off | Verbose logging when set to a non-zero/non-`false` value |
| `SR_WEB_DEV` | no | off | Dev mode when set to `1` |
| `SR_WEB_URL` | no | — | External base URL (used for OAuth redirects) |
| `SR_GOOGLE_CLIENT_ID` / `SR_GOOGLE_CLIENT_SECRET` | no | — | Google OAuth credentials (enables login) |
| `SR_GOOGLE_ALLOWED_DOMAIN` | no | — | Restrict OAuth login to a Google Workspace domain |
| `SR_AUTH_SESSION_TTL_HOURS` | no | `168` | Session lifetime in hours |

## App config defaults

Operational defaults that admins tune through the UI at `/config`. These are stored in the database (the `app_config` table) and take effect without a restart.

Confirmed fields include:

- **Parallelism** (default `5`) — number of scenarios that run concurrently within a single run.
- **Terraform version** — pin the Terraform binary version used by pack detonations.
- **Pack logs** — toggle capture of Terraform output in run logs (enabled by default).
- **SSH logging** — toggle SSH session logging (disabled by default).
- **Run log retention** — automatically purge run logs after a configurable number of days (default 7 days, enabled by default).
- **Run retention** — automatically purge run records after a configurable number of days (default 30 days, disabled by default).

## See also

- [deployment.md](deployment.md) — production deploy checklist, OAuth setup, Docker
- [getting-started.md](getting-started.md) — build SimRun and reach the dashboard for the first time
