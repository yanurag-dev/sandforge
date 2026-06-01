// Command ptysmoke is a manual end-to-end smoke test for interactive PTY
// sessions against a REAL macOS VZ guest VM. It boots the control plane in
// process (so it can enable the AllowInteractive policy), creates a sandbox,
// opens a PTY over the WebSocket SDK, drives a real interactive shell by
// SENDING STDIN mid-session, and asserts the streamed output and exit code.
//
//	go run ./cmd/ptysmoke   (must be built + codesigned for VZ entitlements)
//
// Not part of the test suite — it needs a booted VM and built images.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	client "github.com/yanurag-dev/sandforge/sdks/go"

	"github.com/yanurag-dev/sandforge/internal/backend"
	"github.com/yanurag-dev/sandforge/internal/controlplane"
	"github.com/yanurag-dev/sandforge/internal/policy"
	"github.com/yanurag-dev/sandforge/internal/supervisor"
	"github.com/yanurag-dev/sandforge/pkg/api"
)

const addr = "127.0.0.1:8099"

func main() {
	engine := &policy.Engine{
		MaxCPU:              8,
		MaxMemoryMb:         8192,
		MaxDiskGb:           64,
		AllowedNetworkModes: []string{"offline", "fetch"},
		AllowInteractive:    true,
	}
	sup, err := supervisor.NewSupervisor(backend.New(), engine)
	if err != nil {
		log.Fatalf("supervisor: %v", err)
	}
	srv, err := controlplane.NewServerWithAddr(sup, addr)
	if err != nil {
		log.Fatalf("server: %v", err)
	}
	go func() { _ = srv.Start() }()
	time.Sleep(500 * time.Millisecond)

	c := client.NewClient("http://" + addr)
	ctx := context.Background()

	log.Println("creating sandbox (booting VM)...")
	sb, err := c.CreateSandbox(ctx, api.SandboxSpec{CPU: 2, MemoryMb: 2048, NetworkMode: "offline"})
	if err != nil {
		log.Fatalf("create sandbox: %v", err)
	}
	log.Printf("sandbox %s created", sb.ID)
	defer func() { _ = c.Destroy(ctx, sb.ID) }()

	// Open an INTERACTIVE busybox shell (Alpine has no bash). We drive it by
	// sending stdin lines mid-session — the real full-duplex test.
	log.Println("opening interactive PTY session...")
	pctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	sess, err := c.OpenPTY(pctx, sb.ID, client.PTYOptions{
		Cols:    80,
		Rows:    24,
		Command: []string{"/bin/sh", "-i"},
	})
	if err != nil {
		log.Fatalf("open pty: %v", err)
	}
	defer func() { _ = sess.Close() }()

	// Reader goroutine: accumulate everything the shell emits.
	var out strings.Builder
	exitCode := -1
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			ev, err := sess.NextEvent()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				log.Printf("next event: %v", err)
				return
			}
			switch ev.Event {
			case "stdout":
				out.Write(ev.Data)
			case "exit":
				exitCode = ev.Code
			case "error":
				log.Printf("guest error: %s", ev.Msg)
			}
		}
	}()

	// Type commands into the live shell, with small pauses so the shell
	// processes each before we send the next (mimicking a human).
	send := func(line string) {
		if err := sess.SendStdin([]byte(line)); err != nil {
			log.Fatalf("send stdin: %v", err)
		}
		time.Sleep(400 * time.Millisecond)
	}
	send("echo interactive-stdin-works\n")
	send("uname -s\n") // expect "Linux"
	send("exit 0\n")

	<-done

	got := out.String()
	fmt.Printf("\n--- shell session output ---\n%s\n----------------------------\n", got)
	fmt.Printf("exit code: %d\n", exitCode)

	checks := []struct {
		desc string
		ok   bool
	}{
		{"stdin echoed/executed (interactive-stdin-works)", strings.Contains(got, "interactive-stdin-works")},
		{"uname ran in guest (Linux)", strings.Contains(got, "Linux")},
		{"clean exit code 0", exitCode == 0},
	}
	allOK := true
	for _, c := range checks {
		mark := "✅"
		if !c.ok {
			mark, allOK = "❌", false
		}
		fmt.Printf("%s %s\n", mark, c.desc)
	}
	if !allOK {
		log.Fatal("FAIL: one or more interactive PTY checks failed")
	}
	fmt.Println("\n✅ PASS: interactive PTY over real VM — mid-session stdin, live output, clean exit")
}
