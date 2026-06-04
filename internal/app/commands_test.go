package app

import (
	"errors"
	"strings"
	"testing"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		kind     CommandKind
		source   AudioSource
		question string
		text     string
		action   RealtimeAction
		connect  ConnectionTarget
		err      error
	}{
		{name: "empty input", input: "  ", kind: CommandEmpty},
		{name: "connect options", input: "/connect", kind: CommandConnect},
		{name: "connect openai", input: "/connect openai", kind: CommandConnect, connect: ConnectionTargetOpenAI},
		{name: "connect direct alias", input: "/connect direct", kind: CommandConnect, connect: ConnectionTargetOpenAI},
		{name: "connect codex", input: "/connect codex", kind: CommandConnect, connect: ConnectionTargetCodex},
		{name: "connect codex cli alias", input: "/connect codex-cli", kind: CommandConnect, connect: ConnectionTargetCodex},
		{name: "connect unsupported option", input: "/connect anthropic", kind: CommandConnect, connect: ConnectionTarget("anthropic"), err: ErrUnsupportedConnection},
		{name: "connect extra args", input: "/connect openai now", kind: CommandConnect, err: ErrUnsupportedConnection},
		{name: "models", input: "/models", kind: CommandModels},
		{name: "record mic", input: "/record mic", kind: CommandRecord, source: AudioSourceMic},
		{name: "unsupported system source", input: "/record system", kind: CommandRecord, source: AudioSourceSystem, err: ErrUnsupportedAudioSource},
		{name: "unsupported combined source", input: "/record both", kind: CommandRecord, source: AudioSourceBoth, err: ErrUnsupportedAudioSource},
		{name: "record missing source", input: "/record", kind: CommandRecord, err: ErrMissingCommandArgument},
		{name: "stop", input: "/stop", kind: CommandStop},
		{name: "ask with question", input: "/ask what happened?", kind: CommandAsk, question: "what happened?"},
		{name: "ask missing question", input: "/ask", kind: CommandAsk, err: ErrMissingCommandArgument},
		{name: "translate with text", input: "/translate hello", kind: CommandTranslate, text: "hello"},
		{name: "translate missing text", input: "/translate", kind: CommandTranslate, err: ErrMissingCommandArgument},
		{name: "realtime start shorthand", input: "/realtime start", kind: CommandRealtime, source: AudioSourceMic, action: RealtimeActionStart},
		{name: "realtime start mic", input: "/realtime start mic", kind: CommandRealtime, source: AudioSourceMic, action: RealtimeActionStart},
		{name: "realtime start unsupported source", input: "/realtime start system", kind: CommandRealtime, source: AudioSourceSystem, action: RealtimeActionStart, err: ErrUnsupportedAudioSource},
		{name: "realtime start with extra args", input: "/realtime start mic now", kind: CommandRealtime, action: RealtimeActionStart, err: ErrUnsupportedRealtime},
		{name: "realtime stop", input: "/realtime stop", kind: CommandRealtime, action: RealtimeActionStop},
		{name: "realtime missing action", input: "/realtime", kind: CommandRealtime, err: ErrMissingCommandArgument},
		{name: "realtime invalid action", input: "/realtime pause", kind: CommandRealtime, action: RealtimeAction("pause"), err: ErrUnsupportedRealtime},
		{name: "realtime stop with extra args", input: "/realtime stop mic", kind: CommandRealtime, action: RealtimeActionStop, err: ErrUnsupportedRealtime},
		{name: "clear", input: "/clear", kind: CommandClear},
		{name: "help", input: "/help", kind: CommandHelp},
		{name: "unknown slash command", input: "/wat", kind: CommandEmpty, err: ErrUnknownCommand},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := ParseCommand(tt.input)
			if !errors.Is(err, tt.err) {
				t.Fatalf("error = %v, want %v", err, tt.err)
			}
			if cmd.Kind != tt.kind || cmd.Source != tt.source || cmd.Question != tt.question || cmd.Text != tt.text || cmd.RealtimeAction != tt.action || cmd.Connection != tt.connect {
				t.Fatalf("command = %+v", cmd)
			}
		})
	}
}

func TestUnsupportedConnectionErrorMentionsSupportedOptions(t *testing.T) {
	_, err := ParseCommand("/connect anthropic")
	if !errors.Is(err, ErrUnsupportedConnection) {
		t.Fatalf("error = %v, want %v", err, ErrUnsupportedConnection)
	}
	for _, want := range []string{"/connect", "/connect openai", "/connect codex"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q: %v", want, err)
		}
	}
}

func TestHelpEntriesCoverSupportedCommands(t *testing.T) {
	entries := HelpEntries()
	if len(entries) != 12 {
		t.Fatalf("entries = %d, want 12", len(entries))
	}
	joined := ""
	for _, entry := range entries {
		if entry.Command == "" || entry.Description == "" {
			t.Fatalf("incomplete help entry: %+v", entry)
		}
		joined += entry.Command + "\n"
	}
	for _, want := range []string{"/connect openai", "/connect codex", "/translate <text>", "/ask <question>", "/stop", "/realtime start mic", "/realtime stop"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("help entries missing %q: %s", want, joined)
		}
	}
}

func TestDefaultSetupGuidanceCoversCredentialAndMicrophone(t *testing.T) {
	guidance := strings.Join(DefaultSetupGuidance(), "\n")
	for _, want := range []string{"lingotui login openai", "lingotui login chatgpt", "OpenAI remains the default", "chat_model.provider", "Codex CLI", "codex binary", "does not use Codex CLI for chat yet", "LocalWhisper", "whisper-cli", "local_whisper.binary_path", "macOS Keychain", "auth.json fallback", "/connect", "Microphone", "/record mic", "Provider calls happen only on /stop, /ask, /translate, or realtime chunks"} {
		if !strings.Contains(guidance, want) {
			t.Fatalf("guidance missing %q: %s", want, guidance)
		}
	}
}
