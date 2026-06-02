package credentials

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

func TestKeychainStoreUsesBackendWithoutCommandArguments(t *testing.T) {
	backend := &recordingKeychainBackend{value: "sk-loaded\n", expectedAccount: "openai:api_key"}
	store := NewKeychainStore(backend)

	if err := store.Save(context.Background(), app.ProviderOpenAI, app.Secret{Value: "sk-test"}); err != nil {
		t.Fatal(err)
	}
	secret, err := store.Load(context.Background(), app.ProviderOpenAI)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(context.Background(), app.ProviderOpenAI); err != nil {
		t.Fatal(err)
	}

	if backend.savedSecret != "sk-test" {
		t.Fatalf("saved secret = %q, want key", backend.savedSecret)
	}
	if secret.Value != "sk-loaded" {
		t.Fatalf("secret = %q, want trimmed value", secret.Value)
	}
	for _, call := range []string{"save", "load", "delete"} {
		if backend.calls[call] != 1 {
			t.Fatalf("%s calls = %d, want 1", call, backend.calls[call])
		}
	}
}

func TestKeychainStoreUsesProviderSpecificAccountsAndOAuthJSON(t *testing.T) {
	expires := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	backend := &recordingKeychainBackend{expectedAccount: "chatgpt:oauth"}
	store := NewKeychainStore(backend)
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
	if !strings.Contains(backend.savedSecret, `"provider":"chatgpt"`) || !strings.Contains(backend.savedSecret, `"refresh_token":"refresh-token"`) {
		t.Fatal("saved OAuth JSON missing expected provider or refresh token fields")
	}
	backend.value = backend.savedSecret
	loaded, err := store.LoadCredential(context.Background(), app.ProviderChatGPT, app.CredentialKindOAuth)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.OAuth.AccessToken.Value != "access-token" || loaded.OAuth.RefreshToken.Value != "refresh-token" || loaded.OAuth.AccountID != "acct_123" || !loaded.OAuth.ExpiresAt.Equal(expires) {
		t.Fatalf("oauth credential = %+v", loaded)
	}
}

func TestKeychainStoreLoadsLegacyOpenAIAPIKeyAccount(t *testing.T) {
	backend := &multiAccountKeychainBackend{values: map[string]string{
		legacyOpenAIKeychainAccount: "sk-legacy\n",
	}}
	store := NewKeychainStore(backend)

	secret, err := store.Load(context.Background(), app.ProviderOpenAI)
	if err != nil {
		t.Fatal(err)
	}
	if secret.Value != "sk-legacy" {
		t.Fatalf("secret = %q, want legacy key", secret.Value)
	}
	if backend.loads["openai:api_key"] != 1 {
		t.Fatalf("current account loads = %d, want 1", backend.loads["openai:api_key"])
	}
	if backend.loads[legacyOpenAIKeychainAccount] != 1 {
		t.Fatalf("legacy account loads = %d, want 1", backend.loads[legacyOpenAIKeychainAccount])
	}
}

func TestKeychainStoreDoesNotFallbackToLegacyOnAccessDenied(t *testing.T) {
	backend := &multiAccountKeychainBackend{
		values: map[string]string{legacyOpenAIKeychainAccount: "sk-legacy"},
		errors: map[string]error{"openai:api_key": ErrKeychainAccessDenied},
	}
	store := NewKeychainStore(backend)

	_, err := store.Load(context.Background(), app.ProviderOpenAI)
	if !errors.Is(err, ErrKeychainAccessDenied) {
		t.Fatalf("error = %v, want access denied", err)
	}
	if backend.loads[legacyOpenAIKeychainAccount] != 0 {
		t.Fatalf("legacy account loads = %d, want 0 on access denied", backend.loads[legacyOpenAIKeychainAccount])
	}
}

func TestKeychainStoreMapsMissingUnavailableAndAccessDenied(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "missing explicit", err: ErrSecretNotFound, want: ErrSecretNotFound},
		{name: "missing message", err: errors.New("The specified item could not be found."), want: ErrSecretNotFound},
		{name: "unavailable", err: ErrStoreUnavailable, want: ErrStoreUnavailable},
		{name: "access denied", err: ErrKeychainAccessDenied, want: ErrKeychainAccessDenied},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewKeychainStore(&multiAccountKeychainBackend{errors: map[string]error{"openai:api_key": tt.err}})
			_, err := store.Load(context.Background(), app.ProviderOpenAI)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestKeychainStoreDoesNotMapAccessErrorsToUnavailable(t *testing.T) {
	permissionErr := errors.New("permission denied")
	store := NewKeychainStore(&recordingKeychainBackend{err: permissionErr, expectedAccount: "openai:api_key"})

	_, err := store.Load(context.Background(), app.ProviderOpenAI)
	if !errors.Is(err, permissionErr) {
		t.Fatalf("error = %v, want %v", err, permissionErr)
	}
}

type recordingKeychainBackend struct {
	calls           map[string]int
	value           string
	savedSecret     string
	err             error
	expectedAccount string
}

type multiAccountKeychainBackend struct {
	values map[string]string
	errors map[string]error
	loads  map[string]int
}

func (b *multiAccountKeychainBackend) Save(context.Context, string, string, string) error { return nil }

func (b *multiAccountKeychainBackend) Load(_ context.Context, service, account string) (string, error) {
	if service != keychainService {
		return "", errors.New("unexpected keychain service")
	}
	if b.loads == nil {
		b.loads = make(map[string]int)
	}
	b.loads[account]++
	if err := b.errors[account]; err != nil {
		return "", err
	}
	value, ok := b.values[account]
	if !ok {
		return "", ErrSecretNotFound
	}
	return value, nil
}

func (b *multiAccountKeychainBackend) Delete(context.Context, string, string) error { return nil }

func (b *recordingKeychainBackend) ensureCalls() {
	if b.calls == nil {
		b.calls = make(map[string]int)
	}
}

func (b *recordingKeychainBackend) Save(_ context.Context, service, account, secret string) error {
	b.ensureCalls()
	b.calls["save"]++
	if service != keychainService || account != b.expectedAccount {
		return errors.New("unexpected keychain target")
	}
	b.savedSecret = secret
	return b.err
}

func (b *recordingKeychainBackend) Load(_ context.Context, service, account string) (string, error) {
	b.ensureCalls()
	b.calls["load"]++
	if service != keychainService || account != b.expectedAccount {
		return "", errors.New("unexpected keychain target")
	}
	return b.value, b.err
}

func (b *recordingKeychainBackend) Delete(_ context.Context, service, account string) error {
	b.ensureCalls()
	b.calls["delete"]++
	if service != keychainService || account != b.expectedAccount {
		return errors.New("unexpected keychain target")
	}
	return b.err
}
