package app

import (
	"encoding/json"
	"time"
)

type ProviderID string

const (
	ProviderOpenAI       ProviderID = "openai"
	ProviderChatGPT      ProviderID = "chatgpt"
	ProviderLocalWhisper ProviderID = "localwhisper"
)

type CredentialKind string

const (
	CredentialKindAPIKey CredentialKind = "api_key"
	CredentialKindOAuth  CredentialKind = "oauth"
)

type Credential struct {
	Provider ProviderID      `json:"provider"`
	Kind     CredentialKind  `json:"kind"`
	APIKey   Secret          `json:"api_key,omitempty"`
	OAuth    OAuthCredential `json:"oauth,omitempty"`
}

type OAuthCredential struct {
	AccessToken  Secret    `json:"access_token"`
	RefreshToken Secret    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	AccountID    string    `json:"account_id"`
}

func (c Credential) Redacted() Credential {
	if !c.APIKey.Empty() {
		c.APIKey = Secret{Value: c.APIKey.Redacted()}
	}
	if !c.OAuth.AccessToken.Empty() {
		c.OAuth.AccessToken = Secret{Value: c.OAuth.AccessToken.Redacted()}
	}
	if !c.OAuth.RefreshToken.Empty() {
		c.OAuth.RefreshToken = Secret{Value: c.OAuth.RefreshToken.Redacted()}
	}
	return c
}

func (c Credential) String() string {
	data, err := json.Marshal(c.Redacted())
	if err != nil {
		return "[redacted credential]"
	}
	return string(data)
}

func (c Credential) GoString() string { return c.String() }

func (c *Credential) UnmarshalJSON(data []byte) error {
	type credentialAlias Credential
	var aux struct {
		credentialAlias
		Secret Secret `json:"secret"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*c = Credential(aux.credentialAlias)
	if c.Kind == "" && !aux.Secret.Empty() {
		c.Kind = CredentialKindAPIKey
		c.APIKey = aux.Secret
	}
	return nil
}

type AudioSource string

const (
	AudioSourceMic    AudioSource = "mic"
	AudioSourceSystem AudioSource = "system"
	AudioSourceBoth   AudioSource = "both"
)

type ModelPurpose string

const (
	ModelPurposeTranscription ModelPurpose = "transcription"
	ModelPurposeChat          ModelPurpose = "chat"
)

const (
	DefaultTranscriptionModel = "gpt-4o-transcribe"
	DefaultChatModel          = "gpt-4o-mini"
	CredentialStorageFile     = "file"
)

type ModelRef struct {
	Provider ProviderID   `json:"provider"`
	Name     string       `json:"name"`
	Purpose  ModelPurpose `json:"purpose"`
}

type LocalWhisperConfig struct {
	BinaryPath string   `json:"binary_path,omitempty"`
	ModelPath  string   `json:"model_path,omitempty"`
	Language   string   `json:"language,omitempty"`
	ExtraArgs  []string `json:"extra_args,omitempty"`
}

type Config struct {
	Provider           ProviderID         `json:"provider"`
	TranscriptionModel ModelRef           `json:"transcription_model"`
	ChatModel          ModelRef           `json:"chat_model"`
	CredentialStorage  string             `json:"credential_storage"`
	LocalWhisper       LocalWhisperConfig `json:"local_whisper,omitempty"`
}

func DefaultConfig() Config {
	return Config{
		Provider:           ProviderOpenAI,
		TranscriptionModel: ModelRef{Provider: ProviderOpenAI, Name: DefaultTranscriptionModel, Purpose: ModelPurposeTranscription},
		ChatModel:          ModelRef{Provider: ProviderOpenAI, Name: DefaultChatModel, Purpose: ModelPurposeChat},
		CredentialStorage:  CredentialStorageFile,
	}
}

func NormalizeConfig(cfg Config) Config {
	defaults := DefaultConfig()
	if cfg.Provider == "" {
		cfg.Provider = defaults.Provider
	}
	if cfg.TranscriptionModel.Provider == "" {
		cfg.TranscriptionModel.Provider = cfg.Provider
	}
	if cfg.TranscriptionModel.Name == "" {
		cfg.TranscriptionModel.Name = DefaultTranscriptionModel
	}
	if cfg.TranscriptionModel.Purpose == "" {
		cfg.TranscriptionModel.Purpose = ModelPurposeTranscription
	}
	if cfg.ChatModel.Provider == "" {
		cfg.ChatModel.Provider = cfg.Provider
	}
	if cfg.ChatModel.Name == "" {
		cfg.ChatModel.Name = DefaultChatModel
	}
	if cfg.ChatModel.Purpose == "" {
		cfg.ChatModel.Purpose = ModelPurposeChat
	}
	if cfg.CredentialStorage == "" {
		cfg.CredentialStorage = CredentialStorageFile
	}
	return cfg
}

type Secret struct{ Value string }

func (s Secret) Empty() bool      { return s.Value == "" }
func (s Secret) String() string   { return s.Redacted() }
func (s Secret) GoString() string { return s.Redacted() }
func (s Secret) Redacted() string {
	if s.Value == "" {
		return ""
	}
	return "[redacted]"
}
func (s Secret) MarshalJSON() ([]byte, error)     { return json.Marshal(s.Value) }
func (s *Secret) UnmarshalJSON(data []byte) error { return json.Unmarshal(data, &s.Value) }

type AudioFile struct{ Path string }
type Transcript struct{ Text string }
type Language string

const (
	LanguageSpanish Language = "es"
	LanguageEnglish Language = "en"
	LanguageGerman  Language = "de"
)

func SummaryLanguages() []Language {
	return []Language{LanguageSpanish, LanguageEnglish, LanguageGerman}
}

type Summary map[Language]string
type Question string
type Answer string

type RecentContext struct {
	Transcript Transcript `json:"transcript"`
	Summary    Summary    `json:"summary"`
}
