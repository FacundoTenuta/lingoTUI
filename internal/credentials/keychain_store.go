package credentials

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

const (
	keychainService             = "lingotui"
	legacyOpenAIKeychainAccount = "openai"
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
	return s.SaveCredential(ctx, app.Credential{Provider: provider, Kind: app.CredentialKindAPIKey, APIKey: secret})
}

func (s *KeychainStore) Load(ctx context.Context, provider app.ProviderID) (app.Secret, error) {
	credential, err := s.LoadCredential(ctx, provider, app.CredentialKindAPIKey)
	if err != nil {
		return app.Secret{}, err
	}
	secret := credential.APIKey
	if secret.Empty() {
		return app.Secret{}, ErrSecretNotFound
	}
	return secret, nil
}

func (s *KeychainStore) Delete(ctx context.Context, provider app.ProviderID) error {
	return s.DeleteCredential(ctx, provider, app.CredentialKindAPIKey)
}

func (s *KeychainStore) SaveCredential(ctx context.Context, credential app.Credential) error {
	if credential.Provider == "" || credential.Kind == "" {
		return ErrSecretNotFound
	}
	if s.backend == nil {
		return ErrStoreUnavailable
	}
	value, err := encodeKeychainCredential(credential)
	if err != nil {
		return err
	}
	return mapKeychainError(s.backend.Save(ctx, keychainService, credentialAccount(credential.Provider, credential.Kind), value))
}

func (s *KeychainStore) LoadCredential(ctx context.Context, provider app.ProviderID, kind app.CredentialKind) (app.Credential, error) {
	if provider == "" || kind == "" {
		return app.Credential{}, ErrSecretNotFound
	}
	if s.backend == nil {
		return app.Credential{}, ErrStoreUnavailable
	}
	value, err := s.backend.Load(ctx, keychainService, credentialAccount(provider, kind))
	if err != nil {
		mappedErr := mapKeychainError(err)
		if provider == app.ProviderOpenAI && kind == app.CredentialKindAPIKey && errors.Is(mappedErr, ErrSecretNotFound) {
			legacyValue, legacyErr := s.backend.Load(ctx, keychainService, legacyOpenAIKeychainAccount)
			if legacyErr == nil {
				return decodeKeychainCredential(provider, kind, legacyValue)
			}
			if !errors.Is(mapKeychainError(legacyErr), ErrSecretNotFound) {
				return app.Credential{}, mapKeychainError(legacyErr)
			}
		}
		return app.Credential{}, mappedErr
	}
	return decodeKeychainCredential(provider, kind, value)
}

func (s *KeychainStore) DeleteCredential(ctx context.Context, provider app.ProviderID, kind app.CredentialKind) error {
	if provider == "" || kind == "" {
		return ErrSecretNotFound
	}
	if s.backend == nil {
		return ErrStoreUnavailable
	}
	return mapKeychainError(s.backend.Delete(ctx, keychainService, credentialAccount(provider, kind)))
}

func encodeKeychainCredential(credential app.Credential) (string, error) {
	if credential.Kind == app.CredentialKindAPIKey {
		if credential.APIKey.Empty() {
			return "", ErrSecretNotFound
		}
		return credential.APIKey.Value, nil
	}
	data, err := json.Marshal(credential)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func decodeKeychainCredential(provider app.ProviderID, kind app.CredentialKind, value string) (app.Credential, error) {
	value = strings.TrimRight(value, "\r\n")
	if value == "" {
		return app.Credential{}, ErrSecretNotFound
	}
	if kind == app.CredentialKindAPIKey {
		return app.Credential{Provider: provider, Kind: kind, APIKey: app.Secret{Value: value}}, nil
	}
	var credential app.Credential
	if err := json.Unmarshal([]byte(value), &credential); err != nil {
		return app.Credential{}, err
	}
	if credential.Provider == "" {
		credential.Provider = provider
	}
	if credential.Kind == "" {
		credential.Kind = kind
	}
	return credential, nil
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
