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
		return m.submit(msg.Input)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case RealtimeTickMsg:
		if !m.realtime || m.Status == statusLoading {
			if m.realtime && m.Status == statusLoading {
				m.droppedRealtimeTick = true
			}
			return m, nil
		}
		return m.submit("/realtime chunk")
	case commandFinishedMsg:
		return m.finishCommand(msg)
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
				return m.submitMenuItem()
			}
			if m.inputMode == askMode {
				return m.submitAsk()
			}
			if m.inputMode == translateMode {
				return m.submitTranslate()
			}
			return m.submit(m.Input)
		case "backspace":
			if m.Status == statusLoading {
				return m, nil
			}
			if len(m.Input) > 0 {
				lastRuneIndex := 0
				for index := range m.Input {
					lastRuneIndex = index
				}
				m.Input = m.Input[:lastRuneIndex]
			}
		default:
			if m.Status == statusLoading {
				return m, nil
			}
			if m.inputMode == menuMode {
				m.inputMode = commandMode
			}
			m.Input += msg.String()
		}
	}
	return m, nil
}

func (m Model) submitMenuItem() (Model, tea.Cmd) {
	if m.Status == statusLoading {
		return m, nil
	}
	if len(menuItems) == 0 {
		return m, nil
	}
	if m.MenuIndex < 0 || m.MenuIndex >= len(menuItems) {
		m.MenuIndex = 0
	}
	item := menuItems[m.MenuIndex]
	if item.Ask {
		m.inputMode = askMode
		m.Input = ""
		return m, nil
	}
	if item.Translate {
		m.inputMode = translateMode
		m.Input = ""
		return m, nil
	}
	return m.submit(item.Command)
}

func (m Model) submitAsk() (Model, tea.Cmd) {
	if m.Status == statusLoading {
		return m, nil
	}
	question := strings.TrimSpace(m.Input)
	m.inputMode = menuMode
	m.Input = ""
	if question == "" {
		return m, nil
	}
	return m.submit("/ask " + question)
}

func (m Model) submitTranslate() (Model, tea.Cmd) {
	if m.Status == statusLoading {
		return m, nil
	}
	text := strings.TrimSpace(m.Input)
	m.inputMode = menuMode
	m.Input = ""
	if text == "" {
		return m, nil
	}
	return m.submit("/translate " + text)
}

func (m Model) submit(input string) (Model, tea.Cmd) {
	if m.Status == statusLoading {
		return m, nil
	}
	input = strings.TrimSpace(input)
	m.Input = ""
	m.inputMode = menuMode
	if input == "" {
		return m, nil
	}
	m.Messages = append(m.Messages, "> "+input)
	m.Err = nil
	m.Status = statusLoading
	m.StatusMessage = loadingStatusMessage(input)
	return m, func() tea.Msg {
		result, err := m.app.HandleInput(m.ctx, input)
		return commandFinishedMsg{Input: input, Result: result, Err: err}
	}
}

func (m Model) finishCommand(msg commandFinishedMsg) (Model, tea.Cmd) {
	m.realtime = msg.Result.Realtime
	m.connected = msg.Result.Connected
	m.recording = msg.Result.Recording
	if msg.Err != nil {
		m.Err = msg.Err
		m.Status = statusError
		m.StatusMessage = "Fix the issue below, then try again or run /help."
		m.Messages = append(m.Messages, "Error: "+cleanErrorMessage(msg.Err.Error()))
		cmd := m.nextDroppedRealtimeTick(msg)
		return m, cmd
	}
	m.Err = nil
	m.realtime = msg.Result.Realtime
	m.connected = msg.Result.Connected
	m.recording = msg.Result.Recording
	m.Status = statusForResult(msg.Result)
	m.StatusMessage = statusMessageForResult(msg.Result)
	for _, line := range formatResult(msg.Result) {
		m.Messages = append(m.Messages, line)
	}
	if cmd := m.nextDroppedRealtimeTick(msg); cmd != nil {
		return m, cmd
	}
	return m, m.nextRealtimeTick(msg)
}

