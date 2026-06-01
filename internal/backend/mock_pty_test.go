package backend

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/yanurag-dev/sandforge/pkg/agentproto"
	"github.com/yanurag-dev/sandforge/pkg/api"
)

func testSpec() api.SandboxSpec {
	return api.SandboxSpec{Backend: "mock", CPU: 1, MemoryMb: 256}
}

// TestMockPTYEchoThenExit verifies the test double's contract: input is echoed
// as stdout, the sentinel produces an exit event, and the very next read is
// io.EOF — the exact "exit then EOF" sequence real consumers loop on.
func TestMockPTYEchoThenExit(t *testing.T) {
	m := NewMockBackend()
	handle, err := m.CreateSandbox(testSpec())
	if err != nil {
		t.Fatalf("CreateSandbox: %v", err)
	}

	sess, err := m.StartPTY(handle, agentproto.PTYStartRequest{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("StartPTY: %v", err)
	}

	if err := sess.SendStdin([]byte("ls\n")); err != nil {
		t.Fatalf("SendStdin: %v", err)
	}
	if err := sess.SendStdin([]byte(exitSentinel)); err != nil {
		t.Fatalf("SendStdin(exit): %v", err)
	}

	// 1) echoed stdout
	ev, err := sess.NextEvent()
	if err != nil {
		t.Fatalf("NextEvent #1: %v", err)
	}
	if ev.Event != "stdout" || !bytes.Equal(ev.Data, []byte("ls\n")) {
		t.Errorf("event #1 = %+v, want stdout 'ls\\n'", ev)
	}

	// 2) exit event delivered normally
	ev, err = sess.NextEvent()
	if err != nil {
		t.Fatalf("NextEvent #2: %v", err)
	}
	if ev.Event != "exit" {
		t.Errorf("event #2 = %+v, want exit", ev)
	}

	// 3) io.EOF terminates the stream
	if _, err := sess.NextEvent(); !errors.Is(err, io.EOF) {
		t.Errorf("NextEvent #3 = %v, want io.EOF", err)
	}
}

// TestMockPTYCloseGivesEOF verifies an explicit Close (without the sentinel)
// also surfaces as io.EOF — the abrupt-teardown path.
func TestMockPTYCloseGivesEOF(t *testing.T) {
	m := NewMockBackend()
	handle, _ := m.CreateSandbox(testSpec())
	sess, err := m.StartPTY(handle, agentproto.PTYStartRequest{})
	if err != nil {
		t.Fatalf("StartPTY: %v", err)
	}

	if err := sess.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := sess.NextEvent(); !errors.Is(err, io.EOF) {
		t.Errorf("NextEvent after Close = %v, want io.EOF", err)
	}
	// Close is idempotent (sync.Once) — a second call must not panic.
	if err := sess.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestMockStartPTYMissingSandbox(t *testing.T) {
	m := NewMockBackend()
	if _, err := m.StartPTY("ghost", agentproto.PTYStartRequest{}); err == nil {
		t.Fatal("expected error for missing sandbox, got nil")
	}
}
