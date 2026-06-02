package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

type FileStore struct{ path string }

func NewFileStore(baseDir string) (*FileStore, error) {
	dir, err := appConfigDir(baseDir)
	if err != nil {
		return nil, err
	}
	return &FileStore{path: filepath.Join(dir, FileName)}, nil
}

func (s *FileStore) Path() string { return s.path }

func (s *FileStore) Load(context.Context) (app.Config, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return app.Config{}, err
	}
	var cfg app.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return app.Config{}, err
	}
	return app.NormalizeConfig(cfg), nil
}

func (s *FileStore) Save(_ context.Context, cfg app.Config) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, append(data, '\n'), 0o600)
}

func appConfigDir(baseDir string) (string, error) {
	explicitBaseDir := baseDir != ""
	if baseDir == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		baseDir = filepath.Join(dir, "lingotui")
	}
	abs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	cwd, err := os.Getwd()
	if explicitBaseDir && err == nil && isWithin(abs, cwd) {
		return "", errors.New("config directory must be outside the project workspace")
	}
	return abs, nil
}

func isWithin(path, root string) bool {
	path, root = filepath.Clean(path), filepath.Clean(root)
	return path == root || strings.HasPrefix(path, root+string(os.PathSeparator))
}
