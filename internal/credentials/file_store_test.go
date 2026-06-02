package credentials

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestFileStoreSavesLoadsAPIKeyTypedCredential(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	credential := app.Credential{Provider: app.ProviderOpenAI, Kind: app.CredentialKindAPIKey, APIKey: app.Secret{Value: "sk-typed"}}

	if err := store.SaveCredential(context.Background(), credential); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadCredential(context.Background(), app.ProviderOpenAI, app.CredentialKindAPIKey)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Provider != app.ProviderOpenAI || loaded.Kind != app.CredentialKindAPIKey || loaded.APIKey.Value != "sk-typed" {
		t.Fatalf("credential = %+v", loaded)
	}
}

func TestFileStoreSavesLoadsOAuthCredentialAndRedactsTokens(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	credential := app.Credential{
		Provider: app.ProviderChatGPT,
		Kind:     app.CredentialKindOAuth,
		OAuth: app.OAuthCredential{
			AccessToken:  app.Secret{Value: "access-token"},
			RefreshToken: app.Secret{Value: "refresh-token"},
			ExpiresAt:    expires,
			AccountID:    "acct_123",
		},
	}

	if err := store.SaveCredential(context.Background(), credential); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadCredential(context.Background(), app.ProviderChatGPT, app.CredentialKindOAuth)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.OAuth.AccessToken.Value != "access-token" || loaded.OAuth.RefreshToken.Value != "refresh-token" || loaded.OAuth.AccountID != "acct_123" || !loaded.OAuth.ExpiresAt.Equal(expires) {
		t.Fatalf("oauth credential = %+v", loaded)
	}
	printed := fmt.Sprint(loaded)
	if strings.Contains(printed, "access-token") || strings.Contains(printed, "refresh-token") || !strings.Contains(printed, "[redacted]") {
		t.Fatalf("printed credential = %q, want redacted tokens", printed)
	}
}

func TestFileStoreLoadsOldAuthJSONSecretFormat(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(store.Path()), 0o700); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"openai":{"provider":"openai","secret":"sk-legacy"}}` + "\n")
	if err := os.WriteFile(store.Path(), data, 0o600); err != nil {
		t.Fatal(err)
	}

	secret, err := store.Load(context.Background(), app.ProviderOpenAI)
	if err != nil {
		t.Fatal(err)
	}
	if secret.Value != "sk-legacy" {
		t.Fatalf("secret = %q, want legacy", secret.Value)
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
