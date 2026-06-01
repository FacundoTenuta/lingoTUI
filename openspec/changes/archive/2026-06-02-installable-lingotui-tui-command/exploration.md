## Exploration: installable-lingotui-tui-command

### Current State
The app can be tested today with `go test ./...` and can be run with `go run ./cmd/lingotui`, but the current command is only a safe placeholder. It prints the OS config and credential file paths and does not start Bubble Tea. A real TUI model, use-case service, file config/credential stores, in-memory context store, OpenAI adapter, and ffmpeg macOS microphone recorder already exist behind ports. The missing piece is runtime composition in `cmd/lingotui/main.go`: creating dependencies, choosing safe/live mode, and launching `tea.NewProgram(tui.NewModel(service))`.

`go install github.com/FacundoTenuta/lingoTUI/cmd/lingotui@latest` should already build a binary named `lingotui` once consumers have the Go bin directory on `PATH`, because the command package is under `cmd/lingotui` and the module is public. However, the installed binary currently opens the placeholder output, not the TUI.

### Affected Areas
- `cmd/lingotui/main.go` — replace placeholder with installable runtime entrypoint and Bubble Tea program startup.
- `internal/tui/*` — already supports command input and view rendering; may need startup/status messages for safe/local mode and credential guidance.
- `internal/app/usecases.go` — already handles `/connect`, `/models`, `/record mic`, `/stop`, `/ask`, `/clear`, `/help`; runtime wiring must avoid nil dependencies in the default interactive path.
- `internal/config/*` — already resolves `${UserConfigDir}/lingotui/config.json`; install docs should tell users where config lives.
- `internal/credentials/*` — already uses local `auth.json`; UX needs a safe way to create/update credentials without hand-editing JSON being the only path.
- `internal/context/memory.go` — suitable default runtime context store.
- `internal/provider/openai/*` — live provider exists but requires an API key and may incur cost; should be opt-in through configured credentials/live mode.
- `internal/audio/*` — live ffmpeg recorder exists but is macOS microphone-only and can fail on missing ffmpeg, device, or permissions.
- `internal/testutil/fakes.go` — useful for a demo/safe runtime profile or CLI composition tests, though production code should not import `internal/testutil`.
- `README.md` — needs an install/run quick path, PATH note, config/auth commands, and safe vs live behavior.
- `openspec/specs/tui-command-shell/spec.md` — affected because `lingotui` should launch the shell directly.
- `openspec/specs/provider-configuration/spec.md` — affected by credential UX and optional live mode.
- `openspec/specs/audio-capture/spec.md` — affected if live mode wires ffmpeg recording.
- `openspec/specs/language-context-flow/spec.md` — affected if the default TUI can process fake or live transcript flows.
- `openspec/specs/test-harness-foundation/spec.md` — affected by new command/runtime composition test coverage.
- New capability likely needed: `installable-command-runtime` — install/run behavior, PATH expectations, startup mode, and no-surprise side-effect guarantees.

### Approaches
1. **Installable safe TUI first** — Make `lingotui` launch the Bubble Tea shell with local-safe dependencies that do not touch microphone/network by default; provide clear commands/docs for credentials and future live enablement.
   - Pros: Immediate user can run `lingotui` and see the product shape; no surprise OpenAI cost, microphone prompt, or ffmpeg failure on startup; smaller review; easy deterministic tests.
   - Cons: `/record mic` and `/stop` need either friendly “live mode not enabled” errors or fake/demo behavior; not yet the full live product.
   - Effort: Medium

2. **Wire live OpenAI + ffmpeg immediately** — Compose file stores, in-memory context, OpenAI client from `auth.json`, and ffmpeg recorder in the default `lingotui` command.
   - Pros: Closest to real MVP flow; exercises existing adapters end-to-end from the installed command.
   - Cons: Higher risk: missing credentials, ffmpeg, macOS permissions, provider costs, audio-device differences, and startup/command error handling all land in one slice.
   - Effort: High

3. **Dual-mode runtime** — Default to safe TUI startup, and allow explicit live mode via a flag/env/config such as `lingotui --live` or `LINGOTUI_LIVE=1`; add `lingotui config paths` / `lingotui auth set openai` style helpers later or in a separate slice.
   - Pros: Preserves safety while creating a direct path to real dependencies; clean migration path; can keep first PR under review budget if CLI helpers are scoped tightly.
   - Cons: Requires a small CLI argument layer before Bubble Tea; if over-scoped with full auth management it may exceed 400 changed lines.
   - Effort: Medium/High

### Recommendation
Use **Approach 1 for the first installable slice**, with one deliberate extension from Approach 3: define the live-mode contract in specs/docs but do not wire full live OpenAI/ffmpeg as the default startup behavior yet. The first implementation should make `go install .../cmd/lingotui@latest` and local `go install ./cmd/lingotui` produce a `lingotui` binary that opens the TUI immediately. It should compose safe runtime dependencies from production packages, not `internal/testutil`, so no test-only package leaks into the binary.

The credential UX should start minimal and explicit: README documents `auth.json` path and format, `/connect` reports missing credentials actionably, and a follow-up change can add `lingotui auth set openai` if we want to avoid manual JSON editing. Live OpenAI/ffmpeg wiring should be a second review slice or separate change because it introduces privacy/cost/device failure modes.

Estimated review size: safe installable TUI + docs + tests is likely under 400 changed lines. Adding live mode, credential-writing subcommands, and ffmpeg/OpenAI composition in the same PR risks exceeding 400 lines and cognitive budget. Chained PRs are not required for the safe-first slice, but are recommended if live mode or auth subcommands are included.

### Risks
- Bubble Tea key handling is currently simple (`msg.String()` appends raw key strings); real interactive typing may need refinement after launching the actual program.
- A fake/demo runtime must not be confused with live transcription; messaging needs to be explicit.
- Importing `internal/testutil` from `cmd/lingotui` would be a production/test boundary smell; create small production no-op/demo adapters if needed.
- `go install ...@latest` depends on users having `$GOBIN`/`$GOPATH/bin` on `PATH` and remote module tags/versions if we want stable installs.
- Live mode can trigger network cost, microphone permission prompts, ffmpeg availability errors, and macOS-only behavior.

### Ready for Proposal
Yes — propose an installable safe-first TUI command. Tell the user it can be tried today only as a placeholder (`go run ./cmd/lingotui`), while the next change should make `lingotui` open the TUI directly after `go install`. Keep live OpenAI/ffmpeg runtime wiring either explicit opt-in or a follow-up slice.
