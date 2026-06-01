# Provider Configuration Specification

## Purpose

Defines provider connection, model selection, and credential privacy expectations.

## Requirements

### Requirement: Provider Connection and Model State

The system MUST support OpenAI API-key connection in the first slice. It MUST expose available models, allow model selection, keep credentials outside the project workspace, and MUST NOT print secret values. It SHOULD preserve future provider and browser-login auth paths without promising them in the first slice.

#### Scenario: Connect with configured key

- GIVEN an OpenAI API key is available
- WHEN the user enters `/connect`
- THEN the system marks the provider as connected
- AND model commands may use that connection

#### Scenario: Missing credential is actionable

- GIVEN no usable provider credential exists
- WHEN the user enters `/connect`
- THEN the system explains how to configure credentials
- AND does not start provider requests
