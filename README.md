# lingoTUI MVP foundation

lingoTUI is a terminal-first language survival assistant. The current MVP opens a safe Bubble Tea shell, shows first-run setup guidance, and keeps recording/OpenAI calls behind explicit commands.

## Quick path

1. Install Go and `ffmpeg`.
2. Install the command with `./install.sh`, or use the manual commands below.
3. Run `lingotui login openai` and paste your OpenAI API key when prompted.
4. Run `lingotui`; the first screen shows setup status and waits for your command.

## Install

### Installer script

From a local checkout, run the installer. By default it installs the latest published `lingotui` command with `go install`:

```sh
./install.sh
```

The installer runs `go install`, detects Go's binary directory, and adds it to your shell profile when it is missing from `PATH`.

To install a specific version:

```sh
LINGOTUI_VERSION=v0.1.2 ./install.sh
```

### Local checkout

```sh
go install ./cmd/lingotui
```

### Remote module

```sh
go install github.com/FacundoTenuta/lingoTUI/cmd/lingotui@latest
```

Go writes installed binaries to `$GOBIN` when set, otherwise to `$GOPATH/bin` (usually `~/go/bin`). If your shell cannot find `lingotui`, add that directory to `PATH`:

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
```

After installing, launch the TUI:

```sh
lingotui
```

Save your OpenAI API key to macOS Keychain before using OpenAI-backed flows:

```sh
lingotui login openai
```

`lingotui login` still defaults to the OpenAI API-key prompt for compatibility. `lingotui login chatgpt` opens the ChatGPT Plus/Pro browser OAuth flow and saves the resulting OAuth credential. ChatGPT/Codex runtime and `/connect` support are not enabled yet.

## Update

Update an installed `lingotui` binary with:

```sh
lingotui update
```

The command runs:

```sh
go install github.com/FacundoTenuta/lingoTUI/cmd/lingotui@latest
```

To update to a specific version, set `LINGOTUI_VERSION`:

```sh
LINGOTUI_VERSION=v0.1.2 lingotui update
```

## Version

Print the installed `lingotui` version with any of these forms:

```sh
lingotui version
lingotui -v
lingotui --version
```

## First-run onboarding

On startup, lingoTUI reads local config/credential status and renders setup guidance. Startup is intentionally quiet:

- No microphone recording starts.
- No ffmpeg process starts.
- No OpenAI or network request is made.
- Secret values are never printed.

Use `/help` inside the TUI for supported commands and setup reminders. The first useful sequence is:

1. Run `lingotui login openai` to save an OpenAI key to macOS Keychain, or configure the `auth.json` fallback for development.
2. Run `/connect` to mark the configured provider as connected.
3. Run `/record mic` only when you choose to start microphone recording.
4. Run `/stop` to stop recording and process the captured audio.

## Local files

| File | Default location | Purpose |
|------|------------------|---------|
| `config.json` | `${UserConfigDir}/lingotui/config.json` | Provider/model defaults. |
| `auth.json` | `${UserConfigDir}/lingotui/auth.json` | Development fallback typed credentials. |

`UserConfigDir` is provided by the OS. On macOS this is typically `~/Library/Application Support/lingotui/`.

macOS Keychain is the primary credential store. `auth.json` is kept as a fallback/development path when Keychain is unavailable or intentionally bypassed.

Example fallback `auth.json`:

```json
{
  "openai": {
    "provider": "openai",
    "kind": "api_key",
    "api_key": "sk-your-api-key"
  }
}
```

The old API-key-only format is still loadable for compatibility:

```json
{
  "openai": {
    "provider": "openai",
    "secret": "sk-your-api-key"
  }
}
```

OAuth credential records are used by `lingotui login chatgpt` to store ChatGPT Plus/Pro browser OAuth credentials. Codex runtime requests are not implemented yet. Access and refresh tokens are redacted by app types and must not be printed.

Secrets are loaded from Keychain first, then the `auth.json` fallback, and are redacted by the app types. Do not commit `auth.json`.

## Defaults

| Setting | Default |
|---------|---------|
| Provider | `openai` |
| Transcription model | `gpt-4o-transcribe` |
| Chat model | `gpt-4o-mini` |
| Credential storage | macOS Keychain primary; local `auth.json` fallback |
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

## ChatGPT Plus/Pro scaffold

ChatGPT Plus/Pro is registered as a separate future provider, but it intentionally exposes no usable provider auth methods or models yet. `lingotui login chatgpt` can complete browser OAuth login and save a credential; ChatGPT/Codex runtime, `/connect`, and provider model use remain unimplemented.

## Privacy and cost boundaries

- Recording is explicit: `/record mic` starts, `/stop` processes.
- Startup and onboarding do not start ffmpeg, request/record microphone audio, or call OpenAI.
- Recent transcript and summary context is in memory by default.
- Audio files are temporary by default.
- OpenAI API calls happen only through the OpenAI API-key provider adapter and are triggered by explicit user commands.
- ChatGPT Plus/Pro browser OAuth only runs from `lingotui login chatgpt`; ChatGPT/Codex runtime requests are not executed.
- Default tests do not make network calls or access audio devices.

## Deferred roadmap seams

- Robust system audio and combined microphone/system capture.
- Implement ChatGPT/Codex runtime requests after OAuth credential storage.
- Additional provider registry entries.
- Stricter real-time translation beyond the current record/process flow.
