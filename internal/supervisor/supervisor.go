package supervisor

import (
	"errors"
	"fmt"
	"sync"

	"github.com/yanurag-dev/sandforge/internal/policy"
	"github.com/yanurag-dev/sandforge/pkg/agentproto"
	"github.com/yanurag-dev/sandforge/pkg/api"
)

// State represents the current lifecycle phase of a sandbox.
type State string

const (
	StateRequested        State = "requested"
	StateProvisioning     State = "provisioning"
	StateReady            State = "ready"
	StateExecuting        State = "executing"
	StateInteractive      State = "interactive"
	StateWritingFile      State = "writing_file"
	StateCopyingArtifacts State = "copying_artifacts"
	StateDestroying       State = "destroying"
	StateDestroyed        State = "destroyed"
	StateError            State = "error"
)

// SandboxInstance tracks the runtime state of a single sandbox.
type SandboxInstance struct {
	mu     sync.RWMutex
	ID     string
	Spec   api.SandboxSpec
	State  State
	Handle string // The backend-specific identifier
	Error  error
}

func (i *SandboxInstance) SetState(s State) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.State = s
}

func (i *SandboxInstance) GetState() State {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.State
}

func (i *SandboxInstance) SetHandle(h string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.Handle = h
}

func (i *SandboxInstance) GetHandle() string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.Handle
}

func (i *SandboxInstance) SetError(err error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.Error = err
}

// Supervisor orchestrates sandbox lifecycles and enforces policy.
type Supervisor struct {
	mu        sync.RWMutex
	instances map[string]*SandboxInstance

	backend api.SandboxBackend
	policy  *policy.Engine
}

// getInstance returns the handle and current state for a sandbox by ID.
// Returns an error if the sandbox does not exist.
func (s *Supervisor) getInstance(id string) (handle string, state State, err error) {
	s.mu.RLock()
	instance, exists := s.instances[id]
	s.mu.RUnlock()

	if !exists {
		return "", "", errors.New("sandbox not found")
	}

	instance.mu.RLock()
	handle = instance.Handle
	state = instance.State
	instance.mu.RUnlock()

	return handle, state, nil
}

// lookupInstance returns the raw *SandboxInstance for callers that need to
// hold instance.mu themselves to prevent race conditions on state transitions.
func (s *Supervisor) lookupInstance(id string) (*SandboxInstance, error) {
	s.mu.RLock()
	instance, exists := s.instances[id]
	s.mu.RUnlock()

	if !exists {
		return nil, errors.New("sandbox not found")
	}
	return instance, nil
}

func NewSupervisor(backend api.SandboxBackend, engine *policy.Engine) (*Supervisor, error) {
	if backend == nil {
		return nil, fmt.Errorf("NewSupervisor: backend is nil")
	}
	if engine == nil {
		return nil, fmt.Errorf("NewSupervisor: policy engine is nil")
	}
	return &Supervisor{
		instances: make(map[string]*SandboxInstance),
		backend:   backend,
		policy:    engine,
	}, nil
}

// Start will be your entry point to create and boot a sandbox.
func (s *Supervisor) Start(id string, spec api.SandboxSpec) error {
	// 1. Evaluate policy
	if err := s.policy.EvaluateSandbox(spec); err != nil {
		return err
	}

	// 2. Register instance in 'requested' state
	s.mu.Lock()
	if _, exists := s.instances[id]; exists {
		s.mu.Unlock()
		return errors.New("sandbox ID already exists")
	}

	instance := &SandboxInstance{
		ID:    id,
		Spec:  spec,
		State: StateRequested,
	}
	s.instances[id] = instance
	s.mu.Unlock()

	// 3. Move to 'provisioning' and call backend.CreateSandbox
	instance.SetState(StateProvisioning)
	handle, err := s.backend.CreateSandbox(spec)
	if err != nil {
		instance.SetState(StateError)
		instance.SetError(err)
		return err
	}

	// 4. Update state to 'ready'
	instance.SetHandle(handle)
	instance.SetState(StateReady)

	return nil
}

