package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

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
	runner := &recordingCommandRunner{output: []byte("compiler warning that should stay hidden")}
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
	for _, want := range []string{"Updating lingotui to latest...", "lingotui updated"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
	if strings.Contains(stdout.String(), "compiler warning") {
		t.Fatalf("stdout exposed successful go install output: %q", stdout.String())
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
	runner := &recordingCommandRunner{output: []byte("download failed"), err: errors.New("install failed")}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runCLI([]string{"update"}, &stdout, &stderr, cliOptions{
		launchTUI:  func() error { return nil },
		runCommand: runner.run,
	})

	if code == 0 {
		t.Fatal("code = 0, want non-zero")
	}
	if !strings.Contains(stdout.String(), "Updating lingotui") {
		t.Fatalf("stdout = %q, want loading state", stdout.String())
	}
	for _, want := range []string{"lingotui update: install failed", "download failed"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), want)
		}
	}
	if strings.Contains(stdout.String(), "lingotui updated") {
		t.Fatalf("stdout incorrectly reported success: %q", stdout.String())
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
			if !strings.Contains(stderr.String(), "usage: lingotui [login [openai|chatgpt]|update|version|-v|--version]") {
				t.Fatalf("stderr = %q, want usage", stderr.String())
			}
		})
	}
}

func TestRunCLILoginCallsHandler(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantTarget string
	}{
		{name: "default target", args: []string{"login"}, wantTarget: ""},
		{name: "openai target", args: []string{"login", "openai"}, wantTarget: "openai"},
		{name: "chatgpt target", args: []string{"login", "chatgpt"}, wantTarget: "chatgpt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &recordingCommandRunner{}
			login := &recordingLoginHandler{}
			var launched bool
			var stdout bytes.Buffer
			stdin := strings.NewReader("sk-test\n")

			code := runCLI(tt.args, &stdout, &bytes.Buffer{}, cliOptions{
				launchTUI: func() error {
					launched = true
					return nil
				},
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
			if login.stdin != stdin || login.stdout != &stdout || login.target != tt.wantTarget {
				t.Fatalf("login handler got stdin/stdout/target = %v/%v/%q", login.stdin == stdin, login.stdout == &stdout, login.target)
			}
			if runner.called {
				t.Fatalf("command runner was called: %+v", runner)
			}
			if launched {
				t.Fatal("TUI launcher was called")
			}
		})
	}
}

