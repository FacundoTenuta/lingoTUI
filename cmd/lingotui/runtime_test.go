package main

import (
	"context"
	"strings"
	"testing"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
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

	updated, _ := model.Update(tui.Submit("/record mic"))
	if _, ok := updated.(tui.Model); !ok {
		t.Fatalf("updated model = %T, want tui.Model", updated)
	}
	if recorder.starts != 1 {
		t.Fatalf("recorder starts after explicit command = %d, want 1", recorder.starts)
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
	return app.AudioFile{Path: "recording.m4a"}, nil
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

func (c *countingAudioChecker) Microphone(context.Context) setup.ItemStatus {
	c.checks++
	return c.status
}
