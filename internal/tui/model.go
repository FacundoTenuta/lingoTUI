package tui

import (
	"context"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
	tea "github.com/charmbracelet/bubbletea"
)

type App interface {
	HandleInput(context.Context, string) (app.Result, error)
}

type Model struct {
	app           App
	ctx           context.Context
	Input         string
	Messages      []string
	SetupLines    []string
	Err           error
	Status        statusState
	StatusMessage string
	MenuIndex     int
	inputMode     inputMode
}

type SubmitMsg struct{ Input string }

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
)

const (
	statusIdle statusState = iota
	statusInfo
	statusSuccess
	statusError
	statusLoading
)

const readyStatusMessage = "Ready. Choose an action from the menu."

type menuItem struct {
	Label       string
	Description string
	Command     string
	Ask         bool
}

var menuItems = []menuItem{
	{Label: "Ask", Description: "Ask a question using the current context", Ask: true},
	{Label: "Help", Description: "Show available commands", Command: "/help"},
	{Label: "Models", Description: "List configured models", Command: "/models"},
	{Label: "Record mic", Description: "Start recording from the microphone", Command: "/record mic"},
	{Label: "Stop", Description: "Stop recording and process audio", Command: "/stop"},
	{Label: "Clear", Description: "Clear in-memory context", Command: "/clear"},
	{Label: "Connect", Description: "Connect to the configured provider", Command: "/connect"},
}

func NewModel(service App, onboardingLines ...string) Model {
	return Model{
		app:           service,
		ctx:           context.Background(),
		SetupLines:    append([]string(nil), onboardingLines...),
		Status:        statusIdle,
		StatusMessage: readyStatusMessage,
		inputMode:     menuMode,
	}
}

func Submit(input string) SubmitMsg { return SubmitMsg{Input: input} }

func (m Model) Init() tea.Cmd { return nil }