func TestRunCLILoginRejectsExtraArgsWithoutSideEffects(t *testing.T) {
	runner := &recordingCommandRunner{}
	login := &recordingLoginHandler{}
	var launched bool
	var stderr bytes.Buffer

	code := runCLI([]string{"login", "openai", "now"}, &bytes.Buffer{}, &stderr, cliOptions{
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
	if !strings.Contains(stderr.String(), "usage: lingotui [login [openai|chatgpt]|update|version|-v|--version]") {
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
	if !strings.Contains(stderr.String(), "usage: lingotui [login [openai|chatgpt]|update|version|-v|--version]") {
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
	if !strings.Contains(stderr.String(), "usage: lingotui [login [openai|chatgpt]|update|version|-v|--version]") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestLoginWithStoreDefaultsToOpenAIAPIKeyWithoutExposingSecret(t *testing.T) {
	store := &recordingCredentialStore{}
	var stdout bytes.Buffer

	if err := loginWithStore(context.Background(), strings.NewReader(" sk-secret \n"), &stdout, store, ""); err != nil {
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

func TestLoginWithStoreOpenAISavesAPIKeyWithoutExposingSecret(t *testing.T) {
	store := &recordingCredentialStore{}
	var stdout bytes.Buffer

	if err := loginWithStore(context.Background(), strings.NewReader(" sk-secret \n"), &stdout, store, "openai"); err != nil {
		t.Fatal(err)
	}
	if store.saves != 1 || store.provider != app.ProviderOpenAI || store.secret.Value != "sk-secret" {
		t.Fatalf("saves=%d provider=%q secret=%q", store.saves, store.provider, store.secret.Value)
	}
	if strings.Contains(stdout.String(), "sk-secret") {
		t.Fatalf("stdout exposed secret: %q", stdout.String())
	}
}

func TestLoginWithStoreChatGPTUsesInjectedFlowWithoutReadingStdinOrExposingTokens(t *testing.T) {
	store := &recordingAuthCredentialStore{}
	flow := &fakeChatGPTLoginFlow{
		credential: app.Credential{
			Provider: app.ProviderChatGPT,
			Kind:     app.CredentialKindOAuth,
			OAuth: app.OAuthCredential{
				AccessToken:  app.Secret{Value: "access-token"},
				RefreshToken: app.Secret{Value: "refresh-token"},
			},
		},
	}
	var stdout bytes.Buffer
	stdin := &failingReader{err: errors.New("stdin should not be read")}

	if err := loginWithStoreWithChatGPTFlow(context.Background(), stdin, &stdout, store, "chatgpt", flow); err != nil {
		t.Fatal(err)
	}
	if stdin.reads != 0 {
		t.Fatalf("stdin reads = %d, want 0", stdin.reads)
	}
	if !flow.called {
		t.Fatal("ChatGPT login flow was not called")
	}
	if store.credentialSaves != 1 {
		t.Fatalf("credential saves = %d, want 1", store.credentialSaves)
	}
	if store.credential.Provider != app.ProviderChatGPT || store.credential.Kind != app.CredentialKindOAuth {
		t.Fatalf("saved credential = %+v, want chatgpt oauth", store.credential)
	}
	for _, secret := range []string{"access-token", "refresh-token"} {
		if strings.Contains(stdout.String(), secret) {
			t.Fatalf("stdout exposed token %q: %q", secret, stdout.String())
		}
	}
	if !strings.Contains(stdout.String(), "OAuth credential saved") {
		t.Fatalf("stdout = %q, want success message", stdout.String())
	}
}

func TestLoginWithStoreChatGPTRequiresTypedCredentialStoreBeforeFlow(t *testing.T) {
	store := &recordingCredentialStore{}
	flow := &fakeChatGPTLoginFlow{}
	var stdout bytes.Buffer

	err := loginWithStoreWithChatGPTFlow(context.Background(), strings.NewReader("ignored\n"), &stdout, store, "chatgpt", flow)
	if err == nil || !strings.Contains(err.Error(), "typed credential store") {
		t.Fatalf("error = %v, want typed credential store", err)
	}
	if flow.called || store.saves != 0 || stdout.String() != "" {
		t.Fatalf("side effects: flow=%v saves=%d stdout=%q", flow.called, store.saves, stdout.String())
	}
}

func TestLoginWithStoreChatGPTFlowErrorSavesNothingAndRedactsTokens(t *testing.T) {
	store := &recordingAuthCredentialStore{}
	flow := &fakeChatGPTLoginFlow{err: errors.New("exchange failed with access-token refresh-token")}
	var stdout bytes.Buffer

	err := loginWithStoreWithChatGPTFlow(context.Background(), strings.NewReader("ignored\n"), &stdout, store, "chatgpt", flow)
	if err == nil || !strings.Contains(err.Error(), "ChatGPT OAuth login failed") {
		t.Fatalf("error = %v, want ChatGPT OAuth failure", err)
	}
	if store.credentialSaves != 0 {
		t.Fatalf("credential saves = %d, want 0", store.credentialSaves)
	}
	for _, output := range []string{stdout.String(), err.Error()} {
		for _, secret := range []string{"access-token", "refresh-token"} {
			if strings.Contains(output, secret) {
				t.Fatalf("output exposed token %q: %q", secret, output)
			}
		}
	}
}

func TestDefaultChatGPTLoginFlowFromEnvDisabledReturnsPlaceholderWithoutSideEffects(t *testing.T) {
	t.Setenv(experimentalChatGPTOAuthEnv, "")
	store := &recordingAuthCredentialStore{}
	stdin := &failingReader{err: errors.New("stdin should not be read")}
	var stdout bytes.Buffer

	err := loginWithStoreWithChatGPTFlowFactory(context.Background(), stdin, &stdout, store, "chatgpt", nil)
	if !errors.Is(err, errChatGPTOAuthNotImplemented) {
		t.Fatalf("error = %v, want not implemented", err)
	}
	if stdin.reads != 0 || store.credentialSaves != 0 || stdout.String() != "" {
		t.Fatalf("side effects: stdin reads=%d saves=%d stdout=%q", stdin.reads, store.credentialSaves, stdout.String())
	}
}

func TestLoginWithStoreChatGPTUsesExperimentalFactoryWhenEnabledWithoutReadingStdinOrExposingTokens(t *testing.T) {
	t.Setenv(experimentalChatGPTOAuthEnv, "1")
	store := &recordingAuthCredentialStore{}
	stdin := &failingReader{err: errors.New("stdin should not be read")}
	flow := &fakeChatGPTLoginFlow{
		credential: app.Credential{
			Provider: app.ProviderChatGPT,
			Kind:     app.CredentialKindOAuth,
			OAuth: app.OAuthCredential{
				AccessToken:  app.Secret{Value: "access-token"},
				RefreshToken: app.Secret{Value: "refresh-token"},
			},
		},
	}
	var factoryCalls int
	var stdout bytes.Buffer

	err := loginWithStoreWithChatGPTFlowFactory(context.Background(), stdin, &stdout, store, "chatgpt", func() (chatGPTLoginFlow, error) {
		factoryCalls++
		return flow, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if factoryCalls != 1 || !flow.called {
		t.Fatalf("factory calls=%d flow called=%v, want experimental flow path", factoryCalls, flow.called)
	}
	if stdin.reads != 0 {
		t.Fatalf("stdin reads = %d, want 0", stdin.reads)
	}
	for _, secret := range []string{"access-token", "refresh-token"} {
		if strings.Contains(stdout.String(), secret) {
			t.Fatalf("stdout exposed token %q: %q", secret, stdout.String())
		}
	}
}

func TestExperimentalChatGPTLoginFlowUsesVerifiedOpencodeOAuthConstants(t *testing.T) {
	flow := newExperimentalChatGPTLoginFlow(nil, nil, nil)

	if flow.ClientID != chatGPTOAuthClientID {
		t.Fatalf("client ID = %q, want verified opencode client ID", flow.ClientID)
	}
	if flow.AuthEndpoint != chatGPTOAuthEndpoint {
		t.Fatalf("auth endpoint = %q, want %q", flow.AuthEndpoint, chatGPTOAuthEndpoint)
	}
	if !reflect.DeepEqual(flow.Scopes, []string{"openid", "profile", "email", "offline_access"}) {
		t.Fatalf("scopes = %#v", flow.Scopes)
	}
	for key, want := range map[string]string{
		"id_token_add_organizations": "true",
		"codex_cli_simplified_flow":  "true",
		"originator":                 "opencode",
	} {
		if got := flow.ExtraParams.Get(key); got != want {
			t.Fatalf("extra param %s = %q, want %q", key, got, want)
		}
	}
	if flow.Timeout != 5*time.Minute {
		t.Fatalf("timeout = %s, want 5m", flow.Timeout)
	}
}

func TestLoginWithStoreUnknownTargetRejectsWithoutSideEffects(t *testing.T) {
	store := &recordingCredentialStore{}
	var stdout bytes.Buffer

	err := loginWithStore(context.Background(), strings.NewReader("ignored\n"), &stdout, store, "wat")
	if err == nil || !strings.Contains(err.Error(), "unknown login target") {
		t.Fatalf("error = %v, want unknown target", err)
	}
	if store.saves != 0 || stdout.String() != "" {
		t.Fatalf("side effects: saves=%d stdout=%q", store.saves, stdout.String())
	}
}

type recordingCommandRunner struct {
	called bool
	name   string
	args   []string
	output []byte
	err    error
}

type recordingLoginHandler struct {
	called bool
	stdin  io.Reader
	stdout io.Writer
	target string
	err    error
}

func (h *recordingLoginHandler) run(_ context.Context, stdin io.Reader, stdout io.Writer, target string) error {
	h.called = true
	h.stdin = stdin
	h.stdout = stdout
	h.target = target
	return h.err
}

type recordingCredentialStore struct {
	provider app.ProviderID
	secret   app.Secret
	saves    int
	err      error
}

func (s *recordingCredentialStore) Save(_ context.Context, provider app.ProviderID, secret app.Secret) error {
	s.saves++
	s.provider = provider
	s.secret = secret
	return s.err
}

func (s *recordingCredentialStore) Load(context.Context, app.ProviderID) (app.Secret, error) {
	return s.secret, s.err
}

func (s *recordingCredentialStore) Delete(context.Context, app.ProviderID) error { return s.err }

type recordingAuthCredentialStore struct {
	recordingCredentialStore
	credential      app.Credential
	credentialSaves int
}

func (s *recordingAuthCredentialStore) SaveCredential(_ context.Context, credential app.Credential) error {
	if s.err != nil {
		return s.err
	}
	s.credentialSaves++
	s.credential = credential
	return nil
}

func (s *recordingAuthCredentialStore) LoadCredential(context.Context, app.ProviderID, app.CredentialKind) (app.Credential, error) {
	return s.credential, s.err
}

func (s *recordingAuthCredentialStore) DeleteCredential(context.Context, app.ProviderID, app.CredentialKind) error {
	return s.err
}

type fakeChatGPTLoginFlow struct {
	credential app.Credential
	err        error
	called     bool
}

func (f *fakeChatGPTLoginFlow) Login(ctx context.Context, _ io.Writer, store app.AuthCredentialStore) error {
	f.called = true
	if f.err != nil {
		return f.err
	}
	return store.SaveCredential(ctx, f.credential)
}

type failingReader struct {
	err   error
	reads int
}

func (r *failingReader) Read([]byte) (int, error) {
	r.reads++
	return 0, r.err
}

func (r *recordingCommandRunner) run(name string, args ...string) ([]byte, error) {
	r.called = true
	r.name = name
	r.args = append([]string(nil), args...)
	return r.output, r.err
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
