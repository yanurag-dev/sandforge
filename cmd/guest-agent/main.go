//go:build linux

// Guest agent runs inside the Linux VM. It listens on VSOCK port 2222 and
// handles exec/copyout requests from the host using the agentproto protocol.
//
// Cross-compile from macOS host:
//
//	GOOS=linux GOARCH=amd64 go build -o sandforge-agent ./cmd/guest-agent
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/creack/pty"
	"github.com/mdlayher/vsock"

	"github.com/yanurag-dev/sandforge/pkg/agentproto"
)

const (
	listenPort   uint32        = 2222
	envelopeRead time.Duration = 10 * time.Second
	defaultExec  time.Duration = 30 * time.Second
)

func main() {
	ln, err := vsock.Listen(listenPort, nil)
	if err != nil {
		log.Fatalf("vsock listen port %d: %v", listenPort, err)
	}
	defer func() { _ = ln.Close() }()
	log.Printf("sandforge-agent listening on vsock port %d", listenPort)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			continue
		}
		go handleConn(conn)
	}
}

func handleConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	// Short deadline for reading the envelope only.
	if err := conn.SetReadDeadline(time.Now().Add(envelopeRead)); err != nil {
		log.Printf("setReadDeadline: %v", err)
		return
	}

	var env agentproto.Envelope
	if err := agentproto.ReadEnvelope(conn, &env); err != nil {
		writeResponse(conn, map[string]string{"error": "decode envelope: " + err.Error()})
		return
	}

	// Clear deadline — per-operation handlers set their own.
	if err := conn.SetDeadline(time.Time{}); err != nil {
		log.Printf("clearDeadline: %v", err)
		return
	}

	switch env.Op {
	case "exec":
		handleExec(conn, env.Payload)
	case agentproto.OpPTY:
		handlePTY(conn, env.Payload)
	case "copyout":
		handleCopyOut(conn, env.Payload)
	case "write":
		handleWrite(conn, env.Payload)
	case "list":
		handleList(conn, env.Payload)
	case "stat":
		handleStat(conn, env.Payload)
	default:
		writeResponse(conn, map[string]string{"error": "unknown op: " + env.Op})
	}
}

func handleExec(w io.Writer, raw json.RawMessage) {
	var req agentproto.ExecRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		writeResponse(w, agentproto.ExecResponse{ExitCode: 1, Stderr: "decode exec request: " + err.Error()})
		return
	}
	if len(req.Command) == 0 {
		writeResponse(w, agentproto.ExecResponse{ExitCode: 1, Stderr: "empty command"})
		return
	}

	timeout := defaultExec
	if req.TimeoutSec > 0 {
		timeout = time.Duration(req.TimeoutSec) * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// #nosec G204 - The guest agent's core function is to run arbitrary user commands inside the sandbox VM.
	cmd := exec.CommandContext(ctx, req.Command[0], req.Command[1:]...)

	if req.CWD != "" {
		cmd.Dir = req.CWD
	}

	if len(req.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range req.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Use explicit buffers so stderr is captured even on successful exit.
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	exitCode := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			writeResponse(w, agentproto.ExecResponse{ExitCode: 1, Stderr: err.Error()})
			return
		}
	}

	writeResponse(w, agentproto.ExecResponse{
		ExitCode: exitCode,
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
	})
}

