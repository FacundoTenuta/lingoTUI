package localwhisper

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

func TestTranscribeBuildsDefaultArgvAndTrimsStdout(t *testing.T) {
	runner := &fakeRunner{stdout: []byte("\n hello from whisper \t\n")}
	transcriber, err := New(Config{ModelPath: "/models/ggml.bin", Runner: runner})
	if err != nil {
		t.Fatal(err)
	}

	transcript, err := transcriber.Transcribe(context.Background(), app.AudioFile{Path: "/audio/input.wav"}, app.ModelRef{Name: "ignored"})
	if err != nil {
		t.Fatal(err)
	}

	if transcript.Text != "hello from whisper" {
		t.Fatalf("transcript text = %q", transcript.Text)
	}
	if runner.name != "whisper-cli" {
		t.Fatalf("binary = %q", runner.name)
	}
	wantArgs := []string{"-m", "/models/ggml.bin", "-f", "/audio/input.wav", "-l", "auto", "-nt"}
	if !reflect.DeepEqual(runner.args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", runner.args, wantArgs)
	}
}

func TestNewRejectsMissingModelPath(t *testing.T) {
	_, err := New(Config{Runner: &fakeRunner{}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "model path is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestTranscribeRejectsEmptyAudioPath(t *testing.T) {
	runner := &fakeRunner{}
	transcriber, err := New(Config{ModelPath: "/models/ggml.bin", Runner: runner})
	if err != nil {
		t.Fatal(err)
	}

	_, err = transcriber.Transcribe(context.Background(), app.AudioFile{Path: " \t"}, app.ModelRef{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "audio file path is required") {
		t.Fatalf("error = %v", err)
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls = %d", runner.calls)
	}
}

func TestTranscribeSupportsCustomBinaryLanguageAndExtraArgs(t *testing.T) {
	runner := &fakeRunner{stdout: []byte("hola")}
	transcriber, err := New(Config{
		BinaryPath: "/bin/custom-whisper",
		ModelPath:  "/models/ggml.bin",
		Language:   "es",
		ExtraArgs:  []string{"--threads", "4", "--prompt", "safe value; not a shell"},
		Runner:     runner,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = transcriber.Transcribe(context.Background(), app.AudioFile{Path: "/audio/input.wav"}, app.ModelRef{})
	if err != nil {
		t.Fatal(err)
	}

	if runner.name != "/bin/custom-whisper" {
		t.Fatalf("binary = %q", runner.name)
	}
	wantArgs := []string{"-m", "/models/ggml.bin", "-f", "/audio/input.wav", "-l", "es", "-nt", "--threads", "4", "--prompt", "safe value; not a shell"}
	if !reflect.DeepEqual(runner.args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", runner.args, wantArgs)
	}
}

func TestTranscribeCommandFailureReturnsSanitizedStderrSnippet(t *testing.T) {
	runner := &fakeRunner{
		stderr: []byte("/bin/whisper failed reading /audio/private.wav with /models/private.bin\nfull command omitted"),
		err:    errors.New("exit status 1 with /audio/private.wav"),
	}
	transcriber, err := New(Config{BinaryPath: "/bin/whisper", ModelPath: "/models/private.bin", Runner: runner})
	if err != nil {
		t.Fatal(err)
	}

	_, err = transcriber.Transcribe(context.Background(), app.AudioFile{Path: "/audio/private.wav"}, app.ModelRef{})
	if err == nil {
		t.Fatal("expected error")
	}
	message := err.Error()
	for _, leaked := range []string{"/bin/whisper", "/audio/private.wav", "/models/private.bin"} {
		if strings.Contains(message, leaked) {
			t.Fatalf("error leaked %q: %v", leaked, err)
		}
	}
	if !strings.Contains(message, "[binary] failed reading [audio] with [model]") {
		t.Fatalf("error missing sanitized stderr: %v", err)
	}
	if strings.Contains(message, "exit status 1") {
		t.Fatalf("error leaked raw runner error: %v", err)
	}
}

func TestTranscribeCommandFailurePreservesContextSentinels(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "canceled", err: context.Canceled},
		{name: "deadline", err: context.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{stderr: []byte("runner stopped"), err: tt.err}
			transcriber, err := New(Config{ModelPath: "/models/ggml.bin", Runner: runner})
			if err != nil {
				t.Fatal(err)
			}

			_, err = transcriber.Transcribe(context.Background(), app.AudioFile{Path: "/audio/input.wav"}, app.ModelRef{})
			if !errors.Is(err, tt.err) {
				t.Fatalf("errors.Is(%v, %v) = false", err, tt.err)
			}
		})
	}
}

func TestExecRunnerPreservesCanceledContextBeforeStartingCommand(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := execRunner{}.Run(ctx, "whisper-cli", []string{"-m", "model", "-f", "audio.wav"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("errors.Is(%v, context.Canceled) = false", err)
	}
}

func TestTranscribeEmptyStdoutReturnsSanitizedError(t *testing.T) {
	runner := &fakeRunner{stdout: []byte(" \n\t")}
	transcriber, err := New(Config{ModelPath: "/models/ggml.bin", Runner: runner})
	if err != nil {
		t.Fatal(err)
	}

	_, err = transcriber.Transcribe(context.Background(), app.AudioFile{Path: "/audio/input.wav"}, app.ModelRef{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "produced no transcript") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), "/audio/input.wav") || strings.Contains(err.Error(), "/models/ggml.bin") {
		t.Fatalf("error leaked paths: %v", err)
	}
}

type fakeRunner struct {
	name   string
	args   []string
	stdout []byte
	stderr []byte
	err    error
	calls  int
}

func (r *fakeRunner) Run(_ context.Context, name string, args []string) ([]byte, []byte, error) {
	r.calls++
	r.name = name
	r.args = append([]string(nil), args...)
	return r.stdout, r.stderr, r.err
}
