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
	if !strings.Contains(view, "/record mic") || !strings.Contains(view, "/realtime start mic") || !strings.Contains(view, "/ask <question>") || !strings.Contains(view, "/translate <text>") || !strings.Contains(view, "auth.json") {
		t.Fatalf("view missing help: %s", view)
	}
	for _, want := range []string{"  /connect", "  /translate <text>", "show supported commands"} {
		if !strings.Contains(view, want) {
			t.Fatalf("help output missing readable entry %q: %s", want, view)
		}
	}
}

func TestModelUpdateShowsModelsAsInfo(t *testing.T) {
	model := submitModel(t, NewModel(app.NewService(app.Dependencies{})), "/models")
	view := model.View()

	if model.Status != statusInfo || !strings.Contains(view, "Info: Configured transcription:") || !strings.Contains(view, "chat:") {
		t.Fatalf("expected models info status, status=%v view=%s", model.Status, view)
	}
}

func TestNewModelShowsStartupOnboarding(t *testing.T) {
	fake := &fakeApp{}
	model := NewModel(fake, "Setup status:", "- OpenAI credentials: missing — add auth.json before /connect")
	view := model.View()
	for _, want := range []string{"LingoTUI", "Connection: offline", "Recording: idle", "Realtime: off", "Status: Idle: Ready. Choose an action", "[ Setup ]", "Setup status", "auth.json", "/connect", "[ Footer ]", "Enter run", "Esc back/quit", "Ctrl+C quit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q: %s", want, view)
		}
	}
	if len(model.Messages) != 0 || len(fake.inputs) != 0 {
		t.Fatalf("startup mixed setup/history or called app: messages=%v calls=%v view=%s", model.Messages, fake.inputs, view)
	}
}

func TestNewModelShowsInteractiveMenu(t *testing.T) {
	fake := &fakeApp{}
	model := NewModel(fake)
	view := model.View()

	for _, want := range []string{"[ Menu/Input ]", "Use up/down or k/j", "> Ask", "Translate", "Realtime mic", "Record mic", "Connect", "Shortcuts: Enter run | Esc back/quit | Ctrl+C quit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q: %s", want, view)
		}
	}
	if len(fake.inputs) != 0 {
		t.Fatalf("startup called app with %v", fake.inputs)
	}
}

func TestViewShowsPersistentStatusChips(t *testing.T) {
	model := Model{
		Status:        statusSuccess,
		StatusMessage: "Recording microphone audio. Run /stop to process it.",
		inputMode:     menuMode,
		connected:     true,
		recording:     true,
		realtime:      true,
	}
	view := model.View()

	for _, want := range []string{"Connection: connected", "Recording: active", "Realtime: active", "Status: Success: Recording microphone audio"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q: %s", want, view)
		}
	}
}

func TestModelRecordCommandCompletionUpdatesRecordingIndicator(t *testing.T) {
	fake := &fakeApp{result: app.Result{Command: app.CommandRecord, Message: "Recording microphone audio. Run /stop to process it.", Recording: true}}
	model := submitModel(t, NewModel(fake), "/record mic")

	if !model.recording || !strings.Contains(model.View(), "Recording: active") {
		t.Fatalf("expected recording indicator: %s", model.View())
	}
}

func TestModelRealtimeCommandCompletionUpdatesRealtimeIndicator(t *testing.T) {
	fake := &fakeApp{result: app.Result{Command: app.CommandRealtime, Message: "Realtime translation started.", Realtime: true}}
	model, cmd := submitModelPending(t, NewModel(fake), "/realtime start mic")
	model, _ = applyCommandWithNext(t, model, cmd)

	if !model.realtime || !strings.Contains(model.View(), "Realtime: active") {
		t.Fatalf("expected realtime indicator: %s", model.View())
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
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyDown})
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

