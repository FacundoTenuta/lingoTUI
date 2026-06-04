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
	ErrRealtimeInactive  = errors.New("realtime translation is not active")
)

type Dependencies struct {
	Recorder       Recorder
	ChunkRecorder  ChunkRecorder
	Transcriber    Transcriber
	Chat           Chat
	Config         ConfigStore
	Credentials    CredentialStore
	Context        ContextStore
	SetupGuidance  []string
	CodexCLIStatus func(context.Context) ConnectionOption
}

type Service struct {
	deps      Dependencies
	connected bool
	recording bool
	realtime  bool
	chunks    []RealtimeChunk
}

type Result struct {
	Command      CommandKind
	Message      string
	Models       []ModelRef
	Context      RecentContext
	Answer       Answer
	Translations Translations
	Help         []HelpEntry
	Guidance     []string
	Connections  []ConnectionOption
	Connected    bool
	Recording    bool
	Realtime     bool
	Chunks       []RealtimeChunk
}

type ConnectionOption struct {
	Target  ConnectionTarget
	Label   string
	Status  string
	Message string
	Ready   bool
}

func NewService(deps Dependencies) *Service { return &Service{deps: deps} }

func (s *Service) HandleInput(ctx context.Context, input string) (Result, error) {
	cmd, err := ParseCommand(input)
	if err != nil {
		return Result{Command: cmd.Kind, Connected: s.connected, Recording: s.recording, Realtime: s.realtime}, err
	}
	switch cmd.Kind {
	case CommandEmpty:
		return s.result(CommandEmpty, ""), nil
	case CommandConnect:
		switch cmd.Connection {
		case ConnectionTargetOptions:
			return s.ConnectionOptions(ctx), nil
		case ConnectionTargetOpenAI:
			return s.Connect(ctx)
		case ConnectionTargetCodex:
			return s.ConnectCodex(ctx), nil
		default:
			return s.result(cmd.Kind, ""), ErrUnsupportedConnection
		}
	case CommandModels:
		return s.Models(ctx)
	case CommandRecord:
		return s.Record(ctx, cmd.Source)
	case CommandStop:
		return s.Stop(ctx)
	case CommandAsk:
		return s.Ask(ctx, Question(cmd.Question))
	case CommandTranslate:
		return s.Translate(ctx, cmd.Text)
	case CommandRealtime:
		switch cmd.RealtimeAction {
		case RealtimeActionStart:
			return s.RealtimeStart(ctx, cmd.Source)
		case RealtimeActionChunk:
			return s.RealtimeChunk(ctx)
		case RealtimeActionStop:
			return s.RealtimeStop(ctx)
		default:
			return s.result(cmd.Kind, ""), ErrUnsupportedRealtime
		}
	case CommandClear:
		return s.Clear(ctx)
	case CommandHelp:
		return s.Help(ctx), nil
	default:
		return s.result(cmd.Kind, ""), ErrUnknownCommand
	}
}

func (s *Service) ConnectionOptions(ctx context.Context) Result {
	result := s.result(CommandConnect, "Choose a connection option: /connect openai for direct OpenAI, or /connect codex for Codex CLI setup/status guidance.")
	result.Connected = false
	result.Connections = []ConnectionOption{
		{Target: ConnectionTargetOpenAI, Label: "OpenAI/direct", Status: "available", Message: "Uses configured OpenAI credentials. This is the current chat/transcription provider path.", Ready: true},
		s.codexConnectionOption(ctx),
	}
	return result
}

func (s *Service) ConnectCodex(ctx context.Context) Result {
	option := s.codexConnectionOption(ctx)
	result := s.result(CommandConnect, "Codex CLI is an optional setup/status path. lingoTUI does not run chat through Codex CLI yet; use /connect openai for the supported direct provider check.")
	result.Connected = false
	result.Connections = []ConnectionOption{option}
	return result
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
	return s.result(CommandConnect, fmt.Sprintf("Runtime config is ready: transcription %s/%s; chat %s/%s. Provider calls happen only on /stop, /ask, /translate, or realtime chunks.", cfg.TranscriptionModel.Provider, cfg.TranscriptionModel.Name, cfg.ChatModel.Provider, cfg.ChatModel.Name)), nil
}