func (m *Model) nextDroppedRealtimeTick(msg commandFinishedMsg) tea.Cmd {
	if !m.droppedRealtimeTick || !m.realtime || m.realtimeTick == nil {
		return nil
	}
	m.droppedRealtimeTick = false
	if msg.Input == "/realtime chunk" {
		return nil
	}
	return m.realtimeTick()
}

func (m *Model) nextRealtimeTick(msg commandFinishedMsg) tea.Cmd {
	m.droppedRealtimeTick = false
	if !m.realtime || msg.Result.Command != app.CommandRealtime || m.realtimeTick == nil {
		return nil
	}
	if msg.Input != "/realtime start" && msg.Input != "/realtime start mic" && msg.Input != "/realtime chunk" {
		return nil
	}
	return m.realtimeTick()
}

func loadingStatusMessage(input string) string {
	if strings.HasPrefix(input, "/ask ") || input == "/ask" || strings.HasPrefix(input, "/translate ") || input == "/translate" || input == "/realtime chunk" {
		return "Processing request..."
	}
	command := input
	if fields := strings.Fields(input); len(fields) > 0 {
		command = fields[0]
	}
	return "Running " + command + "..."
}

func statusForResult(result app.Result) statusState {
	switch result.Command {
	case app.CommandHelp, app.CommandModels:
		return statusInfo
	case app.CommandConnect, app.CommandRecord, app.CommandStop, app.CommandAsk, app.CommandTranslate, app.CommandRealtime, app.CommandClear:
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
	case app.CommandTranslate:
		return "Translations are shown below."
	case app.CommandRealtime:
		return "Realtime translation updated."
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
			lines = append(lines, fmt.Sprintf("  %-22s %s", entry.Command, entry.Description))
		}
		if len(result.Guidance) > 0 {
			lines = append(lines, "", "Setup guidance:")
			for _, line := range result.Guidance {
				lines = append(lines, "  "+line)
			}
		}
		return lines
	}
	if result.Command == app.CommandRealtime && len(result.Chunks) > 0 {
		lines := []string{result.Message, "Realtime:"}
		for i, chunk := range result.Chunks {
			lines = append(lines, fmt.Sprintf("Chunk %d:", i+1))
			if chunk.Transcript.Text != "" {
				lines = appendBlock(lines, "  Transcript", chunk.Transcript.Text)
			}
			for _, language := range app.SummaryLanguages() {
				if text := chunk.Translations[language]; text != "" {
					lines = appendBlock(lines, "  "+strings.ToUpper(string(language)), text)
				}
			}
		}
		return lines
	}
	if result.Command == app.CommandStop && (result.Context.Transcript.Text != "" || len(result.Context.Summary) > 0) {
		lines := []string{result.Message}
		if result.Context.Transcript.Text != "" {
			lines = appendBlock(lines, "Transcript", result.Context.Transcript.Text)
		}
		var summaryBlocks []struct {
			label string
			text  string
		}
		for _, language := range app.SummaryLanguages() {
			if text := result.Context.Summary[language]; text != "" {
				summaryBlocks = append(summaryBlocks, struct {
					label string
					text  string
				}{label: strings.ToUpper(string(language)), text: text})
			}
		}
		if len(summaryBlocks) > 0 {
			lines = append(lines, "Summary:")
			for _, block := range summaryBlocks {
				lines = appendBlock(lines, "  "+block.label, block.text)
			}
		}
		return lines
	}
	if result.Command == app.CommandTranslate && len(result.Translations) > 0 {
		lines := []string{result.Message, "Translations:"}
		for _, language := range app.SummaryLanguages() {
			if text := result.Translations[language]; text != "" {
				lines = appendBlock(lines, "  "+strings.ToUpper(string(language)), text)
			}
		}
		return lines
	}
	if result.Message == "" {
		return nil
	}
	return []string{result.Message}
}

func appendBlock(lines []string, label, text string) []string {
	lines = append(lines, label+":")
	for _, line := range strings.Split(text, "\n") {
		lines = append(lines, "  "+line)
	}
	return lines
}