func TestModelUpdateTranslateOptionSubmitsText(t *testing.T) {
	fake := &fakeApp{result: app.Result{Command: app.CommandTranslate, Message: "translated"}}
	model := NewModel(fake)
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyDown})

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.inputMode != translateMode || !strings.Contains(model.View(), "Translate > ") {
		t.Fatalf("expected translate mode, got mode=%v view=%s", model.inputMode, model.View())
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hola mundo")})
	model, cmd := updateModelWithCmd(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	if got := strings.Join(fake.inputs, ","); got != "" {
		t.Fatalf("app called before translate command execution: %q", got)
	}
	if cmd == nil || model.Status != statusLoading || model.StatusMessage != "Processing request..." {
		t.Fatalf("status=%v message=%q cmd=%v", model.Status, model.StatusMessage, cmd)
	}

	model = applyCommand(t, model, cmd)

	if got, want := strings.Join(fake.inputs, ","), "/translate hola mundo"; got != want {
		t.Fatalf("inputs = %q, want %q", got, want)
	}
	if model.inputMode != menuMode || model.Input != "" {
		t.Fatalf("mode=%v input=%q", model.inputMode, model.Input)
	}
}

func TestModelUpdateTranslateOptionIgnoresEmptyText(t *testing.T) {
	fake := &fakeApp{}
	model := NewModel(fake)
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyDown})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
	model, cmd := updateModelWithCmd(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	if cmd != nil || len(fake.inputs) != 0 || model.inputMode != menuMode {
		t.Fatalf("empty translate should only return to menu, cmd=%v inputs=%v mode=%v", cmd, fake.inputs, model.inputMode)
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
		{name: "translate", model: Model{app: &fakeApp{}, inputMode: translateMode, Input: "text"}},
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
		{name: "translate", model: Model{app: &fakeApp{}, inputMode: translateMode, Input: "text"}},
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
		Transcriber: testutil.Provider{Transcript: app.Transcript{Text: "hola mundo\nsegunda linea"}, Summary: app.Summary{
			app.LanguageSpanish: "saludo",
			app.LanguageEnglish: "greeting",
			app.LanguageGerman:  "begrüßung",
		}, AnswerText: app.Answer("A greeting.")},
		Chat: testutil.Provider{Transcript: app.Transcript{Text: "hola mundo\nsegunda linea"}, Summary: app.Summary{
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
	for _, want := range []string{"Runtime config is ready", "Recording microphone", "Transcript:", "hola mundo", "segunda linea", "Summary:", "ES:", "saludo", "EN:", "greeting", "DE:", "begrüßung", "A greeting.", "Cleared in-memory"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q: %s", want, view)
		}
	}
	if count := strings.Count(view, "Transcript:"); count != 1 {
		t.Fatalf("Transcript rendered %d times, want once after stop only: %s", count, view)
	}
}

func TestModelStopResultShowsTranscriptAndSummariesInHistory(t *testing.T) {
	fake := &fakeApp{result: app.Result{
		Command: app.CommandStop,
		Message: "Processed recording and updated ES/EN/DE context.",
		Context: app.RecentContext{
			Transcript: app.Transcript{Text: "full transcript line one\nfull transcript line two"},
			Summary: app.Summary{
				app.LanguageSpanish: "resumen en español\nsegunda línea",
				app.LanguageEnglish: "summary in English\nsecond line",
				app.LanguageGerman:  "Zusammenfassung auf Deutsch\nzweite Zeile",
			},
		},
	}}
	model := submitModel(t, NewModel(fake), "/stop")
	view := model.View()

	for _, want := range []string{"History", "> /stop", "Transcript:", "full transcript line one", "full transcript line two", "Summary:", "ES:", "resumen en español", "segunda línea", "EN:", "summary in English", "second line", "DE:", "Zusammenfassung auf Deutsch", "zweite Zeile"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q: %s", want, view)
		}
	}
}

func TestModelTranslateResultShowsTranslationsInHistory(t *testing.T) {
	fake := &fakeApp{result: app.Result{
		Command: app.CommandTranslate,
		Message: "Translated text into ES/EN/DE.",
		Translations: app.Translations{
			app.LanguageSpanish: "hola",
			app.LanguageEnglish: "hello",
			app.LanguageGerman:  "hallo",
		},
	}}
	model := submitModel(t, NewModel(fake), "/translate hello")
	view := model.View()

	for _, want := range []string{"History", "> /translate hello", "Translations:", "ES:", "hola", "EN:", "hello", "DE:", "hallo"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q: %s", want, view)
		}
	}
}

