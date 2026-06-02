package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
	"github.com/FacundoTenuta/lingoTUI/internal/provider/chatgptauth"
	"golang.org/x/term"
)

var errCredentialStoreUnavailable = errors.New("credential store unavailable")
var errChatGPTOAuthNotImplemented = errors.New("ChatGPT Plus/Pro OAuth login flow is unavailable")

const chatGPTOAuthClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
const chatGPTOAuthEndpoint = "https://auth.openai.com/oauth/authorize"
const chatGPTTokenEndpoint = "https://auth.openai.com/oauth/token"
const chatGPTOAuthCallbackAddress = "localhost:1455"
const chatGPTOAuthCallbackPath = "/auth/callback"

var version = "dev"
var defaultChatGPTLoginFlowFactory chatGPTLoginFlowFactory = newDefaultChatGPTLoginFlow

type tuiLauncher func() error
type commandRunner func(name string, args ...string) ([]byte, error)
type loginHandler func(context.Context, io.Reader, io.Writer, string) error
type chatGPTLoginFlowFactory func() (chatGPTLoginFlow, error)

type chatGPTLoginFlow interface {
	Login(context.Context, io.Writer, app.AuthCredentialStore) error
}

type placeholderChatGPTLoginFlow struct{}

func (placeholderChatGPTLoginFlow) Login(context.Context, io.Writer, app.AuthCredentialStore) error {
	return errChatGPTOAuthNotImplemented
}

type cliOptions struct {
	launchTUI           tuiLauncher
	runCommand          commandRunner
	stdin               io.Reader
	login               loginHandler
	chatGPTLogin        chatGPTLoginFlow
	chatGPTLoginFactory chatGPTLoginFlowFactory
}

