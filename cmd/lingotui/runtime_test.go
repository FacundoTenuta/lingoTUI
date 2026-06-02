package main

import (
	"context"
	"strings"
	"testing"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
	"github.com/FacundoTenuta/lingoTUI/internal/config"
	"github.com/FacundoTenuta/lingoTUI/internal/credentials"
	"github.com/FacundoTenuta/lingoTUI/internal/setup"
	"github.com/FacundoTenuta/lingoTUI/internal/tui"
)

func TestBuildRuntimeStartsWithOnboardingAndNoExternalSideEffects(t *testing.T) {
	baseDir := t.TempDir()
	recorder := &countingRecorder{}
	provider := &countingProvider{}
	audioChecker := &countingAudioChecker{status: setup.ItemStatus{Name: "Microphone", State: setup.StateUnknown, Message: "grant access before /record mic"}}

	store, err := credentials.NewFileStore(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), app.ProviderOpenAI, app.Secret{Value: "sk-runtime-secret"}); err != nil {
		t.Fatal(err)
	}

	model, err := buildRuntimeWithOptions(baseDir, runtimeOptions{
		newProvider: func(secret app.Secret) (providerClient, error) {
			if secret.Value != "sk-runtime-secret" {
				t.Fatalf("secret = %q, want configured secret", secret.Value)
			}
			return provider, nil
		},
		newRecorder:     func() app.Recorder { return recorder },
		audioChecker:    audioChecker,
		credentialStore: store,
	})
	if err != nil {
		t.Fatal(err)
	}

	view := model.View()
	for _, want := range []string{"Ready. Choose an action", "Setup status", "OpenAI credentials", "[redacted]", "Microphone", "/record mic"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
	for _, forbid := range []string{"sk-runtime-secret"} {
		if strings.Contains(view, forbid) {
			t.Fatalf("view exposed %q:\n%s", forbid, view)
		}
	}
	if audioChecker.checks != 1 {
		t.Fatalf("audio checks = %d, want 1 setup-only status check", audioChecker.checks)
	}
	if recorder.starts != 0 || recorder.stops != 0 || provider.transcribes != 0 || provider.summarizes != 0 || provider.answers != 0 {
		t.Fatalf("startup side effects: recorder=%+v provider=%+v", recorder, provider)
	}
}

