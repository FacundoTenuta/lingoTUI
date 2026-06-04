package tui

import (
	"context"
	"time"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
	tea "github.com/charmbracelet/bubbletea"
)

type App interface {
	HandleInput(context.Context, string) (app.Result, error)
}

type Model struct {
	app                 App
	ctx                 context.Context
	Input               string
	Messages            []string
	SetupLines          []string
	Err                 error
	Status              statusState
	StatusMessage       string
	MenuIndex           int
	width               int
	height              int
	inputMode           inputMode
	connected           bool
	recording           bool
	realtime            bool
	droppedRealtimeTick bool
	realtimeTick        func() tea.Cmd
}

type SubmitMsg struct{ Input string }
type RealtimeTickMsg struct{}

type commandFinishedMsg struct {
	Input  string
	Result app.Result
	Err    error
}

type inputMode int

type statusState int

const (
	menuMode inputMode = iota
	commandMode
	askMode
	translateMode
)

const (
	statusIdle statusState = iota
	statusInfo
	statusSuccess
	statusError
	statusLoading
)

const readyStatusMessage = "Ready. Choose an action from the menu."

const realtimeTickInterval = 4 * time.Second

type menuItem struct {
	Label       string
	Description string
	Command     string
	Ask         bool
	Translate   bool
}

var menuItems = []menuItem{
	{Label: "Ask", Description: "Ask a question using the current context", Ask: true},
	{Label: "Translate", Description: "Translate text into ES/EN/DE", Translate: true},
	{Label: "Realtime mic", Description: "Start chunked realtime translation", Command: "/realtime start mic"},
	{Label: "Help", Description: "Show available commands", Command: "/help"},
	{Label: "Models", Description: "List configured models", Command: "/models"},
	{Label: "Record mic", Description: "Start recording from the microphone", Command: "/record mic"},
	{Label: "Stop", Description: "Stop recording and process audio", Command: "/stop"},
	{Label: "Clear", Description: "Clear in-memory context", Command: "/clear"},
	{Label: "Connect", Description: "Choose connection/provider option", Command: "/connect"},
}

func NewModel(service App, onboardingLines ...string) Model {
	return Model{
		app:           service,
		ctx:           context.Background(),
		SetupLines:    append([]string(nil), onboardingLines...),
		Status:        statusIdle,
		StatusMessage: readyStatusMessage,
		inputMode:     menuMode,
		realtimeTick:  defaultRealtimeTick,
	}
}

func Submit(input string) SubmitMsg { return SubmitMsg{Input: input} }

func (m Model) Init() tea.Cmd { return nil }

func defaultRealtimeTick() tea.Cmd {
	return tea.Tick(realtimeTickInterval, func(time.Time) tea.Msg { return RealtimeTickMsg{} })
}
