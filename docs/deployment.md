# Deployment

Run SimRun in production with Docker and optional auth.

## Docker

Build the image from the repository root:

```bash
docker build -t simrun .
```

Run the container, mounting a volume so state persists across restarts:

```bash
docker run -p 8080:8080 \
  -e SR_DATABASE_URL="postgres://..." \
  -v simrun-data:/home/nonroot/.simrun \
  simrun
```

The image bundles `aws`, `gcloud`, and `az` CLIs — the tools used by detonators when executing cloud attack simulations.

The volume mounted at `/home/nonroot/.simrun` is the default `SR_DATA_DIR`. It stores the secret-encryption key. If the key is lost, stored connector credentials become unreadable.

## Authentication (Google OAuth)

By default, SimRun runs without authentication. To enable login:

| Variable | Description |
|---|---|
| `SR_GOOGLE_CLIENT_ID` / `SR_GOOGLE_CLIENT_SECRET` | Google OAuth credentials — setting both enables login |
| `SR_WEB_URL` | External base URL used to construct the OAuth redirect URI (e.g. `https://simrun.example.com`) |
| `SR_GOOGLE_ALLOWED_DOMAIN` | Restrict login to a single Google Workspace domain (e.g. `example.com`) |
| `SR_AUTH_SESSION_TTL_HOURS` | Session lifetime in hours (default `168`, i.e. 7 days) |

See [configuration.md](configuration.md) for the full list of environment variables.

## Data persistence

The encryption key written to `SR_DATA_DIR` protects all secrets stored in the database (connector credentials, secret group values). If the data directory is not persisted — for example, because a container restarts without a volume — the key is regenerated and previously stored secrets become unreadable.

Always mount `SR_DATA_DIR` to durable storage in production.

## See also

- [configuration.md](configuration.md) — all environment variables and app config defaults
- [getting-started.md](getting-started.md) — build SimRun and reach the dashboard for the first time
