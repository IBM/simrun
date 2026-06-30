# Getting Started

Install SimRun and reach the dashboard.

## Prerequisites

- **[mise](https://mise.jdx.dev/)** — manages Go 1.25 and Node 22 automatically. Alternatively, install Go 1.25 and Node 22 yourself.
- **PostgreSQL** — a running instance accessible from the host where SimRun will run.

## Build

Clone the repository, then run:

```bash
mise run build
```

This builds the SvelteKit frontend and compiles the `simrun` binary, placing it at `dist/simrun`.

## Run

Database migrations run automatically on startup. Set the database URL and start the server:

```bash
export SR_DATABASE_URL="postgres://user:pass@localhost:5432/simrun?sslmode=disable"
./dist/simrun
```

The UI and API are served on http://localhost:8080.

## Authentication is optional

Without `SR_GOOGLE_CLIENT_ID` and `SR_GOOGLE_CLIENT_SECRET` set, login is disabled and the app runs unauthenticated. See [deployment.md](deployment.md) for OAuth setup.

## Next steps

- [concepts.md](concepts.md) — understand scenarios, detonators, matchers, and collectors
- [walkthrough.md](walkthrough.md) — run your first detection test end to end
- [configuration.md](configuration.md) — configure connectors, app defaults, and environment variables