// RunCommand will be used to execute something in a ready sandbox.
func (s *Supervisor) RunCommand(id string, req api.ExecRequest) (api.ExecResult, error) {
	// 1. Find the instance
	s.mu.RLock()
	instance, exists := s.instances[id]
	s.mu.RUnlock()

	if !exists {
		return api.ExecResult{}, errors.New("sandbox not found")
	}

	// 2. Validate state and policy
	// We lock the instance to check state and transition atomically
	instance.mu.Lock()
	if instance.State != StateReady {
		instance.mu.Unlock()
		return api.ExecResult{}, errors.New("sandbox is not in 'ready' state")
	}

	if err := s.policy.EvaluateExec(req); err != nil {
		instance.mu.Unlock()
		return api.ExecResult{}, err
	}

	// 3. Move state to 'executing'
	instance.State = StateExecuting
	handle := instance.Handle
	instance.mu.Unlock()

	// Ensure we go back to 'ready' unless a fatal error occurred
	defer func() {
		instance.mu.Lock()
		if instance.State == StateExecuting {
			instance.State = StateReady
		}
		instance.mu.Unlock()
	}()

	// 4. Call backend
	result, err := s.backend.Exec(handle, req)
	if err != nil {
		instance.mu.Lock()
		instance.State = StateError
		instance.Error = err
		instance.mu.Unlock()
		return result, err
	}

	return result, nil
}

// ptySession wraps a backend PTYSession so the supervisor's StateInteractive is
// reset to StateReady exactly when the session ENDS — not when StartPTY returns.
// A PTY session is long-lived: StartPTY hands back a live session that outlives
// the call, so the synchronous defer pattern RunCommand uses would wrongly snap
// the state back to ready while the shell is still running. Resetting on Close
// ties the transition to the session lifecycle (owned by the caller, e.g. the
// WebSocket handler) instead.
type ptySession struct {
	api.PTYSession
	instance *SandboxInstance
	once     sync.Once
}

func (p *ptySession) Close() error {
	p.once.Do(func() {
		p.instance.mu.Lock()
		// Only reset if we still own the state; another operation (e.g. Stop)
		// may have moved it on already.
		if p.instance.State == StateInteractive {
			p.instance.State = StateReady
		}
		p.instance.mu.Unlock()
	})
	return p.PTYSession.Close()
}

// StartPTY opens an interactive PTY session in a ready sandbox. The returned
// session's Close() resets the sandbox back to StateReady, so callers MUST close
// it when the interactive session ends. Requires a backend that implements
// api.PTYBackend; otherwise returns an error.
func (s *Supervisor) StartPTY(id string, req agentproto.PTYStartRequest) (api.PTYSession, error) {
	instance, err := s.lookupInstance(id)
	if err != nil {
		return nil, err
	}

	// Policy gate: interactive sessions are an explicit opt-in.
	if err := s.policy.EvaluatePTY(); err != nil {
		return nil, err
	}

	// The backend may not support PTY (e.g. a future KVM backend before it
	// lands this capability). Degrade gracefully rather than panic.
	ptyBackend, ok := s.backend.(api.PTYBackend)
	if !ok {
		return nil, errors.New("backend does not support interactive PTY sessions")
	}

	// Atomically check ready-state and transition to interactive so a
	// concurrent RunCommand/WriteFile cannot collide with the live session.
	instance.mu.Lock()
	if instance.State != StateReady {
		state := instance.State
		instance.mu.Unlock()
		return nil, fmt.Errorf("sandbox is in state %s, must be %s to start a PTY", state, StateReady)
	}
	instance.State = StateInteractive
	handle := instance.Handle
	instance.mu.Unlock()

	sess, err := ptyBackend.StartPTY(handle, req)
	if err != nil {
		// Roll the state back; the session never started.
		instance.mu.Lock()
		if instance.State == StateInteractive {
			instance.State = StateReady
		}
		instance.mu.Unlock()
		return nil, err
	}

	return &ptySession{PTYSession: sess, instance: instance}, nil
}

// GetState returns the current state of a sandbox by ID.
func (s *Supervisor) GetState(id string) (State, error) {
	s.mu.RLock()
	instance, exists := s.instances[id]
	s.mu.RUnlock()

	if !exists {
		return "", errors.New("sandbox not found")
	}
	return instance.GetState(), nil
}

// Stop will clean up the sandbox.
func (s *Supervisor) Stop(id string) error {
	// 1. Find the instance
	s.mu.RLock()
	instance, exists := s.instances[id]
	s.mu.RUnlock()

	if !exists {
		return errors.New("sandbox not found")
	}

	// 2. Move state to 'destroying'
	instance.mu.Lock()
	handle := instance.Handle
	instance.State = StateDestroying
	instance.mu.Unlock()

	// 3. Call backend.DestroySandbox (without holding the lock)
	if err := s.backend.DestroySandbox(handle); err != nil {
		instance.mu.Lock()
		instance.State = StateError
		instance.Error = err
		instance.mu.Unlock()
		return err
	}

	// 4. Mark destroyed and remove from map
	s.mu.Lock()
	delete(s.instances, id)
	s.mu.Unlock()

	instance.SetState(StateDestroyed)

	return nil
}

