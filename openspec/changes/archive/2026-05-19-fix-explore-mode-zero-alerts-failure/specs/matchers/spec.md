## MODIFIED Requirements

### Requirement: Explore Mode Bypasses Match Logic
The system SHALL, when `RunRequest.exploreMode = true`, query all open
alerts/signals (without rule-name filtering) at each poll, accumulate
those whose serialized form contains any indicator string into
`DiscoveredAlerts`, and run the full timeout without recording pass/fail
outcomes for individual assertions. At the end of the run, the scenario
SHALL be marked successful (`isSuccess = true`) when at least one alert
was accumulated into `DiscoveredAlerts`, and marked failed
(`isSuccess = false`) with a descriptive error message when zero alerts
were discovered.

#### Scenario: Explore run with discoveries
- **WHEN** a 1-minute explore-mode run starts and 4 distinct alerts contain the execution ID at various points
- **THEN** all 4 are recorded in the result's `DiscoveredAlerts`, no per-assertion pass/fail is recorded, and the scenario's `isSuccess` is `true`

#### Scenario: Explore run with no discoveries
- **WHEN** an explore-mode scenario completes its full timeout without any alert containing any of the configured indicators
- **THEN** `DiscoveredAlerts` is empty, the scenario's `isSuccess` is `false`, the scenario's error message indicates that no matching alerts were found, and the parent run's `failed` counter includes this scenario
