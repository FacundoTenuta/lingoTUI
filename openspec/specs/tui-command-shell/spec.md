# TUI Command Shell Specification

## Purpose

Defines the observable command flow for the first lingoTUI shell.

## Requirements

### Requirement: Command Flow

The system MUST accept `/connect`, `/models`, `/record mic`, `/stop`, `/ask`, `/clear`, and `/help`. It MUST maintain visible shell state, route known commands, and show actionable errors for invalid or unavailable actions.

#### Scenario: Help lists supported commands

- GIVEN the shell is running
- WHEN the user enters `/help`
- THEN the system lists supported commands and their purpose

#### Scenario: Unknown command is rejected

- GIVEN the shell is running
- WHEN the user enters an unsupported slash command
- THEN the system reports that the command is unknown
- AND preserves the current shell state
