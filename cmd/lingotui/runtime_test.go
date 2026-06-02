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

func TestBuildRuntimeDoesNotWireProviderForChatGPTScaffold(t *testing.T) {
	baseDir := t.TempDir()
	ctx := context.Background()
	configStore, err := config.NewFileStore(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := app.DefaultConfig()
	cfg.Provider = app.ProviderChatGPT
	if err := configStore.Save(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	store := &countingRuntimeCredentialStore{secret: app.Secret{Value: "chatgpt-token"}}
	providerCalls := 0

	_, err = buildRuntimeWithOptions(baseDir, runtimeOptions{
		newProvider: func(app.Secret) (providerClient, error) {
			providerCalls++
			return &countingProvider{}, nil
		},
		newRecorder:     func() app.Recorder { return &countingRecorder{} },
		audioChecker:    &countingAudioChecker{status: setup.ItemStatus{Name: "Microphone", State: setup.StateReady, Message: "ready"}},
		credentialStore: store,
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.loads != 0 {
		t.Fatalf("credential loads = %d, want 0 for ChatGPT scaffold", store.loads)
	}
	if providerCalls != 0 {
		t.Fatalf("provider constructor calls = %d, want 0 for ChatGPT scaffold", providerCalls)
	}
}

func TestBuildRuntimeDoesNotWireProviderForLocalWhisperConfig(t *testing.T) {
	baseDir := t.TempDir()
	ctx := context.Background()
	configStore, err := config.NewFileStore(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := app.DefaultConfig()
	cfg.Provider = app.ProviderLocalWhisper
	cfg.TranscriptionModel.Provider = app.ProviderLocalWhisper
	cfg.LocalWhisper.ModelPath = "/models/ggml-base.bin"
	if err := configStore.Save(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	store := &countingRuntimeCredentialStore{secret: app.Secret{Value: "unused-secret"}}
	providerCalls := 0

	_, err = buildRuntimeWithOptions(baseDir, runtimeOptions{
		newProvider: func(app.Secret) (providerClient, error) {
			providerCalls++
			return &countingProvider{}, nil
		},
		newRecorder:     func() app.Recorder { return &countingRecorder{} },
		audioChecker:    &countingAudioChecker{status: setup.ItemStatus{Name: "Microphone", State: setup.StateReady, Message: "ready"}},
		credentialStore: store,
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.loads != 0 {
		t.Fatalf("credential loads = %d, want 0 for localwhisper config-only seam", store.loads)
	}
	if providerCalls != 0 {
		t.Fatalf("provider constructor calls = %d, want 0 for localwhisper config-only seam", providerCalls)
	}
}

func TestBuildRuntimeDoesNotUseOpenAIProviderForLocalWhisperTranscriptionModel(t *testing.T) {
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
	recorder := &countingRecorder{}

	model, err := buildRuntimeWithOptions(baseDir, runtimeOptions{
		newProvider: func(secret app.Secret) (providerClient, error) {
			if secret.Value != "sk-runtime-secret" {
				t.Fatalf("secret = %q, want configured OpenAI secret", secret.Value)
			}
			return provider, nil
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

	if provider.transcribes != 0 {
		t.Fatalf("OpenAI transcribes = %d, want 0 when transcription model provider is localwhisper", provider.transcribes)
	}
	if recorder.stops != 1 {
		t.Fatalf("recorder stops = %d, want 1 before processing dependency failure", recorder.stops)
	}
	if provider.summarizes != 0 {
		t.Fatalf("summarizes = %d, want 0 without transcript", provider.summarizes)
	}
	if !strings.Contains(model.View(), "localwhisper transcription runtime is not implemented yet") {
		t.Fatalf("view missing transcription unavailable guidance:\n%s", model.View())
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
	secret app.Secret
	loads  int
}

func (s *countingRuntimeCredentialStore) Save(context.Context, app.ProviderID, app.Secret) error {
	return nil
}
func (s *countingRuntimeCredentialStore) Load(context.Context, app.ProviderID) (app.Secret, error) {
	s.loads++
	return s.secret, nil
}
func (s *countingRuntimeCredentialStore) Delete(context.Context, app.ProviderID) error { return nil }

func (c *countingAudioChecker) Microphone(context.Context) setup.ItemStatus {
	c.checks++
	return c.status
}
