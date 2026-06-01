package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/yanurag-dev/sandforge/pkg/agentproto"
)

// newPTYTestClient spins up a fake PTY WebSocket endpoint at
// /v1/sandboxes/{id}/pty driven by the given handler, and returns a Client
// pointed at it.
func newPTYTestClient(t *testing.T, handler func(ctx context.Context, c *websocket.Conn)) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/sandboxes/{id}/pty", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		handler(r.Context(), c)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL)
}

func TestOpenPTYEchoAndExit(t *testing.T) {
	// Fake guest: echo each stdin frame as stdout, then emit exit + clean close.
	client := newPTYTestClient(t, func(ctx context.Context, c *websocket.Conn) {
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var ev agentproto.StreamEvent
			if err := json.Unmarshal(data, &ev); err != nil {
				continue
			}
			if ev.Event == "stdin" {
				out, _ := json.Marshal(agentproto.StreamEvent{Event: "stdout", Data: ev.Data})
				if err := c.Write(ctx, websocket.MessageText, out); err != nil {
					return
				}
				exit, _ := json.Marshal(agentproto.StreamEvent{Event: "exit", Code: 0})
				_ = c.Write(ctx, websocket.MessageText, exit)
				_ = c.Close(websocket.StatusNormalClosure, "done")
				return
			}
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := client.OpenPTY(ctx, "sbx-1", PTYOptions{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("OpenPTY: %v", err)
	}
	defer func() { _ = sess.Close() }()

	if err := sess.SendStdin([]byte("echo hi")); err != nil {
		t.Fatalf("SendStdin: %v", err)
	}

	// 1) echoed stdout
	ev, err := sess.NextEvent()
	if err != nil {
		t.Fatalf("NextEvent #1: %v", err)
	}
	if ev.Event != "stdout" || string(ev.Data) != "echo hi" {
		t.Errorf("event #1 = %+v, want stdout 'echo hi'", ev)
	}

	// 2) exit event delivered normally
	ev, err = sess.NextEvent()
	if err != nil {
		t.Fatalf("NextEvent #2: %v", err)
	}
	if ev.Event != "exit" || ev.Code != 0 {
		t.Errorf("event #2 = %+v, want exit code 0", ev)
	}

	// 3) clean WS close translated to io.EOF
	if _, err := sess.NextEvent(); !errors.Is(err, io.EOF) {
		t.Errorf("NextEvent #3 = %v, want io.EOF", err)
	}
}

func TestOpenPTYBadURLScheme(t *testing.T) {
	c := NewClient("ftp://nope")
	if _, err := c.OpenPTY(context.Background(), "x", PTYOptions{}); err == nil {
		t.Error("expected error for unsupported scheme, got nil")
	}
}
