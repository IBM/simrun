# Auth Specification

## Purpose
Authenticates operators of the simrun web UI and API via Google OAuth2, manages
server-side sessions, and gates API access. Supports a "no-auth" mode for
development. The only identity provider implemented is Google; this is
intentional and documented as such.

## Requirements

### Requirement: Optional Authentication
The system SHALL operate in one of two modes selected at startup based on
bootstrap environment variables: **enabled** when both `SR_GOOGLE_CLIENT_ID`
and `SR_GOOGLE_CLIENT_SECRET` are set, and **disabled** otherwise. The mode
MUST NOT change without restarting the process.

#### Scenario: Auth disabled when client credentials missing
- **WHEN** the server starts without `SR_GOOGLE_CLIENT_ID` or `SR_GOOGLE_CLIENT_SECRET`
- **THEN** all API routes under `/api/*` SHALL be reachable without a session

#### Scenario: Auth enabled when client credentials present
- **WHEN** both `SR_GOOGLE_CLIENT_ID` and `SR_GOOGLE_CLIENT_SECRET` are set
- **THEN** all API routes under `/api/*` (except the public auth endpoints)
  SHALL require a valid `simrun_session` cookie

### Requirement: Public Endpoints
The system SHALL serve the following endpoints without requiring authentication
regardless of auth mode: `GET /health`, `GET /api/auth/login`,
`GET /api/auth/callback`, `POST /api/auth/logout`.

#### Scenario: Health check is always public
- **WHEN** any client requests `GET /health`
- **THEN** the server responds 200 with `{"status":"ok"}` without consulting any session

### Requirement: Google OAuth2 Login Flow
When auth is enabled, the system SHALL implement the OAuth2 authorization-code
flow: `GET /api/auth/login` issues an `oauth_state` cookie and redirects the
browser to Google's consent screen; `GET /api/auth/callback` verifies the
state, exchanges the code for tokens, fetches the user's profile, and creates
a session.

#### Scenario: Login redirects to Google
- **WHEN** an unauthenticated user requests `GET /api/auth/login` and auth is enabled
- **THEN** the response sets an `oauth_state` cookie (HttpOnly, SameSite=Lax, ~10 min TTL)
- **AND** redirects (302) to `https://accounts.google.com/o/oauth2/auth?...`

#### Scenario: Login when auth is disabled
- **WHEN** any client requests `GET /api/auth/login` and auth is disabled
- **THEN** the response is HTTP 503 with body `"authentication not configured"`

### Requirement: OAuth State Validation
The system SHALL reject any callback whose `state` query parameter does not
match the `oauth_state` cookie issued at login start, using a constant-time
comparison.

#### Scenario: Missing or mismatched state
- **WHEN** `GET /api/auth/callback` is called without an `oauth_state` cookie or with a `state` value that does not match
- **THEN** the response is HTTP 400 and no session is created

### Requirement: Domain Restriction
When `SR_GOOGLE_ALLOWED_DOMAIN` is set to a non-empty value, the system SHALL
reject any user whose Google email does not end with `@<domain>`. When the
variable is empty or unset, all Google accounts are accepted.

#### Scenario: Email outside allowed domain
- **WHEN** `SR_GOOGLE_ALLOWED_DOMAIN=example.com` and the OAuth callback returns the email `someone@other.org`
- **THEN** the response is HTTP 403 with body `"only @example.com accounts are allowed"`
- **AND** no session row is inserted

### Requirement: Session Issuance
On a successful callback, the system SHALL generate a 32-byte random session
ID (URL-safe base64 encoded), insert a row in `auth_sessions` with the user's
email, name, picture, and `expires_at = now() + SessionTTL`, set a
`simrun_session` cookie (HttpOnly, SameSite=Lax, Secure when TLS) with
`MaxAge = SessionTTL.Seconds()`, and redirect the browser to `/`.

#### Scenario: Successful login
- **WHEN** a user from an allowed domain completes the OAuth flow
- **THEN** an `auth_sessions` row is inserted
- **AND** the response sets a `simrun_session` cookie and redirects (302) to `/`

