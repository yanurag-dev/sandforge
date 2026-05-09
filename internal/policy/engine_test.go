package policy

import (
	"testing"

	"github.com/sandforge/sandforge/pkg/api"
)

func TestEvaluateMount(t *testing.T) {
	engine := &Engine{
		AllowedHostPrefixes: []string{
			"/tmp/sandforge/workspaces",
			"/Users/testuser/projects",
		},
		BlockedHostPatterns: []string{
			".ssh",
			"/etc/",
		},
	}

	tests := []struct {
		name      string
		hostPath  string
		wantError error
	}{
		{
			name:      "Valid path in workspace",
			hostPath:  "/tmp/sandforge/workspaces/task-1",
			wantError: nil,
		},
		{
			name:      "Exact match of allowed prefix",
			hostPath:  "/tmp/sandforge/workspaces",
			wantError: nil,
		},
		{
			name:      "Path outside whitelist",
			hostPath:  "/usr/local/bin",
			wantError: ErrForbiddenHostPath,
		},
		{
			name:      "Relative path rejected",
			hostPath:  "relative/path",
			wantError: ErrPathNotAbs,
		},
		{
			name:      "Path with blocked pattern (.ssh)",
			hostPath:  "/Users/testuser/projects/app/.ssh/id_rsa",
			wantError: ErrForbiddenHostPath,
		},
		{
			name:      "Path with blocked pattern (/etc/)",
			hostPath:  "/tmp/sandforge/workspaces/fake-etc/etc/passwd",
			wantError: ErrForbiddenHostPath,
		},
		{
			name:      "Partial match bug prevention",
			hostPath:  "/tmp/sandforge/workspaces-secrets",
			wantError: ErrForbiddenHostPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mount := api.WorkspaceMount{
				HostPath: tt.hostPath,
			}
			err := engine.EvaluateMount(mount)
			if err != tt.wantError {
				t.Errorf("EvaluateMount() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