func TestModelRealtimeMenuStartAndManualTickShowsChunks(t *testing.T) {
	fake := &fakeApp{results: []app.Result{
		{Command: app.CommandRealtime, Message: "Realtime translation started.", Realtime: true},
		{Command: app.CommandRealtime, Message: "Translated realtime chunk.", Realtime: true, Chunks: []app.RealtimeChunk{{
			Transcript: app.Transcript{Text: "hola mundo"},
			Translations: app.Translations{
				app.LanguageSpanish: "hola mundo",
				app.LanguageEnglish: "hello world",
				app.LanguageGerman:  "hallo welt",
			},
		}}},
	}}
	model := NewModel(fake)
	tickCalls := 0
	model.realtimeTick = func() tea.Cmd {
		tickCalls++
		return func() tea.Msg { return RealtimeTickMsg{} }
	}
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyDown})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyDown})
	model, cmd := updateModelWithCmd(t, model, tea.KeyMsg{Type: tea.KeyEnter})
	model, tickCmd := applyCommandWithNext(t, model, cmd)

	if !model.realtime || tickCmd == nil || tickCalls != 1 || strings.Join(fake.inputs, ",") != "/realtime start mic" {
		t.Fatalf("realtime=%v tickCmd=%v tickCalls=%d inputs=%v view=%s", model.realtime, tickCmd, tickCalls, fake.inputs, model.View())
	}
	model, cmd = updateModelWithCmd(t, model, tickCmd())
	if cmd == nil || strings.Join(fake.inputs, ",") != "/realtime start mic" {
		t.Fatalf("tick should return command without eager app call: cmd=%v inputs=%v", cmd, fake.inputs)
	}
	model, tickCmd = applyCommandWithNext(t, model, cmd)
	if tickCmd == nil || tickCalls != 2 {
		t.Fatalf("successful chunk should schedule next tick: tickCmd=%v tickCalls=%d", tickCmd, tickCalls)
	}
	view := model.View()
	for _, want := range []string{"Realtime:", "Chunk 1:", "Transcript:", "hola mundo", "ES:", "hola mundo", "EN:", "hello world", "DE:", "hallo welt"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q: %s", want, view)
		}
	}
	if got, want := strings.Join(fake.inputs, ","), "/realtime start mic,/realtime chunk"; got != want {
		t.Fatalf("inputs = %q, want %q", got, want)
	}
}

func TestModelRealtimeStopDisablesFurtherTickDispatch(t *testing.T) {
	fake := &fakeApp{results: []app.Result{
		{Command: app.CommandRealtime, Message: "Realtime translation started.", Realtime: true},
		{Command: app.CommandRealtime, Message: "Stopped realtime translation.", Realtime: false},
	}}
	model := NewModel(fake)
	model.realtimeTick = func() tea.Cmd { return func() tea.Msg { return RealtimeTickMsg{} } }
	model, cmd := submitModelPending(t, model, "/realtime start mic")
	model, tickCmd := applyCommandWithNext(t, model, cmd)
	if !model.realtime {
		t.Fatalf("expected realtime active after start: %s", model.View())
	}
	if tickCmd == nil {
		t.Fatal("expected realtime start to schedule a tick")
	}
	model = submitModel(t, model, "/realtime stop")
	if model.realtime {
		t.Fatalf("expected realtime inactive after stop: %s", model.View())
	}
	model, cmd = updateModelWithCmd(t, model, RealtimeTickMsg{})
	if cmd != nil || strings.Join(fake.inputs, ",") != "/realtime start mic,/realtime stop" {
		t.Fatalf("tick after stop dispatched: cmd=%v inputs=%v", cmd, fake.inputs)
	}
}

