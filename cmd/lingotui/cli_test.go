package main

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
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
	if !strings.Contains(stderr.String(), "usage: lingotui [update]") {
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
	if !strings.Contains(stderr.String(), "usage: lingotui [update]") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

type recordingCommandRunner struct {
	called bool
	name   string
	args   []string
	err    error
}

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
