package supervisor

import (
	"errors"
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
	ID     string
	Spec   api.SandboxSpec
	State  State
	Handle string // The backend-specific identifier
	Error  error
}

// Supervisor orchestrates sandbox lifecycles and enforces policy.
type Supervisor struct {
	mu        sync.RWMutex
	instances map[string]*SandboxInstance

	backend api.SandboxBackend
	policy  *policy.Engine
}

func NewSupervisor(backend api.SandboxBackend, engine *policy.Engine) *Supervisor {
	return &Supervisor{
		instances: make(map[string]*SandboxInstance),
		backend:   backend,
		policy:    engine,
	}
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
	instance.State = StateProvisioning
	handle, err := s.backend.CreateSandbox(spec)
	if err != nil {
		instance.State = StateError
		instance.Error = err
		return err
	}

	// 4. Update state to 'ready'
	instance.Handle = handle
	instance.State = StateReady

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
	if instance.State != StateReady {
		return api.ExecResult{}, errors.New("sandbox is not in 'ready' state")
	}
	if err := s.policy.EvaluateExec(req); err != nil {
		return api.ExecResult{}, err
	}

	// 3. Update state to 'executing'
	instance.State = StateExecuting

	// Ensure we go back to 'ready' unless a fatal error occurred
	defer func() {
		if instance.State == StateExecuting {
			instance.State = StateReady
		}
	}()

	// 4. Call backend
	result, err := s.backend.Exec(instance.Handle, req)
	if err != nil {
		instance.State = StateError
		instance.Error = err
		return result, err
	}

	return result, nil
}

// Stop will clean up the sandbox.
func (s *Supervisor) Stop(id string) error {
	// 1. Find the instance
	s.mu.Lock() // We use a full Lock because we might delete it
	defer s.mu.Unlock()

	instance, exists := s.instances[id]
	if !exists {
		return errors.New("sandbox not found")
	}

	// 2. Move state to 'destroying'
	instance.State = StateDestroying

	// 3. Call backend.DestroySandbox
	if err := s.backend.DestroySandbox(instance.Handle); err != nil {
		instance.State = StateError
		instance.Error = err
		return err
	}

	// 4. Mark destroyed and remove from map
	instance.State = StateDestroyed
	delete(s.instances, id)

	return nil
}
