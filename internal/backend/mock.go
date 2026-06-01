package backend

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"github.com/yanurag-dev/sandforge/pkg/agentproto"
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

// ── PTY support (test double) ────────────────────────────────────────────────

// Compile-time guarantees that the mock satisfies the optional PTY interfaces.
var (
	_ api.PTYBackend = (*MockBackend)(nil)
	_ api.PTYSession = (*mockPTYSession)(nil)
)

// exitSentinel is the input that makes the mock PTY session emit an exit event.
// Tests send it to drive the session to its terminal state deterministically.
const exitSentinel = "\x04" // Ctrl-D / EOT

// mockPTYSession is an in-memory PTYSession used to exercise consumers (the
// supervisor and the WebSocket handler) without a real VM. It echoes stdin back
// as stdout and ends when it receives exitSentinel, using a buffered channel as
// the event queue so NextEvent blocks and orders exactly like the real thing.
type mockPTYSession struct {
	events chan agentproto.StreamEvent
	once   sync.Once
}

func newMockPTYSession() *mockPTYSession {
	return &mockPTYSession{events: make(chan agentproto.StreamEvent, 64)}
}

// StartPTY returns an in-memory echo session. It satisfies api.PTYBackend so the
// MockBackend can stand in for a real backend in PTY tests.
func (m *MockBackend) StartPTY(handle string, _ agentproto.PTYStartRequest) (api.PTYSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, exists := m.sandboxes[handle]; !exists {
		return nil, fmt.Errorf("sandbox handle not found: %s", handle)
	}
	return newMockPTYSession(), nil
}

func (s *mockPTYSession) Resize(_, _ uint16) error { return nil }

// NextEvent returns the next queued event, or io.EOF once the session has ended
// (the events channel is closed by Close or by sending exitSentinel).
func (s *mockPTYSession) NextEvent() (agentproto.StreamEvent, error) {
	ev, ok := <-s.events
	if !ok {
		return agentproto.StreamEvent{}, io.EOF
	}
	return ev, nil
}

func (s *mockPTYSession) Close() error {
	s.once.Do(func() { close(s.events) })
	return nil
}

// SendStdin drives the mock session. Receiving exitSentinel ends the session
// (emit an exit event, then terminate the stream); any other input is echoed
// straight back as a stdout event — the simplest behavior that still lets a
// consumer prove its read loop handles streamed output AND a clean exit + EOF.
func (s *mockPTYSession) SendStdin(data []byte) error {
	if bytes.Equal(data, []byte(exitSentinel)) {
		// Queue the exit event BEFORE closing. The channel is buffered, so the
		// event survives the close and a consumer drains it before NextEvent
		// reports io.EOF — honoring the "exit then EOF" contract.
		s.events <- agentproto.StreamEvent{Event: "exit", Code: 0}
		return s.Close()
	}
	// A real PTY echoes typed input back to the terminal; mirror that.
	s.events <- agentproto.StreamEvent{Event: "stdout", Data: data}
	return nil
}
