package chatgptauth

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestOpenCommandBrowserRunsMacOSOpenWithURL(t *testing.T) {
	runner := &recordingCommandRunner{}
	browser := OpenCommandBrowser{Run: runner.Run}
	loginURL := "https://auth.example.test/oauth/authorize?state=state-value"

	if err := browser.Open(context.Background(), loginURL); err != nil {
		t.Fatal(err)
	}

	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if runner.name != "open" {
		t.Fatalf("command name = %q, want open", runner.name)
	}
	if len(runner.args) != 1 || runner.args[0] != loginURL {
		t.Fatalf("command args = %#v, want one URL arg", runner.args)
	}
}

func TestOpenCommandBrowserRejectsBlankURLWithoutRunner(t *testing.T) {
	for _, rawURL := range []string{"", " ", "\t\n"} {
		t.Run("blank", func(t *testing.T) {
			runner := &recordingCommandRunner{}
			browser := OpenCommandBrowser{Run: runner.Run}

			err := browser.Open(context.Background(), rawURL)
			if err == nil {
				t.Fatal("error = nil, want blank URL error")
			}
			if runner.calls != 0 {
				t.Fatalf("runner calls = %d, want 0", runner.calls)
			}
		})
	}
}

func TestOpenCommandBrowserSanitizesRunnerError(t *testing.T) {
	loginURL := "https://auth.example.test/oauth/authorize?code=secret-code&state=secret-state"
	runner := &recordingCommandRunner{err: errors.New("open failed for " + loginURL)}
	browser := OpenCommandBrowser{Run: runner.Run}

	err := browser.Open(context.Background(), loginURL)
	if err == nil {
		t.Fatal("error = nil, want runner error")
	}
	if err.Error() != "open browser for ChatGPT OAuth login" {
		t.Fatalf("error = %q, want sanitized browser error", err.Error())
	}
	assertNoSecrets(t, err.Error(), loginURL, "secret-code", "secret-state")
}

func TestOpenCommandBrowserPropagatesContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	runner := &recordingCommandRunner{after: cancel, err: errors.New("command failed")}
	browser := OpenCommandBrowser{Run: runner.Run}

	err := browser.Open(ctx, "https://auth.example.test/oauth/authorize")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
}

func TestOpenCommandBrowserRejectsAlreadyCanceledContextWithoutRunner(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runner := &recordingCommandRunner{}
	browser := OpenCommandBrowser{Run: runner.Run}

	err := browser.Open(ctx, "https://auth.example.test/oauth/authorize")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.calls)
	}
}

type recordingCommandRunner struct {
	calls int
	name  string
	args  []string
	err   error
	after func()
}

func (r *recordingCommandRunner) Run(_ context.Context, name string, args ...string) error {
	r.calls++
	r.name = name
	r.args = append([]string(nil), args...)
	if r.after != nil {
		r.after()
	}
	return r.err
}

func TestOpenCommandBrowserPropagatesRunnerContextError(t *testing.T) {
	runner := &recordingCommandRunner{err: context.DeadlineExceeded}
	browser := OpenCommandBrowser{Run: runner.Run}

	err := browser.Open(context.Background(), "https://auth.example.test/oauth/authorize")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
}

func TestOpenCommandBrowserDoesNotLeakURLFromBlankError(t *testing.T) {
	err := OpenCommandBrowser{Run: (&recordingCommandRunner{}).Run}.Open(context.Background(), "  ")
	if err == nil {
		t.Fatal("error = nil, want blank URL error")
	}
	if strings.Contains(err.Error(), "http") || strings.Contains(err.Error(), "?") {
		t.Fatalf("blank error leaked URL-like content: %q", err.Error())
	}
}
