package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/coder/websocket"

	"github.com/yanurag-dev/sandforge/internal/supervisor"
	"github.com/yanurag-dev/sandforge/pkg/agentproto"
	"github.com/yanurag-dev/sandforge/pkg/api"
)

const (
	defaultAddr       = ":8080"
	maxWriteBodyBytes = 32 * 1024 * 1024 // 32 MiB
)

type Server struct {
	supervisor *supervisor.Supervisor
	addr       string
	httpServer *http.Server
}

func NewServer(sup *supervisor.Supervisor) (*Server, error) {
	return NewServerWithAddr(sup, defaultAddr)
}

func NewServerWithAddr(sup *supervisor.Supervisor, addr string) (*Server, error) {
	if sup == nil {
		return nil, fmt.Errorf("supervisor must not be nil")
	}
	return &Server{supervisor: sup, addr: addr}, nil
}

// handler builds the request multiplexer. Split out from Start so tests can
// mount it on an httptest.Server without the blocking signal-wait loop.
func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/sandboxes", s.handleCreate)
	mux.HandleFunc("POST /v1/sandboxes/{id}/exec", s.handleExec)
	mux.HandleFunc("GET /v1/sandboxes/{id}/pty", s.handlePTY)
	mux.HandleFunc("DELETE /v1/sandboxes/{id}", s.handleDestroy)
	mux.HandleFunc("GET /v1/sandboxes/{id}", s.handleStatus)
	mux.HandleFunc("PUT /v1/sandboxes/{id}/files", s.handleWriteFile)
	mux.HandleFunc("GET /v1/sandboxes/{id}/files", s.handleListDir)
	mux.HandleFunc("GET /v1/sandboxes/{id}/files/read", s.handleReadFile)
	mux.HandleFunc("GET /v1/sandboxes/{id}/stat", s.handleStat)
	mux.HandleFunc("GET /healthz", s.handleHealth)
	return mux
}

func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:         s.addr,
		Handler:      s.handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.addr, err)
	}

	log.Printf("control plane listening on %s", s.addr)

	errCh := make(chan error, 1)
	go func() {
		if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Printf("received %s, shutting down...", sig)
	case err := <-errCh:
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

// ── handlers ──────────────────────────────────────────────────────────────

type createRequest struct {
	ID   string          `json:"id"`
	Spec api.SandboxSpec `json:"spec"`
}

type createResponse struct {
	ID string `json:"id"`
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.ID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if err := s.supervisor.Start(req.ID, req.Spec); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, createResponse{ID: req.ID})
}

type execHTTPRequest struct {
	Command    []string          `json:"command"`
	CWD        string            `json:"cwd"`
	Env        map[string]string `json:"env"`
	TimeoutSec int               `json:"timeout_sec"`
}

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req execHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	result, err := s.supervisor.RunCommand(id, api.ExecRequest{
		Command:    req.Command,
		CWD:        req.CWD,
		Env:        req.Env,
		TimeoutSec: req.TimeoutSec,
	})
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ptyQueryParams reads optional terminal size and command from the WS upgrade
// request's query string (e.g. ?cols=120&rows=40). Size defaults to 80x24.
func ptyQueryParams(r *http.Request) agentproto.PTYStartRequest {
	req := agentproto.PTYStartRequest{Cols: 80, Rows: 24}
	if v, err := strconv.ParseUint(r.URL.Query().Get("cols"), 10, 16); err == nil && v > 0 {
		req.Cols = uint16(v)
	}
	if v, err := strconv.ParseUint(r.URL.Query().Get("rows"), 10, 16); err == nil && v > 0 {
		req.Rows = uint16(v)
	}
	if cmd := r.URL.Query()["cmd"]; len(cmd) > 0 {
		req.Command = cmd
	}
	return req
}