// MountWorkspace allows attaching a host path to the sandbox.
func (s *Supervisor) MountWorkspace(id string, mount api.WorkspaceMount) error {
	// 1. Find the instance
	s.mu.RLock()
	instance, exists := s.instances[id]
	s.mu.RUnlock()

	if !exists {
		return errors.New("sandbox not found")
	}

	// 2. Initial state check
	instance.mu.RLock()
	if instance.State != StateReady {
		instance.mu.RUnlock()
		return fmt.Errorf("sandbox is in state %s, must be %s to mount", instance.State, StateReady)
	}
	instance.mu.RUnlock()

	// 3. Evaluate policy (without holding instance lock)
	if err := s.policy.EvaluateMount(mount); err != nil {
		return err
	}

	// 4. Re-check state and get handle
	instance.mu.Lock()
	if instance.State != StateReady {
		instance.mu.Unlock()
		return fmt.Errorf("sandbox is in state %s, must be %s to mount", instance.State, StateReady)
	}
	handle := instance.Handle
	instance.mu.Unlock()

	// 5. Call backend
	return s.backend.MountWorkspace(handle, mount)
}

// CopyOut retrieves a file or directory from the sandbox.
func (s *Supervisor) CopyOut(id string, path string, dest string) error {
	// 1. Policy check for destination host path
	if err := s.policy.EvaluateHostPath(dest); err != nil {
		return fmt.Errorf("policy denied copy out destination: %w", err)
	}

	// 2. Find instance and validate state
	handle, state, err := s.getInstance(id)
	if err != nil {
		return err
	}

	// We allow CopyOut if it's Ready or Executing (e.g. streaming logs)
	if state != StateReady && state != StateExecuting {
		return fmt.Errorf("sandbox is in state %s, must be %s or %s to copy out", state, StateReady, StateExecuting)
	}

	// 3. Call backend
	return s.backend.CopyOut(handle, path, dest)
}

// ReadFile reads a file from guestPath inside the sandbox and returns its contents.
// Allowed during Ready or Executing — reads are safe concurrently.
func (s *Supervisor) ReadFile(id string, guestPath string) ([]byte, error) {
	handle, state, err := s.getInstance(id)
	if err != nil {
		return nil, err
	}
	if state != StateReady && state != StateExecuting {
		return nil, fmt.Errorf("sandbox is in state %s, must be %s or %s to read", state, StateReady, StateExecuting)
	}
	return s.backend.ReadFile(handle, guestPath)
}

// WriteFile writes data to guestPath inside the sandbox, creating parent directories.
// Only allowed when the sandbox is Ready — writing during execution risks a file race.
func (s *Supervisor) WriteFile(id string, guestPath string, data []byte) (int, error) {
	// 1. Look up instance
	instance, err := s.lookupInstance(id)
	if err != nil {
		return 0, err
	}

	// 2. Atomically check state and transition to WritingFile so RunCommand
	//    cannot move the sandbox to StateExecuting concurrently.
	instance.mu.Lock()
	if instance.State != StateReady {
		state := instance.State
		instance.mu.Unlock()
		return 0, fmt.Errorf("sandbox is in state %s, must be %s to write", state, StateReady)
	}
	instance.State = StateWritingFile
	handle := instance.Handle
	instance.mu.Unlock()

	defer func() {
		instance.mu.Lock()
		if instance.State == StateWritingFile {
			instance.State = StateReady
		}
		instance.mu.Unlock()
	}()

	// 3. Call backend
	return s.backend.WriteFile(handle, guestPath, data)
}

// ListDir returns directory entries for guestPath inside the sandbox.
// Allowed during Ready or Executing — listing is read-only and safe concurrently.
func (s *Supervisor) ListDir(id string, guestPath string) ([]api.DirEntry, error) {
	// 1. Find instance and validate state
	handle, state, err := s.getInstance(id)
	if err != nil {
		return nil, err
	}

	if state != StateReady && state != StateExecuting {
		return nil, fmt.Errorf("sandbox is in state %s, must be %s or %s to list", state, StateReady, StateExecuting)
	}

	// 2. Call backend
	return s.backend.ListDir(handle, guestPath)
}

// StatPath returns metadata for guestPath inside the sandbox.
// Allowed during Ready or Executing — stat is read-only and safe concurrently.
func (s *Supervisor) StatPath(id string, guestPath string) (api.StatInfo, error) {
	// 1. Find instance and validate state
	handle, state, err := s.getInstance(id)
	if err != nil {
		return api.StatInfo{}, err
	}

	if state != StateReady && state != StateExecuting {
		return api.StatInfo{}, fmt.Errorf("sandbox is in state %s, must be %s or %s to stat", state, StateReady, StateExecuting)
	}

	// 2. Call backend
	return s.backend.StatPath(handle, guestPath)
}
