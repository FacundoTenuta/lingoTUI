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
	}))
}

func defaultTUILauncher() error {
	model, err := buildRuntime("")
	if err != nil {
		return fmt.Errorf("lingotui startup: %w", err)
	}
	if _, err := tea.NewProgram(model).Run(); err != nil {
		return fmt.Errorf("lingotui tui: %w", err)
	}
	return nil
}
