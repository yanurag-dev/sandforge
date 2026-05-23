package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sandforge/sandforge/internal/backend"
	"github.com/sandforge/sandforge/internal/controlplane"
	"github.com/sandforge/sandforge/internal/policy"
	"github.com/sandforge/sandforge/internal/supervisor"
	"github.com/sandforge/sandforge/pkg/api"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Sandforge Agent Sandbox CLI\n\n")
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  sandforge server [flags]       - Run the API control plane server\n")
	fmt.Fprintf(os.Stderr, "  sandforge run [flags] <cmd>    - Run a transient command in an isolated sandbox\n\n")
	fmt.Fprintf(os.Stderr, "Use \"sandforge <command> --help\" for more info on flags.\n")
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	var exitCode int
	switch cmd {
	case "server":
		runServer(os.Args[2:])
	case "run":
		exitCode = runTransient(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command %q\n\n", cmd)
		usage()
		os.Exit(1)
	}
	os.Exit(exitCode)
}

func runServer(args []string) {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	addr := fs.String("addr", ":8080", "TCP address for the API server to listen on")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	fmt.Printf("Starting Sandforge API control plane on %s...\n", *addr)

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

	server, err := controlplane.NewServerWithAddr(sup, *addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create server: %v\n", err)
		os.Exit(1)
	}

	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
		os.Exit(1)
	}
}

func runTransient(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dir := fs.String("dir", ".", "Host workspace directory to mount (defaults to current dir)")
	network := fs.String("network", "offline", "Network mode: offline or fetch")
	cpu := fs.Int("cpu", 2, "Number of virtual CPU cores")
	mem := fs.Int("mem", 2048, "Memory size in MB")
	mock := fs.Bool("mock", false, "Use in-memory Mock backend instead of real hypervisor")
	timeout := fs.Int("timeout", 300, "Maximum execution timeout in seconds")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cmdArgs := fs.Args()
	if len(cmdArgs) == 0 {
		fmt.Fprintf(os.Stderr, "Error: command is required (e.g. sandforge run go version)\n")
		return 1
	}

	absDir, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving absolute path for %q: %v\n", *dir, err)
		return 1
	}

	// 1. Policy dynamic overrides for local CLI usage
	// The user is executing this explicitly via terminal, so we trust their chosen directory and binary.
	binary := cmdArgs[0]
	engine := &policy.Engine{
		MaxCPU:              8,
		MaxMemoryMb:         16384,
		MaxDiskGb:           100,
		AllowedNetworkModes: []string{"offline", "fetch"},
		AllowedHostPrefixes: []string{absDir}, // Explicitly allow the chosen mount dir
		AllowedCommands:     []string{binary},  // Explicitly allow the chosen executable
	}

	// Determine backend
	var b api.SandboxBackend
	if *mock {
		fmt.Println("[info] running in mock mode")
		b = backend.NewMockBackend()
	} else {
		b = backend.New()
	}

	sup, err := supervisor.NewSupervisor(b, engine)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create supervisor: %v\n", err)
		return 1
	}

	// Create transient spec with mounts pre-defined
	id := "transient-" + strings.TrimPrefix(fmt.Sprintf("%d", os.Getpid()), "-")
	spec := api.SandboxSpec{
		Backend:       "macos-vz", // default backend indicator
		CPU:           *cpu,
		MemoryMb:      *mem,
		DiskGb:        10,
		TimeoutSec:    *timeout,
		NetworkMode:   *network,
		TaskIsolation: "process", // process isolation on first-stage guest agent
		Mounts: []api.WorkspaceMount{
			{
				HostPath:  absDir,
				GuestPath: "/workspace",
				ReadOnly:  false,
			},
		},
	}

	fmt.Printf("[info] starting sandbox %s (cpu=%d, mem=%dMB, network=%s)...\n", id, *cpu, *mem, *network)
	if err := sup.Start(id, spec); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to start sandbox: %v\n", err)
		return 1
	}

	// Ensure cleanup on exit
	defer func() {
		fmt.Printf("[info] destroying sandbox %s...\n", id)
		if err := sup.Stop(id); err != nil {
			fmt.Fprintf(os.Stderr, "[warn] failed to destroy sandbox: %v\n", err)
		}
	}()

	fmt.Printf("[info] executing command %v in sandbox...\n", cmdArgs)
	res, err := sup.RunCommand(id, api.ExecRequest{
		Command:    cmdArgs,
		CWD:        "/workspace",
		TimeoutSec: *timeout,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: command execution failed: %v\n", err)
		return 1
	}

	if res.Stdout != "" {
		fmt.Print(res.Stdout)
	}
	if res.Stderr != "" {
		fmt.Fprint(os.Stderr, res.Stderr)
	}

	return res.ExitCode
}
