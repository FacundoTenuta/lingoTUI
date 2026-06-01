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
	app       App
	ctx       context.Context
	Input     string
	Messages  []string
	Err       error
	MenuIndex int
	inputMode inputMode
}

type SubmitMsg struct{ Input string }

type inputMode int

const (
	menuMode inputMode = iota
	commandMode
	askMode
)

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
	messages := []string{"lingoTUI ready. Choose an action from the menu."}
	messages = append(messages, onboardingLines...)
	return Model{
		app:       service,
		ctx:       context.Background(),
		Messages:  messages,
		inputMode: menuMode,
	}
}

func Submit(input string) SubmitMsg { return SubmitMsg{Input: input} }

func (m Model) Init() tea.Cmd { return nil }
