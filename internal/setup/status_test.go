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
			want:      []string{"missing", "lingotui login", "auth.json fallback", "/connect", "/record mic", "Nothing records or calls OpenAI"},
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

type fakeAudioChecker struct {
	status ItemStatus
	checks int
}

func (c *fakeAudioChecker) Microphone(context.Context) ItemStatus {
	c.checks++
	return c.status
}
