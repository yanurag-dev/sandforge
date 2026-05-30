package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yanurag-dev/sandforge/pkg/api"
)

// newTestServer spins up a fake control plane. The handler closure lets each
// test inspect the incoming request and decide what to return.
func newTestServer(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL), srv
}

func TestCreateSandbox(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Assert the SDK hit the right method + path.
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/sandboxes" {
			t.Errorf("path = %q, want /v1/sandboxes", r.URL.Path)
		}

		// Assert the SDK sent a generated id and the spec.
		var got createRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if !strings.HasPrefix(got.ID, "sbx-") {
			t.Errorf("id = %q, want sbx- prefix", got.ID)
		}
		if got.Spec.CPU != 2 {
			t.Errorf("spec.CPU = %d, want 2", got.Spec.CPU)
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(createResponse{ID: got.ID})
	})

	sb, err := c.CreateSandbox(context.Background(), api.SandboxSpec{CPU: 2})
	if err != nil {
		t.Fatalf("CreateSandbox: %v", err)
	}
	if !strings.HasPrefix(sb.ID, "sbx-") {
		t.Errorf("returned id = %q, want sbx- prefix", sb.ID)
	}
}

func TestExec(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sandboxes/sbx-123/exec" {
			t.Errorf("path = %q, want /v1/sandboxes/sbx-123/exec", r.URL.Path)
		}

		var got api.ExecRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(got.Command) != 2 || got.Command[0] != "echo" {
			t.Errorf("command = %v, want [echo hello]", got.Command)
		}

		_ = json.NewEncoder(w).Encode(api.ExecResult{ExitCode: 0, Stdout: "hello\n"})
	})

	res, err := c.Exec(context.Background(), "sbx-123", api.ExecRequest{
		Command: []string{"echo", "hello"},
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if res.Stdout != "hello\n" {
		t.Errorf("stdout = %q, want %q", res.Stdout, "hello\n")
	}
}

func TestExecErrorStatus(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(apiError{Error: "command not allowed"})
	})

	_, err := c.Exec(context.Background(), "sbx-123", api.ExecRequest{
		Command: []string{"rm", "-rf", "/"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "command not allowed") {
		t.Errorf("error = %q, want it to contain server message", err.Error())
	}
}

func TestDestroy(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		if r.URL.Path != "/v1/sandboxes/sbx-abc" {
			t.Errorf("path = %q, want /v1/sandboxes/sbx-abc", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := c.Destroy(context.Background(), "sbx-abc"); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
}
