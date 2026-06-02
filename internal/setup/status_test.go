package setup

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

func TestServiceStatusReportsSetupStatesWithoutSecrets(t *testing.T) {
	tests := []struct {
		name      string
		secret    app.Secret
		loadErr   error
		audio     ItemStatus
		wantReady bool
		want      []string
		forbid    []string
	}{
		{
			name:      "ready credential is redacted",
			secret:    app.Secret{Value: "sk-secret"},
			audio:     ItemStatus{Name: "Microphone", State: StateReady, Message: "ready"},
			wantReady: true,
			want:      []string{"OpenAI credentials", "configured via Keychain/auth.json fallback ([redacted])", "auth.json", "Setup ready"},
			forbid:    []string{"sk-secret"},
		},
		{
			name:      "missing credential is actionable",
			loadErr:   errors.New("not found"),
			audio:     ItemStatus{Name: "Microphone", State: StateUnknown, Message: "grant macOS microphone access before /record mic"},
			wantReady: false,
			want:      []string{"missing", "lingotui login openai", "auth.json fallback", "/connect", "/record mic", "Nothing records, opens a browser, or calls a provider"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			credentials := &fakeCredentialStore{secret: tt.secret, err: tt.loadErr}
			audio := &fakeAudioChecker{status: tt.audio}
			service := Service{
				ConfigPath:     fakePath("/tmp/lingotui/config.json"),
				CredentialPath: fakePath("/tmp/lingotui/auth.json"),
				Credentials:    credentials,
				AudioChecker:   audio,
			}

			status := service.Status(context.Background())
			if status.Ready != tt.wantReady {
				t.Fatalf("ready = %v, want %v", status.Ready, tt.wantReady)
			}
			if credentials.loads != 1 || audio.checks != 1 {
				t.Fatalf("loads = %d, checks = %d", credentials.loads, audio.checks)
			}
			rendered := strings.Join(RenderLines(status), "\n")
			for _, want := range tt.want {
				if !strings.Contains(rendered, want) {
					t.Fatalf("rendered status missing %q:\n%s", want, rendered)
				}
			}
			for _, forbid := range tt.forbid {
				if strings.Contains(rendered, forbid) {
					t.Fatalf("rendered status exposed %q:\n%s", forbid, rendered)
				}
			}
		})
	}
}

func TestServiceStatusReportsChatGPTOAuthCredentialReadyWithoutSecrets(t *testing.T) {
	credentials := &fakeAuthCredentialStore{
		credential: app.Credential{
			Provider: app.ProviderChatGPT,
			Kind:     app.CredentialKindOAuth,
			OAuth: app.OAuthCredential{
				AccessToken:  app.Secret{Value: "access-token"},
				RefreshToken: app.Secret{Value: "refresh-token"},
			},
		},
	}
	service := Service{
		CredentialPath: fakePath("/tmp/lingotui/auth.json"),
		Credentials:    credentials,
		AudioChecker:   &fakeAudioChecker{status: ItemStatus{Name: "Microphone", State: StateReady, Message: "ready"}},
		Provider:       app.ProviderChatGPT,
	}

	status := service.Status(context.Background())
	if status.Ready {
		t.Fatalf("status ready = true, want false until ChatGPT runtime is implemented: %+v", status)
	}
	if status.Items[1].State != StateReady {
		t.Fatalf("credential state = %s, want ready", status.Items[1].State)
	}
	if credentials.loads != 0 || credentials.credentialLoads != 1 {
		t.Fatalf("loads = %d credentialLoads = %d, want old=0 typed=1", credentials.loads, credentials.credentialLoads)
	}
	rendered := strings.Join(RenderLines(status), "\n")
	for _, want := range []string{"ChatGPT Plus/Pro credentials", "configured via typed OAuth credential ([redacted])", "ChatGPT/Codex runtime is not implemented yet"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered status missing %q:\n%s", want, rendered)
		}
	}
	for _, forbid := range []string{"access-token", "refresh-token"} {
		if strings.Contains(rendered, forbid) {
			t.Fatalf("rendered status exposed %q:\n%s", forbid, rendered)
		}
	}
}

func TestServiceStatusReportsChatGPTOAuthCredentialMissingWithoutOldLoad(t *testing.T) {
	credentials := &fakeAuthCredentialStore{fakeCredentialStore: fakeCredentialStore{err: errors.New("not found")}}
	service := Service{
		CredentialPath: fakePath("/tmp/lingotui/auth.json"),
		Credentials:    credentials,
		AudioChecker:   &fakeAudioChecker{status: ItemStatus{Name: "Microphone", State: StateReady, Message: "ready"}},
		Provider:       app.ProviderChatGPT,
	}

	status := service.Status(context.Background())
	if status.Ready {
		t.Fatalf("status ready = true, want false: %+v", status)
	}
	if credentials.loads != 0 || credentials.credentialLoads != 1 {
		t.Fatalf("loads = %d credentialLoads = %d, want old=0 typed=1", credentials.loads, credentials.credentialLoads)
	}
	rendered := strings.Join(RenderLines(status), "\n")
	for _, want := range []string{"missing", "lingotui login chatgpt", "ChatGPT/Codex runtime is not implemented yet"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered status missing %q:\n%s", want, rendered)
		}
	}
}

