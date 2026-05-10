package supervisor

import (
	"testing"

	"github.com/sandforge/sandforge/internal/backend"
	"github.com/sandforge/sandforge/internal/policy"
	"github.com/sandforge/sandforge/pkg/api"
)

func TestSupervisorLifecycle(t *testing.T) {
	// Setup
	mockBackend := backend.NewMockBackend()
	engine := &policy.Engine{
		MaxCPU:              4,
		MaxMemoryMb:         4096,
		MaxDiskGb:           10,
		AllowedNetworkModes: []string{"offline"},
		AllowedCommands:     []string{"ls", "echo"},
	}
	sup := NewSupervisor(mockBackend, engine)

	spec := api.SandboxSpec{
		CPU:         2,
		MemoryMb:    1024,
		DiskGb:      5,
		NetworkMode: "offline",
	}

	id := "test-1"

	// 1. Test Start
	t.Run("Start", func(t *testing.T) {
		err := sup.Start(id, spec)
		if err != nil {
			t.Fatalf("Failed to start sandbox: %v", err)
		}

		instance, exists := sup.instances[id]
		if !exists {
			t.Fatal("Instance not found in supervisor map")
		}
		if instance.State != StateReady {
			t.Errorf("Expected state Ready, got %v", instance.State)
		}
	})

	// 2. Test RunCommand
	t.Run("RunCommand", func(t *testing.T) {
		req := api.ExecRequest{
			Command: []string{"ls", "-la"},
		}
		result, err := sup.RunCommand(id, req)
		if err != nil {
			t.Fatalf("Failed to run command: %v", err)
		}

		if result.ExitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", result.ExitCode)
		}
	})

	// 3. Test Policy Violation
	t.Run("PolicyViolation", func(t *testing.T) {
		req := api.ExecRequest{
			Command: []string{"rm", "-rf", "/"},
		}
		_, err := sup.RunCommand(id, req)
		if err == nil {
			t.Error("Expected error for forbidden command, got nil")
		}
	})

	// 4. Test Stop
	t.Run("Stop", func(t *testing.T) {
		err := sup.Stop(id)
		if err != nil {
			t.Fatalf("Failed to stop sandbox: %v", err)
		}

		_, exists := sup.instances[id]
		if exists {
			t.Error("Instance should have been removed from map after Stop")
		}
	})
}
