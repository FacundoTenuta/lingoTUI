# Design: Define lingoTUI MVP Foundation

## Technical Approach

Create a small Go module using Bubble Tea for shell state and Lip Gloss for view styling. The first slice follows the proposed ports/adapters path: TUI commands call application use cases, use cases depend on interfaces, and concrete ffmpeg/OpenAI/filesystem adapters sit at the edge. This maps to the five specs: command routing, provider/model configuration, microphone capture, transcript-summary-context flow, and a fakeable Go test harness. Roadmap capabilities remain represented as seams only: audio source variants, browser auth method, real-time mode flag, and provider registry.

## Architecture Decisions

| Decision | Choice | Alternatives considered | Rationale |
|---|---|---|---|
| App boundary | `internal/app` owns commands/use cases and port interfaces | Put interfaces in each adapter package | Keeps TUI/provider/audio decoupled and makes command flows testable without external services. |
| TUI | Bubble Tea `Model.Update` delegates parsed commands to app service | Business logic inside Bubble Tea model | Preserves stateful terminal UX while avoiding provider/audio details in presentation. |
| Audio | ffmpeg-backed `Recorder` for `mic`; source enum includes `system`/`both` as unsupported | Native CoreAudio now | ffmpeg validates MVP faster; explicit unsupported sources preserve future system audio without overcommitting. |
| Credentials | Local file store under app support; never project dir or stdout | macOS Keychain now, env-only | File store is implementable first; interface allows Keychain later and satisfies privacy boundary. |
| Providers | OpenAI adapter behind transcription/chat interfaces | Hard-code OpenAI calls in use cases | Keeps multiple providers and model selection possible while limiting first implementation to OpenAI API-key. |

## Data Flow

```text
/record mic -> tui.Model -> app.Router -> Recorder.Start(mic) -> recording state
/stop      -> Recorder.Stop() -> AudioFile -> Transcriber.Transcribe()
           -> Chat.Summarize(ES,EN,DE) -> ContextStore.Replace() -> TUI output
/ask text  -> ContextStore.Current() -> Chat.Answer(question, context) -> TUI output
/clear     -> ContextStore.Clear() -> TUI output
```

## File Changes

| File | Action | Description |
|---|---|---|
| `go.mod` | Create | Module and Bubble Tea/Lip Gloss dependencies. |
| `cmd/lingotui/main.go` | Create | Safe entrypoint that reports config/credential paths; full live TUI/provider/audio runtime composition is deferred to avoid surprising network or microphone access. |
| `internal/app/{commands,usecases,ports,types}.go` | Create | Parser, use cases, interfaces, shared domain types. |
| `internal/tui/{model,update,view}.go` | Create | Bubble Tea shell state, command submission, styled output. |
| `internal/audio/{recorder,ffmpeg}.go` | Create | Recorder port and ffmpeg microphone adapter. |
| `internal/provider/{registry,provider}.go` | Create | Provider/model registry and transcription/chat contracts. |
| `internal/provider/openai/client.go` | Create | Minimal OpenAI API-key adapter. |
| `internal/config/{config,file_store}.go` | Create | Model/provider config outside workspace. |
| `internal/credentials/{store,file_store}.go` | Create | Secret persistence boundary with redaction rules. |
| `internal/context/memory.go` | Create | In-memory transcript/summary context store. |
| `internal/testutil/fakes.go` | Create | Fake recorder/provider/stores for deterministic tests. |

## Interfaces / Contracts

```go
type Recorder interface{ Start(context.Context, AudioSource) error; Stop(context.Context) (AudioFile, error) }
type Transcriber interface{ Transcribe(context.Context, AudioFile, ModelRef) (Transcript, error) }
type Chat interface{ Summarize(context.Context, Transcript, []Language, ModelRef) (Summary, error); Answer(context.Context, Question, RecentContext, ModelRef) (Answer, error) }
type CredentialStore interface{ Save(context.Context, ProviderID, Secret) error; Load(context.Context, ProviderID) (Secret, error); Delete(context.Context, ProviderID) error }
type ConfigStore interface{ Load(context.Context) (Config, error); Save(context.Context, Config) error }
type ContextStore interface{ Replace(RecentContext); Current() (RecentContext, bool); Clear() }
```

## Testing Strategy

| Layer | What to Test | Approach |
|---|---|---|
| Unit | Command parser, use-case state, config paths, redaction | Table-driven Go tests and `t.TempDir()`. |
| TUI | `/help`, unknown command, record/stop/clear state | Direct `Model.Update()` tests with fake app service. |
| Integration | ffmpeg command execution and OpenAI adapter | Opt-in tests skipped in `testing.Short()`/without env vars. |
| Flow | connect-record-stop-summarize-ask failures/success | Fakes for recorder, provider, credentials, config, context. |

## Migration / Rollout

No migration required. Roll out as phased first slice: scaffold module and tests, then TUI shell, stores, fake flow, OpenAI seam, ffmpeg mic adapter. System audio, browser login, stricter real-time translation, and additional providers stay deferred behind interfaces.

## Resolved Decisions

- [x] Default transcription model: `gpt-4o-transcribe`.
- [x] Default chat/summarization model: `gpt-4o-mini`.
- [x] MVP credential storage: local `auth.json` outside the repo; macOS Keychain remains a future adapter.

## Open Questions

None for this foundation change.
