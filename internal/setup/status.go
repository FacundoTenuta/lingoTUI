package setup

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

type State string

const (
	StateReady   State = "ready"
	StateMissing State = "missing"
	StateUnknown State = "unknown"
)

type ItemStatus struct {
	Name     string
	Path     string
	Message  string
	State    State
	Secret   bool
	Optional bool
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

type CodexCLIChecker interface {
	CodexCLI(context.Context) ItemStatus
}

type Service struct {
	ConfigPath     PathProvider
	CredentialPath PathProvider
	Credentials    CredentialStore
	AudioChecker   AudioPermissionChecker
	CodexChecker   CodexCLIChecker
	Config         app.Config
	Provider       app.ProviderID
}

func (s Service) Status(ctx context.Context) Status {
	cfg := s.Config
	if cfg.Provider == "" {
		cfg.Provider = s.Provider
	}
	cfg = app.NormalizeConfig(cfg)

	items := []ItemStatus{configStatus(s.ConfigPath)}
	for _, provider := range credentialProviders(cfg) {
		items = append(items, s.credentialStatus(ctx, provider))
	}
	items = append(items, s.codexCLIStatus(ctx))
	items = append(items, s.microphoneStatus(ctx))
	if cfg.TranscriptionModel.Provider == app.ProviderLocalWhisper {
		items = append(items, localWhisperStatus(cfg.LocalWhisper))
	}
	ready := true
	for _, item := range items {
		if !item.Optional && item.State != StateReady {
			ready = false
			break
		}
	}
	if unsupportedRuntime(cfg) {
		ready = false
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

func localWhisperStatus(cfg app.LocalWhisperConfig) ItemStatus {
	item := ItemStatus{Name: "LocalWhisper model", State: StateMissing}
	if strings.TrimSpace(cfg.ModelPath) == "" {
		item.Message = "missing local_whisper.model_path"
		return item
	}
	item.Path = cfg.ModelPath
	item.State = StateReady
	item.Message = "configured in local_whisper.model_path; file is not verified until /stop"
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
		return chatGPTCredentialStatus(ctx, item, s.Credentials)
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

func chatGPTCredentialStatus(ctx context.Context, item ItemStatus, store CredentialStore) ItemStatus {
	authStore, ok := store.(app.AuthCredentialStore)
	if !ok {
		item.Message = "missing typed OAuth credential store; run lingotui login chatgpt"
		return item
	}
	credential, err := authStore.LoadCredential(ctx, app.ProviderChatGPT, app.CredentialKindOAuth)
	if err != nil || credential.Provider != app.ProviderChatGPT || credential.Kind != app.CredentialKindOAuth || !credential.OAuth.CanProvideAccess(time.Now(), 0) {
		item.Message = "missing; run lingotui login chatgpt"
		return item
	}
	item.State = StateReady
	item.Message = "configured via typed OAuth credential ([redacted]); token refresh is deferred until /stop or /ask"
	return item
}

func credentialName(provider app.ProviderID) string {
	if provider == app.ProviderChatGPT {
		return "ChatGPT Plus/Pro credentials"
	}
	if provider == app.ProviderLocalWhisper {
		return "LocalWhisper credentials"
	}
	return "OpenAI credentials"
}

func credentialUnavailableMessage(provider app.ProviderID) string {
	if provider == app.ProviderChatGPT {
		return "credential store unavailable; run lingotui login chatgpt"
	}
	return "credential store unavailable; run lingotui login openai or configure auth.json fallback before /connect"
}

func credentialProviders(cfg app.Config) []app.ProviderID {
	providers := make([]app.ProviderID, 0, 2)
	seen := map[app.ProviderID]bool{}
	for _, provider := range []app.ProviderID{cfg.TranscriptionModel.Provider, cfg.ChatModel.Provider} {
		if provider != app.ProviderOpenAI && provider != app.ProviderChatGPT {
			continue
		}
		if seen[provider] {
			continue
		}
		seen[provider] = true
		providers = append(providers, provider)
	}
	return providers
}

func unsupportedRuntime(cfg app.Config) bool {
	return app.UnsupportedRuntimeReason(cfg) != ""
}

func (s Service) codexCLIStatus(ctx context.Context) ItemStatus {
	if s.CodexChecker == nil {
		return ItemStatus{
			Name:     "Codex CLI",
			State:    StateUnknown,
			Message:  "optional alternative not checked; install codex and authenticate with Codex CLI when this path is enabled",
			Optional: true,
		}
	}
	item := s.CodexChecker.CodexCLI(ctx)
	if strings.TrimSpace(item.Name) == "" {
		item.Name = "Codex CLI"
	}
	if item.State == "" {
		item.State = StateUnknown
	}
	item.Optional = true
	return item
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
