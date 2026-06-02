package app_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

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

func TestServiceConnectChatGPTRuntimeIsNotImplemented(t *testing.T) {
	service := NewService(Dependencies{
		Config:      &testutil.ConfigStore{Config: Config{Provider: ProviderChatGPT}},
		Credentials: &testutil.CredentialStore{Secrets: map[ProviderID]Secret{ProviderChatGPT: {Value: "oauth-token"}}},
	})

	result, err := service.Connect(context.Background())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("error = %v, want %v", err, ErrNotConfigured)
	}
	for _, want := range []string{"ChatGPT Plus/Pro OAuth login is enabled", "ChatGPT/Codex runtime is not implemented yet", "/connect remains OpenAI API-key only"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q: %v", want, err)
		}
	}
	if result.Connected {
		t.Fatalf("result = %+v, must not connect ChatGPT scaffold", result)
	}
}

func TestServiceConnectLocalWhisperRuntimeIsNotImplemented(t *testing.T) {
	service := NewService(Dependencies{
		Config:      &testutil.ConfigStore{Config: Config{Provider: ProviderLocalWhisper}},
		Credentials: &testutil.CredentialStore{Secrets: map[ProviderID]Secret{ProviderLocalWhisper: {Value: "local-secret-not-used"}}},
	})

	result, err := service.Connect(context.Background())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("error = %v, want %v", err, ErrNotConfigured)
	}
	for _, want := range []string{"localwhisper transcription config is accepted", "runtime is not implemented yet", "/connect remains OpenAI API-key only"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q: %v", want, err)
		}
	}
	if result.Connected {
		t.Fatalf("result = %+v, must not connect localwhisper scaffold", result)
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

func TestServiceModelsDoesNotExposeChatGPTScaffoldModels(t *testing.T) {
	service := NewService(Dependencies{Config: &testutil.ConfigStore{Config: Config{Provider: ProviderChatGPT}}})

	result, err := service.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Models) != 0 {
		t.Fatalf("models = %+v, want none for ChatGPT scaffold", result.Models)
	}
	if !strings.Contains(result.Message, "not implemented yet") {
		t.Fatalf("message = %q, want not implemented guidance", result.Message)
	}
}

func TestServiceModelsDoesNotClaimLocalWhisperRuntimeReady(t *testing.T) {
	service := NewService(Dependencies{Config: &testutil.ConfigStore{Config: Config{
		Provider:           ProviderOpenAI,
		TranscriptionModel: ModelRef{Provider: ProviderLocalWhisper, Name: "ggml-small.bin", Purpose: ModelPurposeTranscription},
		ChatModel:          ModelRef{Provider: ProviderOpenAI, Name: DefaultChatModel, Purpose: ModelPurposeChat},
	}}})

	result, err := service.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Models) != 0 {
		t.Fatalf("models = %+v, want none for localwhisper scaffold", result.Models)
	}
	for _, want := range []string{"localwhisper", "runtime model listing is not implemented yet"} {
		if !strings.Contains(result.Message, want) {
			t.Fatalf("message = %q, want %q", result.Message, want)
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
	for _, want := range []string{"lingotui login openai", "lingotui login chatgpt", "not implemented yet", "auth.json fallback", "/connect", "/record mic"} {
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
	transcript Transcript
	summary    Summary
	answer     Answer
	languages  []Language
	question   Question
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
