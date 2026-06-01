# Installable Command Runtime Specification

## Purpose

Defines installed `lingotui` command startup behavior and safety boundaries.

## Requirements

### Requirement: Installed Command Launches TUI

The system MUST provide an installable `lingotui` command that opens the interactive TUI directly. Startup MUST compose runtime dependencies from local stores and MUST NOT require running `go run`.

#### Scenario: Installed command opens shell

- GIVEN `lingotui` is installed on PATH
- WHEN the user runs `lingotui`
- THEN the interactive TUI opens
- AND the startup view shows shell status or onboarding

#### Scenario: Startup avoids external side effects

- GIVEN the installed command is launched
- WHEN startup completes
- THEN no recording is started
- AND no provider or network request is made

### Requirement: Install Guidance Is Actionable

The system SHOULD document install and PATH expectations so users can run `lingotui` after installation.

#### Scenario: Missing PATH guidance

- GIVEN the user installed the binary but the shell cannot find it
- WHEN the user reads install documentation
- THEN the docs explain `$GOBIN` or `$GOPATH/bin` PATH setup
