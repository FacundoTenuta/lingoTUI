package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	model, err := buildRuntime("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "lingotui startup: %v\n", err)
		os.Exit(1)
	}
	if _, err := tea.NewProgram(model).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "lingotui tui: %v\n", err)
		os.Exit(1)
	}
}
