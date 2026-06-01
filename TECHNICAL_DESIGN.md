# lingoTUI — Technical Design

This document defines the initial technical direction for lingoTUI: a macOS-first terminal assistant that records audio, transcribes it with AI, summarizes it in Spanish, English, and German, and answers contextual language questions.

## 1. Recommended stack

| Area | Decision | Why |
|---|---|---|
| Language | Go | Strong fit for terminal apps, simple distribution, good concurrency, easy single-binary packaging. |
| TUI | Bubble Tea | Mature Go TUI framework with a clean update/view architecture. |
| Styling | Lip Gloss | Companion library for Bubble Tea terminal styling. |
| Audio recording | ffmpeg wrapper first | Pragmatic MVP path on macOS; avoids writing native audio capture too early. |
| Audio conversion | ffmpeg | Normalizes audio formats before transcription. |
| AI integration | Provider abstraction | Keeps OpenAI, OpenRouter, Ollama, and future providers swappable. |
| Initial AI provider | OpenAI | Best first target for transcription + chat quality. |
| Config | TOML or JSON config file | Human-editable and easy to debug. |
| Credential storage | macOS Keychain later, local auth file for MVP | Secure storage is important, but local file is simpler for the first working version. |
| Testing | Go unit tests + integration seams | Business logic can be tested without real audio or real provider calls. |

## 2. Why Go

Go is the best initial fit because lingoTUI is a local CLI/TUI tool, not a web app.

Benefits:

- Produces a simple binary.
- Has excellent terminal UI libraries.
- Handles concurrent audio processing and provider calls cleanly.
- Keeps installation simpler than Python dependency environments.
- Avoids shipping a Node runtime or Electron-style weight.

Tradeoff:

- Some AI SDK ecosystems are richer in TypeScript.
- Some audio libraries may be easier to prototype in Python.

Decision: use Go for the app, but call external tools such as `ffmpeg` where that reduces complexity.

## 3. Architecture

```text
TUI
  ↓
Command Router
  ↓
Use Cases
  ├─ Connect Provider
  ├─ Select Models
  ├─ Record Audio
  ├─ Transcribe Audio
  ├─ Summarize Transcript
  └─ Ask Contextual Question
  ↓
Ports / Interfaces
  ├─ AudioRecorder
  ├─ TranscriptionProvider
  ├─ ChatProvider
  ├─ CredentialStore
  ├─ ConfigStore
  └─ ContextStore
  ↓
Adapters
  ├─ ffmpeg / macOS audio adapter
  ├─ OpenAI adapter
  ├─ local auth file adapter
  └─ filesystem config adapter
```

The important rule: the TUI must not know provider details. It should call use cases. Use cases should depend on interfaces. Providers, audio tools, and storage are adapters.

## 4. Proposed project structure

```text
.
├── cmd/
│   └── lingotui/
│       └── main.go
├── internal/
│   ├── app/
│   │   ├── commands.go
│   │   └── usecases.go
│   ├── audio/
│   │   ├── recorder.go
│   │   └── ffmpeg_recorder.go
│   ├── config/
│   │   ├── config.go
│   │   └── file_store.go
│   ├── context/
│   │   └── memory.go
│   ├── credentials/
│   │   ├── store.go
│   │   └── file_store.go
│   ├── provider/
│   │   ├── provider.go
│   │   ├── registry.go
│   │   └── openai/
│   │       └── client.go
│   └── tui/
│       ├── model.go
│       ├── update.go
│       └── view.go
├── PRD.md
└── TECHNICAL_DESIGN.md
```

## 5. Core interfaces

### AudioRecorder

```go
type AudioRecorder interface {
    Start(ctx context.Context, source AudioSource) error
    Stop(ctx context.Context) (AudioFile, error)
}
```

### TranscriptionProvider

```go
type TranscriptionProvider interface {
    Transcribe(ctx context.Context, input TranscriptionInput) (Transcript, error)
}
```

### ChatProvider