// handlePTY runs an interactive PTY-backed session. It opens a pseudo-terminal,
// starts the requested command (default: an interactive login shell) attached to
// it, and bridges the PTY to the persistent VSOCK connection:
//
//	reader goroutine: conn → PTY  (sole reader of conn, sole writer of PTY)
//	this  goroutine : PTY  → conn (sole reader of PTY, sole writer of conn)
//
// Keeping every conn write in this one goroutine preserves the
// single-writer-per-direction invariant (the exit event goes out the same path
// as stdout, never concurrently with it).
func handlePTY(conn net.Conn, raw json.RawMessage) {
	enc := json.NewEncoder(conn)
	writeEv := func(ev agentproto.StreamEvent) { _ = agentproto.WriteEvent(enc, ev) }

	var req agentproto.PTYStartRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		writeEv(agentproto.StreamEvent{Event: "error", Msg: "decode pty request: " + err.Error()})
		return
	}

	command := req.Command
	if len(command) == 0 {
		command = []string{"/bin/bash", "-i", "-l"}
	}

	// #nosec G204 - launching arbitrary user commands inside the sandbox VM is
	// the guest agent's entire purpose; containment is the VM boundary.
	cmd := exec.Command(command[0], command[1:]...)
	if req.CWD != "" {
		cmd.Dir = req.CWD
	}
	if len(req.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range req.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		writeEv(agentproto.StreamEvent{Event: "error", Msg: "start pty: " + err.Error()})
		return
	}
	defer func() { _ = ptmx.Close() }()

	if req.Cols > 0 || req.Rows > 0 {
		_ = pty.Setsize(ptmx, &pty.Winsize{Cols: req.Cols, Rows: req.Rows})
	}

	// Reader goroutine: drain control events from the host into the PTY.
	go func() {
		dec := json.NewDecoder(conn)
		for {
			var ev agentproto.StreamEvent
			if err := dec.Decode(&ev); err != nil {
				// Host closed the connection — close the PTY master so the
				// child receives SIGHUP and we fall through to Wait below.
				_ = ptmx.Close()
				return
			}
			switch ev.Event {
			case "stdin":
				_, _ = ptmx.Write(ev.Data)
			case "resize":
				_ = pty.Setsize(ptmx, &pty.Winsize{Cols: ev.Cols, Rows: ev.Rows})
			}
		}
	}()

	// Pump PTY output to the host until the slave closes (child exited).
	buf := make([]byte, 32*1024)
	for {
		n, err := ptmx.Read(buf)
		if n > 0 {
			writeEv(agentproto.StreamEvent{Event: "stdout", Data: append([]byte(nil), buf[:n]...)})
		}
		if err != nil {
			break
		}
	}

	// Reap the child and report its exit code.
	exitCode := 0
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	writeEv(agentproto.StreamEvent{Event: "exit", Code: exitCode})
}

func handleCopyOut(w io.Writer, raw json.RawMessage) {
	var req agentproto.CopyOutRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		writeResponse(w, agentproto.CopyOutResponse{Error: "decode copyout request: " + err.Error()})
		return
	}
	if req.GuestPath == "" {
		writeResponse(w, agentproto.CopyOutResponse{Error: "guest_path is required"})
		return
	}

	data, err := os.ReadFile(req.GuestPath)
	if err != nil {
		writeResponse(w, agentproto.CopyOutResponse{Error: err.Error()})
		return
	}

	writeResponse(w, agentproto.CopyOutResponse{Data: data})
}

func handleWrite(w io.Writer, raw json.RawMessage) {
	var req agentproto.WriteFileRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		writeResponse(w, agentproto.WriteFileResponse{Error: "decode write request: " + err.Error()})
		return
	}
	if req.GuestPath == "" {
		writeResponse(w, agentproto.WriteFileResponse{Error: "guest_path is required"})
		return
	}

	if err := os.MkdirAll(filepath.Dir(req.GuestPath), 0o750); err != nil {
		writeResponse(w, agentproto.WriteFileResponse{Error: "mkdir: " + err.Error()})
		return
	}
	if err := os.WriteFile(req.GuestPath, req.Data, 0o600); err != nil {
		writeResponse(w, agentproto.WriteFileResponse{Error: err.Error()})
		return
	}
	writeResponse(w, agentproto.WriteFileResponse{Size: len(req.Data)})
}

func handleList(w io.Writer, raw json.RawMessage) {
	var req agentproto.ListRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		writeResponse(w, agentproto.ListResponse{Error: "decode list request: " + err.Error()})
		return
	}
	if req.GuestPath == "" {
		writeResponse(w, agentproto.ListResponse{Error: "guest_path is required"})
		return
	}

	entries, err := os.ReadDir(req.GuestPath)
	if err != nil {
		writeResponse(w, agentproto.ListResponse{Error: err.Error()})
		return
	}

	result := make([]agentproto.DirEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		result = append(result, agentproto.DirEntry{
			Name:    e.Name(),
			Size:    info.Size(),
			Mode:    uint32(info.Mode()),
			IsDir:   e.IsDir(),
			ModTime: info.ModTime().Unix(),
		})
	}
	writeResponse(w, agentproto.ListResponse{Entries: result})
}

func handleStat(w io.Writer, raw json.RawMessage) {
	var req agentproto.StatRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		writeResponse(w, agentproto.StatResponse{Error: "decode stat request: " + err.Error()})
		return
	}
	if req.GuestPath == "" {
		writeResponse(w, agentproto.StatResponse{Error: "guest_path is required"})
		return
	}

	info, err := os.Stat(req.GuestPath)
	if err != nil {
		writeResponse(w, agentproto.StatResponse{Error: err.Error()})
		return
	}
	writeResponse(w, agentproto.StatResponse{
		Name:    info.Name(),
		Size:    info.Size(),
		Mode:    uint32(info.Mode()),
		IsDir:   info.IsDir(),
		ModTime: info.ModTime().Unix(),
	})
}

func writeResponse(w io.Writer, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeResponse: %v", err)
	}
}
