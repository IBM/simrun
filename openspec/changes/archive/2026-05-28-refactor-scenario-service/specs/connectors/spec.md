## ADDED Requirements

### Requirement: Consistent Credential Resolution Across Call Sites
The system SHALL resolve a connector's credentials to the same set of environment variables regardless of whether the resolution is triggered by `POST /api/connectors/test` (test-connection) or by scenario-run execution. There SHALL be exactly one implementation of per-type credential resolution in the codebase; both call sites SHALL invoke it.

#### Scenario: Test-connection and scenario-run agree on credentials
- **WHEN** a connector C of type `aws` is resolved by the test-connection handler, producing env-var map M1
- **AND** the same connector C is resolved during scenario-run execution, producing env-var map M2
- **THEN** M1 and M2 contain the same keys with the same values (modulo time-limited STS session tokens, which may differ between calls but SHALL be produced by the same code path)

#### Scenario: Adding a new connector type updates both call sites at once
- **WHEN** a new connector type is added (e.g., a new cloud provider)
- **THEN** the per-type resolution logic is added in exactly one place
- **AND** both test-connection and scenario-run pick up the new type without requiring per-call-site changes
