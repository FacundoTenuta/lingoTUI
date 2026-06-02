package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const updatePackage = "github.com/FacundoTenuta/lingoTUI/cmd/lingotui"

func runSelfUpdate(run commandRunner, version string) error {
	output, err := run("go", "install", updatePackage+"@"+version)
	if err == nil {
		return nil
	}
	details := strings.TrimSpace(string(output))
	if details == "" {
		return err
	}
	return fmt.Errorf("%w\n%s", err, details)
}

func updateVersion() string {
	version := os.Getenv("LINGOTUI_VERSION")
	if version == "" {
		return "latest"
	}
	return version
}

func defaultCommandRunner(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}
