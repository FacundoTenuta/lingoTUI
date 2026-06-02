package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrMissingCredential = errors.New("missing provider credential")
	ErrNoRecentContext   = errors.New("no recent context")
	ErrNotConfigured     = errors.New("dependency is not configured")
	ErrAlreadyRecording  = errors.New("recording already active")
	ErrNotRecording      = errors.New("no active recording")
)

type Dependencies struct {
	Recorder      Recorder
	Transcriber   Transcriber
	Chat          Chat
	Config        ConfigStore
	Credentials   CredentialStore
	Context       ContextStore
	SetupGuidance []string
}

type Service struct {
	deps      Dependencies
	connected bool
	recording bool
}

type Result struct {
	Command   CommandKind
	Message   string
	Models    []ModelRef
	Context   RecentContext
	Answer    Answer
	Help      []HelpEntry
	Guidance  []string
	Connected bool
	Recording bool
}

func NewService(deps Dependencies) *Service { return &Service{deps: deps} }

func (s *Service) HandleInput(ctx context.Context, input string) (Result, error) {
	cmd, err := ParseCommand(input)
	if err != nil {
		return Result{Command: cmd.Kind, Connected: s.connected, Recording: s.recording}, err
	}
	switch cmd.Kind {
	case CommandEmpty:
		return s.result(CommandEmpty, ""), nil
	case CommandConnect:
		return s.Connect(ctx)
	case CommandModels:
		return s.Models(ctx)
	case CommandRecord:
		return s.Record(ctx, cmd.Source)
	case CommandStop:
		return s.Stop(ctx)
	case CommandAsk:
		return s.Ask(ctx, Question(cmd.Question))
	case CommandClear:
		return s.Clear(ctx)
	case CommandHelp:
		return s.Help(ctx), nil
	default:
		return s.result(cmd.Kind, ""), ErrUnknownCommand
	}
}

func (s *Service) Connect(ctx context.Context) (Result, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return s.result(CommandConnect, ""), err
	}
	if reason := UnsupportedRuntimeReason(cfg); reason != "" {
		return s.result(CommandConnect, ""), fmt.Errorf("%w: %s", ErrNotConfigured, reason)
	}
	if modelUsesProvider(cfg, ProviderOpenAI) {
		if s.deps.Credentials == nil {
			return s.result(CommandConnect, ""), fmt.Errorf("%w: credential store; run lingotui login openai or configure auth.json fallback before /connect", ErrNotConfigured)
		}
		secret, err := s.deps.Credentials.Load(ctx, ProviderOpenAI)
		if err != nil || secret.Empty() {
			if err != nil {
				return s.result(CommandConnect, ""), fmt.Errorf("%w: run lingotui login openai or configure %s in auth.json fallback before /connect: %v", ErrMissingCredential, ProviderOpenAI, err)
			}
			return s.result(CommandConnect, ""), fmt.Errorf("%w: run lingotui login openai or configure %s in auth.json fallback before /connect", ErrMissingCredential, ProviderOpenAI)
		}
	}
	if cfg.ChatModel.Provider == ProviderChatGPT {
		if s.deps.Credentials == nil {
			return s.result(CommandConnect, ""), fmt.Errorf("%w: typed OAuth credential store; run lingotui login chatgpt before /connect", ErrNotConfigured)
		}
		authStore, ok := s.deps.Credentials.(AuthCredentialStore)
		if !ok {
			return s.result(CommandConnect, ""), fmt.Errorf("%w: typed OAuth credential store; run lingotui login chatgpt before /connect", ErrNotConfigured)
		}
		credential, err := authStore.LoadCredential(ctx, ProviderChatGPT, CredentialKindOAuth)
		if err != nil || credential.Provider != ProviderChatGPT || credential.Kind != CredentialKindOAuth || !credential.OAuth.CanProvideAccess(time.Now(), 0) {
			return s.result(CommandConnect, ""), fmt.Errorf("%w: run lingotui login chatgpt before /connect", ErrMissingCredential)
		}
	}
	if cfg.TranscriptionModel.Provider == ProviderLocalWhisper && strings.TrimSpace(cfg.LocalWhisper.ModelPath) == "" {
		return s.result(CommandConnect, ""), fmt.Errorf("%w: local_whisper.model_path is required before /connect", ErrNotConfigured)
	}
	s.connected = true
	return s.result(CommandConnect, fmt.Sprintf("Runtime config is ready: transcription %s/%s; chat %s/%s. Provider calls happen only on /stop or /ask.", cfg.TranscriptionModel.Provider, cfg.TranscriptionModel.Name, cfg.ChatModel.Provider, cfg.ChatModel.Name)), nil
}

func (s *Service) Models(ctx context.Context) (Result, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return s.result(CommandModels, ""), err
	}
	models := []ModelRef{cfg.TranscriptionModel, cfg.ChatModel}
	result := s.result(CommandModels, fmt.Sprintf("Configured transcription: %s/%s; chat: %s/%s. Mixed providers use configured refs; provider registry listing is not required.", cfg.TranscriptionModel.Provider, cfg.TranscriptionModel.Name, cfg.ChatModel.Provider, cfg.ChatModel.Name))
	result.Models = models
	return result, nil
}