func (s *Service) codexConnectionOption(ctx context.Context) ConnectionOption {
	if s.deps.CodexCLIStatus != nil {
		option := s.deps.CodexCLIStatus(ctx)
		if option.Target == "" {
			option.Target = ConnectionTargetCodex
		}
		if strings.TrimSpace(option.Label) == "" {
			option.Label = "Codex CLI"
		}
		return option
	}
	return ConnectionOption{
		Target:  ConnectionTargetCodex,
		Label:   "Codex CLI",
		Status:  "unknown",
		Message: "Optional alternative not checked. Install codex and authenticate with Codex CLI; lingoTUI does not use it for chat yet.",
	}
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
	if s.realtime {
		return s.result(CommandRecord, ""), fmt.Errorf("%w: run /realtime stop before /record mic", ErrAlreadyRecording)
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

func (s *Service) RealtimeStart(ctx context.Context, source AudioSource) (Result, error) {
	if source != AudioSourceMic {
		return s.result(CommandRealtime, ""), ErrUnsupportedAudioSource
	}
	if s.recording {
		return s.result(CommandRealtime, ""), fmt.Errorf("%w: run /stop before starting realtime translation", ErrAlreadyRecording)
	}
	if s.realtime {
		return s.result(CommandRealtime, ""), fmt.Errorf("%w: run /realtime stop before starting another realtime session", ErrAlreadyRecording)
	}
	if s.deps.ChunkRecorder == nil {
		return s.result(CommandRealtime, ""), fmt.Errorf("%w: chunk recorder; realtime translation is not wired in this build", ErrNotConfigured)
	}
	if err := s.deps.ChunkRecorder.Start(ctx, source); err != nil {
		return s.result(CommandRealtime, ""), fmt.Errorf("start realtime recording: %w", err)
	}
	s.realtime = true
	s.chunks = nil
	return s.result(CommandRealtime, "Realtime translation started. Chunks will be translated on realtime ticks."), nil
}

func (s *Service) RealtimeChunk(ctx context.Context) (Result, error) {
	if !s.realtime {
		return s.result(CommandRealtime, ""), fmt.Errorf("%w: run /realtime start mic first", ErrRealtimeInactive)
	}
	if s.deps.ChunkRecorder == nil {
		return s.result(CommandRealtime, ""), fmt.Errorf("%w: chunk recorder", ErrNotConfigured)
	}
	file, ok, err := s.deps.ChunkRecorder.NextChunk(ctx)
	if err != nil {
		return s.result(CommandRealtime, ""), fmt.Errorf("read realtime chunk: %w", err)
	}
	if !ok {
		result := s.result(CommandRealtime, "No realtime chunk available yet.")
		result.Chunks = append([]RealtimeChunk(nil), s.chunks...)
		return result, nil
	}
	return s.processRealtimeChunk(ctx, file, "Translated realtime chunk.")
}

func (s *Service) RealtimeStop(ctx context.Context) (Result, error) {
	if !s.realtime {
		return s.result(CommandRealtime, ""), fmt.Errorf("%w: run /realtime start mic first", ErrRealtimeInactive)
	}
	if s.deps.ChunkRecorder == nil {
		return s.result(CommandRealtime, ""), fmt.Errorf("%w: chunk recorder", ErrNotConfigured)
	}
	files, err := s.deps.ChunkRecorder.Stop(ctx)
	s.realtime = false
	if err != nil {
		s.cleanupRealtimeSession(ctx)
		return s.result(CommandRealtime, ""), fmt.Errorf("stop realtime recording: %w", err)
	}
	if len(files) > 0 {
		result, err := s.processRealtimeChunks(ctx, files, "Stopped realtime translation and translated final chunks.")
		if err == nil {
			s.storeRealtimeContext()
		}
		s.cleanupRealtimeSession(ctx)
		return result, err
	}
	result := s.result(CommandRealtime, "Stopped realtime translation.")
	result.Chunks = append([]RealtimeChunk(nil), s.chunks...)
	s.storeRealtimeContext()
	s.cleanupRealtimeSession(ctx)
	return result, nil
}

func (s *Service) processRealtimeChunk(ctx context.Context, file AudioFile, message string) (Result, error) {
	result, err := s.processRealtimeChunks(ctx, []AudioFile{file}, message)
	s.cleanupRealtimeChunk(ctx, file)
	return result, err
}

func (s *Service) processRealtimeChunks(ctx context.Context, files []AudioFile, message string) (Result, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return s.result(CommandRealtime, ""), err
	}
	if s.deps.Transcriber == nil {
		return s.result(CommandRealtime, ""), fmt.Errorf("%w: transcriber; %s", ErrNotConfigured, missingTranscriberGuidance(cfg))
	}
	if s.deps.Chat == nil {
		return s.result(CommandRealtime, ""), fmt.Errorf("%w: chat; %s", ErrNotConfigured, missingChatGuidance(cfg))
	}
	for _, file := range files {
		transcript, err := s.deps.Transcriber.Transcribe(ctx, file, cfg.TranscriptionModel)
		if err != nil {
			return s.result(CommandRealtime, ""), fmt.Errorf("transcribe realtime chunk: %w", err)
		}
		translations, err := s.deps.Chat.Translate(ctx, transcript.Text, SummaryLanguages(), cfg.ChatModel)
		if err != nil {
			return s.result(CommandRealtime, ""), fmt.Errorf("translate realtime chunk: %w", err)
		}
		s.chunks = append(s.chunks, RealtimeChunk{Transcript: transcript, Translations: translations})
	}
	result := s.result(CommandRealtime, message)
	result.Chunks = append([]RealtimeChunk(nil), s.chunks...)
	return result, nil
}

func (s *Service) cleanupRealtimeChunk(ctx context.Context, file AudioFile) {
	cleaner, ok := s.deps.ChunkRecorder.(ChunkCleaner)
	if !ok {
		return
	}
	_ = cleaner.CleanupChunk(ctx, file)
}

func (s *Service) cleanupRealtimeSession(ctx context.Context) {
	cleaner, ok := s.deps.ChunkRecorder.(ChunkCleaner)
	if !ok {
		return
	}
	_ = cleaner.Cleanup(ctx)
}

func (s *Service) storeRealtimeContext() {
	if s.deps.Context == nil || len(s.chunks) == 0 {
		return
	}
	var transcriptParts []string
	translations := Summary{}
	for _, chunk := range s.chunks {
		if text := strings.TrimSpace(chunk.Transcript.Text); text != "" {
			transcriptParts = append(transcriptParts, text)
		}
		for _, language := range SummaryLanguages() {
			if text := strings.TrimSpace(chunk.Translations[language]); text != "" {
				if translations[language] != "" {
					translations[language] += "\n"
				}
				translations[language] += text
			}
		}
	}
	s.deps.Context.Replace(RecentContext{Transcript: Transcript{Text: strings.Join(transcriptParts, "\n")}, Summary: translations})
}

func (s *Service) Stop(ctx context.Context) (result Result, resultErr error) {
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
	defer func() {
		if err := s.cleanupRecording(ctx, file); err != nil && resultErr == nil {
			result = s.result(CommandStop, "")
			resultErr = fmt.Errorf("cleanup recording: %w", err)
		}
	}()
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
	result = s.result(CommandStop, "Processed recording and updated ES/EN/DE context.")
	result.Context = recent
	return result, nil
}

func (s *Service) cleanupRecording(ctx context.Context, file AudioFile) error {
	cleaner, ok := s.deps.Recorder.(RecordingCleaner)
	if !ok {
		return nil
	}
	return cleaner.Cleanup(ctx, file)
}

func missingTranscriberGuidance(cfg Config) string {
	if cfg.TranscriptionModel.Provider == ProviderLocalWhisper || cfg.Provider == ProviderLocalWhisper {
		return "localwhisper transcriber is not configured; set local_whisper.model_path before processing audio"
	}
	return "run lingotui login openai or configure auth.json fallback, then run /connect before processing audio"
}

func missingChatGuidance(cfg Config) string {
	if cfg.ChatModel.Provider == ProviderChatGPT || cfg.Provider == ProviderChatGPT {
		return "ChatGPT/Codex chat is not configured; run lingotui login chatgpt before /ask, /stop, or /translate"
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

func (s *Service) Translate(ctx context.Context, text string) (Result, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return s.result(CommandTranslate, ""), ErrMissingCommandArgument
	}
	if s.deps.Chat == nil {
		cfg, err := s.loadConfig(ctx)
		if err != nil {
			return s.result(CommandTranslate, ""), err
		}
		return s.result(CommandTranslate, ""), fmt.Errorf("%w: chat; %s", ErrNotConfigured, missingChatGuidance(cfg))
	}
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return s.result(CommandTranslate, ""), err
	}
	translations, err := s.deps.Chat.Translate(ctx, text, SummaryLanguages(), cfg.ChatModel)
	if err != nil {
		return s.result(CommandTranslate, ""), fmt.Errorf("translate text: %w", err)
	}
	result := s.result(CommandTranslate, "Translated text into ES/EN/DE.")
	result.Translations = translations
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
	return Result{Command: command, Message: message, Connected: s.connected, Recording: s.recording, Realtime: s.realtime}
}

func modelUsesProvider(cfg Config, provider ProviderID) bool {
	return cfg.TranscriptionModel.Provider == provider || cfg.ChatModel.Provider == provider
}
