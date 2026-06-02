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

	"github.com/FacundoTenuta/lingoTUI/internal/app"
	"golang.org/x/term"
)

var errCredentialStoreUnavailable = errors.New("credential store unavailable")

var version = "dev"

type tuiLauncher func() error
type commandRunner func(name string, args ...string) error
type loginHandler func(context.Context, io.Reader, io.Writer) error

type cliOptions struct {
	launchTUI  tuiLauncher
	runCommand commandRunner
	stdin      io.Reader
	login      loginHandler
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
		if len(args) != 1 {
			printUsage(stderr)
			return 1
		}
		if err := options.login(context.Background(), options.stdin, stdout); err != nil {
			fmt.Fprintf(stderr, "lingotui login: %v\n", err)
			return 1
		}
		return 0
	case "update":
		if len(args) != 1 {
			printUsage(stderr)
			return 1
		}
		if err := runSelfUpdate(options.runCommand); err != nil {
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
		options.login = defaultLoginHandler
	}
	return options
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: lingotui [login|update|version|-v|--version]")
}

func versionString() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func defaultLoginHandler(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	store, err := buildCredentialStore("")
	if err != nil {
		return fmt.Errorf("credential store: %w", err)
	}
	return loginWithStore(ctx, stdin, stdout, store)
}

func loginWithStore(ctx context.Context, stdin io.Reader, stdout io.Writer, store app.CredentialStore) error {
	if store == nil {
		return fmt.Errorf("credential store: %w", errCredentialStoreUnavailable)
	}
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
