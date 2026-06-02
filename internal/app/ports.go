package app

import "context"

type Recorder interface {
	Start(context.Context, AudioSource) error
	Stop(context.Context) (AudioFile, error)
}

type ChunkRecorder interface {
	Start(context.Context, AudioSource) error
	NextChunk(context.Context) (AudioFile, bool, error)
	Stop(context.Context) ([]AudioFile, error)
}

type ChunkCleaner interface {
	CleanupChunk(context.Context, AudioFile) error
	Cleanup(context.Context) error
}

type Transcriber interface {
	Transcribe(context.Context, AudioFile, ModelRef) (Transcript, error)
}

type Chat interface {
	Summarize(context.Context, Transcript, []Language, ModelRef) (Summary, error)
	Translate(context.Context, string, []Language, ModelRef) (Translations, error)
	Answer(context.Context, Question, RecentContext, ModelRef) (Answer, error)
}

type CredentialStore interface {
	Save(context.Context, ProviderID, Secret) error
	Load(context.Context, ProviderID) (Secret, error)
	Delete(context.Context, ProviderID) error
}

type AuthCredentialStore interface {
	SaveCredential(context.Context, Credential) error
	LoadCredential(context.Context, ProviderID, CredentialKind) (Credential, error)
	DeleteCredential(context.Context, ProviderID, CredentialKind) error
}

type ConfigStore interface {
	Load(context.Context) (Config, error)
	Save(context.Context, Config) error
}

type ContextStore interface {
	Replace(RecentContext)
	Current() (RecentContext, bool)
	Clear()
}
