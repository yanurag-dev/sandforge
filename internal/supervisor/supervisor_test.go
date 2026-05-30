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
	sup, err := NewSupervisor(mockBackend, engine)
	if err != nil {
		t.Fatalf("Failed to create supervisor: %v", err)
	}

	id := "test-mount"
	spec := api.SandboxSpec{CPU: 1, MemoryMb: 512, DiskGb: 1, NetworkMode: "offline"}

	if err := sup.Start(id, spec); err != nil {
		t.Fatalf("Failed to start sandbox: %v", err)
	}
	defer func() { _ = sup.Stop(id) }()

	t.Run("ValidMount", func(t *testing.T) {
		err := sup.MountWorkspace(id, api.WorkspaceMount{
			HostPath:  tmpDir,
			GuestPath: "/workspace",
		})
		if err != nil {
			t.Errorf("Expected valid mount to succeed, got %v", err)
		}
	})

	t.Run("InvalidMount_Path", func(t *testing.T) {
		err := sup.MountWorkspace(id, api.WorkspaceMount{
			HostPath:  "/etc",
			GuestPath: "/workspace",
		})
		if err == nil {
			t.Error("Expected mount to /etc to be blocked by policy")
		}
	})

	t.Run("InvalidMount_State", func(t *testing.T) {
		// Manually set state to Executing to test rejection
		sup.mu.Lock()
		instance := sup.instances[id]
		sup.mu.Unlock()

		originalState := instance.GetState()
		instance.SetState(StateExecuting)
		defer instance.SetState(originalState)

		err := sup.MountWorkspace(id, api.WorkspaceMount{
			HostPath:  tmpDir,
			GuestPath: "/workspace",
		})
		if err == nil {
			t.Error("Expected mount to fail when state is Executing")
		}
	})

	t.Run("CopyOut_Valid", func(t *testing.T) {
		dest := filepath.Join(tmpDir, "log.txt")
		err := sup.CopyOut(id, "/workspace/log.txt", dest)
		if err != nil {
			t.Errorf("Expected CopyOut to succeed, got %v", err)
		}
	})

	t.Run("CopyOut_InvalidPath", func(t *testing.T) {
		err := sup.CopyOut(id, "/workspace/log.txt", "/etc/shadow")
		if err == nil {
			t.Error("Expected CopyOut to /etc/shadow to be blocked by policy")
		}
	})

	t.Run("CopyOut_InvalidState", func(t *testing.T) {
		sup.mu.Lock()
		instance := sup.instances[id]
		sup.mu.Unlock()

		originalState := instance.GetState()
		instance.SetState(StateError)
		defer instance.SetState(originalState)

		err := sup.CopyOut(id, "/workspace/log.txt", filepath.Join(tmpDir, "error.txt"))
		if err == nil {
			t.Error("Expected CopyOut to fail when state is Error")
		}
	})
}

func TestSupervisorFilesystemOps(t *testing.T) {
	mockBackend := backend.NewMockBackend()
	engine := &policy.Engine{
		MaxCPU:              4,
		MaxMemoryMb:         4096,
		MaxDiskGb:           10,
		AllowedNetworkModes: []string{"offline"},
	}
	sup, err := NewSupervisor(mockBackend, engine)
	if err != nil {
		t.Fatalf("Failed to create supervisor: %v", err)
	}

	id := "test-fs"
	spec := api.SandboxSpec{CPU: 1, MemoryMb: 512, DiskGb: 1, NetworkMode: "offline"}

	if err := sup.Start(id, spec); err != nil {
		t.Fatalf("Failed to start sandbox: %v", err)
	}
	defer func() { _ = sup.Stop(id) }()

	t.Run("WriteFile_Valid", func(t *testing.T) {
		size, err := sup.WriteFile(id, "/tmp/hello.txt", []byte("hello"))
		if err != nil {
			t.Errorf("Expected WriteFile to succeed, got %v", err)
		}
		if size != 5 {
			t.Errorf("Expected size 5, got %d", size)
		}
	})

	t.Run("WriteFile_NotFound", func(t *testing.T) {
		_, err := sup.WriteFile("nonexistent", "/tmp/hello.txt", []byte("hello"))
		if err == nil {
			t.Error("Expected error for nonexistent sandbox")
		}
	})

	t.Run("WriteFile_InvalidState", func(t *testing.T) {
		sup.mu.Lock()
		instance := sup.instances[id]
		sup.mu.Unlock()

		originalState := instance.GetState()
		instance.SetState(StateExecuting)
		defer instance.SetState(originalState)

		_, err := sup.WriteFile(id, "/tmp/hello.txt", []byte("hello"))
		if err == nil {
			t.Error("Expected WriteFile to fail when state is Executing")
		}
	})

	t.Run("ListDir_Valid", func(t *testing.T) {
		entries, err := sup.ListDir(id, "/tmp")
		if err != nil {
			t.Errorf("Expected ListDir to succeed, got %v", err)
		}
		if entries == nil {
			t.Error("Expected non-nil entries slice")
		}
	})

	t.Run("ListDir_AllowedDuringExecuting", func(t *testing.T) {
		sup.mu.Lock()
		instance := sup.instances[id]
		sup.mu.Unlock()

		originalState := instance.GetState()
		instance.SetState(StateExecuting)
		defer instance.SetState(originalState)

		_, err := sup.ListDir(id, "/tmp")
		if err != nil {
			t.Errorf("Expected ListDir to succeed during Executing, got %v", err)
		}
	})

	t.Run("ListDir_InvalidState", func(t *testing.T) {
		sup.mu.Lock()
		instance := sup.instances[id]
		sup.mu.Unlock()

		originalState := instance.GetState()
		instance.SetState(StateError)
		defer instance.SetState(originalState)

		_, err := sup.ListDir(id, "/tmp")
		if err == nil {
			t.Error("Expected ListDir to fail when state is Error")
		}
	})

	t.Run("StatPath_Valid", func(t *testing.T) {
		_, err := sup.StatPath(id, "/tmp/hello.txt")
		if err != nil {
			t.Errorf("Expected StatPath to succeed, got %v", err)
		}
	})

	t.Run("StatPath_AllowedDuringExecuting", func(t *testing.T) {
		sup.mu.Lock()
		instance := sup.instances[id]
		sup.mu.Unlock()

		originalState := instance.GetState()
		instance.SetState(StateExecuting)
		defer instance.SetState(originalState)

		_, err := sup.StatPath(id, "/tmp/hello.txt")
		if err != nil {
			t.Errorf("Expected StatPath to succeed during Executing, got %v", err)
		}
	})

	t.Run("StatPath_InvalidState", func(t *testing.T) {
		sup.mu.Lock()
		instance := sup.instances[id]
		sup.mu.Unlock()

		originalState := instance.GetState()
		instance.SetState(StateDestroyed)
		defer instance.SetState(originalState)

		_, err := sup.StatPath(id, "/tmp/hello.txt")
		if err == nil {
			t.Error("Expected StatPath to fail when state is Destroyed")
		}
	})
}
