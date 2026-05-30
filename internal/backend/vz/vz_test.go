//go:build darwin

package vz

import (
	"encoding/json"
	"net"
	"strings"
	"testing"

	"github.com/sandforge/sandforge/pkg/agentproto"
	"github.com/sandforge/sandforge/pkg/api"
)

func TestNewVZBackend(t *testing.T) {
	b := NewVZBackend()
	if b == nil {
		t.Fatal("NewVZBackend returned nil")
	}
	if len(b.sandboxes) != 0 {
		t.Errorf("expected empty sandboxes map, got %d entries", len(b.sandboxes))
	}
}

func TestDialGuestMissingSandbox(t *testing.T) {
	b := NewVZBackend()
	_, err := b.dialGuest("nonexistent-handle")
	if err == nil {
		t.Fatal("expected error for missing sandbox, got nil")
	}
	if !strings.Contains(err.Error(), "sandbox handle not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestDestroySandboxMissing(t *testing.T) {
	b := NewVZBackend()
	err := b.DestroySandbox("ghost-handle")
	if err == nil {
		t.Fatal("expected error destroying non-existent sandbox")
	}
}

func TestWriteReadJSON(t *testing.T) {
	type testPayload struct {
		Value string `json:"value"`
	}

	server, client := net.Pipe()
	defer func() { _ = server.Close() }()
	defer func() { _ = client.Close() }()

	errCh := make(chan error, 1)
	go func() {
		errCh <- agentproto.WriteRequest(client, "test-op", testPayload{Value: "hello"})
	}()

	var env agentproto.Envelope
	if err := agentproto.ReadEnvelope(server, &env); err != nil {
		t.Fatalf("ReadEnvelope failed: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("WriteRequest failed: %v", err)
	}

	if env.Op != "test-op" {
		t.Errorf("expected op 'test-op', got %q", env.Op)
	}

	var p testPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if p.Value != "hello" {
		t.Errorf("expected value 'hello', got %q", p.Value)
	}
}

func TestExecMissingSandbox(t *testing.T) {
	b := NewVZBackend()
	_, err := b.Exec("bad-handle", api.ExecRequest{Command: []string{"ls"}})
	if err == nil {
		t.Fatal("expected error for missing sandbox")
	}
	if !strings.Contains(err.Error(), "sandbox handle not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCopyOutMissingSandbox(t *testing.T) {
	b := NewVZBackend()
	err := b.CopyOut("bad-handle", "/tmp/file", "/tmp/dest")
	if err == nil {
		t.Fatal("expected error for missing sandbox")
	}
}

func TestMountWorkspaceMissingSandbox(t *testing.T) {
	b := NewVZBackend()
	err := b.MountWorkspace("bad-handle", api.WorkspaceMount{
		HostPath:  "/tmp/ws",
		GuestPath: "/workspace",
		ReadOnly:  false,
	})
	if err == nil {
		t.Fatal("expected error for missing sandbox")
	}
}

func TestWriteFileMissingSandbox(t *testing.T) {
	b := NewVZBackend()
	_, err := b.WriteFile("bad-handle", "/tmp/file.txt", []byte("hello"))
	if err == nil {
		t.Fatal("expected error for missing sandbox")
	}
	if !strings.Contains(err.Error(), "sandbox handle not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestListDirMissingSandbox(t *testing.T) {
	b := NewVZBackend()
	_, err := b.ListDir("bad-handle", "/tmp")
	if err == nil {
		t.Fatal("expected error for missing sandbox")
	}
	if !strings.Contains(err.Error(), "sandbox handle not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestStatPathMissingSandbox(t *testing.T) {
	b := NewVZBackend()
	_, err := b.StatPath("bad-handle", "/tmp/file.txt")
	if err == nil {
		t.Fatal("expected error for missing sandbox")
	}
	if !strings.Contains(err.Error(), "sandbox handle not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}
