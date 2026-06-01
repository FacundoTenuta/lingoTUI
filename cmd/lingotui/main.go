package main

import (
	"fmt"
	"os"

	"github.com/FacundoTenuta/lingoTUI/internal/config"
	"github.com/FacundoTenuta/lingoTUI/internal/credentials"
)

func main() {
	configStore, err := config.NewFileStore("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "config store: %v\n", err)
		os.Exit(1)
	}
	credentialStore, err := credentials.NewFileStore("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "credential store: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("lingoTUI foundation ready\nconfig: %s\ncredentials: %s\n", configStore.Path(), credentialStore.Path())
}
