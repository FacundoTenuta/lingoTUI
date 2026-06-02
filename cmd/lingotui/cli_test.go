package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

func TestRunCLILaunchesTUIWithoutArgs(t *testing.T) {
	var launched bool
	runner := &recordingCommandRunner{}

	code := runCLI(nil, &bytes.Buffer{}, &bytes.Buffer{}, cliOptions{
		launchTUI: func() error {
			launched = true
			return nil
		},
		runCommand: runner.run,
	})

	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if !launched {
		t.Fatal("TUI launcher was not called")
	}
	if runner.called {
		t.Fatalf("command runner was called: %+v", runner)
	}
}

func TestRunCLIReturnsNonZeroWhenTUILaunchFails(t *testing.T) {
	runner := &recordingCommandRunner{}
	var stderr bytes.Buffer

	code := runCLI(nil, &bytes.Buffer{}, &stderr, cliOptions{
		launchTUI:  func() error { return errors.New("startup failed") },
		runCommand: runner.run,
	})

	if code == 0 {
		t.Fatal("code = 0, want non-zero")
	}
	if runner.called {
		t.Fatalf("command runner was called: %+v", runner)
	}
	if !strings.Contains(stderr.String(), "startup failed") {
		t.Fatalf("stderr = %q, want startup failure", stderr.String())
	}
}

func TestRunCLIUpdateRunsGoInstallLatest(t *testing.T) {
	t.Setenv("LINGOTUI_VERSION", "")
	runner := &recordingCommandRunner{}
	var launched bool
	var stdout bytes.Buffer

	code := runCLI([]string{"update"}, &stdout, &bytes.Buffer{}, cliOptions{
		launchTUI: func() error {
			launched = true
			return nil
		},
		runCommand: runner.run,
	})

	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if launched {
		t.Fatal("TUI launcher was called")
	}
	assertCommand(t, runner, "go", "install", updatePackage+"@latest")
	if !strings.Contains(stdout.String(), "lingotui updated") {
		t.Fatalf("stdout = %q, want update confirmation", stdout.String())
	}
}

func TestRunCLIUpdateUsesVersionOverride(t *testing.T) {
	t.Setenv("LINGOTUI_VERSION", "v0.1.0")
	runner := &recordingCommandRunner{}

	code := runCLI([]string{"update"}, &bytes.Buffer{}, &bytes.Buffer{}, cliOptions{
		launchTUI:  func() error { return nil },
		runCommand: runner.run,
	})

	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	assertCommand(t, runner, "go", "install", updatePackage+"@v0.1.0")
}

func TestRunCLIUpdateFailureReturnsNonZeroAndWritesStderr(t *testing.T) {
	runner := &recordingCommandRunner{err: errors.New("install failed")}
	var stderr bytes.Buffer

	code := runCLI([]string{"update"}, &bytes.Buffer{}, &stderr, cliOptions{
		launchTUI:  func() error { return nil },
		runCommand: runner.run,
	})

	if code == 0 {
		t.Fatal("code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "lingotui update: install failed") {
		t.Fatalf("stderr = %q, want update failure", stderr.String())
	}
}

func TestRunCLIPrintsVersion(t *testing.T) {
	previous := version
	version = "test-version"
	t.Cleanup(func() { version = previous })

	for _, args := range [][]string{{"version"}, {"-v"}, {"--version"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			runner := &recordingCommandRunner{}
			login := &recordingLoginHandler{}
			var launched bool
			var stdout bytes.Buffer

			code := runCLI(args, &stdout, &bytes.Buffer{}, cliOptions{
				launchTUI: func() error {
					launched = true
					return nil
				},
				runCommand: runner.run,
				login:      login.run,
			})

			if code != 0 {
				t.Fatalf("code = %d, want 0", code)
			}
			if launched {
				t.Fatal("TUI launcher was called")
			}
			if runner.called {
				t.Fatalf("command runner was called: %+v", runner)
			}
			if login.called {
				t.Fatal("login handler was called")
			}
			if got, want := stdout.String(), "lingotui test-version\n"; got != want {
				t.Fatalf("stdout = %q, want %q", got, want)
			}
		})
	}
}

func TestRunCLIVersionRejectsExtraArgsWithoutSideEffects(t *testing.T) {
	for _, args := range [][]string{{"version", "extra"}, {"-v", "extra"}, {"--version", "extra"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			runner := &recordingCommandRunner{}
			login := &recordingLoginHandler{}
			var launched bool
			var stderr bytes.Buffer

			code := runCLI(args, &bytes.Buffer{}, &stderr, cliOptions{
				launchTUI: func() error {
					launched = true
					return nil
				},
				runCommand: runner.run,
				login:      login.run,
			})

			if code == 0 {
				t.Fatal("code = 0, want non-zero")
			}
			if launched {
				t.Fatal("TUI launcher was called")
			}
			if runner.called {
				t.Fatalf("command runner was called: %+v", runner)
			}
			if login.called {
				t.Fatal("login handler was called")
			}
			if !strings.Contains(stderr.String(), "usage: lingotui [login|update|version|-v|--version]") {
				t.Fatalf("stderr = %q, want usage", stderr.String())
			}
		})
	}
}