func TestModelRealtimeStartShorthandSchedulesTick(t *testing.T) {
	fake := &fakeApp{result: app.Result{Command: app.CommandRealtime, Message: "Realtime translation started.", Realtime: true}}
	model := NewModel(fake)
	tickCalls := 0
	model.realtimeTick = func() tea.Cmd {
		tickCalls++
		return func() tea.Msg { return RealtimeTickMsg{} }
	}

	model, cmd := submitModelPending(t, model, "/realtime start")
	model, tickCmd := applyCommandWithNext(t, model, cmd)

	if !model.realtime || tickCmd == nil || tickCalls != 1 || strings.Join(fake.inputs, ",") != "/realtime start" {
		t.Fatalf("realtime=%v tickCmd=%v tickCalls=%d inputs=%v", model.realtime, tickCmd, tickCalls, fake.inputs)
	}
}

func TestModelRealtimeErrorPreservesRealtimeState(t *testing.T) {
	fake := &fakeApp{result: app.Result{Command: app.CommandRealtime, Realtime: true}, err: errors.New("unknown command")}
	model := NewModel(fake)
	model.realtime = true
	tickCalls := 0
	model.realtimeTick = func() tea.Cmd {
		tickCalls++
		return func() tea.Msg { return RealtimeTickMsg{} }
	}

	model, command := submitModelPending(t, model, "/wat")
	model, droppedTick := updateModelWithCmd(t, model, RealtimeTickMsg{})
	if droppedTick != nil {
		t.Fatalf("tick while loading should be dropped: %v", droppedTick)
	}
	model, nextTick := applyCommandWithNext(t, model, command)

	if !model.realtime || nextTick == nil || tickCalls != 1 {
		t.Fatalf("expected realtime to remain active and reschedule after error: realtime=%v nextTick=%v tickCalls=%d view=%s", model.realtime, nextTick, tickCalls, model.View())
	}
}

func TestModelRealtimeSuccessAfterDroppedTickSchedulesReplacement(t *testing.T) {
	fake := &fakeApp{result: app.Result{Command: app.CommandAsk, Message: "answered", Realtime: true}}
	model := NewModel(fake)
	model.realtime = true
	tickCalls := 0
	model.realtimeTick = func() tea.Cmd {
		tickCalls++
		return func() tea.Msg { return RealtimeTickMsg{} }
	}

	model, command := submitModelPending(t, model, "/ask what happened?")
	model, droppedTick := updateModelWithCmd(t, model, RealtimeTickMsg{})
	if droppedTick != nil {
		t.Fatalf("tick while loading should be dropped: %v", droppedTick)
	}
	model, nextTick := applyCommandWithNext(t, model, command)

	if !model.realtime || nextTick == nil || tickCalls != 1 {
		t.Fatalf("expected realtime success to reschedule dropped tick: realtime=%v nextTick=%v tickCalls=%d view=%s", model.realtime, nextTick, tickCalls, model.View())
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

func applyCommandWithNext(t *testing.T, model Model, cmd tea.Cmd) (Model, tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected command")
	}
	updated, nextCmd := model.Update(cmd())
	return modelFromTea(t, updated), nextCmd
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
	result  app.Result
	results []app.Result
	err     error
	errs    []error
	inputs  []string
}

func (f *fakeApp) HandleInput(_ context.Context, input string) (app.Result, error) {
	f.inputs = append(f.inputs, input)
	index := len(f.inputs) - 1
	if len(f.results) > index || len(f.errs) > index {
		var result app.Result
		var err error
		if len(f.results) > index {
			result = f.results[index]
		}
		if len(f.errs) > index {
			err = f.errs[index]
		}
		return result, err
	}
	return f.result, f.err
}

var _ tea.Model = Model{}
