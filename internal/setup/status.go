package setup

import (
	"context"
	"fmt"
	"strings"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

type State string

const (
	StateReady   State = "ready"
	StateMissing State = "missing"
	StateUnknown State = "unknown"
)

type ItemStatus struct {
	Name    string
	Path    string
	Message string
	State   State
	Secret  bool
}

type Status struct {
	Items []ItemStatus
	Ready bool
}

type CredentialStore interface {
	Load(context.Context, app.ProviderID) (app.Secret, error)
}

type PathProvider interface {
	Path() string
}

type AudioPermissionChecker interface {
	Microphone(context.Context) ItemStatus
}

type Service struct {
	ConfigPath     PathProvider
	CredentialPath PathProvider
	Credentials    CredentialStore
	AudioChecker   AudioPermissionChecker
	Provider       app.ProviderID
}

func (s Service) Status(ctx context.Context) Status {
	provider := s.Provider
	if provider == "" {
		provider = app.ProviderOpenAI
	}

	items := []ItemStatus{
		configStatus(s.ConfigPath),
		s.credentialStatus(ctx, provider),
		s.microphoneStatus(ctx),
	}
	ready := true
	for _, item := range items {
		if item.State != StateReady {
			ready = false
			break
		}
	}
	return Status{Items: items, Ready: ready}
}

func RenderLines(status Status) []string {
	lines := []string{"Setup status:"}
	if status.Ready {
		lines = append(lines, "Setup ready. Use /connect, then /record mic when you choose to record.")
	} else {
		lines = append(lines, "Setup needs attention. Nothing records, opens a browser, or calls a provider until you run an explicit command.")
	}
	for _, item := range status.Items {
		lines = append(lines, renderItem(item))
	}
	return lines
}

func configStatus(path PathProvider) ItemStatus {
	item := ItemStatus{Name: "Config", State: StateReady, Message: "defaults are available"}
	if path != nil {
		item.Path = path.Path()
		if item.Path != "" {
			item.Message = "using local config path"
		}
	}
	return item
}

func (s Service) credentialStatus(ctx context.Context, provider app.ProviderID) ItemStatus {
	item := ItemStatus{
		Name:   credentialName(provider),
		State:  StateMissing,
		Secret: true,
	}
	if s.CredentialPath != nil {
		item.Path = s.CredentialPath.Path()
	}
	if s.Credentials == nil {
		item.State = StateUnknown
		item.Message = credentialUnavailableMessage(provider)
		return item
	}
	if provider == app.ProviderChatGPT {
		item.Message = "ChatGPT Plus/Pro OAuth is scaffolded but not implemented yet; lingotui login chatgpt will not open a browser or save credentials"
		return item
	}
	secret, err := s.Credentials.Load(ctx, provider)
	if err != nil || secret.Empty() {
		item.Message = "missing; run lingotui login openai or configure auth.json fallback before /connect"
		return item
	}
	item.State = StateReady
	item.Message = "configured via Keychain/auth.json fallback ([redacted]); run /connect when ready"
	return item
}

func credentialName(provider app.ProviderID) string {
	if provider == app.ProviderChatGPT {
		return "ChatGPT Plus/Pro credentials"
	}
	return "OpenAI credentials"
}

func credentialUnavailableMessage(provider app.ProviderID) string {
	if provider == app.ProviderChatGPT {
		return "credential store unavailable; ChatGPT Plus/Pro OAuth is scaffolded but not implemented yet"
	}
	return "credential store unavailable; run lingotui login openai or configure auth.json fallback before /connect"
}

func (s Service) microphoneStatus(ctx context.Context) ItemStatus {
	if s.AudioChecker == nil {
		return ItemStatus{
			Name:    "Microphone",
			State:   StateUnknown,
			Message: "permission status unknown; grant macOS microphone access before /record mic",
		}
	}
	item := s.AudioChecker.Microphone(ctx)
	if strings.TrimSpace(item.Name) == "" {
		item.Name = "Microphone"
	}
	if item.State == "" {
		item.State = StateUnknown
	}
	return item
}

func renderItem(item ItemStatus) string {
	state := item.State
	if state == "" {
		state = StateUnknown
	}
	message := strings.TrimSpace(item.Message)
	if message == "" {
		message = string(state)
	}
	if item.Path != "" {
		message = fmt.Sprintf("%s (path: %s)", message, item.Path)
	}
	return fmt.Sprintf("- %s: %s — %s", item.Name, state, message)
}
