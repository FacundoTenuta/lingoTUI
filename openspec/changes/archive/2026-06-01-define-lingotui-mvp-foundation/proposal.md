# Proposal: Define lingoTUI MVP Foundation

## Intent

Establish lingoTUI's MVP foundation without shrinking the product vision: Go + Bubble Tea + Lip Gloss, microphone-first recording, OpenAI API-key auth, provider/audio seams, privacy boundaries, and testable structure that can grow into system audio, browser login, stricter real-time translation, and multiple providers.

## Scope

### Product / Roadmap Scope
- Support language survival workflows for recorded conversations, meetings, and ambient audio.
- Preserve capability seams for robust system audio, ChatGPT Plus/Pro browser login if viable, stricter real-time translation, and multiple providers.

### First Implementation Slice
- Initialize a Go module and Bubble Tea TUI with Lip Gloss boundaries.
- Add a command shell for `/connect`, `/models`, `/record mic`, `/stop`, `/ask`, `/clear`, and `/help`.
- Define config/credential storage boundaries outside the project.
- Create provider interfaces with OpenAI API-key auth first.
- Use an ffmpeg-first microphone path with temporary audio handling.
- Establish Go tests with fake recorder/provider/config adapters.

### Deferred From First Implementation Slice
- Robust system audio capture implementation.
- ChatGPT Plus/Pro browser login research/implementation.
- Strict real-time translation mode beyond block recording/process flow.
- Full provider implementations beyond the minimal OpenAI-first seam.

## Capabilities

### New Capabilities
- `tui-command-shell`: command parsing, shell state, help, and errors.
- `provider-configuration`: OpenAI API-key connection now; model, auth, and provider seams for future expansion.
- `audio-capture`: ffmpeg-backed microphone capture now; system-audio source seam for later.
- `language-context-flow`: transcript, multilingual summary, follow-up questions, `/clear`, and future real-time mode seam.
- `test-harness-foundation`: Go tests with fake command, provider, storage, and recorder seams.

### Modified Capabilities
- None. `openspec/specs/` has no existing capabilities.

## Approach

Implement ports/adapters: TUI → command router → use cases → interfaces → adapters. Keep ffmpeg, OpenAI, audio sources, auth, config, and credentials behind interfaces so the first slice is small but roadmap capabilities are not designed out.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `cmd/lingotui/` | New | CLI entrypoint. |
| `internal/tui/`, `internal/app/` | New | Bubble Tea model, router, use cases. |
| `internal/provider/`, `internal/audio/` | New | Provider/auth and audio-source ports/adapters. |
| `internal/config/`, `internal/credentials/` | New | Storage boundaries. |
| `openspec/specs/` | New | Specs for the capabilities above. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| ffmpeg device naming/permissions vary on macOS | Medium | Verify early and surface actionable errors. |
| Audio/provider calls affect privacy and cost | Medium | Show recording/provider state; avoid persistence by default. |
| Roadmap seams over-engineering | Medium | Define interfaces from first-slice needs; document future extension points. |

## Rollback Plan

Revert the change folder and new Go module/source files. No data migration is planned; rollback is file deletion plus local test/config scaffold removal.

## Dependencies

- Go toolchain, Bubble Tea, Lip Gloss, ffmpeg, OpenAI API key.

## Success Criteria

- [ ] Specs can be written for every listed capability.
- [ ] Design can map each capability to ports/adapters and storage boundaries.
- [ ] Tasks can split implementation into reviewable slices under 400 lines where practical.
- [ ] Proposal clearly separates product roadmap capabilities from first-slice commitments.
