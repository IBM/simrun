## MODIFIED Requirements

### Requirement: List Saved Scenarios
The system SHALL serve `GET /api/scenarios` as a paginated, filterable
list ordered by `updated_at DESC`. The response SHALL be a JSON object
`{scenarios, total, page, perPage}` where `scenarios` is the page slice
(possibly empty array, never `null`) and `total` is the row count after
filters but before `LIMIT/OFFSET`.

Query parameters:
- `page` (integer, default `1`, must be `>= 1`).
- `per_page` (integer, default `50`, clamped to `[1, 100]`).
- `name` (string, optional) — case-insensitive substring match against
  `saved_scenarios.name` (`ILIKE %name%`).
- `type` (string, repeatable) — restricts `saved_scenarios.type` to
  the listed values. Allowed values: `standard`, `explore`, `collect`.
  An unrecognized value SHALL return HTTP 400.
- `since` (Go duration string, optional, e.g. `24h`, `168h`) —
  restricts results to `updated_at >= now() - since`. A malformed or
  non-positive duration SHALL return HTTP 400.

#### Scenario: Most-recently-updated first
- **WHEN** a client requests `/api/scenarios` with no parameters
- **THEN** the response is HTTP 200 with `{scenarios, total, page: 1, perPage: 50}` and `scenarios` is ordered with the most recently updated scenario first

#### Scenario: Pagination slice
- **WHEN** a client requests `/api/scenarios?page=2&per_page=25` and there are 60 saved scenarios matching no filters
- **THEN** `total = 60`, `page = 2`, `perPage = 25`, and `scenarios.length` is 25 (rows 26–50 in `updated_at DESC` order)

#### Scenario: Empty page beyond range
- **WHEN** a client requests `page=99` on a table with 10 rows
- **THEN** the response is HTTP 200 with `scenarios: []` and `total: 10`

#### Scenario: Name substring filter
- **WHEN** a client requests `/api/scenarios?name=login`
- **THEN** only scenarios whose `name` contains `"login"` (case-insensitive) are returned, and `total` reflects the filtered count

#### Scenario: Multi-type filter
- **WHEN** a client requests `/api/scenarios?type=standard&type=explore`
- **THEN** the response includes only scenarios whose `type` is `standard` or `explore`

#### Scenario: Invalid type rejected
- **WHEN** a client requests `/api/scenarios?type=bogus`
- **THEN** the response is HTTP 400 and no rows are returned

#### Scenario: Since window filter
- **WHEN** a client requests `/api/scenarios?since=24h`
- **THEN** the response includes only scenarios with `updated_at >= now() - 24h`

#### Scenario: Malformed since rejected
- **WHEN** a client requests `/api/scenarios?since=abc`
- **THEN** the response is HTTP 400

#### Scenario: Combined filters
- **WHEN** a client requests `/api/scenarios?name=ssh&type=explore&since=168h&page=1&per_page=25`
- **THEN** results are scenarios whose name ILIKE `%ssh%` AND type is `explore` AND `updated_at` is within the past week, paginated to the first 25 in `updated_at DESC` order, with `total` reflecting all matches

#### Scenario: per_page clamped to maximum
- **WHEN** a client requests `/api/scenarios?per_page=500`
- **THEN** `perPage` in the response is `100` and at most 100 rows are returned
