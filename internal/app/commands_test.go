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
		err      error
	}{
		{name: "empty input", input: "  ", kind: CommandEmpty},
		{name: "connect", input: "/connect", kind: CommandConnect},
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
			if cmd.Kind != tt.kind || cmd.Source != tt.source || cmd.Question != tt.question || cmd.Text != tt.text {
				t.Fatalf("command = %+v", cmd)
			}
		})
	}
}

func TestHelpEntriesCoverSupportedCommands(t *testing.T) {
	entries := HelpEntries()
	if len(entries) != 8 {
		t.Fatalf("entries = %d, want 8", len(entries))
	}
	joined := ""
	for _, entry := range entries {
		if entry.Command == "" || entry.Description == "" {
			t.Fatalf("incomplete help entry: %+v", entry)
		}
		joined += entry.Command + "\n"
	}
	for _, want := range []string{"/translate <text>", "/ask <question>", "/stop"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("help entries missing %q: %s", want, joined)
		}
	}
}

func TestDefaultSetupGuidanceCoversCredentialAndMicrophone(t *testing.T) {
	guidance := strings.Join(DefaultSetupGuidance(), "\n")
	for _, want := range []string{"lingotui login openai", "lingotui login chatgpt", "OpenAI remains the default", "chat_model.provider", "LocalWhisper", "whisper-cli", "local_whisper.binary_path", "macOS Keychain", "auth.json fallback", "/connect", "Microphone", "/record mic", "Provider calls happen only on /stop, /ask, or /translate"} {
		if !strings.Contains(guidance, want) {
			t.Fatalf("guidance missing %q: %s", want, guidance)
		}
	}
}