func runCLI(args []string, stdout, stderr io.Writer, options cliOptions) int {
	options = normalizeCLIOptions(options)

	if len(args) == 0 {
		if err := options.launchTUI(); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}

	switch args[0] {
	case "version", "-v", "--version":
		if len(args) != 1 {
			printUsage(stderr)
			return 1
		}
		fmt.Fprintf(stdout, "lingotui %s\n", versionString())
		return 0
	case "login":
		if len(args) > 2 {
			printUsage(stderr)
			return 1
		}
		target := ""
		if len(args) == 2 {
			target = args[1]
		}
		if err := options.login(context.Background(), options.stdin, stdout, target); err != nil {
			fmt.Fprintf(stderr, "lingotui login: %v\n", err)
			return 1
		}
		return 0
	case "update":
		if len(args) != 1 {
			printUsage(stderr)
			return 1
		}
		version := updateVersion()
		fmt.Fprintf(stdout, "Updating lingotui to %s...\n", version)
		if err := runSelfUpdate(options.runCommand, version); err != nil {
			fmt.Fprintf(stderr, "lingotui update: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "lingotui updated")
		return 0
	default:
		printUsage(stderr)
		return 1
	}
}

func normalizeCLIOptions(options cliOptions) cliOptions {
	if options.launchTUI == nil {
		options.launchTUI = defaultTUILauncher
	}
	if options.runCommand == nil {
		options.runCommand = defaultCommandRunner
	}
	if options.stdin == nil {
		options.stdin = strings.NewReader("")
	}
	if options.login == nil {
		options.login = func(ctx context.Context, stdin io.Reader, stdout io.Writer, target string) error {
			if options.chatGPTLogin != nil {
				return defaultLoginHandlerWithChatGPTFlow(ctx, stdin, stdout, target, options.chatGPTLogin)
			}
			return defaultLoginHandlerWithChatGPTFlowFactory(ctx, stdin, stdout, target, options.chatGPTLoginFactory)
		}
	}
	return options
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: lingotui [login [openai|chatgpt]|update|version|-v|--version]")
}

func versionString() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func defaultLoginHandler(ctx context.Context, stdin io.Reader, stdout io.Writer, target string) error {
	return defaultLoginHandlerWithChatGPTFlowFactory(ctx, stdin, stdout, target, nil)
}

func defaultLoginHandlerWithChatGPTFlow(ctx context.Context, stdin io.Reader, stdout io.Writer, target string, flow chatGPTLoginFlow) error {
	return defaultLoginHandlerWithChatGPTFlowFactory(ctx, stdin, stdout, target, func() (chatGPTLoginFlow, error) {
		if flow == nil {
			return placeholderChatGPTLoginFlow{}, nil
		}
		return flow, nil
	})
}

func defaultLoginHandlerWithChatGPTFlowFactory(ctx context.Context, stdin io.Reader, stdout io.Writer, target string, factory chatGPTLoginFlowFactory) error {
	if err := validateLoginTarget(target); err != nil {
		return err
	}
	store, err := buildCredentialStore("")
	if err != nil {
		return fmt.Errorf("credential store: %w", err)
	}
	return loginWithStoreWithChatGPTFlowFactory(ctx, stdin, stdout, store, target, factory)
}

func newDefaultChatGPTLoginFlow() (chatGPTLoginFlow, error) {
	callback, err := chatgptauth.NewLocalCallbackWaiterWithConfig(chatgptauth.LocalCallbackWaiterConfig{
		Address: chatGPTOAuthCallbackAddress,
		Path:    chatGPTOAuthCallbackPath,
	})
	if err != nil {
		return nil, err
	}
	exchanger, err := chatgptauth.NewHTTPTokenExchanger(chatgptauth.HTTPTokenExchangerConfig{
		ClientID:      chatGPTOAuthClientID,
		TokenEndpoint: chatGPTTokenEndpoint,
	})
	if err != nil {
		_ = callback.Close()
		return nil, err
	}
	return newChatGPTLoginFlow(chatgptauth.OpenCommandBrowser{}, callback, exchanger), nil
}

func newChatGPTLoginFlow(browser chatgptauth.Browser, callback chatgptauth.CallbackWaiter, exchanger chatgptauth.TokenExchanger) chatgptauth.Flow {
	return chatgptauth.Flow{
		ClientID:     chatGPTOAuthClientID,
		AuthEndpoint: chatGPTOAuthEndpoint,
		Scopes:       []string{"openid", "profile", "email", "offline_access"},
		ExtraParams: map[string][]string{
			"id_token_add_organizations": {"true"},
			"codex_cli_simplified_flow":  {"true"},
			"originator":                 {"opencode"},
		},
		Browser:   browser,
		Callback:  callback,
		Exchanger: exchanger,
		Timeout:   5 * time.Minute,
	}
}

func loginWithStore(ctx context.Context, stdin io.Reader, stdout io.Writer, store app.CredentialStore, target string) error {
	return loginWithStoreWithChatGPTFlow(ctx, stdin, stdout, store, target, placeholderChatGPTLoginFlow{})
}

func loginWithStoreWithChatGPTFlow(ctx context.Context, stdin io.Reader, stdout io.Writer, store app.CredentialStore, target string, flow chatGPTLoginFlow) error {
	return loginWithStoreWithChatGPTFlowFactory(ctx, stdin, stdout, store, target, func() (chatGPTLoginFlow, error) {
		if flow == nil {
			return placeholderChatGPTLoginFlow{}, nil
		}
		return flow, nil
	})
}

func loginWithStoreWithChatGPTFlowFactory(ctx context.Context, stdin io.Reader, stdout io.Writer, store app.CredentialStore, target string, factory chatGPTLoginFlowFactory) error {
	if err := validateLoginTarget(target); err != nil {
		return err
	}
	if store == nil {
		return fmt.Errorf("credential store: %w", errCredentialStoreUnavailable)
	}
	if normalizedLoginTarget(target) == "chatgpt" {
		if _, ok := store.(app.AuthCredentialStore); !ok {
			return fmt.Errorf("ChatGPT OAuth login requires typed credential store")
		}
		flowFactory := factory
		if flowFactory == nil {
			flowFactory = defaultChatGPTLoginFlowFactory
		}
		flow, err := flowFactory()
		if err != nil {
			return fmt.Errorf("prepare ChatGPT OAuth login: %w", err)
		}
		return loginChatGPT(ctx, stdout, store, flow)
	}
	return loginOpenAI(ctx, stdin, stdout, store)
}

func validateLoginTarget(target string) error {
	switch normalizedLoginTarget(target) {
	case "", "openai":
		return nil
	case "chatgpt":
		return nil
	default:
		return fmt.Errorf("unknown login target %q; usage: lingotui login [openai|chatgpt]", target)
	}
}

func normalizedLoginTarget(target string) string { return strings.TrimSpace(strings.ToLower(target)) }

func loginOpenAI(ctx context.Context, stdin io.Reader, stdout io.Writer, store app.CredentialStore) error {
	fmt.Fprint(stdout, "OpenAI API key: ")
	key, err := readSecret(stdin, stdout)
	if err != nil {
		return err
	}
	if key == "" {
		return fmt.Errorf("missing API key")
	}
	if err := store.Save(ctx, app.ProviderOpenAI, app.Secret{Value: key}); err != nil {
		return fmt.Errorf("save OpenAI API key: %w", err)
	}
	fmt.Fprintln(stdout, "OpenAI API key saved to macOS Keychain.")
	return nil
}

func loginChatGPT(ctx context.Context, stdout io.Writer, store app.CredentialStore, flow chatGPTLoginFlow) error {
	authStore, ok := store.(app.AuthCredentialStore)
	if !ok {
		return fmt.Errorf("ChatGPT OAuth login requires typed credential store")
	}
	if flow == nil {
		flow = placeholderChatGPTLoginFlow{}
	}
	fmt.Fprintln(stdout, "ChatGPT Plus/Pro OAuth login is enabled. OpenAI remains the default runtime; use manual config opt-in for experimental ChatGPT/Codex chat.")
	if err := flow.Login(ctx, stdout, authStore); err != nil {
		if errors.Is(err, errChatGPTOAuthNotImplemented) {
			return err
		}
		return fmt.Errorf("ChatGPT OAuth login failed; no credentials saved")
	}
	fmt.Fprintln(stdout, "ChatGPT Plus/Pro OAuth credential saved.")
	fmt.Fprintln(stdout, "To opt in manually, set chat_model.provider to \"chatgpt\" in ~/Library/Application Support/lingotui/config.json.")
	return nil
}

func readSecret(stdin io.Reader, stdout io.Writer) (string, error) {
	if file, ok := stdin.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		secret, err := term.ReadPassword(int(file.Fd()))
		fmt.Fprintln(stdout)
		if err != nil {
			return "", fmt.Errorf("read API key: %w", err)
		}
		return strings.TrimSpace(string(secret)), nil
	}

	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("read API key: %w", err)
		}
		return "", fmt.Errorf("missing API key")
	}
	return strings.TrimSpace(scanner.Text()), nil
}