func TestRunCLILoginCallsHandler(t *testing.T) {
	runner := &recordingCommandRunner{}
	login := &recordingLoginHandler{}
	var stdout bytes.Buffer
	stdin := strings.NewReader("sk-test\n")

	code := runCLI([]string{"login"}, &stdout, &bytes.Buffer{}, cliOptions{
		launchTUI:  func() error { return nil },
		runCommand: runner.run,
		stdin:      stdin,
		login:      login.run,
	})

	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if !login.called {
		t.Fatal("login handler was not called")
	}
	if login.stdin != stdin || login.stdout != &stdout {
		t.Fatal("login handler did not receive configured streams")
	}
	if runner.called {
		t.Fatalf("command runner was called: %+v", runner)
	}
}

func TestRunCLILoginRejectsExtraArgsWithoutSideEffects(t *testing.T) {
	runner := &recordingCommandRunner{}
	login := &recordingLoginHandler{}
	var launched bool
	var stderr bytes.Buffer

	code := runCLI([]string{"login", "now"}, &bytes.Buffer{}, &stderr, cliOptions{
		launchTUI: func() error {
			launched = true
			return nil
		},
		runCommand: runner.run,
		login:      login.run,
	})

	if code == 0 {
		t.Fatal("code = 0, want non-zero")
	}
	if launched {
		t.Fatal("TUI launcher was called")
	}
	if runner.called {
		t.Fatalf("command runner was called: %+v", runner)
	}
	if login.called {
		t.Fatal("login handler was called")
	}
	if !strings.Contains(stderr.String(), "usage: lingotui [login|update|version|-v|--version]") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestRunCLILoginFailureReturnsNonZeroAndWritesStderr(t *testing.T) {
	var stderr bytes.Buffer

	code := runCLI([]string{"login"}, &bytes.Buffer{}, &stderr, cliOptions{
		launchTUI: func() error { return nil },
		login:     (&recordingLoginHandler{err: errors.New("save failed")}).run,
	})

	if code == 0 {
		t.Fatal("code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "lingotui login: save failed") {
		t.Fatalf("stderr = %q, want login failure", stderr.String())
	}
}

func TestRunCLIUnknownCommandReturnsNonZeroWithoutSideEffects(t *testing.T) {
	runner := &recordingCommandRunner{}
	var launched bool
	var stderr bytes.Buffer

	code := runCLI([]string{"wat"}, &bytes.Buffer{}, &stderr, cliOptions{
		launchTUI: func() error {
			launched = true
			return nil
		},
		runCommand: runner.run,
	})

	if code == 0 {
		t.Fatal("code = 0, want non-zero")
	}
	if launched {
		t.Fatal("TUI launcher was called")
	}
	if runner.called {
		t.Fatalf("command runner was called: %+v", runner)
	}
	if !strings.Contains(stderr.String(), "usage: lingotui [login|update|version|-v|--version]") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestRunCLIUpdateRejectsExtraArgsWithoutSideEffects(t *testing.T) {
	runner := &recordingCommandRunner{}
	var launched bool
	var stderr bytes.Buffer

	code := runCLI([]string{"update", "now"}, &bytes.Buffer{}, &stderr, cliOptions{
		launchTUI: func() error {
			launched = true
			return nil
		},
		runCommand: runner.run,
	})

	if code == 0 {
		t.Fatal("code = 0, want non-zero")
	}
	if launched {
		t.Fatal("TUI launcher was called")
	}
	if runner.called {
		t.Fatalf("command runner was called: %+v", runner)
	}
	if !strings.Contains(stderr.String(), "usage: lingotui [login|update|version|-v|--version]") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestLoginWithStoreSavesOpenAIKeyWithoutExposingSecret(t *testing.T) {
	store := &recordingCredentialStore{}
	var stdout bytes.Buffer

	if err := loginWithStore(context.Background(), strings.NewReader(" sk-secret \n"), &stdout, store); err != nil {
		t.Fatal(err)
	}
	if store.provider != "openai" || store.secret.Value != "sk-secret" {
		t.Fatalf("saved provider=%q secret=%q", store.provider, store.secret.Value)
	}
	if strings.Contains(stdout.String(), "sk-secret") {
		t.Fatalf("stdout exposed secret: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "saved to macOS Keychain") {
		t.Fatalf("stdout = %q, want success message", stdout.String())
	}
}

type recordingCommandRunner struct {
	called bool
	name   string
	args   []string
	err    error
}

type recordingLoginHandler struct {
	called bool
	stdin  io.Reader
	stdout io.Writer
	err    error
}

func (h *recordingLoginHandler) run(_ context.Context, stdin io.Reader, stdout io.Writer) error {
	h.called = true
	h.stdin = stdin
	h.stdout = stdout
	return h.err
}

type recordingCredentialStore struct {
	provider app.ProviderID
	secret   app.Secret
	err      error
}

func (s *recordingCredentialStore) Save(_ context.Context, provider app.ProviderID, secret app.Secret) error {
	s.provider = provider
	s.secret = secret
	return s.err
}

func (s *recordingCredentialStore) Load(context.Context, app.ProviderID) (app.Secret, error) {
	return s.secret, s.err
}

func (s *recordingCredentialStore) Delete(context.Context, app.ProviderID) error { return s.err }

func (r *recordingCommandRunner) run(name string, args ...string) error {
	r.called = true
	r.name = name
	r.args = append([]string(nil), args...)
	return r.err
}

func assertCommand(t *testing.T, runner *recordingCommandRunner, wantName string, wantArgs ...string) {
	t.Helper()
	if !runner.called {
		t.Fatal("command runner was not called")
	}
	if runner.name != wantName {
		t.Fatalf("command name = %q, want %q", runner.name, wantName)
	}
	if !reflect.DeepEqual(runner.args, wantArgs) {
		t.Fatalf("command args = %#v, want %#v", runner.args, wantArgs)
	}
}
