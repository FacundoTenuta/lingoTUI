package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
	memory "github.com/FacundoTenuta/lingoTUI/internal/context"
	"github.com/FacundoTenuta/lingoTUI/internal/testutil"
	tea "github.com/charmbracelet/bubbletea"
)

func TestModelUpdateShowsHelp(t *testing.T) {
	model := NewModel(app.NewService(app.Dependencies{}))

	updated := submitModel(t, model, "/help")
	view := updated.View()
	if updated.Status != statusInfo || !strings.Contains(view, "Info: Supported commands:") {
		t.Fatalf("expected info status, status=%v view=%s", updated.Status, view)
	}
	if !strings.Contains(view, "/record mic") || !strings.Contains(view, "/ask <question>") || !strings.Contains(view, "auth.json") {
		t.Fatalf("view missing help: %s", view)
	}
}

func TestModelUpdateShowsModelsAsInfo(t *testing.T) {
	model := submitModel(t, NewModel(app.NewService(app.Dependencies{})), "/models")
	view := model.View()

	if model.Status != statusInfo || !strings.Contains(view, "Info: Transcription:") || !strings.Contains(view, "Chat:") {
		t.Fatalf("expected models info status, status=%v view=%s", model.Status, view)
	}
}

func TestNewModelShowsStartupOnboarding(t *testing.T) {
	fake := &fakeApp{}
	model := NewModel(fake, "Setup status:", "- OpenAI credentials: missing — add auth.json before /connect")
	view := model.View()
	for _, want := range []string{"lingoTUI", "Status", "Idle: Ready. Choose an action", "Setup", "Setup status", "auth.json", "/connect"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q: %s", want, view)
		}
	}
	if strings.Contains(view, "History") || len(model.Messages) != 0 || len(fake.inputs) != 0 {
		t.Fatalf("startup mixed setup/history or called app: messages=%v calls=%v view=%s", model.Messages, fake.inputs, view)
	}
}

func TestNewModelShowsInteractiveMenu(t *testing.T) {
	fake := &fakeApp{}
	model := NewModel(fake)
	view := model.View()

	for _, want := range []string{"Use up/down or k/j", "> Ask", "Record mic", "Connect"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q: %s", want, view)
		}
	}
	if len(fake.inputs) != 0 {
		t.Fatalf("startup called app with %v", fake.inputs)
	}
}

func TestModelUpdateMenuNavigationDoesNotCallApp(t *testing.T) {
	fake := &fakeApp{}
	model := NewModel(fake)

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyDown})
	if model.MenuIndex != 1 || model.Input != "" || len(fake.inputs) != 0 {
		t.Fatalf("after down: index=%d input=%q calls=%v", model.MenuIndex, model.Input, fake.inputs)
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyUp})
	if model.MenuIndex != 0 || model.Input != "" || len(fake.inputs) != 0 {
		t.Fatalf("after up: index=%d input=%q calls=%v", model.MenuIndex, model.Input, fake.inputs)
	}
}

func TestModelSubmitStartsLoadingBeforeCommandRuns(t *testing.T) {
	fake := &fakeApp{result: app.Result{Command: app.CommandConnect, Message: "connected"}}
	model, cmd := submitModelPending(t, NewModel(fake), "  /connect  ")
	view := model.View()

	if cmd == nil {
		t.Fatal("expected submit to return command")
	}
	if got := strings.Join(fake.inputs, ","); got != "" {
		t.Fatalf("app called before command execution: %q", got)
	}
	if model.Status != statusLoading || model.StatusMessage != "Running /connect..." {
		t.Fatalf("status=%v message=%q", model.Status, model.StatusMessage)
	}
	if model.Input != "" || model.inputMode != menuMode {
		t.Fatalf("input=%q mode=%v", model.Input, model.inputMode)
	}
	if !strings.Contains(view, "> /connect") || strings.Contains(view, "connected") {
		t.Fatalf("view should show command but not result before cmd runs: %s", view)
	}
}

