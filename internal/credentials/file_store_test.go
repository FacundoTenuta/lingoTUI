package credentials

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

func TestFileStoreSavesLoadsAndDeletesSecret(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	secret := app.Secret{Value: "sk-test"}
	if err := store.Save(context.Background(), app.ProviderOpenAI, secret); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(context.Background(), app.ProviderOpenAI)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Value != secret.Value {
		t.Fatalf("secret = %q, want %q", loaded.Value, secret.Value)
	}
	if err := store.Delete(context.Background(), app.ProviderOpenAI); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(context.Background(), app.ProviderOpenAI); err != ErrSecretNotFound {
		t.Fatalf("error = %v, want %v", err, ErrSecretNotFound)
	}
}

func TestFileStoreUsesAuthJSONOutsideWorkspaceWithPrivateMode(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(store.Path()) != FileName {
		t.Fatalf("path = %q, want %s", store.Path(), FileName)
	}
	if strings.Contains(store.Path(), mustGetwd(t)) {
		t.Fatalf("credential path %q is inside workspace", store.Path())
	}
	if err := store.Save(context.Background(), app.ProviderOpenAI, app.Secret{Value: "sk-private"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestFileStoreTightensExistingAuthJSONPermissions(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(store.Path()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Path(), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), app.ProviderOpenAI, app.Secret{Value: "sk-private"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestSecretStringRedactsValue(t *testing.T) {
	secret := app.Secret{Value: "sk-test"}
	if got := fmt.Sprint(secret); strings.Contains(got, "sk-test") || got != "[redacted]" {
		t.Fatalf("printed secret = %q", got)
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
