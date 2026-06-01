# Tasks: Define lingoTUI MVP Foundation

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 650-900 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 scaffold/ports/stores -> PR 2 TUI/fake flow -> PR 3 OpenAI/ffmpeg/docs |
| Delivery strategy | ask-on-risk / ask-always |
| Chain strategy | pending |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Module, app ports, parser, config/auth stores, fakes | PR 1 | Autonomous foundation with unit tests; no live provider/audio. |
| 2 | Bubble Tea shell and fake vertical flow | PR 2 | Depends on Unit 1; verifies command, context, and error states. |
| 3 | OpenAI seam, ffmpeg mic adapter, docs/config notes | PR 3 | Depends on Unit 2; opt-in integrations only. |

## Phase 1: Scaffold and Boundaries

- [x] 1.1 Create `go.mod` and `cmd/lingotui/main.go` placeholder wiring; add Bubble Tea/Lip Gloss when the TUI imports them.
- [x] 1.2 Create `internal/app/types.go` and `internal/app/ports.go` with recorder, provider, config, credential, and context interfaces.
- [x] 1.3 Create `internal/app/commands.go` parser for `/connect`, `/models`, `/record mic`, `/stop`, `/ask`, `/clear`, `/help`, and unsupported sources.
- [x] 1.4 Create `internal/config/config.go`, `internal/config/file_store.go`, `internal/credentials/store.go`, and `internal/credentials/file_store.go`; store outside project dir and redact secrets.
- [x] 1.5 Create `internal/testutil/fakes.go` for recorder, provider, config, credentials, and context substitutes.

## Phase 2: Fakeable App Flow and TUI

- [x] 2.1 Create `internal/context/memory.go` for in-memory transcript, ES/EN/DE summary, answer context, and `/clear` behavior.
- [x] 2.2 Create `internal/app/usecases.go` routing connect, models, record, stop, ask, and clear through ports with configured defaults `gpt-4o-transcribe` and `gpt-4o-mini`.
- [x] 2.3 Create `internal/tui/model.go`, `internal/tui/update.go`, and `internal/tui/view.go`; delegate to app use cases and show actionable errors.

## Phase 3: Provider and Audio Adapters

- [x] 3.1 Create `internal/provider/registry.go` and `internal/provider/provider.go` with OpenAI-first model registry and future provider/browser-auth seams.
- [x] 3.2 Create `internal/provider/openai/client.go` for minimal API-key transcription/chat adapter; keep live calls behind interfaces and env-gated tests.
- [x] 3.3 Create `internal/audio/recorder.go` and `internal/audio/ffmpeg.go` ffmpeg-first mic adapter with temporary audio and clear unsupported system/both errors.

## Phase 4: Tests and Docs

- [x] 4.1 Add table-driven tests in `internal/app/commands_test.go`, `internal/app/usecases_test.go`, `internal/config/file_store_test.go`, and `internal/credentials/file_store_test.go` for command, provider, storage, context, and failure specs.
- [x] 4.2 Add `internal/tui/model_test.go` `Model.Update` tests using fakes for `/help`, unknown command, connect-record-stop-summarize-ask, `/clear`, and provider/recorder errors.
- [x] 4.3 Add opt-in `internal/audio/ffmpeg_integration_test.go` and `internal/provider/openai/client_integration_test.go`, skipped in `testing.Short()` or without env/config.
- [x] 4.4 Create `README.md` with credential path, `auth.json`, model defaults, ffmpeg requirement, privacy/cost boundaries, and deferred roadmap seams.
