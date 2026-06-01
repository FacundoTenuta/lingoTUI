# Delta for Test Harness Foundation

## MODIFIED Requirements

### Requirement: Verifiable Command and Adapter Outcomes

The system MUST make command routing, provider outcomes, credential/storage outcomes, recorder outcomes, installable runtime startup, and onboarding status testable with controlled substitutes. Tests SHALL cover successful flows and error states for first-slice commands, and MUST verify startup/onboarding without live audio devices, recording, network calls, or paid provider calls.
(Previously: tests covered command and adapter outcomes, but not installed runtime startup or onboarding safety.)

#### Scenario: Successful flow is testable without external services

- GIVEN controlled provider and recorder outcomes
- WHEN a test runs connect, record, stop, summarize, and ask actions
- THEN the system state and messages are deterministic

#### Scenario: Failure flow is testable without external services

- GIVEN controlled provider, credential, or recorder failures
- WHEN a first-slice command encounters a failure
- THEN the system returns the expected user-facing error state

#### Scenario: Startup onboarding is testable safely

- GIVEN controlled credential and permission statuses
- WHEN a test launches the runtime model
- THEN onboarding messages are deterministic
- AND no recorder or provider call is invoked
