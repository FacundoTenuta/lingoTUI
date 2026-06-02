package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	bannerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("204"))
	loadingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	promptStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
)

const banner = " _  _                      _____  _   _  ___\n" +
	"| |(_) _ _   __ _  ___   |_   _|| | | ||_ _|\n" +
	"| || || ' \\ / _` |/ _ \\    | |  | | | | | |\n" +
	"|_||_||_||_|\\__, |\\___/    |_|  |_| |_||___|\n" +
	"             |___/"

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(bannerStyle.Render(banner))
	b.WriteByte('\n')
	b.WriteString(titleStyle.Render("lingoTUI"))
	b.WriteString("\n\n")
	b.WriteString(titleStyle.Render("Status"))
	b.WriteByte('\n')
	b.WriteString(m.statusLine())
	b.WriteString("\n\n")
	if len(m.SetupLines) > 0 {
		b.WriteString(titleStyle.Render("Setup"))
		b.WriteByte('\n')
		for _, line := range m.SetupLines {
			b.WriteString(line)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	if len(m.Messages) > 0 {
		b.WriteString(titleStyle.Render("History"))
		b.WriteByte('\n')
	}
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
	if len(m.Messages) > 0 {
		b.WriteByte('\n')
	}
	if m.inputMode == menuMode {
		b.WriteString(titleStyle.Render("Menu"))
		b.WriteByte('\n')
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

func (m Model) statusLine() string {
	message := m.StatusMessage
	if message == "" {
		message = readyStatusMessage
	}
	line := "Idle: " + message
	switch m.Status {
	case statusInfo:
		line = "Info: " + message
		return infoStyle.Render(line)
	case statusSuccess:
		line = "Success: " + message
		return successStyle.Render(line)
	case statusError:
		line = "Error: " + message
		return errorStyle.Render(line)
	case statusLoading:
		line = "Loading: " + message
		return loadingStyle.Render(line)
	default:
		return line
	}
}
