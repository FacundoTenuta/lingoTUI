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
	if !strings.Contains(view, "/record mic") || !strings.Contains(view, "/ask <question>") {
		t.Fatalf("view missing help: %s", view)
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
	result, ok := updated.(Model)
	if !ok {
		t.Fatalf("updated model type = %T", updated)
	}
	return result
}

type fakeApp struct{ result app.Result }

func (f fakeApp) HandleInput(context.Context, string) (app.Result, error) { return f.result, nil }

var _ tea.Model = Model{}
