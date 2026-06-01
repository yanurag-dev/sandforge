//go:build linux

package main

import (
	"bytes"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/yanurag-dev/sandforge/pkg/agentproto"
)

// TestHandlePTYEcho drives the real handlePTY against a live `cat` subprocess
// over a net.Pipe (no VSOCK, no VM). `cat` echoes stdin to stdout and exits on
// stdin EOF, giving deterministic output to assert the full stream contract:
// stdin → stdout streaming, then a clean exit event.
func TestHandlePTYEcho(t *testing.T) {
	hostSide, agentSide := net.Pipe()
	defer func() { _ = hostSide.Close() }()

	// Run the agent handler against its side of the pipe. The initial envelope
	// payload selects `cat` so output is predictable.
	start := agentproto.PTYStartRequest{Command: []string{"cat"}, Cols: 80, Rows: 24}
	payload, _ := json.Marshal(start)

	go handlePTY(agentSide, payload)

	enc := json.NewEncoder(hostSide)
	dec := json.NewDecoder(hostSide)

	// Send a line of input.
	if err := enc.Encode(agentproto.StreamEvent{Event: "stdin", Data: []byte("hello pty\n")}); err != nil {
		t.Fatalf("send stdin: %v", err)
	}

	// Expect that text echoed back as stdout (a PTY echoes input too, so it may
	// arrive across one or more stdout events — accumulate until we see it).
	_ = hostSide.SetReadDeadline(time.Now().Add(5 * time.Second))
	var out bytes.Buffer
	for !bytes.Contains(out.Bytes(), []byte("hello pty")) {
		var ev agentproto.StreamEvent
		if err := dec.Decode(&ev); err != nil {
			t.Fatalf("decode stdout (got %q so far): %v", out.String(), err)
		}
		if ev.Event == "stdout" {
			out.Write(ev.Data)
		}
	}

	// Send EOT (Ctrl-D) at the start of a line: the PTY driver turns it into a
	// zero-byte read on cat's stdin, so cat exits 0 on its own. This exercises
	// the GRACEFUL exit path (child exits → we reap → emit exit) over the same
	// live connection, rather than closing the pipe (which would test abrupt
	// SIGHUP teardown). Keep reading until the exit event arrives.
	if err := enc.Encode(agentproto.StreamEvent{Event: "stdin", Data: []byte{0x04}}); err != nil {
		t.Fatalf("send EOT: %v", err)
	}

	for {
		var ev agentproto.StreamEvent
		if err := dec.Decode(&ev); err != nil {
			t.Fatalf("decode waiting for exit: %v", err)
		}
		if ev.Event == "exit" {
			if ev.Code != 0 {
				t.Errorf("exit code = %d, want 0 (clean cat EOF)", ev.Code)
			}
			return
		}
		// Trailing stdout (echoed EOT, etc.) is ignored.
	}
}

// TestHandlePTYExitCode verifies a non-zero exit code propagates in the exit
// event. `sh -c "exit 3"` exits immediately with code 3.
func TestHandlePTYExitCode(t *testing.T) {
	hostSide, agentSide := net.Pipe()
	defer func() { _ = hostSide.Close() }()

	start := agentproto.PTYStartRequest{Command: []string{"sh", "-c", "exit 3"}}
	payload, _ := json.Marshal(start)
	go handlePTY(agentSide, payload)

	dec := json.NewDecoder(hostSide)
	_ = hostSide.SetReadDeadline(time.Now().Add(5 * time.Second))

	for {
		var ev agentproto.StreamEvent
		if err := dec.Decode(&ev); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if ev.Event == "exit" {
			if ev.Code != 3 {
				t.Errorf("exit code = %d, want 3", ev.Code)
			}
			return
		}
		// stdout events (shell noise) are ignored.
	}
}
