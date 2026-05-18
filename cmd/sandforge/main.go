package main

import (
	"fmt"
	"os"

	"github.com/sandforge/sandforge/internal/backend"
	"github.com/sandforge/sandforge/internal/controlplane"
	"github.com/sandforge/sandforge/internal/policy"
	"github.com/sandforge/sandforge/internal/supervisor"
)

func main() {
	fmt.Println("Sandforge Agent Sandbox starting...")

	engine := &policy.Engine{
		MaxCPU:              8,
		MaxMemoryMb:         16384,
		MaxDiskGb:           100,
		AllowedNetworkModes: []string{"offline", "fetch"},
		AllowedCommands:     []string{"sh", "bash", "ls", "cat", "echo", "go", "python3", "node"},
	}

	sup, err := supervisor.NewSupervisor(backend.New(), engine)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create supervisor: %v\n", err)
		os.Exit(1)
	}

	server := controlplane.NewServer(sup)
	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
		os.Exit(1)
	}
}
