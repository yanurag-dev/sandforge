package supervisor

import (
	"fmt"
	"path/filepath"
	"sync"
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
	sup, err := NewSupervisor(mockBackend, engine)
	if err != nil {
		t.Fatalf("Failed to create supervisor: %v", err)
	}

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

		sup.mu.RLock()
		instance, exists := sup.instances[id]
		sup.mu.RUnlock()

		if !exists {
			t.Fatal("Instance not found in supervisor map")
		}
		if instance.GetState() != StateReady {
			t.Errorf("Expected state Ready, got %v", instance.GetState())
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

	// 4. Test Concurrent Access
	t.Run("ConcurrentAccess", func(t *testing.T) {
		var wg sync.WaitGroup
		numWorkers := 10
		idPrefix := "concurrent-"

		// Start multiple sandboxes
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				cid := fmt.Sprintf("%s%d", idPrefix, idx)
				if err := sup.Start(cid, spec); err != nil {
					t.Errorf("Failed to start concurrent sandbox %d: %v", idx, err)
				}
				
				// Run a command
				req := api.ExecRequest{Command: []string{"echo", "hello"}}
				if _, err := sup.RunCommand(cid, req); err != nil {
					t.Errorf("Failed to run command in concurrent sandbox %d: %v", idx, err)
				}

				// Stop
				if err := sup.Stop(cid); err != nil {
					t.Errorf("Failed to stop concurrent sandbox %d: %v", idx, err)
				}
			}(i)
		}
		wg.Wait()
	})

	// 5. Test Stop
	t.Run("Stop", func(t *testing.T) {
		err := sup.Stop(id)
		if err != nil {
			t.Fatalf("Failed to stop sandbox: %v", err)
		}

		sup.mu.RLock()
		_, exists := sup.instances[id]
		sup.mu.RUnlock()

		if exists {
			t.Error("Instance should have been removed from map after Stop")
		}
	})
}

func TestSupervisorMountAndCopy(t *testing.T) {
	mockBackend := backend.NewMockBackend()
	
	// Create a temp dir for allowed mounts
	tmpDir := t.TempDir()
	
	engine := &policy.Engine{
		AllowedHostPrefixes: []string{tmpDir},
		MaxCPU:              4,
		MaxMemoryMb:         4096,
		MaxDiskGb:           10,
		AllowedNetworkModes: []string{"offline"},
	}
	sup, _ := NewSupervisor(mockBackend, engine)

	id := "test-mount"
	spec := api.SandboxSpec{CPU: 1, MemoryMb: 512, DiskGb: 1, NetworkMode: "offline"}
	
	_ = sup.Start(id, spec)

	t.Run("ValidMount", func(t *testing.T) {
		err := sup.MountWorkspace(id, api.WorkspaceMount{
			HostPath:  tmpDir,
			GuestPath: "/workspace",
		})
		if err != nil {
			t.Errorf("Expected valid mount to succeed, got %v", err)
		}
	})

	t.Run("InvalidMount", func(t *testing.T) {
		err := sup.MountWorkspace(id, api.WorkspaceMount{
			HostPath:  "/etc",
			GuestPath: "/workspace",
		})
		if err == nil {
			t.Error("Expected mount to /etc to be blocked by policy")
		}
	})

	t.Run("CopyOut", func(t *testing.T) {
		dest := filepath.Join(tmpDir, "log.txt")
		err := sup.CopyOut(id, "/workspace/log.txt", dest)
		if err != nil {
			t.Errorf("Expected CopyOut to succeed, got %v", err)
		}
	})
}