// handlePTY upgrades the request to a WebSocket and bridges it to an interactive
// guest PTY session. This is the outward (TCP) face of the two-hop transport:
//
//	client ⇄ (this WebSocket) ⇄ host ⇄ (VSOCK PTYSession) ⇄ guest agent
//
// Two goroutines pump bytes, each the sole reader of one side and sole writer of
// the other, preserving single-writer-per-direction on both hops. A shared
// cancellable context ties them together: when either exits it cancels the
// context, unblocking the other's Read/Write.
func (s *Server) handlePTY(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Disable the server's 60s WriteTimeout for this hijacked connection — an
	// interactive session is long-lived and must not be killed mid-stream.
	if rc := http.NewResponseController(w); rc != nil {
		_ = rc.SetWriteDeadline(time.Time{})
		_ = rc.SetReadDeadline(time.Time{})
	}

	// Start the guest session BEFORE upgrading, so policy/state errors surface
	// as a normal HTTP error rather than a WebSocket close the client must decode.
	sess, err := s.supervisor.StartPTY(id, ptyQueryParams(r))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	defer func() { _ = sess.Close() }()

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Printf("pty: websocket accept: %v", err)
		return
	}
	defer func() { _ = conn.CloseNow() }()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// WS → backend: sole WS reader, sole SendStdin/Resize caller.
	go func() {
		defer cancel()
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var ev agentproto.StreamEvent
			if err := json.Unmarshal(data, &ev); err != nil {
				continue // ignore malformed client frames
			}
			switch ev.Event {
			case "stdin":
				if err := sess.SendStdin(ev.Data); err != nil {
					return
				}
			case "resize":
				_ = sess.Resize(ev.Cols, ev.Rows)
			}
		}
	}()

	// backend → WS: sole NextEvent caller, sole WS writer. Runs on this
	// goroutine so the handler blocks until the session ends.
	for {
		ev, err := sess.NextEvent()
		if err != nil {
			return // io.EOF (clean) or transport error — session over
		}
		out, err := json.Marshal(ev)
		if err != nil {
			continue
		}
		if err := conn.Write(ctx, websocket.MessageText, out); err != nil {
			return
		}
		if ev.Event == "exit" {
			// Tell the client we're done, then let defers tear down.
			_ = conn.Close(websocket.StatusNormalClosure, "session ended")
			return
		}
	}
}

func (s *Server) handleDestroy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.supervisor.Stop(id); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type statusResponse struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	state, err := s.supervisor.GetState(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, statusResponse{ID: id, State: string(state)})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type writeFileRequest struct {
	GuestPath string `json:"guest_path"`
	Data      []byte `json:"data"`
}

type writeFileResponse struct {
	Size int `json:"size"`
}

func (s *Server) handleWriteFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	r.Body = http.MaxBytesReader(w, r.Body, maxWriteBodyBytes)
	var req writeFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.GuestPath == "" {
		writeError(w, http.StatusBadRequest, "guest_path is required")
		return
	}
	size, err := s.supervisor.WriteFile(id, req.GuestPath, req.Data)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, writeFileResponse{Size: size})
}

type readFileResponse struct {
	Data []byte `json:"data"`
}

func (s *Server) handleReadFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	guestPath := r.URL.Query().Get("path")
	if guestPath == "" {
		writeError(w, http.StatusBadRequest, "path query param is required")
		return
	}
	data, err := s.supervisor.ReadFile(id, guestPath)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, readFileResponse{Data: data})
}

func (s *Server) handleListDir(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	guestPath := r.URL.Query().Get("path")
	if guestPath == "" {
		writeError(w, http.StatusBadRequest, "path query param is required")
		return
	}
	entries, err := s.supervisor.ListDir(id, guestPath)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (s *Server) handleStat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	guestPath := r.URL.Query().Get("path")
	if guestPath == "" {
		writeError(w, http.StatusBadRequest, "path query param is required")
		return
	}
	info, err := s.supervisor.StatPath(id, guestPath)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// ── helpers ───────────────────────────────────────────────────────────────

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, errorResponse{Error: msg})
}
