# Tasks: Installable lingoTUI TUI Command

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 500-750 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 setup/status + TUI guidance → PR 2 runtime wiring + docs/smoke |
| Delivery strategy | ask-on-risk / ask-always |
| Chain strategy | pending |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Setup status contracts, audio guidance, TUI/help onboarding | PR 1 | Includes unit/TUI tests; no runtime switch yet. |
| 2 | `lingotui` runtime builder, Bubble Tea launch, install docs | PR 2 | Depends on PR 1; includes smoke/no-side-effect tests. |

## Phase 1: Setup Contracts

- [x] 1.1 Create `internal/setup/status.go` with `State`, `ItemStatus`, `Status`, `Service`, and `RenderLines` contracts from the design.
- [x] 1.2 Add credential/config path adapters using `Path()` from `internal/config.FileStore` and `internal/credentials.FileStore`; render only redacted OpenAI status.
- [x] 1.3 Create `internal/audio/permission.go` with macOS microphone readiness guidance/checking that never opens an audio stream.

## Phase 2: TUI and App Guidance

- [x] 2.1 Modify `internal/tui/model.go` and `view.go` so `NewModel` accepts initial onboarding lines and shows setup status on first render.
- [x] 2.2 Modify `internal/tui/update.go` and `internal/app/commands.go` so `/help` includes credential and microphone setup guidance when incomplete.
- [x] 2.3 Update `internal/app/usecases.go` missing credential/provider/recorder messages to mention `auth.json`, `/connect`, and `/record mic` without invoking providers.

## Phase 3: Runtime Wiring

- [ ] 3.1 Create `cmd/lingotui/runtime.go` with `buildRuntime(baseDir string)` composing config, credentials, setup service, memory context, OpenAI adapter, ffmpeg recorder, and `tui.Model`.
- [ ] 3.2 Modify `cmd/lingotui/main.go` so installed `lingotui` runs `tea.NewProgram(model).Run()` and prints startup errors to stderr.

## Phase 4: Tests

- [x] 4.1 Add `internal/setup/status_test.go` table tests for missing/ready/redacted OpenAI credential status and setup-only audio guidance.
- [x] 4.2 Update `internal/tui/model_test.go` for startup onboarding, `/help`, and missing setup messages via direct `Model.Update()` assertions.
- [ ] 4.3 Add `cmd/lingotui/runtime_test.go` smoke/no-side-effect tests with `t.TempDir()`, asserting startup performs no recorder/provider/network calls.
- [x] 4.4 Run `gofmt` and `go test ./...`.

## Phase 5: Documentation

- [ ] 5.1 Update `README.md` with local install, remote `go install github.com/FacundoTenuta/lingoTUI/cmd/lingotui@latest`, PATH, onboarding, `auth.json`, and safety boundaries.