func TestModelCommandCompletionAppliesResultAndError(t *testing.T) {
	tests := []struct {
		name       string
		result     app.Result
		err        error
		wantStatus statusState
		wantView   string
	}{
		{
			name:       "success",
			result:     app.Result{Command: app.CommandConnect, Message: "connected"},
			wantStatus: statusSuccess,
			wantView:   "Success: connected",
		},
		{
			name:       "info",
			result:     app.Result{Command: app.CommandModels, Message: "listed"},
			wantStatus: statusInfo,
			wantView:   "Info: listed",
		},
		{
			name:       "error",
			err:        errors.New("provider unavailable"),
			wantStatus: statusError,
			wantView:   "Error: provider unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeApp{result: tt.result, err: tt.err}
			model, cmd := submitModelPending(t, NewModel(fake), "/connect")
			model = applyCommand(t, model, cmd)

			if model.Status != tt.wantStatus {
				t.Fatalf("status=%v want=%v", model.Status, tt.wantStatus)
			}
			if !strings.Contains(model.View(), tt.wantView) {
				t.Fatalf("view missing %q: %s", tt.wantView, model.View())
			}
		})
	}
}

func TestModelUpdateEnterDispatchesSelectedStaticCommand(t *testing.T) {
	fake := &fakeApp{result: app.Result{Message: "help shown"}}
	model := NewModel(fake)
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyDown})
	model, cmd := updateModelWithCmd(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	if got := strings.Join(fake.inputs, ","); got != "" {
		t.Fatalf("app called before command execution: %q", got)
	}
	if cmd == nil || model.Status != statusLoading || !strings.Contains(model.View(), "Loading: Running /help...") {
		t.Fatalf("expected loading with returned command, status=%v cmd=%v view=%s", model.Status, cmd, model.View())
	}

	model = applyCommand(t, model, cmd)

	if got, want := strings.Join(fake.inputs, ","), "/help"; got != want {
		t.Fatalf("inputs = %q, want %q", got, want)
	}
	if !strings.Contains(model.View(), "> /help") || !strings.Contains(model.View(), "help shown") {
		t.Fatalf("view missing submitted command/result: %s", model.View())
	}
}

func TestModelUpdateAskOptionSubmitsQuestion(t *testing.T) {
	fake := &fakeApp{result: app.Result{Command: app.CommandAsk, Message: "answered"}}
	model := NewModel(fake)

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.inputMode != askMode || !strings.Contains(model.View(), "Ask > ") {
		t.Fatalf("expected ask mode, got mode=%v view=%s", model.inputMode, model.View())
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("what happened?")})
	model, cmd := updateModelWithCmd(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	if got := strings.Join(fake.inputs, ","); got != "" {
		t.Fatalf("app called before ask command execution: %q", got)
	}
	if cmd == nil || model.Status != statusLoading || model.StatusMessage != "Processing request..." {
		t.Fatalf("status=%v message=%q cmd=%v", model.Status, model.StatusMessage, cmd)
	}

	model = applyCommand(t, model, cmd)

	if got, want := strings.Join(fake.inputs, ","), "/ask what happened?"; got != want {
		t.Fatalf("inputs = %q, want %q", got, want)
	}
	if model.inputMode != menuMode || model.Input != "" {
		t.Fatalf("mode=%v input=%q", model.inputMode, model.Input)
	}
}

func TestModelUpdateIgnoresNewSubmissionsWhileLoading(t *testing.T) {
	fake := &fakeApp{result: app.Result{Command: app.CommandConnect, Message: "connected"}}
	model, firstCmd := submitModelPending(t, NewModel(fake), "/connect")

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyDown})
	if model.MenuIndex != 1 || model.Status != statusLoading {
		t.Fatalf("navigation while loading failed: index=%d status=%v", model.MenuIndex, model.Status)
	}
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/models")})
	if model.inputMode != menuMode || model.Input != "" {
		t.Fatalf("typing while loading should be ignored: mode=%v input=%q", model.inputMode, model.Input)
	}

	model, secondCmd := updateModelWithCmd(t, model, tea.KeyMsg{Type: tea.KeyEnter})
	if secondCmd != nil {
		t.Fatal("expected enter submission while loading to be ignored")
	}
	model, thirdCmd := updateModelWithCmd(t, model, Submit("/models"))
	if thirdCmd != nil {
		t.Fatal("expected SubmitMsg while loading to be ignored")
	}
	if got := strings.Join(fake.inputs, ","); got != "" {
		t.Fatalf("app called before first command execution: %q", got)
	}

	model = applyCommand(t, model, firstCmd)
	if got, want := strings.Join(fake.inputs, ","), "/connect"; got != want {
		t.Fatalf("inputs = %q, want %q", got, want)
	}
}

func TestModelUpdateCtrlCQuitsFromAllModes(t *testing.T) {
	tests := []struct {
		name  string
		model Model
	}{
		{name: "menu", model: NewModel(&fakeApp{})},
		{name: "ask", model: Model{app: &fakeApp{}, inputMode: askMode, Input: "question"}},
		{name: "command", model: Model{app: &fakeApp{}, inputMode: commandMode, Input: "/help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cmd := tt.model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
			if cmd == nil {
				t.Fatalf("expected ctrl+c to quit from %s mode", tt.name)
			}
		})
	}
}