func TestServiceStatusReportsChatGPTTypedStoreUnsupportedWithoutOldLoad(t *testing.T) {
	credentials := &fakeCredentialStore{secret: app.Secret{Value: "legacy-secret"}}
	service := Service{
		CredentialPath: fakePath("/tmp/lingotui/auth.json"),
		Credentials:    credentials,
		AudioChecker:   &fakeAudioChecker{status: ItemStatus{Name: "Microphone", State: StateReady, Message: "ready"}},
		Provider:       app.ProviderChatGPT,
	}

	status := service.Status(context.Background())
	if status.Ready {
		t.Fatalf("status ready = true, want false: %+v", status)
	}
	if credentials.loads != 0 {
		t.Fatalf("old credential loads = %d, want 0", credentials.loads)
	}
	rendered := strings.Join(RenderLines(status), "\n")
	for _, want := range []string{"missing typed OAuth credential store", "lingotui login chatgpt", "ChatGPT/Codex runtime is not implemented yet"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered status missing %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "legacy-secret") {
		t.Fatalf("rendered status exposed legacy secret:\n%s", rendered)
	}
}

func TestServiceStatusReportsMissingLocalWhisperModelPathAsAttention(t *testing.T) {
	credentials := &fakeCredentialStore{secret: app.Secret{Value: "sk-openai"}}
	audio := &fakeAudioChecker{status: ItemStatus{Name: "Microphone", State: StateReady, Message: "ready"}}
	service := Service{
		ConfigPath:     fakePath("/tmp/lingotui/config.json"),
		CredentialPath: fakePath("/tmp/lingotui/auth.json"),
		Credentials:    credentials,
		AudioChecker:   audio,
		Config: app.Config{
			Provider:           app.ProviderOpenAI,
			TranscriptionModel: app.ModelRef{Provider: app.ProviderLocalWhisper, Name: "ggml-small.bin", Purpose: app.ModelPurposeTranscription},
			ChatModel:          app.ModelRef{Provider: app.ProviderOpenAI, Name: app.DefaultChatModel, Purpose: app.ModelPurposeChat},
		},
	}

	status := service.Status(context.Background())
	if status.Ready {
		t.Fatalf("status ready = true, want false until localwhisper runtime is implemented: %+v", status)
	}
	if credentials.loads != 1 || audio.checks != 1 {
		t.Fatalf("loads = %d checks = %d, want one passive status read each", credentials.loads, audio.checks)
	}
	rendered := strings.Join(RenderLines(status), "\n")
	for _, want := range []string{"LocalWhisper model", "missing local_whisper.model_path", "localwhisper runtime is not implemented yet", "Setup needs attention"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered status missing %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "sk-openai") || strings.Contains(rendered, "runtime ready") {
		t.Fatalf("rendered status is misleading or exposed a secret:\n%s", rendered)
	}
}

func TestServiceStatusReportsConfiguredLocalWhisperModelPathWithoutReadyClaim(t *testing.T) {
	service := Service{
		Credentials:  &fakeCredentialStore{secret: app.Secret{Value: "sk-openai"}},
		AudioChecker: &fakeAudioChecker{status: ItemStatus{Name: "Microphone", State: StateReady, Message: "ready"}},
		Config: app.Config{
			Provider:           app.ProviderOpenAI,
			TranscriptionModel: app.ModelRef{Provider: app.ProviderLocalWhisper, Name: "ggml-small.bin", Purpose: app.ModelPurposeTranscription},
			ChatModel:          app.ModelRef{Provider: app.ProviderOpenAI, Name: app.DefaultChatModel, Purpose: app.ModelPurposeChat},
			LocalWhisper:       app.LocalWhisperConfig{ModelPath: "/models/ggml-small.bin"},
		},
	}

	status := service.Status(context.Background())
	if status.Ready {
		t.Fatalf("status ready = true, want false until localwhisper runtime is implemented: %+v", status)
	}
	rendered := strings.Join(RenderLines(status), "\n")
	for _, want := range []string{"LocalWhisper model", "unknown", "configured in local_whisper.model_path", "/models/ggml-small.bin", "localwhisper runtime is not implemented yet"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered status missing %q:\n%s", want, rendered)
		}
	}
}

func TestServiceStatusFallsBackToUnknownWithoutCheckers(t *testing.T) {
	status := Service{}.Status(context.Background())
	if status.Ready {
		t.Fatalf("status ready = true, want false: %+v", status)
	}
	rendered := strings.Join(RenderLines(status), "\n")
	for _, want := range []string{"credential store unavailable", "permission status unknown"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered status missing %q:\n%s", want, rendered)
		}
	}
}

type fakePath string

func (p fakePath) Path() string { return string(p) }

type fakeCredentialStore struct {
	secret app.Secret
	err    error
	loads  int
}

func (s *fakeCredentialStore) Load(context.Context, app.ProviderID) (app.Secret, error) {
	s.loads++
	return s.secret, s.err
}

type fakeAuthCredentialStore struct {
	fakeCredentialStore
	credential      app.Credential
	credentialLoads int
}

func (s *fakeAuthCredentialStore) SaveCredential(context.Context, app.Credential) error { return nil }

func (s *fakeAuthCredentialStore) LoadCredential(context.Context, app.ProviderID, app.CredentialKind) (app.Credential, error) {
	s.credentialLoads++
	return s.credential, s.err
}

func (s *fakeAuthCredentialStore) DeleteCredential(context.Context, app.ProviderID, app.CredentialKind) error {
	return nil
}

type fakeAudioChecker struct {
	status ItemStatus
	checks int
}

func (c *fakeAudioChecker) Microphone(context.Context) ItemStatus {
	c.checks++
	return c.status
}
