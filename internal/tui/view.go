package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	bannerStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	subtleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	infoStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("204"))
	loadingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	promptStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	shortcutStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("248"))
)

const banner = "+-- LingoTUI ------------------------------------------------+\n" +
	"| terminal language copilot                                |\n" +
	"+-----------------------------------------------------------+"

const footerShortcuts = "Shortcuts: Enter run | Esc back/quit | Ctrl+C quit"

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(bannerStyle.Render(banner))
	b.WriteByte('\n')
	b.WriteString(m.statusStrip())
	b.WriteString("\n\n")
	b.WriteString(sectionTitle("Setup"))
	b.WriteByte('\n')
	if len(m.SetupLines) > 0 {
		for _, line := range m.SetupLines {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	} else {
		b.WriteString(subtleStyle.Render("No setup warnings. Run /connect to verify local runtime configuration."))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(sectionTitle("History"))
	b.WriteByte('\n')
	for _, message := range m.Messages {
		if strings.HasPrefix(message, "Error:") {
			b.WriteString(errorStyle.Render(message))
		} else {
			b.WriteString(message)
		}
		b.WriteByte('\n')
	}
	if len(m.Messages) == 0 {
		b.WriteString(subtleStyle.Render("No commands run yet."))
		b.WriteByte('\n')
	}
	if m.Err != nil {
		b.WriteString(errorStyle.Render("Fix the error above, or run /help for available commands."))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	if m.inputMode == menuMode {
		b.WriteString(sectionTitle("Menu/Input"))
		b.WriteByte('\n')
		b.WriteString(subtleStyle.Render("Use up/down or k/j to choose. Type / for a command."))
		b.WriteByte('\n')
		for i, item := range menuItems {
			cursor := "  "
			if i == m.MenuIndex {
				cursor = "> "
			}
			b.WriteString(promptStyle.Render(cursor))
			b.WriteString(titleStyle.Render(padRight(item.Label, 14)))
			if item.Description != "" {
				b.WriteString(" ")
				b.WriteString(item.Description)
			}
			b.WriteByte('\n')
		}
		b.WriteString("\n")
		b.WriteString(sectionTitle("Footer"))
		b.WriteByte('\n')
		b.WriteString(shortcutStyle.Render(footerShortcuts))
		return m.fillTerminalHeight(b.String())
	}
	if m.inputMode == connectMode {
		b.WriteString(sectionTitle("Connect"))
		b.WriteByte('\n')
		b.WriteString(subtleStyle.Render("Choose a connection option. Esc returns to the main menu."))
		b.WriteByte('\n')
		for i, item := range connectMenuItems {
			cursor := "  "
			if i == m.ConnectIndex {
				cursor = "> "
			}
			b.WriteString(promptStyle.Render(cursor))
			b.WriteString(titleStyle.Render(padRight(item.Label, 14)))
			if item.Description != "" {
				b.WriteString(" ")
				b.WriteString(item.Description)
			}
			b.WriteByte('\n')
		}
		b.WriteString("\n")
		b.WriteString(sectionTitle("Footer"))
		b.WriteByte('\n')
		b.WriteString(shortcutStyle.Render(footerShortcuts))
		return m.fillTerminalHeight(b.String())
	}
	b.WriteString(sectionTitle("Menu/Input"))
	b.WriteByte('\n')
	if m.inputMode == askMode {
		b.WriteString(promptStyle.Render("Ask > "))
	} else if m.inputMode == translateMode {
		b.WriteString(promptStyle.Render("Translate > "))
	} else {
		b.WriteString(promptStyle.Render("> "))
	}
	b.WriteString(m.Input)
	b.WriteString("\n\n")
	b.WriteString(sectionTitle("Footer"))
	b.WriteByte('\n')
	b.WriteString(shortcutStyle.Render(footerShortcuts))
	return m.fillTerminalHeight(b.String())
}

func (m Model) fillTerminalHeight(view string) string {
	if m.height <= 0 {
		return view
	}
	lineCount := strings.Count(view, "\n") + 1
	if lineCount >= m.height {
		return view
	}
	return view + strings.Repeat("\n", m.height-lineCount)
}

func sectionTitle(label string) string {
	return titleStyle.Render("[ " + label + " ]")
}

func padRight(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(value))
}

func (m Model) statusStrip() string {
	return strings.Join([]string{
		m.connectionChip(),
		m.recordingChip(),
		m.realtimeChip(),
		"Status: " + m.statusLine(),
	}, " | ")
}

func (m Model) connectionChip() string {
	if m.connected {
		return successStyle.Render("Connection: connected")
	}
	return errorStyle.Render("Connection: offline")
}

func (m Model) recordingChip() string {
	if m.recording {
		return warningStyle.Render("Recording: active")
	}
	return subtleStyle.Render("Recording: idle")
}

func (m Model) realtimeChip() string {
	if m.realtime {
		return infoStyle.Render("Realtime: active")
	}
	return subtleStyle.Render("Realtime: off")
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
