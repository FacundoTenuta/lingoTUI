package codexcli

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

func TestChatTranslateRunsCodexExecAndParsesJSON(t *testing.T) {
	runner := &fakeRunner{output: `{"es":"hola","en":"hello","de":"hallo"}`}
	chat, err := New(Config{BinaryPath: "/opt/bin/codex", WorkDir: "/workspace", Runner: runner})
	if err != nil {
		t.Fatal(err)
	}

	translations, err := chat.Translate(context.Background(), "hello", app.SummaryLanguages(), app.ModelRef{Provider: app.ProviderCodexCLI, Name: "gpt-5.1", Purpose: app.ModelPurposeChat})
	if err != nil {
		t.Fatal(err)
	}

	want := app.Translations{app.LanguageSpanish: "hola", app.LanguageEnglish: "hello", app.LanguageGerman: "hallo"}
	if !reflect.DeepEqual(translations, want) {
		t.Fatalf("translations = %+v, want %+v", translations, want)
	}
	if runner.name != "/opt/bin/codex" {
		t.Fatalf("name = %q", runner.name)
	}
	for _, wantArg := range []string{"exec", "--sandbox", "read-only", "--output-last-message", "--cd", "/workspace", "--model", "gpt-5.1", "-"} {
		if !hasArg(runner.args, wantArg) {
			t.Fatalf("args missing %q: %+v", wantArg, runner.args)
		}
	}
	for _, unsupportedArg := range []string{"--ask-for-approval", "never"} {
		if hasArg(runner.args, unsupportedArg) {
			t.Fatalf("args include unsupported approval flag %q: %+v", unsupportedArg, runner.args)
		}
	}
	for _, wantPrompt := range []string{"Respond only with a compact JSON object", "Languages: es, en, de", "Text:\nhello"} {
		if !strings.Contains(runner.stdin, wantPrompt) {
			t.Fatalf("stdin missing %q: %s", wantPrompt, runner.stdin)
		}
	}
}

func TestChatOmitsModelUnlessCodexModelConfigured(t *testing.T) {
	runner := &fakeRunner{output: `{"es":"hola"}`}
	chat, err := New(Config{Runner: runner})
	if err != nil {
		t.Fatal(err)
	}

	_, err = chat.Translate(context.Background(), "hello", []app.Language{app.LanguageSpanish}, app.ModelRef{Provider: app.ProviderOpenAI, Name: app.DefaultChatModel, Purpose: app.ModelPurposeChat})
	if err != nil {
		t.Fatal(err)
	}
	if hasArg(runner.args, "--model") || hasArg(runner.args, app.DefaultChatModel) {
		t.Fatalf("args must not pass OpenAI default model to Codex: %+v", runner.args)
	}
}

func TestChatPassesExplicitCodexModel(t *testing.T) {
	runner := &fakeRunner{output: `{"es":"hola"}`}
	chat, err := New(Config{Runner: runner})
	if err != nil {
		t.Fatal(err)
	}

	_, err = chat.Translate(context.Background(), "hello", []app.Language{app.LanguageSpanish}, app.ModelRef{Provider: app.ProviderCodexCLI, Name: "gpt-5.1", Purpose: app.ModelPurposeChat})
	if err != nil {
		t.Fatal(err)
	}
	if !hasArg(runner.args, "--model") || !hasArg(runner.args, "gpt-5.1") {
		t.Fatalf("args missing explicit codex model: %+v", runner.args)
	}
}

func TestChatAnswerReturnsFinalMessageText(t *testing.T) {
	runner := &fakeRunner{output: "They discussed the release."}
	chat, err := New(Config{Runner: runner})
	if err != nil {
		t.Fatal(err)
	}

	answer, err := chat.Answer(context.Background(), app.Question("what happened?"), app.RecentContext{
		Transcript: app.Transcript{Text: "They shipped a release."},
		Summary:    app.Summary{app.LanguageEnglish: "Release shipped"},
	}, app.ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	if answer != "They discussed the release." {
		t.Fatalf("answer = %q", answer)
	}
	if !strings.Contains(runner.stdin, "Answer using only the recent transcript") || !strings.Contains(runner.stdin, "Question: what happened?") {
		t.Fatalf("stdin = %s", runner.stdin)
	}
}

func TestChatReturnsSanitizedCommandFailure(t *testing.T) {
	runner := &fakeRunner{err: errors.New("exit 1"), stderr: []byte("not logged in\nrun codex login")}
	chat, err := New(Config{Runner: runner})
	if err != nil {
		t.Fatal(err)
	}

	_, err = chat.Translate(context.Background(), "hello", app.SummaryLanguages(), app.ModelRef{})
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"codex cli chat failed", "not logged in"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q: %v", want, err)
		}
	}
}

func TestChatRejectsEmptyFinalMessage(t *testing.T) {
	chat, err := New(Config{Runner: &fakeRunner{output: "  "}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = chat.Summarize(context.Background(), app.Transcript{Text: "hola"}, app.SummaryLanguages(), app.ModelRef{})
	if err == nil || !strings.Contains(err.Error(), "produced no final message") {
		t.Fatalf("error = %v", err)
	}
}

type fakeRunner struct {
	name   string
	args   []string
	stdin  string
	output string
	stderr []byte
	err    error
}

func (r *fakeRunner) Run(_ context.Context, name string, args []string, stdin string) ([]byte, []byte, error) {
	r.name = name
	r.args = append([]string(nil), args...)
	r.stdin = stdin
	if r.err != nil {
		return nil, r.stderr, r.err
	}
	for i, arg := range args {
		if arg == "--output-last-message" && i+1 < len(args) {
			if err := os.WriteFile(args[i+1], []byte(r.output), 0o600); err != nil {
				return nil, nil, err
			}
			return []byte("codex log output"), nil, nil
		}
	}
	return nil, nil, errors.New("missing --output-last-message")
}

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}