```go
type ChatProvider interface {
    Complete(ctx context.Context, input ChatInput) (ChatResponse, error)
}
```

### CredentialStore

```go
type CredentialStore interface {
    Save(ctx context.Context, provider string, credentials Credentials) error
    Load(ctx context.Context, provider string) (Credentials, error)
    Delete(ctx context.Context, provider string) error
}
```

## 6. Commands

Initial commands:

```text
/connect       Connect an AI provider
/models        Select transcription and chat models
/record mic    Start microphone recording
/stop          Stop recording and process audio
/ask           Ask a contextual question
/clear         Clear recent context
/help          Show commands
```

Later commands:

```text
/record system     Record system audio
/record both       Record microphone + system audio
/history           Show recent summaries
/privacy           Show what is stored and what is sent to providers
```

## 7. Provider connection flow

MVP flow:

```text
> /connect
Select provider: OpenAI
Auth method: Enter API key manually
Paste API key: ********
Connected to OpenAI.
```

Future flow:

```text
> /connect
Select provider: OpenAI
Auth method: Login with ChatGPT Plus/Pro
Opening browser...
Waiting for authentication...
Connected to OpenAI.
```

The ChatGPT Plus/Pro login flow must be researched before implementation. API key authentication is the reliable MVP path.

## 8. Storage locations

Suggested macOS paths:

```text
~/Library/Application Support/lingotui/config.toml
~/Library/Application Support/lingotui/auth.json
~/Library/Caches/lingotui/audio/
```

For MVP:

- Audio files should be temporary.
- Transcripts should not be persisted by default.
- Credentials must never be stored in the project directory.

Future improvement:

- Store credentials in macOS Keychain.

## 9. Audio strategy

### MVP

Use `ffmpeg` or another external capture tool to record microphone audio on macOS.

Advantages:

- Faster implementation.
- Easier format conversion.
- Avoids native CoreAudio complexity early.

### Later

Investigate native CoreAudio or AVFoundation integration if `ffmpeg` becomes too limited.

System audio should be treated separately and may require BlackHole or Loopback.

## 10. Processing flow

```text
1. User starts recording.
2. AudioRecorder writes temporary audio file.
3. User stops recording.
4. Audio is normalized if needed.
5. TranscriptionProvider generates transcript.
6. ChatProvider summarizes transcript in ES/EN/DE.
7. ContextStore keeps transcript + summary in memory.
8. User asks follow-up questions.
9. ChatProvider answers using recent context.
```

## 11. Privacy defaults

Default behavior:

- Do not record until the user explicitly starts recording.
- Show recording status clearly.
- Delete temporary audio after processing unless debug mode is enabled.
- Keep transcript and summary in memory only.
- Provide `/clear` to erase context.
- Document that audio/transcripts are sent to the selected AI provider.

## 12. Testing strategy

Test the app by isolating external systems.

| Area | Test approach |
|---|---|
| Command parsing | Unit tests |
| Use cases | Unit tests with fake providers and fake recorder |
| Provider adapters | Integration tests behind opt-in environment variables |
| TUI state | Update/model tests |
| Audio recording | Manual macOS verification first |

MVP should avoid tests that require real microphone access in CI.

## 13. First implementation slice

Build the smallest useful vertical slice:

1. Initialize Go module.
2. Create Bubble Tea TUI shell.
3. Add command parser.
4. Add `/connect` with OpenAI API key storage.
5. Add `/models` config.
6. Add fake recorder and fake provider for development.
7. Add real OpenAI chat adapter.
8. Add real transcription adapter.
9. Add microphone recording.
10. Wire `/record mic` → `/stop` → transcript → summary.

## 14. Open technical questions

- Which OpenAI transcription model should be the default?
- Which Go package or command is most reliable for macOS microphone capture?
- Should credentials start with local file storage or go directly to macOS Keychain?
- Can ChatGPT Plus/Pro browser login be implemented in a stable and permitted way?
- Should summaries always include all three languages or should that be configurable?
