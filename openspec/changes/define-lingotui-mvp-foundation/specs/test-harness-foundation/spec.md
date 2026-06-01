# Test Harness Foundation Specification

## Purpose

Defines the behaviors that must be verifiable without live audio devices or paid provider calls.

## Requirements

### Requirement: Verifiable Command and Adapter Outcomes

The system MUST make command routing, provider outcomes, credential/storage outcomes, and recorder outcomes testable with controlled substitutes. Tests SHALL cover successful flows and error states for the first-slice commands.

#### Scenario: Successful flow is testable without external services

- GIVEN controlled provider and recorder outcomes
- WHEN a test runs connect, record, stop, summarize, and ask actions
- THEN the system state and messages are deterministic

#### Scenario: Failure flow is testable without external services

- GIVEN controlled provider, credential, or recorder failures
- WHEN a first-slice command encounters a failure
- THEN the system returns the expected user-facing error state
