package app

import (
	"errors"
	"strings"
)

var (
	ErrUnknownCommand         = errors.New("unknown command")
	ErrMissingCommandArgument = errors.New("missing command argument")
	ErrUnsupportedAudioSource = errors.New("audio source is not available in this slice")
	ErrUnsupportedRealtime    = errors.New("unsupported realtime command")
)

type CommandKind string

const (
	CommandEmpty     CommandKind = "empty"
	CommandConnect   CommandKind = "connect"
	CommandModels    CommandKind = "models"
	CommandRecord    CommandKind = "record"
	CommandStop      CommandKind = "stop"
	CommandAsk       CommandKind = "ask"
	CommandTranslate CommandKind = "translate"
	CommandRealtime  CommandKind = "realtime"
	CommandClear     CommandKind = "clear"
	CommandHelp      CommandKind = "help"
)

type RealtimeAction string

const (
	RealtimeActionStart RealtimeAction = "start"
	RealtimeActionChunk RealtimeAction = "chunk"
	RealtimeActionStop  RealtimeAction = "stop"
)

type Command struct {
	Kind           CommandKind
	Raw            string
	Source         AudioSource
	Question       string
	Text           string
	RealtimeAction RealtimeAction
}

func ParseCommand(input string) (Command, error) {
	raw := strings.TrimSpace(input)
	cmd := Command{Kind: CommandEmpty, Raw: raw}
	if raw == "" {
		return cmd, nil
	}
	parts := strings.Fields(raw)
	switch parts[0] {
	case "/connect":
		cmd.Kind = CommandConnect
	case "/models":
		cmd.Kind = CommandModels
	case "/stop":
		cmd.Kind = CommandStop
	case "/clear":
		cmd.Kind = CommandClear
	case "/help":
		cmd.Kind = CommandHelp
	case "/ask":
		cmd.Kind = CommandAsk
		question := strings.TrimSpace(strings.TrimPrefix(raw, "/ask"))
		if question == "" {
			return cmd, ErrMissingCommandArgument
		}
		cmd.Question = question
	case "/translate":
		cmd.Kind = CommandTranslate
		text := strings.TrimSpace(strings.TrimPrefix(raw, "/translate"))
		if text == "" {
			return cmd, ErrMissingCommandArgument
		}
		cmd.Text = text
	case "/realtime":
		cmd.Kind = CommandRealtime
		if len(parts) < 2 {
			return cmd, ErrMissingCommandArgument
		}
		cmd.RealtimeAction = RealtimeAction(parts[1])
		switch cmd.RealtimeAction {
		case RealtimeActionStart:
			if len(parts) > 3 {
				return cmd, ErrUnsupportedRealtime
			}
			cmd.Source = AudioSourceMic
			if len(parts) >= 3 {
				cmd.Source = AudioSource(parts[2])
			}
			if cmd.Source != AudioSourceMic {
				return cmd, ErrUnsupportedAudioSource
			}
		case RealtimeActionChunk, RealtimeActionStop:
			if len(parts) != 2 {
				return cmd, ErrUnsupportedRealtime
			}
		default:
			return cmd, ErrUnsupportedRealtime
		}
	case "/record":
		cmd.Kind = CommandRecord
		if len(parts) < 2 {
			return cmd, ErrMissingCommandArgument
		}
		cmd.Source = AudioSource(parts[1])
		if cmd.Source != AudioSourceMic {
			return cmd, ErrUnsupportedAudioSource
		}
	default:
		return cmd, ErrUnknownCommand
	}
	return cmd, nil
}

type HelpEntry struct {
	Command     string
	Description string
}

func HelpEntries() []HelpEntry {
	return []HelpEntry{
		{"/connect", "check configured runtime credentials and local settings without calling providers"},
		{"/models", "show configured transcription and chat models"},
		{"/record mic", "start microphone recording"},
		{"/stop", "stop recording for processing"},
		{"/realtime start mic", "start chunked realtime translation from the microphone"},
		{"/realtime stop", "stop chunked realtime translation"},
		{"/ask <question>", "ask about the recent transcript and summary"},
		{"/translate <text>", "translate text into ES/EN/DE using the configured chat model"},
		{"/clear", "clear in-memory transcript and summary context"},
		{"/help", "show supported commands"},
	}
}

func DefaultSetupGuidance() []string {
	return []string{
		"OpenAI: run lingotui login openai to save your API key to macOS Keychain, or configure auth.json fallback for development, then run /connect. Secret values are never printed.",
		"ChatGPT Plus/Pro: run lingotui login chatgpt to complete browser OAuth login. OpenAI remains the default; set chat_model.provider to chatgpt manually for experimental ChatGPT/Codex chat. Provider calls happen only on /stop, /ask, /translate, or realtime chunks.",
		"LocalWhisper: install whisper-cli externally and configure local_whisper.binary_path and local_whisper.model_path for local transcription. Setup does not verify model files or execute whisper until /stop.",
		"Microphone: grant macOS microphone permission before /record mic. Recording starts only after that command.",
	}
}
