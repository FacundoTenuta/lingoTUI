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
	if m.inputMode == menuMode {
		b.WriteString("Use up/down or k/j to choose, Enter to run. Type / for a command.\n")
		for i, item := range menuItems {
			cursor := "  "
			if i == m.MenuIndex {
				cursor = "> "
			}
			b.WriteString(promptStyle.Render(cursor))
			b.WriteString(item.Label)
			if item.Description != "" {
				b.WriteString(" - ")
				b.WriteString(item.Description)
			}
			b.WriteByte('\n')
		}
		return b.String()
	}
	if m.inputMode == askMode {
		b.WriteString(promptStyle.Render("Ask > "))
	} else {
		b.WriteString(promptStyle.Render("> "))
	}
	b.WriteString(m.Input)
	return b.String()
}
