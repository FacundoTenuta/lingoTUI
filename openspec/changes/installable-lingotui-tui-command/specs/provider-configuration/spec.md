# Delta for Provider Configuration

## MODIFIED Requirements

### Requirement: Provider Connection and Model State

The system MUST support OpenAI API-key connection in the first slice. It MUST expose available models only after explicit provider commands, allow model selection, keep credentials outside the project workspace in the local credential store or `auth.json`, and MUST NOT print secret values. Missing credentials MUST produce guidance without starting provider requests. It SHOULD preserve future provider and browser-login auth paths without promising them in the first slice.
(Previously: provider configuration required API-key connection and privacy, but did not define onboarding-safe local `auth.json` status.)

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

#### Scenario: Onboarding reports credential status safely

- GIVEN onboarding inspects local provider setup
- WHEN credential status is rendered
- THEN it shows the credential path and redacted status
- AND it does not make provider or network calls
