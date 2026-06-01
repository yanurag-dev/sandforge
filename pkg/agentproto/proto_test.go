package agentproto

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

// TestStreamEventsFrameOverOneStream proves that many StreamEvent values stream
// as consecutive JSON over a single connection and decode back in order — the
// core property the persistent PTY connection relies on.
func TestStreamEventsFrameOverOneStream(t *testing.T) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)

	want := []StreamEvent{
		{Event: "stdout", Data: []byte("hello\r\n")},
		{Event: "stdout", Data: []byte{0x1b, '[', '0', 'm'}}, // raw ANSI / non-printable
		{Event: "resize", Cols: 120, Rows: 40},
		{Event: "exit", Code: 7},
	}
	for _, ev := range want {
		if err := WriteEvent(enc, ev); err != nil {
			t.Fatalf("WriteEvent(%+v): %v", ev, err)
		}
	}

	dec := json.NewDecoder(&buf)
	for i, w := range want {
		var got StreamEvent
		if err := dec.Decode(&got); err != nil {
			t.Fatalf("Decode #%d: %v", i, err)
		}
		if got.Event != w.Event || got.Code != w.Code || got.Cols != w.Cols || got.Rows != w.Rows {
			t.Errorf("event #%d = %+v, want %+v", i, got, w)
		}
		if !bytes.Equal(got.Data, w.Data) {
			t.Errorf("event #%d data = %v, want %v", i, got.Data, w.Data)
		}
	}

	// Stream is exhausted: the next decode is io.EOF (the terminator the
	// session contract relies on after the exit event).
	var extra StreamEvent
	if err := dec.Decode(&extra); err != io.EOF {
		t.Errorf("expected io.EOF after last event, got %v", err)
	}
}

// TestPTYStartRequestRoundTrip checks the start payload survives an Envelope.
func TestPTYStartRequestRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	req := PTYStartRequest{
		Command: []string{"/bin/bash", "-i"},
		CWD:     "/workspace",
		Env:     map[string]string{"TERM": "xterm-256color"},
		Cols:    80,
		Rows:    24,
	}
	if err := WriteRequest(&buf, OpPTY, req); err != nil {
		t.Fatalf("WriteRequest: %v", err)
	}

	var env Envelope
	if err := ReadEnvelope(&buf, &env); err != nil {
		t.Fatalf("ReadEnvelope: %v", err)
	}
	if env.Op != OpPTY {
		t.Errorf("op = %q, want %q", env.Op, OpPTY)
	}

	var got PTYStartRequest
	if err := json.Unmarshal(env.Payload, &got); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got.Cols != 80 || got.Rows != 24 || got.CWD != "/workspace" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if len(got.Command) != 2 || got.Command[0] != "/bin/bash" {
		t.Errorf("command mismatch: %v", got.Command)
	}
	if got.Env["TERM"] != "xterm-256color" {
		t.Errorf("env mismatch: %v", got.Env)
	}
}

// TestExecEnvelopeBackwardCompatible guards the acceptance criterion that the
// existing one-shot exec op is untouched by the PTY additions.
func TestExecEnvelopeBackwardCompatible(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteRequest(&buf, "exec", ExecRequest{Command: []string{"echo", "hi"}, TimeoutSec: 5}); err != nil {
		t.Fatalf("WriteRequest: %v", err)
	}

	var env Envelope
	if err := ReadEnvelope(&buf, &env); err != nil {
		t.Fatalf("ReadEnvelope: %v", err)
	}
	if env.Op != "exec" {
		t.Errorf("op = %q, want \"exec\"", env.Op)
	}
	var got ExecRequest
	if err := json.Unmarshal(env.Payload, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Command) != 2 || got.Command[1] != "hi" || got.TimeoutSec != 5 {
		t.Errorf("exec request mismatch: %+v", got)
	}
}
