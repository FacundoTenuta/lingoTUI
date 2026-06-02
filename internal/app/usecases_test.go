package app_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	. "github.com/FacundoTenuta/lingoTUI/internal/app"
	"github.com/FacundoTenuta/lingoTUI/internal/testutil"
)

func TestServiceConnectUsesConfiguredCredential(t *testing.T) {
	credentials := &testutil.CredentialStore{Secrets: map[ProviderID]Secret{ProviderOpenAI: {Value: "sk-test"}}}
	service := NewService(Dependencies{Credentials: credentials})

	result, err := service.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Connected || result.Message == "" {
		t.Fatalf("result = %+v", result)
	}
}

func TestServiceConnectMissingCredentialIsActionable(t *testing.T) {
	service := NewService(Dependencies{Credentials: &testutil.CredentialStore{}})

	_, err := service.Connect(context.Background())
	if !errors.Is(err, ErrMissingCredential) {
		t.Fatalf("error = %v, want %v", err, ErrMissingCredential)
	}
	for _, want := range []string{"lingotui login openai", "auth.json fallback", "/connect"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q: %v", want, err)
		}
	}
}

func TestServiceConnectChecksMixedRuntimeLocallyWithoutProviderCalls(t *testing.T) {
	credentials := &mixedCredentialStore{credential: Credential{Provider: ProviderChatGPT, Kind: CredentialKindOAuth, OAuth: OAuthCredential{RefreshToken: Secret{Value: "refresh-token"}}}}
	service := NewService(Dependencies{
		Config: &testutil.ConfigStore{Config: Config{
			TranscriptionModel: ModelRef{Provider: ProviderLocalWhisper, Name: "ggml-small.bin", Purpose: ModelPurposeTranscription},
			ChatModel:          ModelRef{Provider: ProviderChatGPT, Name: "configured-chatgpt-model", Purpose: ModelPurposeChat},
			LocalWhisper:       LocalWhisperConfig{ModelPath: "/models/ggml-small.bin"},
		}},
		Credentials: credentials,
	})

	result, err := service.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"localwhisper/ggml-small.bin", "chatgpt/configured-chatgpt-model", "Provider calls happen only on /stop, /ask, or /translate"} {
		if !strings.Contains(result.Message, want) {
			t.Fatalf("message missing %q: %s", want, result.Message)
		}
	}
	if !result.Connected || credentials.loads != 0 || credentials.credentialLoads != 1 {
		t.Fatalf("result = %+v loads=%d credentialLoads=%d", result, credentials.loads, credentials.credentialLoads)
	}
}

func TestServiceConnectRejectsChatGPTAccessOnlyCredentialWithoutValidExpiry(t *testing.T) {
	credentials := &mixedCredentialStore{credential: Credential{Provider: ProviderChatGPT, Kind: CredentialKindOAuth, OAuth: OAuthCredential{AccessToken: Secret{Value: "access-token"}}}}
	service := NewService(Dependencies{
		Config: &testutil.ConfigStore{Config: Config{
			TranscriptionModel: ModelRef{Provider: ProviderLocalWhisper, Name: "ggml-small.bin", Purpose: ModelPurposeTranscription},
			ChatModel:          ModelRef{Provider: ProviderChatGPT, Name: "configured-chatgpt-model", Purpose: ModelPurposeChat},
			LocalWhisper:       LocalWhisperConfig{ModelPath: "/models/ggml-small.bin"},
		}},
		Credentials: credentials,
	})

	result, err := service.Connect(context.Background())
	if !errors.Is(err, ErrMissingCredential) {
		t.Fatalf("error = %v, want %v", err, ErrMissingCredential)
	}
	if result.Connected {
		t.Fatalf("result = %+v, must not connect", result)
	}
	if !strings.Contains(err.Error(), "lingotui login chatgpt") {
		t.Fatalf("error missing ChatGPT login guidance: %v", err)
	}
}

