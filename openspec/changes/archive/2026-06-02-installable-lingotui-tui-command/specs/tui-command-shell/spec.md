# Delta for TUI Command Shell

## MODIFIED Requirements

### Requirement: Command Flow

The system MUST accept `/connect`, `/models`, `/record mic`, `/stop`, `/ask`, `/clear`, and `/help`. It MUST maintain visible shell state, route known commands, show actionable errors for invalid or unavailable actions, and include onboarding/help messaging when setup is incomplete.
(Previously: command flow listed supported commands and error handling without first-run onboarding messaging.)

#### Scenario: Help lists supported commands

- GIVEN the shell is running
- WHEN the user enters `/help`
- THEN the system lists supported commands and their purpose
- AND includes setup guidance for credentials and microphone readiness

#### Scenario: Unknown command is rejected

- GIVEN the shell is running
- WHEN the user enters an unsupported slash command
- THEN the system reports that the command is unknown
- AND preserves the current shell state

#### Scenario: Startup shows shell or onboarding

- GIVEN the installed command starts the TUI
- WHEN setup is incomplete
- THEN the shell shows onboarding status and next actions
- AND waits for explicit user commands
