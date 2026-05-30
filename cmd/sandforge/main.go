package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yanurag-dev/sandforge/internal/backend"
	"github.com/yanurag-dev/sandforge/internal/controlplane"
	"github.com/yanurag-dev/sandforge/internal/policy"
	"github.com/yanurag-dev/sandforge/internal/supervisor"
	"github.com/yanurag-dev/sandforge/pkg/api"
)

var (
	exitCode int
	rootCmd  = &cobra.Command{
		Use:   "sandforge",
		Short: "Sandforge is a secure, ephemeral virtualization sandbox",
		Long:  `A portable, host-isolated sandbox architecture designed to run coding agents in restricted guest microVMs on macOS.`,
	}
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
	os.Exit(exitCode)
}

func init() {
	rootCmd.AddCommand(newServerCmd())
	rootCmd.AddCommand(newRunCmd())
}

func newServerCmd() *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Run the API control plane server",
		Long:  `Launches the background supervisor control plane server, providing a REST API to manage active microVM sandbox instances.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Starting Sandforge API control plane on %s...\n", addr)

			engine := &policy.Engine{
				MaxCPU:              8,
				MaxMemoryMb:         16384,
				MaxDiskGb:           100,
				AllowedNetworkModes: []string{"offline", "fetch"},
				AllowedCommands:     []string{"sh", "bash", "ls", "cat", "echo", "go", "python3", "node"},
			}

			sup, err := supervisor.NewSupervisor(backend.New(), engine)
			if err != nil {
				return fmt.Errorf("failed to create supervisor: %w", err)
			}

			server, err := controlplane.NewServerWithAddr(sup, addr)
			if err != nil {
				return fmt.Errorf("failed to create server: %w", err)
			}

			if err := server.Start(); err != nil {
				return fmt.Errorf("error starting server: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&addr, "addr", "a", ":8080", "TCP address for the API server to listen on")
	return cmd
}

func newRunCmd() *cobra.Command {
	var (
		dir     string
		network string
		cpu     int
		mem     int
		mock    bool
		timeout int
	)

	cmd := &cobra.Command{
		Use:   "run [flags] <cmd> [args...]",
		Short: "Run a transient command in an isolated sandbox",
		Long:  `Launches a transient, ephemeral microVM sandbox, mounts the host directory, executes the command, streams output, and destroys the sandbox on exit.`,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			absDir, err := filepath.Abs(dir)
			if err != nil {
				return fmt.Errorf("resolving absolute path for %q: %w", dir, err)
			}

			// Policy dynamic overrides for local CLI usage
			// The user is executing this explicitly via terminal, so we trust their chosen directory and binary.
			binary := args[0]
			engine := &policy.Engine{
				MaxCPU:              8,
				MaxMemoryMb:         16384,
				MaxDiskGb:           100,
				AllowedNetworkModes: []string{"offline", "fetch"},
				AllowedHostPrefixes: []string{absDir}, // Explicitly allow the chosen mount dir
				AllowedCommands:     []string{binary}, // Explicitly allow the chosen executable
			}

			// Determine backend
			var b api.SandboxBackend
			if mock {
				fmt.Println("[info] running in mock mode")
				b = backend.NewMockBackend()
			} else {
				b = backend.New()
			}

			sup, err := supervisor.NewSupervisor(b, engine)
			if err != nil {
				return fmt.Errorf("failed to create supervisor: %w", err)
			}

			// Create transient spec with mounts pre-defined
			id := "transient-" + strings.TrimPrefix(fmt.Sprintf("%d", os.Getpid()), "-")
			spec := api.SandboxSpec{
				Backend:       "macos-vz", // default backend indicator
				CPU:           cpu,
				MemoryMb:      mem,
				DiskGb:        10,
				TimeoutSec:    timeout,
				NetworkMode:   network,
				TaskIsolation: "process", // process isolation on first-stage guest agent
				Mounts: []api.WorkspaceMount{
					{
						HostPath:  absDir,
						GuestPath: "/workspace",
						ReadOnly:  false,
					},
				},
			}

			fmt.Printf("[info] starting sandbox %s (cpu=%d, mem=%dMB, network=%s)...\n", id, cpu, mem, network)
			if err := sup.Start(id, spec); err != nil {
				return fmt.Errorf("failed to start sandbox: %w", err)
			}

			// Ensure cleanup on exit
			defer func() {
				fmt.Printf("[info] destroying sandbox %s...\n", id)
				if err := sup.Stop(id); err != nil {
					fmt.Fprintf(os.Stderr, "[warn] failed to destroy sandbox: %v\n", err)
				}
			}()

			fmt.Printf("[info] executing command %v in sandbox...\n", args)
			res, err := sup.RunCommand(id, api.ExecRequest{
				Command:    args,
				CWD:        "/workspace",
				TimeoutSec: timeout,
			})
			if err != nil {
				return fmt.Errorf("command execution failed: %w", err)
			}

			if res.Stdout != "" {
				fmt.Print(res.Stdout)
			}
			if res.Stderr != "" {
				fmt.Fprint(os.Stderr, res.Stderr)
			}

			// Capture the exit status code so it is passed to os.Exit in main()
			exitCode = res.ExitCode
			return nil
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Host workspace directory to mount")
	cmd.Flags().StringVarP(&network, "network", "n", "offline", "Network mode: offline or fetch")
	cmd.Flags().IntVarP(&cpu, "cpu", "c", 2, "Number of virtual CPU cores")
	cmd.Flags().IntVarP(&mem, "mem", "m", 2048, "Memory size in MB")
	cmd.Flags().BoolVar(&mock, "mock", false, "Use in-memory Mock backend instead of real hypervisor")
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 300, "Maximum execution timeout in seconds")

	return cmd
}
