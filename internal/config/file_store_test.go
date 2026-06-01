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

func mustGetwd(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return cwd
}
