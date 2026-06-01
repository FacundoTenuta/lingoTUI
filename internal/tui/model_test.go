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
	if !strings.Contains(view, "/record mic") || !strings.Contains(view, "/ask <question>") || !strings.Contains(view, "auth.json") {
		t.Fatalf("view missing help: %s", view)
	}
}

func TestNewModelShowsStartupOnboarding(t *testing.T) {
	model := NewModel(app.NewService(app.Dependencies{}), "Setup status:", "- OpenAI credentials: missing — add auth.json before /connect")
	view := model.View()
	for _, want := range []string{"lingoTUI ready", "Setup status", "auth.json", "/connect"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q: %s", want, view)
		}
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

func TestModelUpdateEnterDispatchesSelectedStaticCommand(t *testing.T) {
	fake := &fakeApp{result: app.Result{Message: "help shown"}}
	model := NewModel(fake)
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyDown})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	if got, want := strings.Join(fake.inputs, ","), "/help"; got != want {
		t.Fatalf("inputs = %q, want %q", got, want)
	}
	if !strings.Contains(model.View(), "> /help") || !strings.Contains(model.View(), "help shown") {
		t.Fatalf("view missing submitted command/result: %s", model.View())
	}
}

func TestModelUpdateAskOptionSubmitsQuestion(t *testing.T) {
	fake := &fakeApp{result: app.Result{Message: "answered"}}
	model := NewModel(fake)

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.inputMode != askMode || !strings.Contains(model.View(), "Ask > ") {
		t.Fatalf("expected ask mode, got mode=%v view=%s", model.inputMode, model.View())
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("what happened?")})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	if got, want := strings.Join(fake.inputs, ","), "/ask what happened?"; got != want {
		t.Fatalf("inputs = %q, want %q", got, want)
	}
	if model.inputMode != menuMode || model.Input != "" {
		t.Fatalf("mode=%v input=%q", model.inputMode, model.Input)
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
	fake := &fakeApp{result: app.Result{Message: "listed"}}
	model := submitModel(t, NewModel(fake), "/models")

	if got, want := strings.Join(fake.inputs, ","), "/models"; got != want {
		t.Fatalf("inputs = %q, want %q", got, want)
	}
	if !strings.Contains(model.View(), "listed") {
		t.Fatalf("view missing result: %s", model.View())
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
	updated, _ := model.Update(Submit(input))
	return modelFromTea(t, updated)
}

func updateModel(t *testing.T, model Model, msg tea.Msg) Model {
	t.Helper()
	updated, _ := model.Update(msg)
	return modelFromTea(t, updated)
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
	inputs []string
}

func (f *fakeApp) HandleInput(_ context.Context, input string) (app.Result, error) {
	f.inputs = append(f.inputs, input)
	return f.result, nil
}

var _ tea.Model = Model{}
