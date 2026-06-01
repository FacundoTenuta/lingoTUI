# Audio Capture Specification

## Purpose

Defines recording behavior for microphone-first audio capture and future source expansion.

## Requirements

### Requirement: Audio Source Lifecycle

The system MUST support `/record mic` and `/stop` as the first recording flow. It MUST show recording state, process only after stop, and keep captured audio temporary by default. It SHOULD reserve source choices for later system audio and combined microphone/system audio.

#### Scenario: Microphone recording completes

- GIVEN the provider can process audio
- WHEN the user records with `/record mic` and then enters `/stop`
- THEN the system ends recording and makes the captured audio available for processing

#### Scenario: Unsupported source is deferred

- GIVEN system audio is not implemented
- WHEN the user requests system or combined audio
- THEN the system reports the source is not available in this slice
