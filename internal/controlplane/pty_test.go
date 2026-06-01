package controlplane

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/yanurag-dev/sandforge/internal/backend"
	"github.com/yanurag-dev/sandforge/internal/policy"
	"github.com/yanurag-dev/sandforge/internal/supervisor"
	"github.com/yanurag-dev/sandforge/pkg/agentproto"
	"github.com/yanurag-dev/sandforge/pkg/api"
)

// ptyTestServer wires a Server over a MockBackend and exposes it via httptest.
// allowInteractive toggles the policy opt-in. Returns the ws:// base URL.
func ptyTestServer(t *testing.T, allowInteractive bool) (string, string) {
	t.Helper()
	sup, err := supervisor.NewSupervisor(backend.NewMockBackend(), &policy.Engine{
		MaxCPU:              4,
		MaxMemoryMb:         4096,
		MaxDiskGb:           10,
		AllowedNetworkModes: []string{"offline"},
		AllowInteractive:    allowInteractive,
	})
	if err != nil {
		t.Fatalf("NewSupervisor: %v", err)
	}
	srv, err := NewServer(sup)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	id := "ws-sbx"
	if err := sup.Start(id, api.SandboxSpec{CPU: 2, MemoryMb: 1024, DiskGb: 5, NetworkMode: "offline"}); err != nil {
		t.Fatalf("Start sandbox: %v", err)
	}

	ts := httptest.NewServer(srv.handler())
	t.Cleanup(ts.Close)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	return wsURL, id
}

func TestHandlePTYEchoOverWebSocket(t *testing.T) {
	wsURL, id := ptyTestServer(t, true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL+"/v1/sandboxes/"+id+"/pty", nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()

	// Send a stdin frame; the mock backend echoes it straight back as stdout.
	stdin, _ := json.Marshal(agentproto.StreamEvent{Event: "stdin", Data: []byte("hello ws")})
	if err := conn.Write(ctx, websocket.MessageText, stdin); err != nil {
		t.Fatalf("ws write stdin: %v", err)
	}

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("ws read stdout: %v", err)
	}
	var ev agentproto.StreamEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if ev.Event != "stdout" || string(ev.Data) != "hello ws" {
		t.Errorf("got %+v (data=%q), want stdout 'hello ws'", ev, ev.Data)
	}
}

func TestHandlePTYExitOverWebSocket(t *testing.T) {
	wsURL, id := ptyTestServer(t, true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL+"/v1/sandboxes/"+id+"/pty", nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()

	// The mock's exit sentinel (Ctrl-D) drives the session to its exit event.
	exit, _ := json.Marshal(agentproto.StreamEvent{Event: "stdin", Data: []byte("\x04")})
	if err := conn.Write(ctx, websocket.MessageText, exit); err != nil {
		t.Fatalf("ws write sentinel: %v", err)
	}

	// Read until we see the exit event forwarded from the backend.
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("ws read (never saw exit): %v", err)
		}
		var ev agentproto.StreamEvent
		if err := json.Unmarshal(data, &ev); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if ev.Event == "exit" {
			if ev.Code != 0 {
				t.Errorf("exit code = %d, want 0", ev.Code)
			}
			return
		}
	}
}

// TestHandlePTYSurvivesWriteTimeout proves the per-connection WriteTimeout
// override: a server with a short WriteTimeout must NOT kill a long-lived PTY
// stream. We hold the connection past the timeout, then exchange a frame.
func TestHandlePTYSurvivesWriteTimeout(t *testing.T) {
	sup, err := supervisor.NewSupervisor(backend.NewMockBackend(), &policy.Engine{
		MaxCPU: 4, MaxMemoryMb: 4096, MaxDiskGb: 10,
		AllowedNetworkModes: []string{"offline"},
		AllowInteractive:    true,
	})
	if err != nil {
		t.Fatalf("NewSupervisor: %v", err)
	}
	srv, _ := NewServer(sup)
	id := "ws-timeout"
	if err := sup.Start(id, api.SandboxSpec{CPU: 2, MemoryMb: 1024, DiskGb: 5, NetworkMode: "offline"}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Unlike httptest.NewServer (no timeouts), construct one WITH a short
	// WriteTimeout so the override path is actually exercised.
	const writeTimeout = 300 * time.Millisecond
	ts := httptest.NewUnstartedServer(srv.handler())
	ts.Config.WriteTimeout = writeTimeout
	ts.Start()
	t.Cleanup(ts.Close)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL+"/v1/sandboxes/"+id+"/pty", nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()

	// Idle past the server's WriteTimeout. Without the per-conn deadline reset
	// the hijacked connection would be torn down here.
	time.Sleep(2 * writeTimeout)

	stdin, _ := json.Marshal(agentproto.StreamEvent{Event: "stdin", Data: []byte("alive")})
	if err := conn.Write(ctx, websocket.MessageText, stdin); err != nil {
		t.Fatalf("write after timeout window: %v", err)
	}
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read after timeout window (connection was killed?): %v", err)
	}
	var ev agentproto.StreamEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ev.Event != "stdout" || string(ev.Data) != "alive" {
		t.Errorf("got %+v, want stdout 'alive'", ev)
	}
}

// TestHandlePTYPolicyRejected verifies that with interactive disabled the
// upgrade is refused with an HTTP error BEFORE the WebSocket handshake — the
// reason StartPTY runs before websocket.Accept.
func TestHandlePTYPolicyRejected(t *testing.T) {
	wsURL, id := ptyTestServer(t, false) // interactive disabled

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, resp, err := websocket.Dial(ctx, wsURL+"/v1/sandboxes/"+id+"/pty", nil)
	if err == nil {
		_ = conn.CloseNow()
		t.Fatal("expected dial to fail (policy), but it succeeded")
	}
	if resp == nil {
		t.Fatalf("expected an HTTP response on failed upgrade, got nil (err=%v)", err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
	}
}
