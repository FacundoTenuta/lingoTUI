package tui

import (
	"fmt"
	"strings"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SubmitMsg:
		return m.submit(msg.Input), nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			return m.submit(m.Input), nil
		case "backspace":
			if len(m.Input) > 0 {
				m.Input = m.Input[:len(m.Input)-1]
			}
		default:
			m.Input += msg.String()
		}
	}
	return m, nil
}

func (m Model) submit(input string) Model {
	input = strings.TrimSpace(input)
	m.Input = ""
	if input == "" {
		return m
	}
	m.Messages = append(m.Messages, "> "+input)
	result, err := m.app.HandleInput(m.ctx, input)
	if err != nil {
		m.Err = err
		m.Messages = append(m.Messages, "Error: "+err.Error())
		return m
	}
	m.Err = nil
	for _, line := range formatResult(result) {
		m.Messages = append(m.Messages, line)
	}
	return m
}

func formatResult(result app.Result) []string {
	if len(result.Help) > 0 {
		lines := []string{result.Message}
		for _, entry := range result.Help {
			lines = append(lines, fmt.Sprintf("  %s — %s", entry.Command, entry.Description))
		}
		if len(result.Guidance) > 0 {
			lines = append(lines, "", "Setup guidance:")
			for _, line := range result.Guidance {
				lines = append(lines, "  "+line)
			}
		}
		return lines
	}
	if result.Context.Transcript.Text != "" || len(result.Context.Summary) > 0 {
		lines := []string{result.Message}
		if result.Context.Transcript.Text != "" {
			lines = append(lines, "Transcript: "+result.Context.Transcript.Text)
		}
		for _, language := range app.SummaryLanguages() {
			if text := result.Context.Summary[language]; text != "" {
				lines = append(lines, fmt.Sprintf("%s: %s", strings.ToUpper(string(language)), text))
			}
		}
		return lines
	}
	if result.Message == "" {
		return nil
	}
	return []string{result.Message}
}
