package credentials

import (
	"context"
	"errors"
	"testing"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

func TestKeychainStoreUsesBackendWithoutCommandArguments(t *testing.T) {
	backend := &recordingKeychainBackend{value: "sk-loaded\n"}
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
			store := NewKeychainStore(&recordingKeychainBackend{err: tt.err})
			_, err := store.Load(context.Background(), app.ProviderOpenAI)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestKeychainStoreDoesNotMapAccessErrorsToUnavailable(t *testing.T) {
	permissionErr := errors.New("permission denied")
	store := NewKeychainStore(&recordingKeychainBackend{err: permissionErr})

	_, err := store.Load(context.Background(), app.ProviderOpenAI)
	if !errors.Is(err, permissionErr) {
		t.Fatalf("error = %v, want %v", err, permissionErr)
	}
}

type recordingKeychainBackend struct {
	calls       map[string]int
	value       string
	savedSecret string
	err         error
}

func (b *recordingKeychainBackend) ensureCalls() {
	if b.calls == nil {
		b.calls = make(map[string]int)
	}
}

func (b *recordingKeychainBackend) Save(_ context.Context, service, account, secret string) error {
	b.ensureCalls()
	b.calls["save"]++
	if service != keychainService || account != keychainAccount {
		return errors.New("unexpected keychain target")
	}
	b.savedSecret = secret
	return b.err
}

func (b *recordingKeychainBackend) Load(_ context.Context, service, account string) (string, error) {
	b.ensureCalls()
	b.calls["load"]++
	if service != keychainService || account != keychainAccount {
		return "", errors.New("unexpected keychain target")
	}
	return b.value, b.err
}

func (b *recordingKeychainBackend) Delete(_ context.Context, service, account string) error {
	b.ensureCalls()
	b.calls["delete"]++
	if service != keychainService || account != keychainAccount {
		return errors.New("unexpected keychain target")
	}
	return b.err
}
