package main

import (
	"os"
	"os/exec"
)

const updatePackage = "github.com/FacundoTenuta/lingoTUI/cmd/lingotui"

func runSelfUpdate(run commandRunner) error {
	version := os.Getenv("LINGOTUI_VERSION")
	if version == "" {
		version = "latest"
	}
	return run("go", "install", updatePackage+"@"+version)
}

func defaultCommandRunner(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
