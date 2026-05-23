package policy

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sandforge/sandforge/pkg/api"
)

func TestEvaluateMount(t *testing.T) {
	// Create a real temp directory for testing symlinks and path resolution
	tempBase := t.TempDir()

	workspacesDir := filepath.Join(tempBase, "workspaces")
	err := os.MkdirAll(workspacesDir, 0750)
	if err != nil {
		t.Fatal(err)
	}

	// Create a "forbidden" directory outside the allowed base
	forbiddenDir := filepath.Join(tempBase, "forbidden")
	err = os.MkdirAll(forbiddenDir, 0750)
	if err != nil {
		t.Fatal(err)
	}

	// Create a symlink that tries to "escape"
	escapeSymlink := filepath.Join(workspacesDir, "escape-link")
	err = os.Symlink(forbiddenDir, escapeSymlink)
	if err != nil {
		t.Fatal(err)
	}

	// Create a path that has a blocked pattern as a substring but not a segment
	falsePositivePath := filepath.Join(workspacesDir, "my-ssh-notes.txt")
	err = os.WriteFile(falsePositivePath, []byte("test"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	// Create a real blocked segment
	blockedSegmentDir := filepath.Join(workspacesDir, ".ssh")
	err = os.MkdirAll(blockedSegmentDir, 0750)
	if err != nil {
		t.Fatal(err)
	}

	engine := &Engine{
		AllowedHostPrefixes: []string{
			workspacesDir,
		},
		BlockedHostPatterns: []string{
			".ssh",
			"forbidden",
		},
	}

	tests := []struct {
		name      string
		hostPath  string
		wantError bool
	}{
		{
			name:      "Valid path in workspace",
			hostPath:  filepath.Join(workspacesDir, "task-1"),
			wantError: false, // We'll create it first
		},
		{
			name:      "Exact match of allowed prefix",
			hostPath:  workspacesDir,
			wantError: false,
		},
		{
			name:      "Path outside whitelist",
			hostPath:  tempBase, // The parent dir is not whitelisted
			wantError: true,
		},
		{
			name:      "Relative path rejected",
			hostPath:  "relative/path",
			wantError: true,
		},
		{
			name:      "Symlink escape rejected",
			hostPath:  escapeSymlink,
			wantError: true,
		},
		{
			name:      "False positive (substring .ssh) now ALLOWED",
			hostPath:  falsePositivePath,
			wantError: false,
		},
		{
			name:      "Real blocked segment (.ssh) rejected",
			hostPath:  blockedSegmentDir,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure the path exists so EvalSymlinks doesn't just fail on 'not found'
			if !tt.wantError || tt.name == "Real blocked segment (.ssh) rejected" || tt.name == "Symlink escape rejected" {
				_ = os.MkdirAll(tt.hostPath, 0750)
			}

			mount := api.WorkspaceMount{
				HostPath: tt.hostPath,
			}
			err := engine.EvaluateMount(mount)
			if (err != nil) != tt.wantError {
				t.Errorf("EvaluateMount() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestEvaluateSandbox(t *testing.T) {
	engine := &Engine{
		MaxCPU:              2,
		MaxMemoryMb:         2048,
		MaxDiskGb:           10,
		AllowedNetworkModes: []string{"offline", "fetch"},
	}

	tests := []struct {
		name      string
		spec      api.SandboxSpec
		wantError error
	}{
		{
			name: "Valid spec",
			spec: api.SandboxSpec{
				CPU:         1,
				MemoryMb:    1024,
				DiskGb:      5,
				NetworkMode: "offline",
			},
			wantError: nil,
		},
		{
			name: "CPU limit exceeded",
			spec: api.SandboxSpec{
				CPU: 4,
			},
			wantError: ErrResourceLimitExceeded,
		},
		{
			name: "Memory limit exceeded",
			spec: api.SandboxSpec{
				CPU:      1,
				MemoryMb: 4096,
			},
			wantError: ErrResourceLimitExceeded,
		},
		{
			name: "Disk limit exceeded",
			spec: api.SandboxSpec{
				CPU:      1,
				MemoryMb: 1024,
				DiskGb:   20,
			},
			wantError: ErrResourceLimitExceeded,
		},
		{
			name: "Forbidden network mode",
			spec: api.SandboxSpec{
				CPU:         1,
				MemoryMb:    1024,
				DiskGb:      5,
				NetworkMode: "full",
			},
			wantError: ErrInvalidNetworkMode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.EvaluateSandbox(tt.spec)
			if err != tt.wantError {
				t.Errorf("EvaluateSandbox() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestEvaluateExec(t *testing.T) {
	engine := &Engine{
		AllowedCommands: []string{"git", "npm", "ls"},
	}

	tests := []struct {
		name      string
		command   []string
		wantError error
	}{
		{
			name:      "Allowed command (git)",
			command:   []string{"git", "push"},
			wantError: nil,
		},
		{
			name:      "Allowed command (npm)",
			command:   []string{"npm", "test"},
			wantError: nil,
		},
		{
			name:      "Forbidden command (sudo)",
			command:   []string{"sudo", "rm", "-rf", "/"},
			wantError: ErrForbiddenCommand,
		},
		{
			name:      "Empty command slice",
			command:   []string{},
			wantError: errors.New("no command provided"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := api.ExecRequest{
				Command: tt.command,
			}
			err := engine.EvaluateExec(req)

			if tt.name == "Empty command slice" {
				if err == nil || err.Error() != tt.wantError.Error() {
					t.Errorf("EvaluateExec() error = %v, wantError %v", err, tt.wantError)
				}
				return
			}

			if err != tt.wantError {
				t.Errorf("EvaluateExec() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
