# Proposal: Installable lingoTUI TUI Command

## Intent

Make installed `lingotui` open the TUI and guide first-run audio/OpenAI setup, without surprise recording, network, or costs.

## Scope

### In Scope
- Safe `cmd/lingotui` runtime composition.
- First-run onboarding with setup status and next actions.
- macOS-oriented microphone/audio permission guidance or best-effort checks.
- OpenAI setup via existing local `auth.json`/credential store; no hardcoded secrets.
- README install, PATH, onboarding, and safety notes.

### Out of Scope
- Automatic recording, provider requests, or paid calls during startup/onboarding.
- Browser login, Keychain storage, multi-provider auth wizard.
- System audio capture beyond deferred seams.

## Capabilities

### New Capabilities
- `installable-command-runtime`: installed command behavior, install expectations, safe startup.
- `first-run-onboarding`: setup readiness, microphone/audio permission guidance, and OpenAI credential setup.

### Modified Capabilities
- `tui-command-shell`: launch shell plus onboarding/help messaging.
- `provider-configuration`: explain/use local `auth.json`; missing credentials never start provider requests.
- `audio-capture`: permission checks stay setup-only until explicit `/record mic`.
- `test-harness-foundation`: test runtime/onboarding without audio devices or paid calls.

## Approach

Replace placeholder `main.go` with a production runtime builder: config store, credential store, memory context, app service, and Bubble Tea model. First run shows onboarding: config/auth paths, OpenAI credential status, and macOS microphone permission guidance. Setup may inspect local files/permissions; recording and OpenAI calls require explicit user commands.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `cmd/lingotui/main.go` | Modified | Launch safe TUI runtime. |
| `internal/tui`, `internal/app` | Modified | Onboarding/status messages. |
| `internal/config`, `internal/credentials` | Modified | Local setup paths; no secret printing. |
| `internal/audio` | Modified | Permission guidance/check before recording. |
| `README.md` | Modified | Install/run/onboarding/safety notes. |
| `openspec/specs/*` | Modified/New | Runtime/onboarding/provider/audio deltas. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Onboarding seems live | Med | Separate checks/guidance from `/record mic` and provider commands. |
| Secrets exposed in docs/UI | Low | Show paths and redacted status only. |
| macOS permission variance | Med | Keep checks best effort and guidance actionable. |

## Rollback Plan

Revert `cmd/lingotui` to placeholder and remove onboarding/runtime README/spec additions. Existing foundation specs/adapters remain valid.

## Dependencies

- Go install tooling with `$GOBIN`/`$GOPATH/bin` on `PATH`.
- Existing Bubble Tea, config, credential, app, audio, provider packages.

## Success Criteria

- [ ] `go install` produces `lingotui` that opens the TUI.
- [ ] First-run onboarding covers audio permission and OpenAI setup.
- [ ] Startup/onboarding perform no default recording, network, or cost actions.
- [ ] Missing credentials/permissions produce actionable non-secret messages.
- [ ] README covers install, PATH, onboarding, credentials, and safety.
- [ ] `go test ./...` remains green with feasible runtime/onboarding coverage.
