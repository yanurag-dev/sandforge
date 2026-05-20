//go:build linux

// Guest agent runs inside the Linux VM. It listens on VSOCK port 2222 and
// handles exec/copyout requests from the host using the agentproto protocol.
//
// Cross-compile from macOS host:
//
//	GOOS=linux GOARCH=amd64 go build -o sandforge-agent ./cmd/guest-agent
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/mdlayher/vsock"
	"github.com/sandforge/sandforge/pkg/agentproto"
)

const listenPort uint32 = 2222

func main() {
	ln, err := vsock.Listen(listenPort, nil)
	if err != nil {
		log.Fatalf("vsock listen port %d: %v", listenPort, err)
	}
	defer ln.Close()
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
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	var env agentproto.Envelope
	if err := agentproto.ReadEnvelope(conn, &env); err != nil {
		writeResponse(conn, map[string]string{"error": "decode envelope: " + err.Error()})
		return
	}

	switch env.Op {
	case "exec":
		handleExec(conn, env.Payload)
	case "copyout":
		handleCopyOut(conn, env.Payload)
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

	timeout := 30 * time.Second
	if req.TimeoutSec > 0 {
		timeout = time.Duration(req.TimeoutSec) * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

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

	stdout, err := cmd.Output()
	var stderr []byte
	exitCode := 0

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			stderr = exitErr.Stderr
		} else {
			writeResponse(w, agentproto.ExecResponse{ExitCode: 1, Stderr: err.Error()})
			return
		}
	}

	writeResponse(w, agentproto.ExecResponse{
		ExitCode: exitCode,
		Stdout:   string(stdout),
		Stderr:   string(stderr),
	})
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

func writeResponse(w io.Writer, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeResponse: %v", err)
	}
}
