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
