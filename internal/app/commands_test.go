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
			if cmd.Kind != tt.kind || cmd.Source != tt.source || cmd.Question != tt.question {
				t.Fatalf("command = %+v", cmd)
			}
		})
	}
}

func TestHelpEntriesCoverSupportedCommands(t *testing.T) {
	entries := HelpEntries()
	if len(entries) != 7 {
		t.Fatalf("entries = %d, want 7", len(entries))
	}
	for _, entry := range entries {
		if entry.Command == "" || entry.Description == "" {
			t.Fatalf("incomplete help entry: %+v", entry)
		}
	}
}

func TestDefaultSetupGuidanceCoversCredentialAndMicrophone(t *testing.T) {
	guidance := strings.Join(DefaultSetupGuidance(), "\n")
	for _, want := range []string{"lingotui login openai", "lingotui login chatgpt", "not implemented yet", "macOS Keychain", "auth.json fallback", "/connect", "Microphone", "/record mic"} {
		if !strings.Contains(guidance, want) {
			t.Fatalf("guidance missing %q: %s", want, guidance)
		}
	}
}
