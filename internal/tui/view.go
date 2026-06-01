package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("204"))
	promptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
)

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("lingoTUI"))
	b.WriteString("\n\n")
	for _, message := range m.Messages {
		if strings.HasPrefix(message, "Error:") {
			b.WriteString(errorStyle.Render(message))
		} else {
			b.WriteString(message)
		}
		b.WriteByte('\n')
	}
	if m.Err != nil {
		b.WriteString(errorStyle.Render("Fix the error above, or run /help for available commands."))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(promptStyle.Render("> "))
	b.WriteString(m.Input)
	return b.String()
}