func TestModelUpdateEscCancelsInputModes(t *testing.T) {
	tests := []struct {
		name  string
		model Model
	}{
		{name: "ask", model: Model{app: &fakeApp{}, inputMode: askMode, Input: "question"}},
		{name: "command", model: Model{app: &fakeApp{}, inputMode: commandMode, Input: "/help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated, cmd := tt.model.Update(tea.KeyMsg{Type: tea.KeyEsc})
			if cmd != nil {
				t.Fatalf("expected esc to cancel without quitting")
			}
			model := modelFromTea(t, updated)
			if model.inputMode != menuMode || model.Input != "" {
				t.Fatalf("mode=%v input=%q", model.inputMode, model.Input)
			}
		})
	}
}

func TestModelUpdateEscQuitsFromMenu(t *testing.T) {
	_, cmd := NewModel(&fakeApp{}).Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected esc to quit from menu mode")
	}
}

func TestModelUpdateEnterClampsInvalidMenuIndex(t *testing.T) {
	fake := &fakeApp{result: app.Result{Message: "answered"}}
	model := NewModel(fake)
	model.MenuIndex = len(menuItems) + 10

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	if model.inputMode != askMode || model.MenuIndex != 0 || len(fake.inputs) != 0 {
		t.Fatalf("mode=%v index=%d calls=%v", model.inputMode, model.MenuIndex, fake.inputs)
	}
}

func TestModelUpdateSubmitMsgDirectCommandStillWorks(t *testing.T) {
	fake := &fakeApp{result: app.Result{Command: app.CommandModels, Message: "listed"}}
	model, cmd := submitModelPending(t, NewModel(fake), "/models")

	if got := strings.Join(fake.inputs, ","); got != "" {
		t.Fatalf("app called before SubmitMsg command execution: %q", got)
	}
	model = applyCommand(t, model, cmd)

	if got, want := strings.Join(fake.inputs, ","), "/models"; got != want {
		t.Fatalf("inputs = %q, want %q", got, want)
	}
	if model.Status != statusInfo || !strings.Contains(model.View(), "Info: listed") || !strings.Contains(model.View(), "listed") {
		t.Fatalf("view missing result: %s", model.View())
	}
}

func TestModelUpdateSuccessfulActionShowsSuccessStatus(t *testing.T) {
	fake := &fakeApp{result: app.Result{Command: app.CommandConnect, Message: "Connected to openai with local credentials."}}
	model := submitModel(t, NewModel(fake), "/connect")
	view := model.View()

	if got, want := strings.Join(fake.inputs, ","), "/connect"; got != want {
		t.Fatalf("inputs = %q, want %q", got, want)
	}
	if model.Status != statusSuccess || model.Err != nil || !strings.Contains(view, "Success: Connected to openai") {
		t.Fatalf("expected success status, status=%v err=%v view=%s", model.Status, model.Err, view)
	}
}

func TestModelUpdateErrorShowsRecoveryHint(t *testing.T) {
	fake := &fakeApp{err: errors.New("provider unavailable")}
	model := submitModel(t, NewModel(fake), "/connect")
	view := model.View()

	if model.Status != statusError || model.Err == nil {
		t.Fatalf("expected error status, status=%v err=%v", model.Status, model.Err)
	}
	if strings.Contains(view, "Error: Error.") {
		t.Fatalf("view has duplicated error prefix: %s", view)
	}
	for _, want := range []string{"Error: Fix the issue below", "provider unavailable", "Fix the error above, or run /help"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q: %s", want, view)
		}
	}
}

func TestModelUpdateErrorStripsDuplicateExternalErrorPrefix(t *testing.T) {
	fake := &fakeApp{err: errors.New("Error. provider unavailable")}
	model := submitModel(t, NewModel(fake), "/connect")
	view := model.View()

	if strings.Contains(view, "Error: Error") {
		t.Fatalf("view has duplicated external error prefix: %s", view)
	}
	if !strings.Contains(view, "Error: provider unavailable") {
		t.Fatalf("view missing cleaned error: %s", view)
	}
}

