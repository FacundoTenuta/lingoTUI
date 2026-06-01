# First Run Onboarding Specification

## Purpose

Defines setup status, next actions, and safe first-run guidance.

## Requirements

### Requirement: Setup Status and Next Steps

The system MUST show first-run setup status for local credentials and microphone readiness. It MUST provide next steps without exposing secrets, starting recording, or making provider calls.

#### Scenario: Missing setup shows next steps

- GIVEN credentials or microphone readiness are missing or unknown
- WHEN the TUI opens
- THEN onboarding shows actionable next steps
- AND startup performs no paid or recording action

#### Scenario: Ready setup is acknowledged

- GIVEN local credentials exist and microphone readiness is acceptable
- WHEN the TUI opens
- THEN onboarding indicates setup is ready
- AND the user can proceed with explicit commands

### Requirement: OpenAI and Microphone Guidance

The system MUST guide OpenAI setup through the local credential store or `auth.json` path and MUST guide macOS microphone/audio permissions with best-effort status. Secret values MUST NOT be displayed.

#### Scenario: OpenAI credential status is redacted

- GIVEN an OpenAI credential exists locally
- WHEN onboarding renders provider status
- THEN it shows a redacted configured status
- AND it does not print the API key

#### Scenario: macOS permission guidance is setup-only

- GIVEN microphone permission is missing or unknown
- WHEN onboarding renders audio status
- THEN it explains the macOS permission action
- AND it does not open an audio stream
