package app

import "encoding/json"

type ProviderID string

const ProviderOpenAI ProviderID = "openai"

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

type Config struct {
	Provider           ProviderID `json:"provider"`
	TranscriptionModel ModelRef   `json:"transcription_model"`
	ChatModel          ModelRef   `json:"chat_model"`
	CredentialStorage  string     `json:"credential_storage"`
}

func DefaultConfig() Config {
	return Config{
		Provider:           ProviderOpenAI,
		TranscriptionModel: ModelRef{Provider: ProviderOpenAI, Name: DefaultTranscriptionModel, Purpose: ModelPurposeTranscription},
		ChatModel:          ModelRef{Provider: ProviderOpenAI, Name: DefaultChatModel, Purpose: ModelPurposeChat},
		CredentialStorage:  CredentialStorageFile,
	}
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
