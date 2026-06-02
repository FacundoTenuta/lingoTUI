package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
	"github.com/FacundoTenuta/lingoTUI/internal/audio"
	"github.com/FacundoTenuta/lingoTUI/internal/config"
	memory "github.com/FacundoTenuta/lingoTUI/internal/context"
	"github.com/FacundoTenuta/lingoTUI/internal/credentials"
	"github.com/FacundoTenuta/lingoTUI/internal/provider/openai"
	"github.com/FacundoTenuta/lingoTUI/internal/setup"
	"github.com/FacundoTenuta/lingoTUI/internal/tui"
)

type providerClient interface {
	app.Transcriber
	app.Chat
}

type runtimeOptions struct {
	newProvider     func(app.Secret) (providerClient, error)
	newRecorder     func() app.Recorder
	audioChecker    setup.AudioPermissionChecker
	credentialStore app.CredentialStore
	credentialPath  setup.PathProvider
}

func buildRuntime(baseDir string) (tui.Model, error) {
	return buildRuntimeWithOptions(baseDir, defaultRuntimeOptions())
}

func buildRuntimeWithOptions(baseDir string, options runtimeOptions) (tui.Model, error) {
	options = normalizeRuntimeOptions(options)
	ctx := context.Background()

	configStore, err := config.NewFileStore(baseDir)
	if err != nil {
		return tui.Model{}, fmt.Errorf("config store: %w", err)
	}
	credentialStore := options.credentialStore
	credentialPath := options.credentialPath
	if credentialStore == nil {
		store, err := buildCredentialStore(baseDir)
		if err != nil {
			return tui.Model{}, fmt.Errorf("credential store: %w", err)
		}
		credentialStore = store
		credentialPath = store
	}
	cfg, err := configStore.Load(ctx)
	if err != nil {
		return tui.Model{}, fmt.Errorf("load config: %w", err)
	}

	setupService := setup.Service{
		ConfigPath:     configStore,
		CredentialPath: credentialPath,
		Credentials:    credentialStore,
		AudioChecker:   options.audioChecker,
		Provider:       cfg.Provider,
	}
	setupLines := setup.RenderLines(setupService.Status(ctx))

	var provider providerClient
	if cfg.Provider != app.ProviderChatGPT {
		secret, err := credentialStore.Load(ctx, cfg.Provider)
		if err == nil && !secret.Empty() {
			provider, err = options.newProvider(secret)
			if err != nil {
				return tui.Model{}, fmt.Errorf("provider %s: %w", cfg.Provider, err)
			}
		}
	}

	service := app.NewService(app.Dependencies{
		Recorder:      options.newRecorder(),
		Transcriber:   provider,
		Chat:          provider,
		Config:        configStore,
		Credentials:   credentialStore,
		Context:       memory.NewMemory(),
		SetupGuidance: setupLines,
	})
	return tui.NewModel(service, setupLines...), nil
}

func buildCredentialStore(baseDir string) (*credentials.CompositeStore, error) {
	fileStore, err := credentials.NewFileStore(baseDir)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("macOS Keychain primary; auth.json fallback: %s", fileStore.Path())
	return credentials.NewCompositeStore(credentials.NewKeychainStore(nil), fileStore, path), nil
}

func defaultRuntimeOptions() runtimeOptions {
	return runtimeOptions{
		newProvider: func(secret app.Secret) (providerClient, error) {
			return openai.NewClient(secret.Value)
		},
		newRecorder: func() app.Recorder {
			return audio.NewFFmpegRecorder(
				audio.WithInputDevice(os.Getenv("LINGOTUI_FFMPEG_MIC_DEVICE")),
				audio.WithTempDir(filepath.Join(os.TempDir(), "lingotui")),
			)
		},
		audioChecker: audio.NewMicrophonePermissionChecker(),
	}
}

func normalizeRuntimeOptions(options runtimeOptions) runtimeOptions {
	defaults := defaultRuntimeOptions()
	if options.newProvider == nil {
		options.newProvider = defaults.newProvider
	}
	if options.newRecorder == nil {
		options.newRecorder = defaults.newRecorder
	}
	if options.audioChecker == nil {
		options.audioChecker = defaults.audioChecker
	}
	if options.credentialPath == nil {
		if path, ok := options.credentialStore.(setup.PathProvider); ok {
			options.credentialPath = path
		}
	}
	return options
}
