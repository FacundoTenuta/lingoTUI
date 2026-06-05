package codexcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

const (
	defaultBinaryPath = "codex"
	maxStderrBytes    = 512
)

var _ app.Chat = (*Chat)(nil)

type Runner interface {
	Run(ctx context.Context, name string, args []string, stdin string) (stdout []byte, stderr []byte, err error)
}

type Config struct {
	BinaryPath string
	WorkDir    string
	Runner     Runner
}

type Chat struct {
	binaryPath string
	workDir    string
	runner     Runner
}

func New(config Config) (*Chat, error) {
	binaryPath := strings.TrimSpace(config.BinaryPath)
	if binaryPath == "" {
		binaryPath = defaultBinaryPath
	}
	runner := config.Runner
	if runner == nil {
		runner = execRunner{}
	}
	return &Chat{binaryPath: binaryPath, workDir: strings.TrimSpace(config.WorkDir), runner: runner}, nil
}

func (c *Chat) Summarize(ctx context.Context, transcript app.Transcript, languages []app.Language, model app.ModelRef) (app.Summary, error) {
	languageNames := languageCodes(languages)
	content, err := c.run(ctx, model, fmt.Sprintf("You summarize transcripts for language learners. Respond only with a compact JSON object whose keys are the requested language codes and whose values are concise summaries.\n\nLanguages: %s\nTranscript:\n%s", strings.Join(languageNames, ", "), transcript.Text))
	if err != nil {
		return nil, err
	}
	var raw map[string]string
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("parse codex summary response: %w", err)
	}
	return pickLanguages(raw, languages), nil
}

func (c *Chat) Translate(ctx context.Context, text string, languages []app.Language, model app.ModelRef) (app.Translations, error) {
	languageNames := languageCodes(languages)
	content, err := c.run(ctx, model, fmt.Sprintf("Translate user text for language learners. Respond only with a compact JSON object whose keys are the requested language codes and whose values are direct translations.\n\nLanguages: %s\nText:\n%s", strings.Join(languageNames, ", "), text))
	if err != nil {
		return nil, err
	}
	var raw map[string]string
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("parse codex translation response: %w", err)
	}
	translations := app.Translations{}
	for language, translated := range pickLanguages(raw, languages) {
		translations[language] = translated
	}
	return translations, nil
}

func (c *Chat) Answer(ctx context.Context, question app.Question, recent app.RecentContext, model app.ModelRef) (app.Answer, error) {
	content, err := c.run(ctx, model, fmt.Sprintf("Answer using only the recent transcript and multilingual summary. If the answer is not present, say so briefly.\n\nTranscript:\n%s\n\nSummary:\n%s\n\nQuestion: %s", recent.Transcript.Text, formatSummary(recent.Summary), question))
	if err != nil {
		return "", err
	}
	return app.Answer(strings.TrimSpace(content)), nil
}

func (c *Chat) run(ctx context.Context, model app.ModelRef, prompt string) (string, error) {
	file, err := os.CreateTemp("", "lingotui-codex-*.txt")
	if err != nil {
		return "", fmt.Errorf("create codex output file: %w", err)
	}
	outputPath := file.Name()
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close codex output file: %w", err)
	}
	defer os.Remove(outputPath)

	args := []string{"exec", "--sandbox", "read-only", "--output-last-message", outputPath}
	if c.workDir != "" {
		args = append(args, "--cd", c.workDir)
	}
	if name := strings.TrimSpace(model.Name); model.Provider == app.ProviderCodexCLI && name != "" {
		args = append(args, "--model", name)
	}
	args = append(args, "-")

	_, stderr, err := c.runner.Run(ctx, c.binaryPath, args, prompt)
	if err != nil {
		return "", codexError(err, stderr)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return "", fmt.Errorf("read codex output: %w", err)
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return "", errors.New("codex cli produced no final message")
	}
	return content, nil
}

func languageCodes(languages []app.Language) []string {
	codes := make([]string, 0, len(languages))
	for _, language := range languages {
		codes = append(codes, string(language))
	}
	return codes
}

func pickLanguages(raw map[string]string, languages []app.Language) app.Summary {
	result := app.Summary{}
	for _, language := range languages {
		if text := strings.TrimSpace(raw[string(language)]); text != "" {
			result[language] = text
		}
	}
	return result
}

func formatSummary(summary app.Summary) string {
	var builder strings.Builder
	for _, language := range app.SummaryLanguages() {
		if text := strings.TrimSpace(summary[language]); text != "" {
			fmt.Fprintf(&builder, "%s: %s\n", strings.ToUpper(string(language)), text)
		}
	}
	return builder.String()
}

func codexError(err error, stderr []byte) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	message := "codex cli chat failed"
	if snippet := sanitizeStderr(stderr); snippet != "" {
		message += ": " + snippet
	}
	return errors.New(message)
}

func sanitizeStderr(stderr []byte) string {
	message := strings.TrimSpace(string(stderr))
	if message == "" {
		return ""
	}
	if len(message) > maxStderrBytes {
		message = message[:maxStderrBytes] + "..."
	}
	return message
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args []string, stdin string) ([]byte, []byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdin = strings.NewReader(stdin)
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

func cleanContextError(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return nil
}
