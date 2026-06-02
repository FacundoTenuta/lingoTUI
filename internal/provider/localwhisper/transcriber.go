package localwhisper

import (
	"context"
	"errors"
	"os/exec"
	"strings"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

const (
	defaultBinaryPath = "whisper-cli"
	defaultLanguage   = "auto"
	maxStderrBytes    = 512
)

var _ app.Transcriber = (*Transcriber)(nil)

type Runner interface {
	Run(ctx context.Context, name string, args []string) (stdout []byte, stderr []byte, err error)
}

type Config struct {
	BinaryPath string
	ModelPath  string
	Language   string
	ExtraArgs  []string
	Runner     Runner
}

type Transcriber struct {
	binaryPath string
	modelPath  string
	language   string
	extraArgs  []string
	runner     Runner
}

func New(config Config) (*Transcriber, error) {
	binaryPath := strings.TrimSpace(config.BinaryPath)
	if binaryPath == "" {
		binaryPath = defaultBinaryPath
	}

	modelPath := strings.TrimSpace(config.ModelPath)
	if modelPath == "" {
		return nil, errors.New("local whisper model path is required")
	}

	language := strings.TrimSpace(config.Language)
	if language == "" {
		language = defaultLanguage
	}

	runner := config.Runner
	if runner == nil {
		runner = execRunner{}
	}

	extraArgs := make([]string, len(config.ExtraArgs))
	copy(extraArgs, config.ExtraArgs)

	return &Transcriber{
		binaryPath: binaryPath,
		modelPath:  modelPath,
		language:   language,
		extraArgs:  extraArgs,
		runner:     runner,
	}, nil
}

func (t *Transcriber) Transcribe(ctx context.Context, file app.AudioFile, _ app.ModelRef) (app.Transcript, error) {
	audioPath := strings.TrimSpace(file.Path)
	if audioPath == "" {
		return app.Transcript{}, errors.New("audio file path is required")
	}

	args := []string{"-m", t.modelPath, "-f", audioPath, "-l", t.language, "-nt"}
	args = append(args, t.extraArgs...)

	stdout, stderr, err := t.runner.Run(ctx, t.binaryPath, args)
	if err != nil {
		return app.Transcript{}, t.commandError(err, stderr, audioPath)
	}

	text := strings.TrimSpace(string(stdout))
	if text == "" {
		return app.Transcript{}, errors.New("local whisper produced no transcript")
	}
	return app.Transcript{Text: text}, nil
}

func (t *Transcriber) commandError(err error, stderr []byte, audioPath string) error {
	if contextErr := cleanContextError(err); contextErr != nil {
		return contextErr
	}
	message := "local whisper transcription failed"
	if snippet := t.sanitizeStderr(stderr, audioPath); snippet != "" {
		message += ": " + snippet
	}
	return errors.New(message)
}

func cleanContextError(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return nil
}

func (t *Transcriber) sanitizeStderr(stderr []byte, audioPath string) string {
	message := strings.TrimSpace(string(stderr))
	if message == "" {
		return ""
	}
	for _, sensitive := range []struct {
		value       string
		replacement string
	}{
		{t.binaryPath, "[binary]"},
		{t.modelPath, "[model]"},
		{audioPath, "[audio]"},
	} {
		if sensitive.value != "" {
			message = strings.ReplaceAll(message, sensitive.value, sensitive.replacement)
		}
	}
	if len(message) > maxStderrBytes {
		message = message[:maxStderrBytes] + "..."
	}
	return message
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args []string) ([]byte, []byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	stdout, err := command.Output()
	if err == nil {
		return stdout, nil, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		if contextErr := cleanContextError(ctx.Err()); contextErr != nil {
			return stdout, exitError.Stderr, contextErr
		}
		return stdout, exitError.Stderr, err
	}
	if contextErr := cleanContextError(ctx.Err()); contextErr != nil {
		return stdout, nil, contextErr
	}
	return stdout, nil, err
}
