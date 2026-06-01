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

func NewModel(service App) Model {
	return Model{
		app: service,
		ctx: context.Background(),
		Messages: []string{
			"lingoTUI ready. Type /help for commands.",
		},
	}
}

func Submit(input string) SubmitMsg { return SubmitMsg{Input: input} }

func (m Model) Init() tea.Cmd { return nil }
