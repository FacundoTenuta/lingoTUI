package credentials

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
	"github.com/FacundoTenuta/lingoTUI/internal/config"
)

type FileStore struct{ path string }

func NewFileStore(baseDir string) (*FileStore, error) {
	dir, err := credentialDir(baseDir)
	if err != nil {
		return nil, err
	}
	return &FileStore{path: filepath.Join(dir, FileName)}, nil
}

func (s *FileStore) Path() string { return s.path }

func (s *FileStore) Save(ctx context.Context, provider app.ProviderID, secret app.Secret) error {
	return s.SaveCredential(ctx, app.Credential{Provider: provider, Kind: app.CredentialKindAPIKey, APIKey: secret})
}

func (s *FileStore) Load(ctx context.Context, provider app.ProviderID) (app.Secret, error) {
	credential, err := s.LoadCredential(ctx, provider, app.CredentialKindAPIKey)
	if err != nil {
		return app.Secret{}, err
	}
	if credential.APIKey.Empty() {
		return app.Secret{}, ErrSecretNotFound
	}
	return credential.APIKey, nil
}

func (s *FileStore) Delete(ctx context.Context, provider app.ProviderID) error {
	return s.DeleteCredential(ctx, provider, app.CredentialKindAPIKey)
}

func (s *FileStore) SaveCredential(_ context.Context, credential app.Credential) error {
	if credential.Provider == "" || credential.Kind == "" {
		return ErrSecretNotFound
	}
	if credential.Kind == app.CredentialKindAPIKey && credential.APIKey.Empty() {
		return ErrSecretNotFound
	}
	if credential.Kind == app.CredentialKindOAuth && credential.OAuth.RefreshToken.Empty() && credential.OAuth.AccessToken.Empty() {
		return ErrSecretNotFound
	}
	records, err := s.readAll()
	if err != nil {
		return err
	}
	key := string(credential.Provider)
	if credential.Kind != app.CredentialKindAPIKey {
		key = credentialAccount(credential.Provider, credential.Kind)
	}
	records[key] = credential
	return s.writeAll(records)
}

func (s *FileStore) LoadCredential(_ context.Context, provider app.ProviderID, kind app.CredentialKind) (app.Credential, error) {
	records, err := s.readAll()
	if err != nil {
		return app.Credential{}, err
	}
	for _, key := range []string{credentialAccount(provider, kind), string(provider)} {
		credential, ok := records[key]
		if !ok || credential.Kind != kind {
			continue
		}
		if credential.Provider == "" {
			credential.Provider = provider
		}
		return credential, nil
	}
	return app.Credential{}, ErrSecretNotFound
}

func (s *FileStore) DeleteCredential(_ context.Context, provider app.ProviderID, kind app.CredentialKind) error {
	records, err := s.readAll()
	if err != nil {
		return err
	}
	delete(records, credentialAccount(provider, kind))
	if kind == app.CredentialKindAPIKey {
		delete(records, string(provider))
	}
	return s.writeAll(records)
}

func (s *FileStore) readAll() (map[string]app.Credential, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]app.Credential{}, nil
	}
	if err != nil {
		return nil, err
	}
	records := map[string]app.Credential{}
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	for key, credential := range records {
		if credential.Provider == "" {
			credential.Provider = app.ProviderID(key)
		}
		records[key] = credential
	}
	return records, nil
}

func (s *FileStore) writeAll(records map[string]app.Credential) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.path, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Chmod(s.path, 0o600)
}

func credentialDir(baseDir string) (string, error) {
	store, err := config.NewFileStore(baseDir)
	if err != nil {
		return "", err
	}
	return filepath.Dir(store.Path()), nil
}
