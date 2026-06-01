# lingoTUI MVP foundation

lingoTUI is a terminal-first language survival assistant. This MVP foundation provides the Go/Bubble Tea shell, app ports, OpenAI-first provider seam, ffmpeg microphone adapter, local config/credential stores, and opt-in integration tests.

## Quick path

1. Install Go and `ffmpeg`.
2. Run `go test ./...` to verify the default offline suite.
3. Create local credentials in `auth.json` before using OpenAI-backed flows.

## Local files

| File | Default location | Purpose |
|------|------------------|---------|
| `config.json` | `${UserConfigDir}/lingotui/config.json` | Provider/model defaults. |
| `auth.json` | `${UserConfigDir}/lingotui/auth.json` | Local API-key credentials. |

`UserConfigDir` is provided by the OS. On macOS this is typically `~/Library/Application Support/lingotui/`.

Example `auth.json`:

```json
{
  "openai": {
    "provider": "openai",
    "secret": "sk-your-api-key"
  }
}
```

Secrets are loaded from the local credential store and are redacted by the app types. Do not commit `auth.json`.

## Defaults

| Setting | Default |
|---------|---------|
| Provider | `openai` |
| Transcription model | `gpt-4o-transcribe` |
| Chat model | `gpt-4o-mini` |
| Credential storage | local `auth.json` first; macOS Keychain later |
| Audio source | microphone via ffmpeg; system/both are deferred seams |

## ffmpeg microphone adapter

The current live recorder is macOS `avfoundation` first. Set the microphone device explicitly for integration tests:

```sh
LINGOTUI_FFMPEG_MIC_DEVICE=0 go test ./internal/audio -run Integration
```

Use `ffmpeg -f avfoundation -list_devices true -i ""` to inspect available macOS devices. The app writes captured audio to a temporary file and returns that path for processing; persistent audio storage is not enabled by default.

## OpenAI adapter

Live provider tests are opt-in and never run during default `go test ./...` without credentials:

```sh
LINGOTUI_OPENAI_API_KEY=sk-... go test ./internal/provider/openai -run Integration
```

Live transcription is additionally gated because it sends an audio fixture to OpenAI:

```sh
LINGOTUI_OPENAI_API_KEY=sk-... LINGOTUI_OPENAI_TRANSCRIBE=1 go test ./internal/provider/openai -run Integration
```

Provider calls may incur OpenAI costs. The adapter avoids logging request headers and redacts the configured API key from provider error messages.

## Privacy and cost boundaries

- Recording is explicit: `/record mic` starts, `/stop` processes.
- Recent transcript and summary context is in memory by default.
- Audio files are temporary by default.
- OpenAI API calls happen only through the provider adapter and require an API key.
- Default tests do not make network calls or access audio devices.

## Runtime composition status

The foundation keeps TUI/use cases independent from provider and audio implementation details. The real OpenAI and ffmpeg adapters exist behind the app interfaces; full CLI runtime wiring is intentionally deferred until the next runtime-composition slice so startup can handle missing credentials/devices without surprising network or microphone access.

## Deferred roadmap seams

- macOS Keychain credential storage.
- Robust system audio and combined microphone/system capture.
- Browser-auth research for ChatGPT Plus/Pro if viable.
- Additional provider registry entries.
- Stricter real-time translation beyond the current record/process flow.