### Requirement: Session TTL
The system SHALL read session TTL from `SR_AUTH_SESSION_TTL_HOURS` at startup,
defaulting to 168 (7 days). A non-positive or unparseable value MUST fall
back to the default. Sessions MUST expire exactly at the issued
`expires_at`; there is no sliding renewal.

#### Scenario: Default TTL when env var missing
- **WHEN** the server starts with `SR_AUTH_SESSION_TTL_HOURS` unset
- **THEN** session `expires_at` is set to `created_at + 168h`

#### Scenario: Invalid TTL falls back to default
- **WHEN** `SR_AUTH_SESSION_TTL_HOURS=0` or `SR_AUTH_SESSION_TTL_HOURS=abc`
- **THEN** the default of 168 hours is used

### Requirement: Session Validation Middleware
For every protected request, the system SHALL look up the `simrun_session`
cookie in `auth_sessions` and reject the request 401 if the row is missing or
`expires_at <= now()`. Expired sessions MUST also clear the browser cookie
(set with `MaxAge: -1`).

#### Scenario: No session cookie
- **WHEN** a request to `/api/scenarios` arrives without a `simrun_session` cookie and auth is enabled
- **THEN** the response is HTTP 401 with `{"error":"authentication required"}`

#### Scenario: Expired session
- **WHEN** the cookie's session row exists but `expires_at` is in the past
- **THEN** the response is HTTP 401 with `{"error":"session expired"}`
- **AND** a `Set-Cookie: simrun_session=; Max-Age=-1` header is set

### Requirement: Logout
`POST /api/auth/logout` SHALL delete the session row identified by the
`simrun_session` cookie (if present) and clear the cookie with `MaxAge: -1`.
The endpoint SHALL always return HTTP 204, regardless of whether a session
existed.

#### Scenario: Logout with active session
- **WHEN** an authenticated user posts to `/api/auth/logout`
- **THEN** the `auth_sessions` row is deleted
- **AND** the response is 204 with a cookie-clearing header

#### Scenario: Logout without session
- **WHEN** a client without a session cookie posts to `/api/auth/logout`
- **THEN** the response is 204

### Requirement: Current User Endpoint
`GET /api/auth/me` SHALL return `{email, name, picture}` for the authenticated
user when auth is enabled. When auth is disabled, the endpoint SHALL return
`{"email":"anonymous","name":"Anonymous"}`.

#### Scenario: Authenticated user
- **WHEN** an authenticated user requests `/api/auth/me`
- **THEN** the response is 200 with their email, name, and picture

#### Scenario: Auth disabled
- **WHEN** auth is disabled and any client requests `/api/auth/me`
- **THEN** the response is 200 with `{"email":"anonymous","name":"Anonymous"}`

### Requirement: WebSocket Authentication
The `GET /api/ws` endpoint SHALL validate the session before performing the
WebSocket upgrade when auth is enabled, returning HTTP 401 (text/plain) for
invalid sessions.

#### Scenario: WebSocket without session
- **WHEN** auth is enabled and an unauthenticated client requests `/api/ws`
- **THEN** the response is HTTP 401 before the upgrade handshake

### Requirement: User Attribution on Mutations
The system SHALL extract the authenticated user's email from the session and
record it in `created_by` / `updated_by` columns on resources that have such
columns (scenarios, packs, secret groups, connectors). When auth is disabled,
these columns SHALL be set to the empty string.

#### Scenario: Saved scenario records the author
- **WHEN** an authenticated user creates a saved scenario
- **THEN** the inserted row has `created_by = <user.email>`

### Requirement: OAuth Redirect URL
The system SHALL construct the OAuth redirect URI from `SR_WEB_URL` when set;
otherwise from the request's host and forwarding headers
(`X-Forwarded-Proto`, `X-Forwarded-Host` or `Host`).

#### Scenario: Explicit base URL
- **WHEN** `SR_WEB_URL=https://simrun.example.com` is set
- **THEN** the OAuth redirect URI used in the login flow is `https://simrun.example.com/api/auth/callback`

### Requirement: Expired Session Cleanup
The system SHALL run a background goroutine that periodically deletes
`auth_sessions` rows where `expires_at < now()`.

#### Scenario: Periodic cleanup
- **WHEN** the cleanup interval elapses
- **THEN** all rows with `expires_at` in the past are removed from `auth_sessions`
