# Language Context Flow Specification

## Purpose

Defines transcription, summaries, recent-context questions, and clearing behavior.

## Requirements

### Requirement: Transcript Summary and Questions

The system MUST turn completed recordings into a transcript and summaries in Spanish, English, and German. It MUST answer `/ask` using recent transcript/summary context, and `/clear` MUST remove that context. Context SHOULD remain in memory by default and MUST NOT be persisted unless the user explicitly enables storage later.

#### Scenario: Recording produces multilingual summary

- GIVEN a stopped recording is ready for processing
- WHEN transcription and summarization complete
- THEN the system shows a transcript-derived summary in ES, EN, and DE

#### Scenario: Clear removes context

- GIVEN recent context exists
- WHEN the user enters `/clear`
- THEN subsequent `/ask` requests do not use the cleared context
