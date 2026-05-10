package supervisor

import (
	"errors"
	"fmt"
	"sync"

	"github.com/sandforge/sandforge/internal/policy"
	"github.com/sandforge/sandforge/pkg/api"
)

// State represents the current lifecycle phase of a sandbox.
type State string

const (
	StateRequested          State = "requested"
	StateProvisioning       State = "provisioning"
	StateReady              State = "ready"
	StateExecuting          State = "executing"
	StateCopyingArtifacts   State = "copying_artifacts"
	StateDestroying         State = "destroying"
	StateDestroyed          State = "destroyed"
	StateError              State = "error"
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