func TestServiceConnectAllowsChatGPTAccessOnlyCredentialWithValidExpiry(t *testing.T) {
	credentials := &mixedCredentialStore{credential: Credential{Provider: ProviderChatGPT, Kind: CredentialKindOAuth, OAuth: OAuthCredential{AccessToken: Secret{Value: "access-token"}, ExpiresAt: time.Now().Add(time.Hour)}}}
	service := NewService(Dependencies{
		Config: &testutil.ConfigStore{Config: Config{
			TranscriptionModel: ModelRef{Provider: ProviderLocalWhisper, Name: "ggml-small.bin", Purpose: ModelPurposeTranscription},
			ChatModel:          ModelRef{Provider: ProviderChatGPT, Name: "configured-chatgpt-model", Purpose: ModelPurposeChat},
			LocalWhisper:       LocalWhisperConfig{ModelPath: "/models/ggml-small.bin"},
		}},
		Credentials: credentials,
	})

	result, err := service.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Connected {
		t.Fatalf("result = %+v, want connected", result)
	}
}

func TestServiceConnectChatGPTRequiresTypedOAuthStore(t *testing.T) {
	service := NewService(Dependencies{
		Config: &testutil.ConfigStore{Config: Config{
			TranscriptionModel: ModelRef{Provider: ProviderLocalWhisper, Name: "ggml-small.bin", Purpose: ModelPurposeTranscription},
			ChatModel:          ModelRef{Provider: ProviderChatGPT, Name: "configured-chatgpt-model", Purpose: ModelPurposeChat},
			LocalWhisper:       LocalWhisperConfig{ModelPath: "/models/ggml-small.bin"},
		}},
		Credentials: &testutil.CredentialStore{},
	})

	result, err := service.Connect(context.Background())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("error = %v, want %v", err, ErrNotConfigured)
	}
	for _, want := range []string{"typed OAuth credential store", "lingotui login chatgpt"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q: %v", want, err)
		}
	}
	if result.Connected {
		t.Fatalf("result = %+v, must not connect", result)
	}
}

func TestServiceConnectRejectsUnsupportedRuntimeRefs(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*Config)
		want      string
	}{
		{
			name: "chatgpt transcription",
			configure: func(cfg *Config) {
				cfg.TranscriptionModel = ModelRef{Provider: ProviderChatGPT, Name: "configured-chatgpt-model", Purpose: ModelPurposeTranscription}
				cfg.ChatModel = ModelRef{Provider: ProviderOpenAI, Name: DefaultChatModel, Purpose: ModelPurposeChat}
			},
			want: "ChatGPT/Codex transcription is not supported",
		},
		{
			name: "localwhisper chat",
			configure: func(cfg *Config) {
				cfg.TranscriptionModel = ModelRef{Provider: ProviderOpenAI, Name: DefaultTranscriptionModel, Purpose: ModelPurposeTranscription}
				cfg.ChatModel = ModelRef{Provider: ProviderLocalWhisper, Name: "local-whisper", Purpose: ModelPurposeChat}
			},
			want: "localwhisper chat is not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.configure(&cfg)
			service := NewService(Dependencies{
				Config:      &testutil.ConfigStore{Config: cfg},
				Credentials: &testutil.CredentialStore{Secrets: map[ProviderID]Secret{ProviderOpenAI: {Value: "sk-test"}}},
			})

			result, err := service.Connect(context.Background())
			if !errors.Is(err, ErrNotConfigured) {
				t.Fatalf("error = %v, want %v", err, ErrNotConfigured)
			}
			if result.Connected {
				t.Fatalf("result = %+v, must not connect", result)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error missing %q: %v", tt.want, err)
			}
		})
	}
}

func TestServiceModelsUsesDefaultModels(t *testing.T) {
	service := NewService(Dependencies{Config: &testutil.ConfigStore{}})

	result, err := service.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Models) != 2 || result.Models[0].Name != DefaultTranscriptionModel || result.Models[1].Name != DefaultChatModel {
		t.Fatalf("models = %+v", result.Models)
	}
}

