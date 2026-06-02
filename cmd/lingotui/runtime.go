package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
	"github.com/FacundoTenuta/lingoTUI/internal/audio"
	"github.com/FacundoTenuta/lingoTUI/internal/config"
	memory "github.com/FacundoTenuta/lingoTUI/internal/context"
	"github.com/FacundoTenuta/lingoTUI/internal/credentials"
	"github.com/FacundoTenuta/lingoTUI/internal/provider/chatgptauth"
	"github.com/FacundoTenuta/lingoTUI/internal/provider/chatgptcodex"
	"github.com/FacundoTenuta/lingoTUI/internal/provider/localwhisper"
	"github.com/FacundoTenuta/lingoTUI/internal/provider/openai"
	"github.com/FacundoTenuta/lingoTUI/internal/setup"
	"github.com/FacundoTenuta/lingoTUI/internal/tui"
)

type providerClient interface {
	app.Transcriber
	app.Chat
}

type runtimeOptions struct {
	newProvider                func(app.Secret) (providerClient, error)
	newOpenAIProvider          func(app.Secret) (providerClient, error)
	newLocalWhisperTranscriber func(app.LocalWhisperConfig) (app.Transcriber, error)
	newChatGPTChat             func(app.AuthCredentialStore) (app.Chat, error)
	newRecorder                func() app.Recorder
	newChunkRecorder           func() app.ChunkRecorder
	audioChecker               setup.AudioPermissionChecker
	credentialStore            app.CredentialStore
	credentialPath             setup.PathProvider
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
		Config:         cfg,
		Provider:       cfg.Provider,
	}
	setupLines := setup.RenderLines(setupService.Status(ctx))

	var openAIProvider providerClient
	if usesProvider(cfg, app.ProviderOpenAI) {
		secret, err := credentialStore.Load(ctx, app.ProviderOpenAI)
		if err == nil && !secret.Empty() {
			openAIProvider, err = options.newOpenAIProvider(secret)
			if err != nil {
				return tui.Model{}, fmt.Errorf("provider %s: %w", app.ProviderOpenAI, err)
			}
		}
	}
	var transcriber app.Transcriber
	var chat app.Chat
	if cfg.TranscriptionModel.Provider == app.ProviderOpenAI && openAIProvider != nil {
		transcriber = openAIProvider
	}
	if cfg.ChatModel.Provider == app.ProviderOpenAI && openAIProvider != nil {
		chat = openAIProvider
	}
	if cfg.TranscriptionModel.Provider == app.ProviderLocalWhisper && strings.TrimSpace(cfg.LocalWhisper.ModelPath) != "" {
		transcriber, err = options.newLocalWhisperTranscriber(cfg.LocalWhisper)
		if err != nil {
			return tui.Model{}, fmt.Errorf("provider %s: %w", app.ProviderLocalWhisper, err)
		}
	}
	if cfg.ChatModel.Provider == app.ProviderChatGPT {
		authStore, ok := credentialStore.(app.AuthCredentialStore)
		if !ok {
			return tui.Model{}, fmt.Errorf("provider %s: typed OAuth credential store is required", app.ProviderChatGPT)
		}
		chat, err = options.newChatGPTChat(authStore)
		if err != nil {
			return tui.Model{}, fmt.Errorf("provider %s: %w", app.ProviderChatGPT, err)
		}
	}

	service := app.NewService(app.Dependencies{
		Recorder:      options.newRecorder(),
		ChunkRecorder: options.newChunkRecorder(),
		Transcriber:   transcriber,
		Chat:          chat,
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
		newOpenAIProvider: func(secret app.Secret) (providerClient, error) {
			return openai.NewClient(secret.Value)
		},
		newLocalWhisperTranscriber: func(config app.LocalWhisperConfig) (app.Transcriber, error) {
			return localwhisper.New(localwhisper.Config{
				BinaryPath: config.BinaryPath,
				ModelPath:  config.ModelPath,
				Language:   config.Language,
				ExtraArgs:  config.ExtraArgs,
			})
		},
		newChatGPTChat: func(store app.AuthCredentialStore) (app.Chat, error) {
			exchanger, err := chatgptauth.NewHTTPTokenExchanger(chatgptauth.HTTPTokenExchangerConfig{
				ClientID:      chatGPTOAuthClientID,
				TokenEndpoint: chatGPTTokenEndpoint,
			})
			if err != nil {
				return nil, err
			}
			session := chatgptauth.SessionManager{Store: store, Refresher: exchanger}
			client, err := chatgptcodex.NewClient(session, chatgptcodex.WithAccountIDProvider(session))
			if err != nil {
				return nil, err
			}
			return chatgptcodex.NewChat(client), nil
		},
		newRecorder: func() app.Recorder {
			return audio.NewFFmpegRecorder(
				audio.WithInputDevice(os.Getenv("LINGOTUI_FFMPEG_MIC_DEVICE")),
				audio.WithTempDir(filepath.Join(os.TempDir(), "lingotui")),
			)
		},
		newChunkRecorder: func() app.ChunkRecorder {
			return audio.NewFFmpegChunkRecorder(
				audio.WithChunkInputDevice(os.Getenv("LINGOTUI_FFMPEG_MIC_DEVICE")),
				audio.WithChunkTempDir(filepath.Join(os.TempDir(), "lingotui")),
			)
		},
		audioChecker: audio.NewMicrophonePermissionChecker(),
	}
}

func normalizeRuntimeOptions(options runtimeOptions) runtimeOptions {
	defaults := defaultRuntimeOptions()
	if options.newOpenAIProvider == nil {
		if options.newProvider != nil {
			options.newOpenAIProvider = options.newProvider
		} else {
			options.newOpenAIProvider = defaults.newOpenAIProvider
		}
	}
	if options.newProvider == nil {
		options.newProvider = options.newOpenAIProvider
	}
	if options.newLocalWhisperTranscriber == nil {
		options.newLocalWhisperTranscriber = defaults.newLocalWhisperTranscriber
	}
	if options.newChatGPTChat == nil {
		options.newChatGPTChat = defaults.newChatGPTChat
	}
	if options.newRecorder == nil {
		options.newRecorder = defaults.newRecorder
	}
	if options.newChunkRecorder == nil {
		options.newChunkRecorder = defaults.newChunkRecorder
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

func usesProvider(cfg app.Config, provider app.ProviderID) bool {
	return cfg.TranscriptionModel.Provider == provider || cfg.ChatModel.Provider == provider
}
