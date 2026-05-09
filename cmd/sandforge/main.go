package main

import (
	"fmt"
	"os"

	"github.com/sandforge/sandforge/internal/controlplane"
)

func main() {
	fmt.Println("Sandforge Agent Sandbox starting...")
	
	// TODO: Initialize config, logger, and policy engine
	
	server := controlplane.NewServer()
	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
		os.Exit(1)
	}
}
