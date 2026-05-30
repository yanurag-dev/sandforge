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
	"syscall"
	"time"

	"github.com/yanurag-dev/sandforge/internal/supervisor"
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

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/sandboxes", s.handleCreate)
	mux.HandleFunc("POST /v1/sandboxes/{id}/exec", s.handleExec)
	mux.HandleFunc("DELETE /v1/sandboxes/{id}", s.handleDestroy)
	mux.HandleFunc("GET /v1/sandboxes/{id}", s.handleStatus)
	mux.HandleFunc("PUT /v1/sandboxes/{id}/files", s.handleWriteFile)
	mux.HandleFunc("GET /v1/sandboxes/{id}/files", s.handleListDir)
	mux.HandleFunc("GET /v1/sandboxes/{id}/stat", s.handleStat)
	mux.HandleFunc("GET /healthz", s.handleHealth)

	s.httpServer = &http.Server{
		Addr:         s.addr,
		Handler:      mux,
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