func (s *Service) Record(ctx context.Context, source AudioSource) (Result, error) {
	if source != AudioSourceMic {
		return s.result(CommandRecord, ""), ErrUnsupportedAudioSource
	}
	if s.recording {
		return s.result(CommandRecord, ""), fmt.Errorf("%w: run /stop before starting another recording", ErrAlreadyRecording)
	}
	if s.deps.Recorder == nil {
		return s.result(CommandRecord, ""), fmt.Errorf("%w: recorder; /record mic is unavailable until microphone recording is configured", ErrNotConfigured)
	}
	if err := s.deps.Recorder.Start(ctx, source); err != nil {
		return s.result(CommandRecord, ""), fmt.Errorf("start recording: %w", err)
	}
	s.recording = true
	return s.result(CommandRecord, "Recording microphone audio. Run /stop to process it."), nil
}

func (s *Service) Stop(ctx context.Context) (Result, error) {
	if !s.recording {
		return s.result(CommandStop, ""), fmt.Errorf("%w: run /record mic before /stop", ErrNotRecording)
	}
	if s.deps.Recorder == nil {
		return s.result(CommandStop, ""), fmt.Errorf("%w: recorder", ErrNotConfigured)
	}
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return s.result(CommandStop, ""), err
	}
	file, err := s.deps.Recorder.Stop(ctx)
	s.recording = false
	if err != nil {
		return s.result(CommandStop, ""), fmt.Errorf("stop recording: %w", err)
	}
	if s.deps.Transcriber == nil {
		return s.result(CommandStop, ""), fmt.Errorf("%w: transcriber; %s", ErrNotConfigured, missingTranscriberGuidance(cfg))
	}
	if s.deps.Chat == nil {
		return s.result(CommandStop, ""), fmt.Errorf("%w: chat; %s", ErrNotConfigured, missingChatGuidance(cfg))
	}
	if s.deps.Context == nil {
		return s.result(CommandStop, ""), fmt.Errorf("%w: context store", ErrNotConfigured)
	}
	transcript, err := s.deps.Transcriber.Transcribe(ctx, file, cfg.TranscriptionModel)
	if err != nil {
		return s.result(CommandStop, ""), fmt.Errorf("transcribe audio: %w", err)
	}
	summary, err := s.deps.Chat.Summarize(ctx, transcript, SummaryLanguages(), cfg.ChatModel)
	if err != nil {
		return s.result(CommandStop, ""), fmt.Errorf("summarize transcript: %w", err)
	}
	recent := RecentContext{Transcript: transcript, Summary: summary}
	s.deps.Context.Replace(recent)
	result := s.result(CommandStop, "Processed recording and updated ES/EN/DE context.")
	result.Context = recent
	return result, nil
}

func missingTranscriberGuidance(cfg Config) string {
	if cfg.TranscriptionModel.Provider == ProviderLocalWhisper || cfg.Provider == ProviderLocalWhisper {
		return "localwhisper transcriber is not configured; set local_whisper.model_path before processing audio"
	}
	return "run lingotui login openai or configure auth.json fallback, then run /connect before processing audio"
}

func missingChatGuidance(cfg Config) string {
	if cfg.ChatModel.Provider == ProviderChatGPT || cfg.Provider == ProviderChatGPT {
		return "ChatGPT/Codex chat is not configured; run lingotui login chatgpt before /ask or /stop"
	}
	return "run lingotui login openai or configure auth.json fallback, then run /connect before processing audio"
}

func (s *Service) Ask(ctx context.Context, question Question) (Result, error) {
	if strings.TrimSpace(string(question)) == "" {
		return s.result(CommandAsk, ""), ErrMissingCommandArgument
	}
	if s.deps.Context == nil {
		return s.result(CommandAsk, ""), fmt.Errorf("%w: context store", ErrNotConfigured)
	}
	if s.deps.Chat == nil {
		cfg, err := s.loadConfig(ctx)
		if err != nil {
			return s.result(CommandAsk, ""), err
		}
		return s.result(CommandAsk, ""), fmt.Errorf("%w: chat; %s", ErrNotConfigured, missingChatGuidance(cfg))
	}
	recent, ok := s.deps.Context.Current()
	if !ok {
		return s.result(CommandAsk, ""), fmt.Errorf("%w: record and stop audio before asking a follow-up question", ErrNoRecentContext)
	}
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return s.result(CommandAsk, ""), err
	}
	answer, err := s.deps.Chat.Answer(ctx, question, recent, cfg.ChatModel)
	if err != nil {
		return s.result(CommandAsk, ""), fmt.Errorf("answer question: %w", err)
	}
	result := s.result(CommandAsk, string(answer))
	result.Answer = answer
	result.Context = recent
	return result, nil
}

func (s *Service) Clear(context.Context) (Result, error) {
	if s.deps.Context == nil {
		return s.result(CommandClear, ""), fmt.Errorf("%w: context store", ErrNotConfigured)
	}
	s.deps.Context.Clear()
	return s.result(CommandClear, "Cleared in-memory transcript and summary context."), nil
}

func (s *Service) Help(context.Context) Result {
	result := s.result(CommandHelp, "Supported commands:")
	result.Help = HelpEntries()
	if s.deps.SetupGuidance == nil {
		result.Guidance = DefaultSetupGuidance()
	} else {
		result.Guidance = append([]string(nil), s.deps.SetupGuidance...)
	}
	return result
}

func (s *Service) loadConfig(ctx context.Context) (Config, error) {
	if s.deps.Config == nil {
		return DefaultConfig(), nil
	}
	cfg, err := s.deps.Config.Load(ctx)
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	return NormalizeConfig(cfg), nil
}

func (s *Service) result(command CommandKind, message string) Result {
	return Result{Command: command, Message: message, Connected: s.connected, Recording: s.recording}
}

func modelUsesProvider(cfg Config, provider ProviderID) bool {
	return cfg.TranscriptionModel.Provider == provider || cfg.ChatModel.Provider == provider
}