func TestServiceAskMissingChatUsesConfiguredProviderGuidance(t *testing.T) {
	service := NewService(Dependencies{
		Config: &testutil.ConfigStore{Config: Config{
			ChatModel: ModelRef{Provider: ProviderChatGPT, Name: "configured-chatgpt-model", Purpose: ModelPurposeChat},
		}},
		Context: &testutil.ContextStore{Has: true, Context: RecentContext{Transcript: Transcript{Text: "hola"}}},
	})

	_, err := service.Ask(context.Background(), Question("what happened?"))
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("error = %v, want %v", err, ErrNotConfigured)
	}
	for _, want := range []string{"ChatGPT/Codex chat", "lingotui login chatgpt"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "login openai") {
		t.Fatalf("error used OpenAI-only guidance: %v", err)
	}
}

func TestServiceModelsShowsMixedConfiguredRefs(t *testing.T) {
	service := NewService(Dependencies{Config: &testutil.ConfigStore{Config: Config{
		TranscriptionModel: ModelRef{Provider: ProviderLocalWhisper, Name: "ggml-small.bin", Purpose: ModelPurposeTranscription},
		ChatModel:          ModelRef{Provider: ProviderChatGPT, Name: "configured-chatgpt-model", Purpose: ModelPurposeChat},
	}}})

	result, err := service.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Models) != 2 || result.Models[0].Provider != ProviderLocalWhisper || result.Models[1].Provider != ProviderChatGPT {
		t.Fatalf("models = %+v", result.Models)
	}
	for _, want := range []string{"localwhisper/ggml-small.bin", "chatgpt/configured-chatgpt-model", "configured refs", "provider registry"} {
		if !strings.Contains(result.Message, want) {
			t.Fatalf("message missing %q: %s", want, result.Message)
		}
	}
}

