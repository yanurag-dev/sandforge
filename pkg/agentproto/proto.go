// Package agentproto defines the JSON envelope protocol shared between the
// host-side VZ backend and the in-guest sandforge-agent binary.
package agentproto

import (
	"encoding/json"
	"io"
)

// Envelope is the top-level framing wrapper for all agent messages.
type Envelope struct {
	Op      string          `json:"op"`
	Payload json.RawMessage `json:"payload"`
}

// OpPTY is the op for an interactive PTY-backed session. Unlike the one-shot
// ops (exec/copyout/write/list/stat), it keeps the connection open and streams
// many StreamEvent values in both directions until the child process exits.
const OpPTY = "pty"

// PTYStartRequest is the first (and only non-event) host → guest payload for
// OpPTY. It is sent once, wrapped in an Envelope, to start the session. All
// subsequent traffic on the connection is bare StreamEvent values.
type PTYStartRequest struct {
	// Command is the program + args to run in the PTY. When empty the guest
	// defaults to an interactive login shell (/bin/bash -i -l).
	Command []string          `json:"command,omitempty"`
	CWD     string            `json:"cwd,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Cols    uint16            `json:"cols"`
	Rows    uint16            `json:"rows"`
}

// StreamEvent is a single framed event on the persistent PTY connection. After
// the session opens, both sides stream these as consecutive JSON values over
// the same connection:
//
//	host → guest:  Event ∈ {"stdin", "resize"}
//	guest → host:  Event ∈ {"stdout", "exit", "error"}
//
// Data carries raw terminal bytes. It is a []byte (not string) so it survives
// JSON round-tripping as base64 — terminal output contains control codes and
// possibly invalid UTF-8 that a string field would corrupt.
type StreamEvent struct {
	Event string `json:"event"`
	Data  []byte `json:"data,omitempty"` // stdin / stdout bytes (base64 over JSON)
	Code  int    `json:"code,omitempty"` // exit code, when Event == "exit"
	Cols  uint16 `json:"cols,omitempty"` // terminal width, when Event == "resize"
	Rows  uint16 `json:"rows,omitempty"` // terminal height, when Event == "resize"
	Msg   string `json:"msg,omitempty"`  // error detail, when Event == "error"
}

// ExecRequest is the payload for the "exec" op (host → guest).
type ExecRequest struct {
	Command    []string          `json:"command"`
	CWD        string            `json:"cwd"`
	Env        map[string]string `json:"env"`
	TimeoutSec int               `json:"timeout_sec"`
}

// ExecResponse is the payload for the "exec" op (guest → host).
type ExecResponse struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

// CopyOutRequest is the payload for the "copyout" op (host → guest).
type CopyOutRequest struct {
	GuestPath string `json:"guest_path"`
}

// CopyOutResponse is the payload for the "copyout" op (guest → host).
type CopyOutResponse struct {
	Data  []byte `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

// WriteFileRequest is the payload for the "write" op (host → guest).
type WriteFileRequest struct {
	GuestPath string `json:"guest_path"`
	Data      []byte `json:"data"`
}

// WriteFileResponse is the payload for the "write" op (guest → host).
type WriteFileResponse struct {
	Size  int    `json:"size,omitempty"`
	Error string `json:"error,omitempty"`
}

// ListRequest is the payload for the "list" op (host → guest).
type ListRequest struct {
	GuestPath string `json:"guest_path"`
}

// DirEntry is a single entry returned by the "list" op.
type DirEntry struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Mode    uint32 `json:"mode"`
	IsDir   bool   `json:"is_dir"`
	ModTime int64  `json:"mod_time"` // Unix timestamp
}

// ListResponse is the payload for the "list" op (guest → host).
type ListResponse struct {
	Entries []DirEntry `json:"entries,omitempty"`
	Error   string     `json:"error,omitempty"`
}

// StatRequest is the payload for the "stat" op (host → guest).
type StatRequest struct {
	GuestPath string `json:"guest_path"`
}

// StatResponse is the payload for the "stat" op (guest → host).
type StatResponse struct {
	Name    string `json:"name,omitempty"`
	Size    int64  `json:"size,omitempty"`
	Mode    uint32 `json:"mode,omitempty"`
	IsDir   bool   `json:"is_dir,omitempty"`
	ModTime int64  `json:"mod_time,omitempty"` // Unix timestamp
	Error   string `json:"error,omitempty"`
}

// WriteRequest encodes op + payload as a newline-delimited JSON envelope.
func WriteRequest(w io.Writer, op string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.NewEncoder(w).Encode(Envelope{Op: op, Payload: json.RawMessage(raw)})
}

// ReadEnvelope decodes a single newline-delimited JSON envelope from r.
func ReadEnvelope(r io.Reader, env *Envelope) error {
	return json.NewDecoder(r).Decode(env)
}

// ReadResponse decodes a single newline-delimited JSON value from r into v.
func ReadResponse(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}

// WriteEvent encodes a single StreamEvent onto enc. Callers must funnel all
// writes for one direction through a single goroutine: a connection tolerates
// one concurrent reader and one concurrent writer, but two concurrent Encode
// calls interleave bytes and corrupt the JSON framing.
func WriteEvent(enc *json.Encoder, ev StreamEvent) error {
	return enc.Encode(ev)
}
