package backend

import (
	"fmt"
	"sync"

	"github.com/sandforge/sandforge/pkg/api"
)

type MockBackend struct {
	mu        sync.RWMutex
	sandboxes map[string]api.SandboxSpec
}

func NewMockBackend() *MockBackend {
	return &MockBackend{
		sandboxes: make(map[string]api.SandboxSpec),
	}
}

func (m *MockBackend) CreateSandbox(spec api.SandboxSpec) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	handle := fmt.Sprintf("mock-%d", len(m.sandboxes))
	m.sandboxes[handle] = spec
	return handle, nil
}

func (m *MockBackend) MountWorkspace(handle string, mount api.WorkspaceMount) error {
	return nil
}

func (m *MockBackend) Exec(handle string, req api.ExecRequest) (api.ExecResult, error) {
	return api.ExecResult{
		ExitCode: 0,
		Stdout:   fmt.Sprintf("mock output for %v", req.Command),
	}, nil
}

func (m *MockBackend) CopyOut(handle string, path string, dest string) error {
	return nil
}

func (m *MockBackend) DestroySandbox(handle string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sandboxes, handle)
	return nil
}
