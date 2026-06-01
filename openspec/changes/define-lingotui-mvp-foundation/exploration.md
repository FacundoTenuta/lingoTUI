# Exploration: define-lingotui-mvp-foundation

## Current State

The repository contains planning artifacts only: `PRD.md`, `TECHNICAL_DESIGN.md`, and hybrid SDD/OpenSpec configuration. There is no Go module, package manifest, implementation code, CI, linter, formatter, or test runner yet.

The current technical design should be treated as a hypothesis: Go + Bubble Tea + Lip Gloss, ffmpeg-first microphone recording, ports/interfaces, provider adapters, and OpenAI as the first provider. The PRD supports this direction but still lists the stack, default transcription model, credential security, ChatGPT Plus/Pro login viability, and live-vs-block recording as open questions.

Stack decisions absolutely belong in SDD, but at different levels by phase: proposal validates the strategic direction and scope, design records the concrete architecture and tradeoffs, and tasks turn that design into reviewable implementation slices.

## Affected Areas

- `PRD.md` — defines macOS-only MVP, microphone-first audio, OpenAI API key fallback, provider configurability, privacy defaults, and open stack questions.
- `TECHNICAL_DESIGN.md` — contains the preliminary stack and architecture hypothesis to validate before proposal/design hardening.
- `openspec/config.yaml` — sets hybrid artifact persistence, phase rules, MVP focus on macOS microphone capture, and port/adapter boundary constraints.
- `openspec/changes/define-lingotui-mvp-foundation/` — active SDD change folder for the foundation decision artifacts.

## Approaches

1. **Go + Bubble Tea + Lip Gloss** — single-binary TUI with ports/adapters and external ffmpeg audio capture.
   - Pros: strong terminal-app fit, simple distribution, good concurrency for recording/provider calls, clean Bubble Tea update/view model, straightforward unit testing around command parsing/use cases/TUI state, avoids Node/Python runtime management.
   - Cons: AI SDK ecosystem is less rich than TypeScript, native macOS audio may require extra work later, Bubble Tea apps need discipline to keep side effects out of the TUI model.
   - Effort: Medium.

2. **TypeScript + Ink** — React-style terminal UI with Node runtime and npm ecosystem.
   - Pros: excellent AI SDK/provider ecosystem, React mental model, Ink provides component-based CLI UI with flexbox-style layout, faster if the team strongly prefers TS/React.
   - Cons: distribution is heavier, runtime/package management is more fragile for a local CLI, audio capture still needs native/external tooling, React-style async state can complicate long-running recording/process flows.
   - Effort: Medium.

3. **Python + Textual** — Python TUI with rich widgets and rapid prototyping.
   - Pros: fast iteration, strong audio/prototyping ecosystem, Textual supports sophisticated cross-platform terminal UIs with a simple Python API and testing support.
   - Cons: packaging and dependency isolation are more annoying for end-user CLI distribution, long-term binary-style distribution is weaker, app may drift toward framework-heavy UI before core audio/provider seams are proven.
   - Effort: Low for prototype, Medium/High for polished distribution.

4. **Native macOS audio first** — implement AVFoundation/CoreAudio capture before using ffmpeg.
   - Pros: better control, fewer external dependencies, likely stronger long-term macOS integration.
   - Cons: higher first-slice complexity, delays product validation, system audio remains hard without virtual drivers or permissions complexity.
   - Effort: High.

## Recommendation

Use Go + Bubble Tea + Lip Gloss for the MVP foundation, with ffmpeg-first microphone capture and strict ports/adapters. This is the best default for a macOS terminal product that should be easy to install, test, and evolve.

Keep ffmpeg as an adapter, not a domain dependency. FFmpeg's AVFoundation input supports macOS device enumeration and audio-device selection, which makes it a pragmatic MVP path for microphone capture. Treat system audio as a later experimental capability requiring BlackHole/Loopback-style setup unless native capture research proves otherwise.

Keep provider abstraction from day one, but make OpenAI API-key auth the only committed MVP provider path. OpenAI-first is justified because the MVP needs both transcription and chat/summarization quality. Do not commit to ChatGPT Plus/Pro browser login in MVP; record it as a research/future capability because the PRD already flags viability and permission risk.

Decision placement:

- Proposal: commit to the problem scope and candidate foundation: macOS-only, microphone-first, Go/Bubble Tea, OpenAI API-key MVP, provider abstraction, privacy defaults, and system audio out of MVP.
- Spec: describe observable behavior only: connect provider, record mic, stop/process, summarize, ask with recent context, clear context, error handling, privacy expectations.
- Design: finalize stack rationale, interfaces, command/process flow, ffmpeg adapter strategy, provider boundaries, credential storage tradeoff, testing seams.
- Tasks: initialize Go module, create TUI shell, command parser, config/credential store, fake adapters, OpenAI adapter, ffmpeg recorder, vertical slice tests, manual macOS audio verification.

## Risks

- macOS microphone permission and ffmpeg device naming may vary by machine and must be manually verified early.
- System audio capture can easily explode MVP scope; keep it out of the first proposal unless explicitly marked experimental.
- Local credential file storage is expedient but weaker than Keychain; design must document path, permissions, and migration.
- Provider abstraction can be over-engineered; define only the interfaces required by OpenAI transcription/chat for the first vertical slice.
- ChatGPT Plus/Pro login may be unsupported or unstable for third-party CLI apps; do not promise it before research.
- No implementation/test harness exists yet; early tasks must establish Go tooling before feature slices.

## Ready for Proposal

Yes. The orchestrator should tell the user that SDD is exactly the right place to validate stack decisions: exploration validates the hypothesis, proposal commits to scope and direction, design locks the technical choices, and tasks split the work. Recommended next step: create the SDD proposal for `define-lingotui-mvp-foundation` using the Go/Bubble Tea/ffmpeg/OpenAI-first foundation while explicitly keeping system audio and ChatGPT Plus/Pro login out of the committed MVP.
