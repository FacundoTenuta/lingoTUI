# Design: Installable lingoTUI TUI Command

## Technical Approach

Replace the placeholder command with a small production runtime builder. `cmd/lingotui/main.go` creates local config and credential stores, in-memory context, status-only onboarding checks, safe provider/audio adapters, then runs a Bubble Tea `tui.Model`. Startup may read local files and compute guidance, but it MUST NOT call OpenAI or start ffmpeg; provider calls remain behind `/stop`/`/ask`, and recording remains behind `/record mic`.

## Architecture Decisions

| Option | Tradeoff | Decision |
|---|---|---|
| Build runtime in `main.go` | Simple, but hard to test if all inline | Use `cmd/lingotui/runtime.go` `buildRuntime` so `main` only handles errors and `tea.NewProgram(...).Run()`. |
| Onboarding in TUI only | Fast, but mixes file/permission details into UI | Add `internal/setup` status service; TUI receives rendered status lines only. Preserves port/adapter boundary. |
| Probe microphone by opening audio | Accurate, but violates safety | Use status-only macOS guidance/checker that never opens an audio stream; real `FFmpegRecorder.Start` stays explicit. |
| Create OpenAI client at startup when key exists | Loads secret but no network; missing key leaves providers absent | Accept: `openai.NewClient` has no network side effect. Commands surface guidance when providers/credentials are missing. |

## Data Flow

```text
lingotui
  └─ buildRuntime
     ├─ config.FileStore + credentials.FileStore ──→ setup.StatusService
     ├─ credentials.Load(openai) ──→ optional openai.Client (no request)
     ├─ audio.NewFFmpegRecorder ──→ app.Service explicit /record only
     └─ tui.NewModel(service, onboarding lines) ──→ Bubble Tea program

Startup: read config/auth + permission status → render onboarding → wait.
/connect: load credential → mark connected or show auth.json guidance; no provider request.
/record mic: start ffmpeg only now; missing permission shows actionable recorder error.
/stop or /ask: use provider only after explicit command.
```

## File Changes

| File | Action | Description |
|---|---|---|
| `cmd/lingotui/main.go` | Modify | Run Bubble Tea program from `buildRuntime`; print startup errors to stderr. |
| `cmd/lingotui/runtime.go` | Create | Compose stores, setup service, memory context, optional OpenAI client, ffmpeg recorder, and TUI model. |
| `cmd/lingotui/runtime_test.go` | Create | Smoke-test runtime composition with temp config dirs and no live provider/audio calls. |
| `internal/setup/status.go` | Create | Status DTOs and service for credential/config path and microphone readiness guidance. |
| `internal/setup/status_test.go` | Create | Unit tests for redacted credential status, missing setup, and no side effects. |
| `internal/audio/permission.go` | Create | Status-only microphone checker/guidance; no stream opening. |
| `internal/tui/model.go`, `view.go`, `update.go` | Modify | Accept initial onboarding messages; include setup guidance in `/help` output. |
| `internal/app/usecases.go`, `commands.go` | Modify | Improve missing credential/provider/recorder messages without invoking providers. |
| `README.md` | Modify | Document `go install ./cmd/lingotui`, remote install when repo path is valid, PATH, onboarding, `auth.json`, and safety boundaries. |

## Interfaces / Contracts

```go
package setup

type State string // "ready" | "missing" | "unknown"
type ItemStatus struct { Name, Path, Message string; State State; Secret bool }
type Status struct { Items []ItemStatus; Ready bool }

type CredentialStore interface { Load(context.Context, app.ProviderID) (app.Secret, error) }
type PathProvider interface { Path() string }
type AudioPermissionChecker interface { Microphone(context.Context) ItemStatus }

type Service struct { /* config path, credential path/store, audio checker */ }
func (s Service) Status(context.Context) Status
func RenderLines(Status) []string
```

Secret values are never included; only path plus configured/missing/unknown state is rendered.

## Testing Strategy

| Layer | What to Test | Approach |
|---|---|---|
| Unit | `setup.Status`, credential redaction, audio guidance | Table-driven tests with `t.TempDir()` and fake checkers. |
| TUI update | Initial onboarding, `/help`, missing setup messages | Direct `Model.Update()`/`View()` assertions; no `teatest` needed. |
| Runtime smoke | `buildRuntime` creates a model from real file stores | `cmd/lingotui` test with temp base dir; assert initial view and no command side effects. |
| Side effects | Startup/onboarding never records or calls provider/network | Counting fakes/checkers; assert recorder start and HTTP/provider calls stay zero until explicit commands. |

## Migration / Rollout

No data migration required. Existing `config.json` and `auth.json` formats remain. Roll out by documenting local and remote `go install`; rollback is reverting runtime/onboarding files to the current placeholder command.

## Open Questions

- [ ] Confirm the final GitHub module path for remote `go install github.com/FacundoTenuta/lingoTUI/cmd/lingotui@latest`.
