package credentials

import (
	"context"
	"errors"
	"testing"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

func TestCompositeStoreLoadsPrimaryBeforeFallback(t *testing.T) {
	primary := &fakeStore{secret: app.Secret{Value: "sk-keychain"}}
	fallback := &fakeStore{secret: app.Secret{Value: "sk-file"}}
	store := NewCompositeStore(primary, fallback, "auth path")

	secret, err := store.Load(context.Background(), app.ProviderOpenAI)
	if err != nil {
		t.Fatal(err)
	}
	if secret.Value != "sk-keychain" {
		t.Fatalf("secret = %q, want primary", secret.Value)
	}
	if primary.loads != 1 || fallback.loads != 0 {
		t.Fatalf("loads primary=%d fallback=%d", primary.loads, fallback.loads)
	}
}

func TestCompositeStoreFallsBackWhenPrimaryMissingOrUnavailable(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "missing", err: ErrSecretNotFound},
		{name: "unavailable", err: ErrStoreUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			primary := &fakeStore{err: tt.err}
			fallback := &fakeStore{secret: app.Secret{Value: "sk-file"}}
			store := NewCompositeStore(primary, fallback, "auth path")

			secret, err := store.Load(context.Background(), app.ProviderOpenAI)
			if err != nil {
				t.Fatal(err)
			}
			if secret.Value != "sk-file" {
				t.Fatalf("secret = %q, want fallback", secret.Value)
			}
			if primary.loads != 1 || fallback.loads != 1 {
				t.Fatalf("loads primary=%d fallback=%d", primary.loads, fallback.loads)
			}
		})
	}
}

func TestCompositeStoreDoesNotFallbackOnKeychainAccessDenied(t *testing.T) {
	primary := &fakeStore{err: ErrKeychainAccessDenied}
	fallback := &fakeStore{secret: app.Secret{Value: "sk-file"}}
	store := NewCompositeStore(primary, fallback, "auth path")

	_, err := store.Load(context.Background(), app.ProviderOpenAI)
	if !errors.Is(err, ErrKeychainAccessDenied) {
		t.Fatalf("error = %v, want %v", err, ErrKeychainAccessDenied)
	}
	if primary.loads != 1 || fallback.loads != 0 {
		t.Fatalf("loads primary=%d fallback=%d", primary.loads, fallback.loads)
	}
}

func TestCompositeStoreSaveWritesPrimaryOnly(t *testing.T) {
	primary := &fakeStore{}
	fallback := &fakeStore{}
	store := NewCompositeStore(primary, fallback, "auth path")

	if err := store.Save(context.Background(), app.ProviderOpenAI, app.Secret{Value: "sk-save"}); err != nil {
		t.Fatal(err)
	}
	if primary.saves != 1 || fallback.saves != 0 {
		t.Fatalf("saves primary=%d fallback=%d", primary.saves, fallback.saves)
	}
	if primary.secret.Value != "sk-save" {
		t.Fatalf("primary secret = %q, want saved value", primary.secret.Value)
	}
}

func TestCompositeStoreDeleteAttemptsBothAndIgnoresNotFoundUnavailable(t *testing.T) {
	primary := &fakeStore{err: ErrSecretNotFound}
	fallback := &fakeStore{err: ErrStoreUnavailable}
	store := NewCompositeStore(primary, fallback, "auth path")

	if err := store.Delete(context.Background(), app.ProviderOpenAI); err != nil {
		t.Fatal(err)
	}
	if primary.deletes != 1 || fallback.deletes != 1 {
		t.Fatalf("deletes primary=%d fallback=%d", primary.deletes, fallback.deletes)
	}
}

func TestCompositeStoreDeleteReturnsRealErrorsAfterAttemptingBoth(t *testing.T) {
	primaryErr := errors.New("permission denied")
	primary := &fakeStore{err: primaryErr}
	fallback := &fakeStore{}
	store := NewCompositeStore(primary, fallback, "auth path")

	if err := store.Delete(context.Background(), app.ProviderOpenAI); !errors.Is(err, primaryErr) {
		t.Fatalf("error = %v, want %v", err, primaryErr)
	}
	if primary.deletes != 1 || fallback.deletes != 1 {
		t.Fatalf("deletes primary=%d fallback=%d", primary.deletes, fallback.deletes)
	}
}

type fakeStore struct {
	secret  app.Secret
	err     error
	loads   int
	saves   int
	deletes int
}

func (s *fakeStore) Save(_ context.Context, _ app.ProviderID, secret app.Secret) error {
	s.saves++
	s.secret = secret
	return s.err
}

func (s *fakeStore) Load(context.Context, app.ProviderID) (app.Secret, error) {
	s.loads++
	return s.secret, s.err
}

func (s *fakeStore) Delete(context.Context, app.ProviderID) error {
	s.deletes++
	return s.err
}