func TestModelUpdateUnknownCommandPreservesState(t *testing.T) {
	model := NewModel(app.NewService(app.Dependencies{}))
	before := len(model.Messages)

	updated := submitModel(t, model, "/wat")
	if updated.Err == nil || len(updated.Messages) <= before {
		t.Fatalf("updated = %+v", updated)
	}
	if !strings.Contains(updated.View(), "unknown command") {
		t.Fatalf("view missing error: %s", updated.View())
	}
}

func TestModelUpdateConnectRecordStopAskAndClear(t *testing.T) {
	store := memory.NewMemory()
	service := app.NewService(app.Dependencies{
		Recorder: &testutil.Recorder{File: app.AudioFile{Path: "mic.wav"}},
		Transcriber: testutil.Provider{Transcript: app.Transcript{Text: "hola"}, Summary: app.Summary{
			app.LanguageSpanish: "saludo",
			app.LanguageEnglish: "greeting",
			app.LanguageGerman:  "begrüßung",
		}, AnswerText: app.Answer("A greeting.")},
		Chat: testutil.Provider{Transcript: app.Transcript{Text: "hola"}, Summary: app.Summary{
			app.LanguageSpanish: "saludo",
			app.LanguageEnglish: "greeting",
			app.LanguageGerman:  "begrüßung",
		}, AnswerText: app.Answer("A greeting.")},
		Credentials: &testutil.CredentialStore{Secrets: map[app.ProviderID]app.Secret{app.ProviderOpenAI: {Value: "sk-test"}}},
		Context:     store,
	})
	model := NewModel(service)
	for _, input := range []string{"/connect", "/record mic", "/stop", "/ask what happened?", "/clear"} {
		model = submitModel(t, model, input)
		if model.Err != nil {
			t.Fatalf("%s error = %v; view: %s", input, model.Err, model.View())
		}
		if model.Status != statusSuccess {
			t.Fatalf("%s status = %v; view: %s", input, model.Status, model.View())
		}
	}
	view := model.View()
	for _, want := range []string{"Connected to openai", "Recording microphone", "ES: saludo", "A greeting.", "Cleared in-memory"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q: %s", want, view)
		}
	}
}

func TestModelUpdateShowsProviderAndRecorderErrors(t *testing.T) {
	tests := []struct {
		name    string
		service *app.Service
		input   string
		want    string
	}{
		{
			name:    "recorder error",
			service: app.NewService(app.Dependencies{Recorder: &testutil.Recorder{StartErr: errors.New("microphone denied")}}),
			input:   "/record mic",
			want:    "microphone denied",
		},
		{
			name: "provider error",
			service: app.NewService(app.Dependencies{
				Recorder:    &testutil.Recorder{},
				Transcriber: testutil.Provider{Err: errors.New("provider unavailable")},
				Chat:        testutil.Provider{},
				Context:     &testutil.ContextStore{},
			}),
			input: "/record mic\n/stop",
			want:  "provider unavailable",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel(tt.service)
			for _, input := range strings.Split(tt.input, "\n") {
				model = submitModel(t, model, input)
			}
			if model.Err == nil || !strings.Contains(model.View(), tt.want) {
				t.Fatalf("view missing %q: %s", tt.want, model.View())
			}
		})
	}
}

func submitModel(t *testing.T, model Model, input string) Model {
	t.Helper()
	updated, cmd := model.Update(Submit(input))
	result := modelFromTea(t, updated)
	if cmd == nil {
		return result
	}
	return applyCommand(t, result, cmd)
}

func submitModelPending(t *testing.T, model Model, input string) (Model, tea.Cmd) {
	t.Helper()
	return updateModelWithCmd(t, model, Submit(input))
}

func applyCommand(t *testing.T, model Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected command")
	}
	updated, nextCmd := model.Update(cmd())
	if nextCmd != nil {
		t.Fatal("expected command completion not to return another command")
	}
	return modelFromTea(t, updated)
}

func updateModel(t *testing.T, model Model, msg tea.Msg) Model {
	t.Helper()
	updated, _ := model.Update(msg)
	return modelFromTea(t, updated)
}

func updateModelWithCmd(t *testing.T, model Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := model.Update(msg)
	return modelFromTea(t, updated), cmd
}

func modelFromTea(t *testing.T, updated tea.Model) Model {
	t.Helper()
	result, ok := updated.(Model)
	if !ok {
		t.Fatalf("updated model type = %T", updated)
	}
	return result
}

type fakeApp struct {
	result app.Result
	err    error
	inputs []string
}

func (f *fakeApp) HandleInput(_ context.Context, input string) (app.Result, error) {
	f.inputs = append(f.inputs, input)
	return f.result, f.err
}

var _ tea.Model = Model{}
