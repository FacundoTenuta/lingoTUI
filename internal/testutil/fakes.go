package testutil

import (
	"context"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

var (
	_ app.Recorder         = (*Recorder)(nil)
	_ app.RecordingCleaner = (*Recorder)(nil)
	_ app.ChunkRecorder    = (*ChunkRecorder)(nil)
	_ app.Transcriber      = Provider{}
	_ app.Chat             = Provider{}
	_ app.ConfigStore      = (*ConfigStore)(nil)
	_ app.CredentialStore  = (*CredentialStore)(nil)
	_ app.ContextStore     = (*ContextStore)(nil)
)

type Recorder struct {
	Started      app.AudioSource
	File         app.AudioFile
	StartErr     error
	StopErr      error
	CleanupCalls int
	CleanedFile  app.AudioFile
	CleanupErr   error
}

func (r *Recorder) Start(_ context.Context, source app.AudioSource) error {
	r.Started = source
	return r.StartErr
}

func (r *Recorder) Stop(context.Context) (app.AudioFile, error) { return r.File, r.StopErr }

func (r *Recorder) Cleanup(_ context.Context, file app.AudioFile) error {
	r.CleanupCalls++
	r.CleanedFile = file
	return r.CleanupErr
}

type ChunkRecorder struct {
	Started           app.AudioSource
	Chunks            []app.AudioFile
	Final             []app.AudioFile
	StartCalls        int
	NextCalls         int
	StopCalls         int
	CleanupCalls      int
	CleanupChunkCalls int
	CleanedChunks     []app.AudioFile
	StartErr          error
	NextErr           error
	StopErr           error
	CleanupErr        error
}

func (r *ChunkRecorder) Start(_ context.Context, source app.AudioSource) error {
	r.StartCalls++
	r.Started = source
	return r.StartErr
}

func (r *ChunkRecorder) NextChunk(context.Context) (app.AudioFile, bool, error) {
	r.NextCalls++
	if r.NextErr != nil {
		return app.AudioFile{}, false, r.NextErr
	}
	if len(r.Chunks) == 0 {
		return app.AudioFile{}, false, nil
	}
	chunk := r.Chunks[0]
	r.Chunks = r.Chunks[1:]
	return chunk, true, nil
}

func (r *ChunkRecorder) Stop(context.Context) ([]app.AudioFile, error) {
	r.StopCalls++
	return append([]app.AudioFile(nil), r.Final...), r.StopErr
}

func (r *ChunkRecorder) CleanupChunk(_ context.Context, file app.AudioFile) error {
	r.CleanupChunkCalls++
	r.CleanedChunks = append(r.CleanedChunks, file)
	return r.CleanupErr
}

func (r *ChunkRecorder) Cleanup(context.Context) error {
	r.CleanupCalls++
	return r.CleanupErr
}

type Provider struct {
	Transcript   app.Transcript
	Summary      app.Summary
	Translations app.Translations
	AnswerText   app.Answer
	Err          error
}

func (p Provider) Transcribe(context.Context, app.AudioFile, app.ModelRef) (app.Transcript, error) {
	return p.Transcript, p.Err
}

func (p Provider) Summarize(context.Context, app.Transcript, []app.Language, app.ModelRef) (app.Summary, error) {
	return p.Summary, p.Err
}

func (p Provider) Answer(context.Context, app.Question, app.RecentContext, app.ModelRef) (app.Answer, error) {
	return p.AnswerText, p.Err
}

func (p Provider) Translate(context.Context, string, []app.Language, app.ModelRef) (app.Translations, error) {
	return p.Translations, p.Err
}

type ConfigStore struct {
	Config app.Config
	Err    error
}

func (s *ConfigStore) Load(context.Context) (app.Config, error)     { return s.Config, s.Err }
func (s *ConfigStore) Save(_ context.Context, cfg app.Config) error { s.Config = cfg; return s.Err }

type CredentialStore struct {
	Secrets map[app.ProviderID]app.Secret
	Err     error
}

func (s *CredentialStore) Save(_ context.Context, provider app.ProviderID, secret app.Secret) error {
	if s.Secrets == nil {
		s.Secrets = map[app.ProviderID]app.Secret{}
	}
	s.Secrets[provider] = secret
	return s.Err
}

func (s *CredentialStore) Load(_ context.Context, provider app.ProviderID) (app.Secret, error) {
	return s.Secrets[provider], s.Err
}

func (s *CredentialStore) Delete(_ context.Context, provider app.ProviderID) error {
	delete(s.Secrets, provider)
	return s.Err
}

type ContextStore struct {
	Context app.RecentContext
	Has     bool
}

func (s *ContextStore) Replace(ctx app.RecentContext)      { s.Context, s.Has = ctx, true }
func (s *ContextStore) Current() (app.RecentContext, bool) { return s.Context, s.Has }
func (s *ContextStore) Clear()                             { s.Context, s.Has = app.RecentContext{}, false }
