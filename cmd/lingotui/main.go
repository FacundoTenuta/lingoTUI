package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr, cliOptions{
		launchTUI:  defaultTUILauncher,
		runCommand: defaultCommandRunner,
		stdin:      os.Stdin,
	}))
}

func defaultTUILauncher() error {
	model, err := buildRuntime("")
	if err != nil {
		return fmt.Errorf("lingotui startup: %w", err)
	}
	if _, err := tea.NewProgram(model, defaultTUIProgramOptions()...).Run(); err != nil {
		return fmt.Errorf("lingotui tui: %w", err)
	}
	return nil
}

func defaultTUIProgramOptions() []tea.ProgramOption {
	return []tea.ProgramOption{tea.WithAltScreen()}
}
