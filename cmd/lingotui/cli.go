package main

import (
	"fmt"
	"io"
)

type tuiLauncher func() error
type commandRunner func(name string, args ...string) error

type cliOptions struct {
	launchTUI  tuiLauncher
	runCommand commandRunner
}

func runCLI(args []string, stdout, stderr io.Writer, options cliOptions) int {
	options = normalizeCLIOptions(options)

	if len(args) == 0 {
		if err := options.launchTUI(); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}

	switch args[0] {
	case "update":
		if len(args) != 1 {
			printUsage(stderr)
			return 1
		}
		if err := runSelfUpdate(options.runCommand); err != nil {
			fmt.Fprintf(stderr, "lingotui update: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "lingotui updated")
		return 0
	default:
		printUsage(stderr)
		return 1
	}
}

func normalizeCLIOptions(options cliOptions) cliOptions {
	if options.launchTUI == nil {
		options.launchTUI = defaultTUILauncher
	}
	if options.runCommand == nil {
		options.runCommand = defaultCommandRunner
	}
	return options
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: lingotui [update]")
}
