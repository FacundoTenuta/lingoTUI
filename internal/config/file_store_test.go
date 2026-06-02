package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

func TestFileStoreLoadsDefaultsWhenMissing(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TranscriptionModel.Name != app.DefaultTranscriptionModel || cfg.ChatModel.Name != app.DefaultChatModel {
		t.Fatalf("config defaults = %+v", cfg)
	}
}

func TestFileStoreSavesAndLoadsConfigOutsideWorkspace(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(store.Path(), mustGetwd(t)) {
		t.Fatalf("config path %q is inside workspace", store.Path())
	}

	cfg := Default()
	cfg.ChatModel.Name = "custom-chat"
	if err := store.Save(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ChatModel.Name != "custom-chat" {
		t.Fatalf("loaded config = %+v", loaded)
	}
	if filepath.Base(store.Path()) != FileName {
		t.Fatalf("path = %q, want %s", store.Path(), FileName)
	}
}

func TestFileStoreNormalizesOldConfigMissingModelProviders(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	oldConfig := `{
  "provider": "chatgpt",
  "transcription_model": {"name": "legacy-transcribe", "purpose": "transcription"},
  "chat_model": {"name": "legacy-chat", "purpose": "chat"}
}
`
	if err := os.MkdirAll(filepath.Dir(store.Path()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Path(), []byte(oldConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Provider != app.ProviderChatGPT || loaded.TranscriptionModel.Provider != app.ProviderChatGPT || loaded.ChatModel.Provider != app.ProviderChatGPT {
		t.Fatalf("loaded config = %+v", loaded)
	}
	if loaded.CredentialStorage != app.CredentialStorageFile {
		t.Fatalf("credential storage = %q, want default", loaded.CredentialStorage)
	}
}

func TestFileStoreRoundTripsLocalWhisperConfigAndMixedModelProviders(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := Default()
	cfg.TranscriptionModel = app.ModelRef{Provider: app.ProviderLocalWhisper, Name: "ggml-small.bin", Purpose: app.ModelPurposeTranscription}
	cfg.ChatModel = app.ModelRef{Provider: app.ProviderChatGPT, Name: "codex-mini", Purpose: app.ModelPurposeChat}
	cfg.LocalWhisper = app.LocalWhisperConfig{
		BinaryPath: "/opt/whisper/main",
		ModelPath:  "/models/ggml-small.bin",
		Language:   "es",
		ExtraArgs:  []string{"--threads", "4"},
	}

	if err := store.Save(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TranscriptionModel.Provider != app.ProviderLocalWhisper || loaded.ChatModel.Provider != app.ProviderChatGPT {
		t.Fatalf("loaded model providers = transcription %q chat %q", loaded.TranscriptionModel.Provider, loaded.ChatModel.Provider)
	}
	if loaded.LocalWhisper.ModelPath != "/models/ggml-small.bin" || strings.Join(loaded.LocalWhisper.ExtraArgs, " ") != "--threads 4" {
		t.Fatalf("loaded local_whisper = %+v", loaded.LocalWhisper)
	}
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return cwd
}
