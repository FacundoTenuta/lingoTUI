package credentials

import (
	"context"
	"errors"
	"strings"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

const (
	keychainService = "lingotui"
	keychainAccount = "openai"
)

var ErrKeychainAccessDenied = errors.New("keychain access denied")

type KeychainBackend interface {
	Save(context.Context, string, string, string) error
	Load(context.Context, string, string) (string, error)
	Delete(context.Context, string, string) error
}

type KeychainStore struct {
	backend KeychainBackend
}

func NewKeychainStore(backend KeychainBackend) *KeychainStore {
	if backend == nil {
		backend = defaultKeychainBackend()
	}
	return &KeychainStore{backend: backend}
}

func (s *KeychainStore) Save(ctx context.Context, provider app.ProviderID, secret app.Secret) error {
	if provider != app.ProviderOpenAI || secret.Empty() {
		return ErrSecretNotFound
	}
	if s.backend == nil {
		return ErrStoreUnavailable
	}
	return mapKeychainError(s.backend.Save(ctx, keychainService, keychainAccount, secret.Value))
}

func (s *KeychainStore) Load(ctx context.Context, provider app.ProviderID) (app.Secret, error) {
	if provider != app.ProviderOpenAI {
		return app.Secret{}, ErrSecretNotFound
	}
	if s.backend == nil {
		return app.Secret{}, ErrStoreUnavailable
	}
	value, err := s.backend.Load(ctx, keychainService, keychainAccount)
	if err != nil {
		return app.Secret{}, mapKeychainError(err)
	}
	secret := app.Secret{Value: strings.TrimRight(value, "\r\n")}
	if secret.Empty() {
		return app.Secret{}, ErrSecretNotFound
	}
	return secret, nil
}

func (s *KeychainStore) Delete(ctx context.Context, provider app.ProviderID) error {
	if provider != app.ProviderOpenAI {
		return ErrSecretNotFound
	}
	if s.backend == nil {
		return ErrStoreUnavailable
	}
	return mapKeychainError(s.backend.Delete(ctx, keychainService, keychainAccount))
}

func mapKeychainError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrSecretNotFound) || errors.Is(err, ErrStoreUnavailable) || errors.Is(err, ErrKeychainAccessDenied) {
		return err
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "could not be found") || strings.Contains(message, "not found") {
		return ErrSecretNotFound
	}
	return err
}
