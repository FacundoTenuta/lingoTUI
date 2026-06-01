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

func (s *FileStore) Save(_ context.Context, provider app.ProviderID, secret app.Secret) error {
	records, err := s.readAll()
	if err != nil {
		return err
	}
	records[provider] = Record{Provider: provider, Secret: secret}
	return s.writeAll(records)
}

func (s *FileStore) Load(_ context.Context, provider app.ProviderID) (app.Secret, error) {
	records, err := s.readAll()
	if err != nil {
		return app.Secret{}, err
	}
	record, ok := records[provider]
	if !ok || record.Secret.Empty() {
		return app.Secret{}, ErrSecretNotFound
	}
	return record.Secret, nil
}

func (s *FileStore) Delete(_ context.Context, provider app.ProviderID) error {
	records, err := s.readAll()
	if err != nil {
		return err
	}
	delete(records, provider)
	return s.writeAll(records)
}

func (s *FileStore) readAll() (map[app.ProviderID]Record, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return map[app.ProviderID]Record{}, nil
	}
	if err != nil {
		return nil, err
	}
	records := map[app.ProviderID]Record{}
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *FileStore) writeAll(records map[app.ProviderID]Record) error {
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