func TestServiceRecordStopSummarizeAskAndClear(t *testing.T) {
	ctx := context.Background()
	recorder := &testutil.Recorder{File: AudioFile{Path: "meeting.wav"}}
	provider := &recordingProvider{
		transcript: Transcript{Text: "hola mundo"},
		summary: Summary{
			LanguageSpanish: "saludo",
			LanguageEnglish: "greeting",
			LanguageGerman:  "begrüßung",
		},
		answer: Answer("They greeted each other."),
	}
	contexts := &testutil.ContextStore{}
	service := NewService(Dependencies{
		Recorder:    recorder,
		Transcriber: provider,
		Chat:        provider,
		Config:      &testutil.ConfigStore{},
		Context:     contexts,
	})

	if result, err := service.Record(ctx, AudioSourceMic); err != nil || !result.Recording || recorder.Started != AudioSourceMic {
		t.Fatalf("record result = %+v, err = %v", result, err)
	}
	result, err := service.Stop(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.Recording || !contexts.Has || result.Context.Summary[LanguageGerman] == "" {
		t.Fatalf("stop result = %+v, stored = %+v", result, contexts)
	}
	if result.Context.Transcript.Text != "hola mundo" {
		t.Fatalf("stop transcript = %q", result.Context.Transcript.Text)
	}
	wantSummary := Summary{LanguageSpanish: "saludo", LanguageEnglish: "greeting", LanguageGerman: "begrüßung"}
	for language, want := range wantSummary {
		if got := result.Context.Summary[language]; got != want {
			t.Fatalf("stop summary[%s] = %q, want %q", language, got, want)
		}
	}
	if !reflect.DeepEqual(contexts.Context, result.Context) {
		t.Fatalf("stored context = %+v, want result context %+v", contexts.Context, result.Context)
	}
	if !reflect.DeepEqual(provider.languages, SummaryLanguages()) {
		t.Fatalf("summary languages = %+v", provider.languages)
	}

	answer, err := service.Ask(ctx, Question("what happened?"))
	if err != nil {
		t.Fatal(err)
	}
	if answer.Answer != "They greeted each other." || provider.question != "what happened?" {
		t.Fatalf("answer result = %+v, provider question = %q", answer, provider.question)
	}

	if _, err := service.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Ask(ctx, Question("still there?")); !errors.Is(err, ErrNoRecentContext) {
		t.Fatalf("error = %v, want %v", err, ErrNoRecentContext)
	}
}

func TestServiceTranslateUsesChatModelWithoutRecentContext(t *testing.T) {
	provider := &recordingProvider{translations: Translations{
		LanguageSpanish: "hola",
		LanguageEnglish: "hello",
		LanguageGerman:  "hallo",
	}}
	contexts := &testutil.ContextStore{}
	service := NewService(Dependencies{
		Chat: provider,
		Config: &testutil.ConfigStore{Config: Config{
			TranscriptionModel: ModelRef{Provider: ProviderLocalWhisper, Name: "ggml-small.bin", Purpose: ModelPurposeTranscription},
			ChatModel:          ModelRef{Provider: ProviderChatGPT, Name: "configured-chatgpt-model", Purpose: ModelPurposeChat},
		}},
		Context: contexts,
	})

	result, err := service.Translate(context.Background(), " hello ")
	if err != nil {
		t.Fatal(err)
	}
	if result.Command != CommandTranslate || result.Translations[LanguageGerman] != "hallo" {
		t.Fatalf("result = %+v", result)
	}
	if provider.translationText != "hello" {
		t.Fatalf("translation text = %q, want hello", provider.translationText)
	}
	if !reflect.DeepEqual(provider.translationLanguages, SummaryLanguages()) {
		t.Fatalf("translation languages = %+v", provider.translationLanguages)
	}
	if provider.translationModel.Provider != ProviderChatGPT || provider.translationModel.Name != "configured-chatgpt-model" {
		t.Fatalf("translation model = %+v", provider.translationModel)
	}
	if contexts.Has {
		t.Fatalf("translate should not store or require recent context: %+v", contexts)
	}
}

func TestServiceTranslateMissingTextUsesMissingArgumentError(t *testing.T) {
	service := NewService(Dependencies{Chat: &recordingProvider{}})

	_, err := service.Translate(context.Background(), "  ")
	if !errors.Is(err, ErrMissingCommandArgument) {
		t.Fatalf("error = %v, want %v", err, ErrMissingCommandArgument)
	}
}

func TestServicePropagatesRecorderAndProviderErrors(t *testing.T) {
	tests := []struct {
		name string
		deps Dependencies
		call func(*Service) error
	}{
		{
			name: "record start error",
			deps: Dependencies{Recorder: &testutil.Recorder{StartErr: errors.New("no microphone")}},
			call: func(s *Service) error { _, err := s.Record(context.Background(), AudioSourceMic); return err },
		},
		{
			name: "stop transcribe error",
			deps: Dependencies{Recorder: &testutil.Recorder{}, Transcriber: testutil.Provider{Err: errors.New("provider down")}, Chat: testutil.Provider{}, Context: &testutil.ContextStore{}},
			call: func(s *Service) error {
				if _, err := s.Record(context.Background(), AudioSourceMic); err != nil {
					return err
				}
				_, err := s.Stop(context.Background())
				return err
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(NewService(tt.deps)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestServiceRejectsStopWithoutActiveRecording(t *testing.T) {
	service := NewService(Dependencies{
		Recorder:    &testutil.Recorder{},
		Transcriber: testutil.Provider{},
		Chat:        testutil.Provider{},
		Context:     &testutil.ContextStore{},
	})

	result, err := service.Stop(context.Background())
	if !errors.Is(err, ErrNotRecording) {
		t.Fatalf("error = %v, want %v", err, ErrNotRecording)
	}
	if result.Recording {
		t.Fatalf("result = %+v, want not recording", result)
	}
}

func TestServiceRejectsDuplicateRecord(t *testing.T) {
	recorder := &testutil.Recorder{}
	service := NewService(Dependencies{Recorder: recorder})

	if _, err := service.Record(context.Background(), AudioSourceMic); err != nil {
		t.Fatal(err)
	}
	result, err := service.Record(context.Background(), AudioSourceMic)
	if !errors.Is(err, ErrAlreadyRecording) {
		t.Fatalf("error = %v, want %v", err, ErrAlreadyRecording)
	}
	if !result.Recording {
		t.Fatalf("result = %+v, want still recording", result)
	}
}

func TestServiceHelpIncludesSetupGuidance(t *testing.T) {
	service := NewService(Dependencies{})
	result := service.Help(context.Background())
	if len(result.Help) == 0 || len(result.Guidance) == 0 {
		t.Fatalf("help result = %+v", result)
	}
	joined := strings.Join(result.Guidance, "\n")
	for _, want := range []string{"lingotui login openai", "lingotui login chatgpt", "auth.json fallback", "/connect", "/record mic", "Provider calls happen only on /stop, /ask, or /translate"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("guidance missing %q: %s", want, joined)
		}
	}
}

func TestServiceNotConfiguredMessagesAreActionable(t *testing.T) {
	tests := []struct {
		name string
		call func(*Service) error
		want string
	}{
		{
			name: "missing credential store",
			call: func(s *Service) error { _, err := s.Connect(context.Background()); return err },
			want: "lingotui login openai",
		},
		{
			name: "missing recorder",
			call: func(s *Service) error { _, err := s.Record(context.Background(), AudioSourceMic); return err },
			want: "/record mic",
		},
		{
			name: "missing chat for ask",
			call: func(s *Service) error { _, err := s.Ask(context.Background(), Question("hello?")); return err },
			want: "/connect",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call(NewService(Dependencies{Context: &testutil.ContextStore{Has: true}}))
			if !errors.Is(err, ErrNotConfigured) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want ErrNotConfigured with %q", err, tt.want)
			}
		})
	}
}

func TestServiceStopFailureClearsRecordingState(t *testing.T) {
	recorder := &testutil.Recorder{StopErr: errors.New("ffmpeg stopped with error")}
	service := NewService(Dependencies{
		Recorder:    recorder,
		Transcriber: testutil.Provider{},
		Chat:        testutil.Provider{},
		Context:     &testutil.ContextStore{},
	})

	if _, err := service.Record(context.Background(), AudioSourceMic); err != nil {
		t.Fatal(err)
	}
	result, err := service.Stop(context.Background())
	if err == nil {
		t.Fatal("expected stop error")
	}
	if result.Recording {
		t.Fatalf("result = %+v, want recording cleared after stop attempt", result)
	}
	if _, err := service.Record(context.Background(), AudioSourceMic); err != nil {
		t.Fatalf("record after failed stop = %v", err)
	}
}

type recordingProvider struct {
	transcript           Transcript
	summary              Summary
	answer               Answer
	translations         Translations
	languages            []Language
	question             Question
	translationText      string
	translationLanguages []Language
	translationModel     ModelRef
}

type mixedCredentialStore struct {
	Secrets         map[ProviderID]Secret
	credential      Credential
	loads           int
	credentialLoads int
}

func (s *mixedCredentialStore) Save(_ context.Context, provider ProviderID, secret Secret) error {
	if s.Secrets == nil {
		s.Secrets = map[ProviderID]Secret{}
	}
	s.Secrets[provider] = secret
	return nil
}

func (s *mixedCredentialStore) Load(_ context.Context, provider ProviderID) (Secret, error) {
	s.loads++
	return s.Secrets[provider], nil
}

func (s *mixedCredentialStore) Delete(_ context.Context, provider ProviderID) error {
	delete(s.Secrets, provider)
	return nil
}

func (s *mixedCredentialStore) SaveCredential(context.Context, Credential) error { return nil }

func (s *mixedCredentialStore) LoadCredential(context.Context, ProviderID, CredentialKind) (Credential, error) {
	s.credentialLoads++
	return s.credential, nil
}

func (s *mixedCredentialStore) DeleteCredential(context.Context, ProviderID, CredentialKind) error {
	return nil
}

func (p *recordingProvider) Translate(_ context.Context, text string, languages []Language, model ModelRef) (Translations, error) {
	p.translationText = text
	p.translationLanguages = append([]Language(nil), languages...)
	p.translationModel = model
	return p.translations, nil
}

func (p *recordingProvider) Transcribe(context.Context, AudioFile, ModelRef) (Transcript, error) {
	return p.transcript, nil
}

func (p *recordingProvider) Summarize(_ context.Context, _ Transcript, languages []Language, _ ModelRef) (Summary, error) {
	p.languages = append([]Language(nil), languages...)
	return p.summary, nil
}

func (p *recordingProvider) Answer(_ context.Context, question Question, _ RecentContext, _ ModelRef) (Answer, error) {
	p.question = question
	return p.answer, nil
}
