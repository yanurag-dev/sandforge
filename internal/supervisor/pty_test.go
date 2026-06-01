package supervisor

import (
	"errors"
	"testing"

	"github.com/yanurag-dev/sandforge/internal/backend"
	"github.com/yanurag-dev/sandforge/internal/policy"
	"github.com/yanurag-dev/sandforge/pkg/agentproto"
	"github.com/yanurag-dev/sandforge/pkg/api"
)

// newPTYSupervisor builds a supervisor over a MockBackend with one ready
// sandbox, with interactive sessions allowed/denied per the flag.
func newPTYSupervisor(t *testing.T, allowInteractive bool) (*Supervisor, string) {
	t.Helper()
	sup, err := NewSupervisor(backend.NewMockBackend(), &policy.Engine{
		MaxCPU:              4,
		MaxMemoryMb:         4096,
		MaxDiskGb:           10,
		AllowedNetworkModes: []string{"offline"},
		AllowInteractive:    allowInteractive,
	})
	if err != nil {
		t.Fatalf("NewSupervisor: %v", err)
	}
	id := "pty-sbx"
	if err := sup.Start(id, api.SandboxSpec{CPU: 2, MemoryMb: 1024, DiskGb: 5, NetworkMode: "offline"}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return sup, id
}

func TestStartPTYPolicyGate(t *testing.T) {
	sup, id := newPTYSupervisor(t, false) // interactive disabled
	_, err := sup.StartPTY(id, agentproto.PTYStartRequest{})
	if !errors.Is(err, policy.ErrInteractiveDisabled) {
		t.Errorf("StartPTY error = %v, want ErrInteractiveDisabled", err)
	}
	// State must be untouched by a rejected start.
	if st, _ := sup.GetState(id); st != StateReady {
		t.Errorf("state = %s, want %s (policy rejection must not transition)", st, StateReady)
	}
}

func TestStartPTYStateTransitions(t *testing.T) {
	sup, id := newPTYSupervisor(t, true)

	sess, err := sup.StartPTY(id, agentproto.PTYStartRequest{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("StartPTY: %v", err)
	}

	// While the session is live, the sandbox is interactive...
	if st, _ := sup.GetState(id); st != StateInteractive {
		t.Errorf("state during session = %s, want %s", st, StateInteractive)
	}
	// ...so a concurrent RunCommand must be rejected (not ready).
	if _, err := sup.RunCommand(id, api.ExecRequest{Command: []string{"ls"}}); err == nil {
		t.Error("RunCommand during interactive session should fail, got nil")
	}

	// Closing the session resets the sandbox to ready.
	if err := sess.Close(); err != nil {
		t.Fatalf("session Close: %v", err)
	}
	if st, _ := sup.GetState(id); st != StateReady {
		t.Errorf("state after Close = %s, want %s", st, StateReady)
	}

	// Close is idempotent — a second call must not panic or change state.
	if err := sess.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
	if st, _ := sup.GetState(id); st != StateReady {
		t.Errorf("state after second Close = %s, want %s", st, StateReady)
	}
}

func TestStartPTYNotReady(t *testing.T) {
	sup, id := newPTYSupervisor(t, true)

	// Open one session to move the sandbox out of ready.
	sess, err := sup.StartPTY(id, agentproto.PTYStartRequest{})
	if err != nil {
		t.Fatalf("first StartPTY: %v", err)
	}
	defer func() { _ = sess.Close() }()

	// A second StartPTY must be rejected because state != ready.
	if _, err := sup.StartPTY(id, agentproto.PTYStartRequest{}); err == nil {
		t.Error("second StartPTY on non-ready sandbox should fail, got nil")
	}
}

func TestStartPTYMissingSandbox(t *testing.T) {
	sup, _ := newPTYSupervisor(t, true)
	if _, err := sup.StartPTY("ghost", agentproto.PTYStartRequest{}); err == nil {
		t.Error("StartPTY on missing sandbox should fail, got nil")
	}
}