func TestBuildRuntimeAllowsExplicitRecordOnlyAfterUserCommand(t *testing.T) {
	baseDir := t.TempDir()
	recorder := &countingRecorder{}

	model, err := buildRuntimeWithOptions(baseDir, runtimeOptions{
		newRecorder:     func() app.Recorder { return recorder },
		audioChecker:    &countingAudioChecker{status: setup.ItemStatus{Name: "Microphone", State: setup.StateReady, Message: "ready"}},
		credentialStore: &missingRuntimeCredentialStore{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if recorder.starts != 0 {
		t.Fatalf("startup recorder starts = %d, want 0", recorder.starts)
	}

	updated, cmd := model.Update(tui.Submit("/record mic"))
	if _, ok := updated.(tui.Model); !ok {
		t.Fatalf("updated model = %T, want tui.Model", updated)
	}
	if cmd == nil {
		t.Fatal("expected explicit command to return async command")
	}
	if recorder.starts != 0 {
		t.Fatalf("recorder starts before async command executes = %d, want 0", recorder.starts)
	}
	updated, nextCmd := updated.Update(cmd())
	if _, ok := updated.(tui.Model); !ok {
		t.Fatalf("completed model = %T, want tui.Model", updated)
	}
	if nextCmd != nil {
		t.Fatal("expected command completion not to return another command")
	}
	if recorder.starts != 1 {
		t.Fatalf("recorder starts after explicit command = %d, want 1", recorder.starts)
	}
}

func TestBuildRuntimeWiresPureLocalWhisperAndChatGPTWithoutOpenAILoad(t *testing.T) {
	baseDir := t.TempDir()
	ctx := context.Background()
	configStore, err := config.NewFileStore(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := app.DefaultConfig()
	cfg.TranscriptionModel = app.ModelRef{Provider: app.ProviderLocalWhisper, Name: "ggml-small.bin", Purpose: app.ModelPurposeTranscription}
	cfg.ChatModel = app.ModelRef{Provider: app.ProviderChatGPT, Name: "configured-chatgpt-model", Purpose: app.ModelPurposeChat}
	cfg.LocalWhisper = app.LocalWhisperConfig{BinaryPath: "/bin/whisper-cli", ModelPath: "/models/ggml-small.bin", Language: "es", ExtraArgs: []string{"--speed-up"}}
	if err := configStore.Save(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	store := &countingRuntimeCredentialStore{secret: app.Secret{Value: "unused-openai"}}
	local := &countingProvider{}
	chat := &countingProvider{}
	var localConfig app.LocalWhisperConfig
	chatCalls := 0
	openAICalls := 0

	_, err = buildRuntimeWithOptions(baseDir, runtimeOptions{
		newOpenAIProvider: func(app.Secret) (providerClient, error) {
			openAICalls++
			return &countingProvider{}, nil
		},
		newLocalWhisperTranscriber: func(config app.LocalWhisperConfig) (app.Transcriber, error) {
			localConfig = config
			return local, nil
		},
		newChatGPTChat: func(authStore app.AuthCredentialStore) (app.Chat, error) {
			chatCalls++
			if authStore != store {
				t.Fatalf("auth store = %T, want counting store", authStore)
			}
			return chat, nil
		},
		newRecorder:     func() app.Recorder { return &countingRecorder{} },
		audioChecker:    &countingAudioChecker{status: setup.ItemStatus{Name: "Microphone", State: setup.StateReady, Message: "ready"}},
		credentialStore: store,
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.loads != 0 {
		t.Fatalf("OpenAI credential loads = %d, want 0", store.loads)
	}
	if openAICalls != 0 {
		t.Fatalf("OpenAI constructor calls = %d, want 0", openAICalls)
	}
	if localConfig.ModelPath != "/models/ggml-small.bin" || localConfig.BinaryPath != "/bin/whisper-cli" || localConfig.Language != "es" || len(localConfig.ExtraArgs) != 1 {
		t.Fatalf("local whisper config = %+v", localConfig)
	}
	if chatCalls != 1 {
		t.Fatalf("ChatGPT chat constructor calls = %d, want 1", chatCalls)
	}
	if local.transcribes != 0 || chat.summarizes != 0 || chat.answers != 0 {
		t.Fatalf("startup provider calls: local=%+v chat=%+v", local, chat)
	}
}

func TestBuildRuntimeKeepsStartupPassiveWhenLocalWhisperModelPathMissing(t *testing.T) {
	baseDir := t.TempDir()
	ctx := context.Background()
	configStore, err := config.NewFileStore(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := app.DefaultConfig()
	cfg.TranscriptionModel = app.ModelRef{Provider: app.ProviderLocalWhisper, Name: "ggml-small.bin", Purpose: app.ModelPurposeTranscription}
	if err := configStore.Save(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	localCalls := 0

	model, err := buildRuntimeWithOptions(baseDir, runtimeOptions{
		newLocalWhisperTranscriber: func(app.LocalWhisperConfig) (app.Transcriber, error) {
			localCalls++
			return &countingProvider{}, nil
		},
		newRecorder:     func() app.Recorder { return &countingRecorder{} },
		audioChecker:    &countingAudioChecker{status: setup.ItemStatus{Name: "Microphone", State: setup.StateReady, Message: "ready"}},
		credentialStore: &missingRuntimeCredentialStore{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if localCalls != 0 {
		t.Fatalf("localwhisper constructor calls = %d, want 0 while model path is missing", localCalls)
	}
	view := model.View()
	for _, want := range []string{"Setup needs attention", "missing local_whisper.model_path"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestBuildRuntimeLoadsOpenAIOnlyWhenModelRefUsesOpenAI(t *testing.T) {
	tests := []struct {
		name        string
		configure   func(*app.Config)
		wantLoads   int
		wantOpenAI  int
		wantLocal   int
		wantChatGPT int
	}{
		{
			name: "openai chat with localwhisper transcription",
			configure: func(cfg *app.Config) {
				cfg.TranscriptionModel = app.ModelRef{Provider: app.ProviderLocalWhisper, Name: "ggml-small.bin", Purpose: app.ModelPurposeTranscription}
				cfg.LocalWhisper.ModelPath = "/models/ggml-small.bin"
			},
			wantLoads:  2,
			wantOpenAI: 1,
			wantLocal:  1,
		},
		{
			name: "openai transcription with chatgpt chat",
			configure: func(cfg *app.Config) {
				cfg.ChatModel = app.ModelRef{Provider: app.ProviderChatGPT, Name: "configured-chatgpt-model", Purpose: app.ModelPurposeChat}
			},
			wantLoads:   2,
			wantOpenAI:  1,
			wantChatGPT: 1,
		},
		{
			name: "no openai refs",
			configure: func(cfg *app.Config) {
				cfg.TranscriptionModel = app.ModelRef{Provider: app.ProviderLocalWhisper, Name: "ggml-small.bin", Purpose: app.ModelPurposeTranscription}
				cfg.ChatModel = app.ModelRef{Provider: app.ProviderChatGPT, Name: "configured-chatgpt-model", Purpose: app.ModelPurposeChat}
				cfg.LocalWhisper.ModelPath = "/models/ggml-small.bin"
			},
			wantLocal:   1,
			wantChatGPT: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseDir := t.TempDir()
			ctx := context.Background()
			configStore, err := config.NewFileStore(baseDir)
			if err != nil {
				t.Fatal(err)
			}
			cfg := app.DefaultConfig()
			tt.configure(&cfg)
			if err := configStore.Save(ctx, cfg); err != nil {
				t.Fatal(err)
			}
			store := &countingRuntimeCredentialStore{secret: app.Secret{Value: "sk-runtime-secret"}}
			openAICalls := 0
			localCalls := 0
			chatGPTCalls := 0

			_, err = buildRuntimeWithOptions(baseDir, runtimeOptions{
				newOpenAIProvider: func(secret app.Secret) (providerClient, error) {
					openAICalls++
					if secret.Value != "sk-runtime-secret" {
						t.Fatalf("secret = %q, want configured OpenAI secret", secret.Value)
					}
					return &countingProvider{}, nil
				},
				newLocalWhisperTranscriber: func(app.LocalWhisperConfig) (app.Transcriber, error) {
					localCalls++
					return &countingProvider{}, nil
				},
				newChatGPTChat: func(app.AuthCredentialStore) (app.Chat, error) {
					chatGPTCalls++
					return &countingProvider{}, nil
				},
				newRecorder:     func() app.Recorder { return &countingRecorder{} },
				audioChecker:    &countingAudioChecker{status: setup.ItemStatus{Name: "Microphone", State: setup.StateReady, Message: "ready"}},
				credentialStore: store,
			})
			if err != nil {
				t.Fatal(err)
			}
			if store.loads != tt.wantLoads || openAICalls != tt.wantOpenAI || localCalls != tt.wantLocal || chatGPTCalls != tt.wantChatGPT {
				t.Fatalf("loads/openai/local/chatgpt = %d/%d/%d/%d, want %d/%d/%d/%d", store.loads, openAICalls, localCalls, chatGPTCalls, tt.wantLoads, tt.wantOpenAI, tt.wantLocal, tt.wantChatGPT)
			}
		})
	}
}

func TestBuildRuntimeUsesLocalWhisperForTranscriptionAndOpenAIForChat(t *testing.T) {
	baseDir := t.TempDir()
	ctx := context.Background()
	configStore, err := config.NewFileStore(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := app.DefaultConfig()
	cfg.Provider = app.ProviderOpenAI
	cfg.TranscriptionModel.Provider = app.ProviderLocalWhisper
	cfg.TranscriptionModel.Name = "local-whisper"
	cfg.LocalWhisper.ModelPath = "/models/ggml-base.bin"
	if err := configStore.Save(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	store, err := credentials.NewFileStore(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, app.ProviderOpenAI, app.Secret{Value: "sk-runtime-secret"}); err != nil {
		t.Fatal(err)
	}
	provider := &countingProvider{}
	local := &countingProvider{}
	recorder := &countingRecorder{}

	model, err := buildRuntimeWithOptions(baseDir, runtimeOptions{
		newOpenAIProvider: func(secret app.Secret) (providerClient, error) {
			if secret.Value != "sk-runtime-secret" {
				t.Fatalf("secret = %q, want configured OpenAI secret", secret.Value)
			}
			return provider, nil
		},
		newLocalWhisperTranscriber: func(config app.LocalWhisperConfig) (app.Transcriber, error) {
			if config.ModelPath != "/models/ggml-base.bin" {
				t.Fatalf("local model path = %q", config.ModelPath)
			}
			return local, nil
		},
		newRecorder:     func() app.Recorder { return recorder },
		audioChecker:    &countingAudioChecker{status: setup.ItemStatus{Name: "Microphone", State: setup.StateReady, Message: "ready"}},
		credentialStore: store,
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, cmd := model.Update(tui.Submit("/record mic"))
	if cmd == nil {
		t.Fatal("expected /record mic to return async command")
	}
	updated, _ = updated.Update(cmd())
	updated, cmd = updated.Update(tui.Submit("/stop"))
	if cmd == nil {
		t.Fatal("expected /stop to return async command")
	}
	updated, _ = updated.Update(cmd())
	model = updated.(tui.Model)

	if local.transcribes != 1 {
		t.Fatalf("local transcribes = %d, want 1", local.transcribes)
	}
	if provider.transcribes != 0 {
		t.Fatalf("OpenAI transcribes = %d, want 0 when transcription model provider is localwhisper", provider.transcribes)
	}
	if recorder.stops != 1 {
		t.Fatalf("recorder stops = %d, want 1 before processing dependency failure", recorder.stops)
	}
	if provider.summarizes != 1 {
		t.Fatalf("OpenAI summarizes = %d, want 1", provider.summarizes)
	}
	if !strings.Contains(model.View(), "Processed recording") {
		t.Fatalf("view missing processed recording result:\n%s", model.View())
	}
}

func TestDefaultChatGPTChatConstructionHasNoCredentialSideEffects(t *testing.T) {
	store := &countingRuntimeCredentialStore{
		credential: app.Credential{
			Provider: app.ProviderChatGPT,
			Kind:     app.CredentialKindOAuth,
			OAuth: app.OAuthCredential{
				AccessToken:  app.Secret{Value: "access-token"},
				RefreshToken: app.Secret{Value: "refresh-token"},
				AccountID:    "account-id",
			},
		},
	}

	chat, err := defaultRuntimeOptions().newChatGPTChat(store)
	if err != nil {
		t.Fatal(err)
	}
	if store.credentialLoads != 0 || store.saves != 0 {
		t.Fatalf("startup credential side effects loads/saves = %d/%d, want 0/0", store.credentialLoads, store.saves)
	}
	if chat == nil {
		t.Fatal("chat = nil, want constructed ChatGPT chat")
	}
}

type countingRecorder struct {
	starts int
	stops  int
}

func (r *countingRecorder) Start(context.Context, app.AudioSource) error {
	r.starts++
	return nil
}

func (r *countingRecorder) Stop(context.Context) (app.AudioFile, error) {
	r.stops++
	return app.AudioFile{Path: "recording.wav"}, nil
}

type countingProvider struct {
	transcribes int
	summarizes  int
	answers     int
}

func (p *countingProvider) Transcribe(context.Context, app.AudioFile, app.ModelRef) (app.Transcript, error) {
	p.transcribes++
	return app.Transcript{Text: "hola"}, nil
}

func (p *countingProvider) Summarize(context.Context, app.Transcript, []app.Language, app.ModelRef) (app.Summary, error) {
	p.summarizes++
	return app.Summary{}, nil
}

func (p *countingProvider) Answer(context.Context, app.Question, app.RecentContext, app.ModelRef) (app.Answer, error) {
	p.answers++
	return app.Answer("answer"), nil
}

type countingAudioChecker struct {
	status setup.ItemStatus
	checks int
}

type missingRuntimeCredentialStore struct{}

func (s *missingRuntimeCredentialStore) Save(context.Context, app.ProviderID, app.Secret) error {
	return nil
}
func (s *missingRuntimeCredentialStore) Load(context.Context, app.ProviderID) (app.Secret, error) {
	return app.Secret{}, credentials.ErrSecretNotFound
}
func (s *missingRuntimeCredentialStore) Delete(context.Context, app.ProviderID) error { return nil }

type countingRuntimeCredentialStore struct {
	secret          app.Secret
	credential      app.Credential
	loads           int
	saves           int
	credentialLoads int
}

func (s *countingRuntimeCredentialStore) Save(context.Context, app.ProviderID, app.Secret) error {
	return nil
}
func (s *countingRuntimeCredentialStore) Load(context.Context, app.ProviderID) (app.Secret, error) {
	s.loads++
	return s.secret, nil
}
func (s *countingRuntimeCredentialStore) Delete(context.Context, app.ProviderID) error { return nil }

func (s *countingRuntimeCredentialStore) SaveCredential(context.Context, app.Credential) error {
	s.saves++
	return nil
}

func (s *countingRuntimeCredentialStore) LoadCredential(context.Context, app.ProviderID, app.CredentialKind) (app.Credential, error) {
	s.credentialLoads++
	if s.credential.Provider != "" {
		return s.credential, nil
	}
	return app.Credential{Provider: app.ProviderChatGPT, Kind: app.CredentialKindOAuth, OAuth: app.OAuthCredential{RefreshToken: app.Secret{Value: "refresh-token"}}}, nil
}

func (s *countingRuntimeCredentialStore) DeleteCredential(context.Context, app.ProviderID, app.CredentialKind) error {
	return nil
}

func (c *countingAudioChecker) Microphone(context.Context) setup.ItemStatus {
	c.checks++
	return c.status
}
