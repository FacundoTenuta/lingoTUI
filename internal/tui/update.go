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
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.inputMode != menuMode {
				m.inputMode = menuMode
				m.Input = ""
				return m, nil
			}
			return m, tea.Quit
		case "up":
			if m.inputMode == menuMode {
				m.MenuIndex = (m.MenuIndex + len(menuItems) - 1) % len(menuItems)
			}
			return m, nil
		case "k":
			if m.inputMode == menuMode {
				m.MenuIndex = (m.MenuIndex + len(menuItems) - 1) % len(menuItems)
				return m, nil
			}
			m.Input += msg.String()
		case "down":
			if m.inputMode == menuMode {
				m.MenuIndex = (m.MenuIndex + 1) % len(menuItems)
			}
			return m, nil
		case "j":
			if m.inputMode == menuMode {
				m.MenuIndex = (m.MenuIndex + 1) % len(menuItems)
				return m, nil
			}
			m.Input += msg.String()
		case "enter":
			if m.inputMode == menuMode {
				return m.submitMenuItem(), nil
			}
			if m.inputMode == askMode {
				return m.submitAsk(), nil
			}
			return m.submit(m.Input), nil
		case "backspace":
			if len(m.Input) > 0 {
				m.Input = m.Input[:len(m.Input)-1]
			}
		default:
			if m.inputMode == menuMode {
				m.inputMode = commandMode
			}
			m.Input += msg.String()
		}
	}
	return m, nil
}

func (m Model) submitMenuItem() Model {
	if len(menuItems) == 0 {
		return m
	}
	if m.MenuIndex < 0 || m.MenuIndex >= len(menuItems) {
		m.MenuIndex = 0
	}
	item := menuItems[m.MenuIndex]
	if item.Ask {
		m.inputMode = askMode
		m.Input = ""
		return m
	}
	return m.submit(item.Command)
}

func (m Model) submitAsk() Model {
	question := strings.TrimSpace(m.Input)
	m.inputMode = menuMode
	m.Input = ""
	if question == "" {
		return m
	}
	return m.submit("/ask " + question)
}

func (m Model) submit(input string) Model {
	input = strings.TrimSpace(input)
	m.Input = ""
	m.inputMode = menuMode
	if input == "" {
		return m
	}
	m.Messages = append(m.Messages, "> "+input)
	result, err := m.app.HandleInput(m.ctx, input)
	if err != nil {
		m.Err = err
		m.Status = statusError
		m.StatusMessage = "Fix the issue below, then try again or run /help."
		m.Messages = append(m.Messages, "Error: "+cleanErrorMessage(err.Error()))
		return m
	}
	m.Err = nil
	m.Status = statusForResult(result)
	m.StatusMessage = statusMessageForResult(result)
	for _, line := range formatResult(result) {
		m.Messages = append(m.Messages, line)
	}
	return m
}

func statusForResult(result app.Result) statusState {
	switch result.Command {
	case app.CommandHelp, app.CommandModels:
		return statusInfo
	case app.CommandConnect, app.CommandRecord, app.CommandStop, app.CommandAsk, app.CommandClear:
		return statusSuccess
	default:
		return statusIdle
	}
}

func statusMessageForResult(result app.Result) string {
	if result.Message != "" {
		return result.Message
	}
	switch result.Command {
	case app.CommandHelp:
		return "Help is shown below."
	case app.CommandModels:
		return "Configured models are shown below."
	case app.CommandConnect:
		return "Connection command completed."
	case app.CommandRecord:
		return "Recording command completed."
	case app.CommandStop:
		return "Stop command completed."
	case app.CommandAsk:
		return "Answer is shown below."
	case app.CommandClear:
		return "Context cleared."
	default:
		return readyStatusMessage
	}
}

func cleanErrorMessage(message string) string {
	cleaned := strings.TrimSpace(message)
	for {
		lower := strings.ToLower(cleaned)
		var next string
		switch {
		case strings.HasPrefix(lower, "error: "):
			next = cleaned[len("error: "):]
		case strings.HasPrefix(lower, "error. "):
			next = cleaned[len("error. "):]
		default:
			next = cleaned
		}
		if next == cleaned {
			break
		}
		cleaned = strings.TrimSpace(next)
	}
	return cleaned
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
