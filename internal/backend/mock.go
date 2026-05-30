package backend

import (
	"fmt"
	"sync"

	"github.com/yanurag-dev/sandforge/pkg/api"
)

type MockBackend struct {
	mu        sync.RWMutex
	sandboxes map[string]api.SandboxSpec
	nextID    int
}

func NewMockBackend() *MockBackend {
	return &MockBackend{
		sandboxes: make(map[string]api.SandboxSpec),
		nextID:    1,
	}
}

func (m *MockBackend) CreateSandbox(spec api.SandboxSpec) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	handle := fmt.Sprintf("mock-%d", m.nextID)
	m.nextID++
	m.sandboxes[handle] = spec
	return handle, nil
}

func (m *MockBackend) MountWorkspace(handle string, mount api.WorkspaceMount) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, exists := m.sandboxes[handle]; !exists {
		return fmt.Errorf("sandbox handle not found: %s", handle)
	}
	return nil
}

func (m *MockBackend) Exec(handle string, req api.ExecRequest) (api.ExecResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, exists := m.sandboxes[handle]; !exists {
		return api.ExecResult{}, fmt.Errorf("sandbox handle not found: %s", handle)
	}

	return api.ExecResult{
		ExitCode: 0,
		Stdout:   fmt.Sprintf("mock output for %v", req.Command),
	}, nil
}

func (m *MockBackend) CopyOut(handle string, path string, dest string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, exists := m.sandboxes[handle]; !exists {
		return fmt.Errorf("sandbox handle not found: %s", handle)
	}
	return nil
}

func (m *MockBackend) ReadFile(handle string, guestPath string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, exists := m.sandboxes[handle]; !exists {
		return nil, fmt.Errorf("sandbox handle not found: %s", handle)
	}
	return []byte("mock file contents"), nil
}

func (m *MockBackend) WriteFile(handle string, guestPath string, data []byte) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, exists := m.sandboxes[handle]; !exists {
		return 0, fmt.Errorf("sandbox handle not found: %s", handle)
	}
	return len(data), nil
}

func (m *MockBackend) ListDir(handle string, guestPath string) ([]api.DirEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, exists := m.sandboxes[handle]; !exists {
		return nil, fmt.Errorf("sandbox handle not found: %s", handle)
	}
	return []api.DirEntry{}, nil
}

func (m *MockBackend) StatPath(handle string, guestPath string) (api.StatInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, exists := m.sandboxes[handle]; !exists {
		return api.StatInfo{}, fmt.Errorf("sandbox handle not found: %s", handle)
	}
	return api.StatInfo{}, nil
}

func (m *MockBackend) DestroySandbox(handle string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.sandboxes[handle]; !exists {
		return fmt.Errorf("sandbox handle not found: %s", handle)
	}
	delete(m.sandboxes, handle)
	return nil
}
