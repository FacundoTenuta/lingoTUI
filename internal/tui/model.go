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
	app      App
	ctx      context.Context
	Input    string
	Messages []string
	Err      error
}

type SubmitMsg struct{ Input string }

func NewModel(service App, onboardingLines ...string) Model {
	messages := []string{"lingoTUI ready. Type /help for commands."}
	messages = append(messages, onboardingLines...)
	return Model{
		app:      service,
		ctx:      context.Background(),
		Messages: messages,
	}
}

func Submit(input string) SubmitMsg { return SubmitMsg{Input: input} }

func (m Model) Init() tea.Cmd { return nil }
