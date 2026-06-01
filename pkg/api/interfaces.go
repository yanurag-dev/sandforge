package api

import "github.com/yanurag-dev/sandforge/pkg/agentproto"

// From ARCHITECTURE.md Section 15

type SandboxSpec struct {
	Backend       string           `json:"backend"` // "linux-kvm", "linux-firecracker", "macos-vz"
	CPU           int              `json:"cpu"`
	MemoryMb      int              `json:"memory_mb"`
	DiskGb        int              `json:"disk_gb"`
	TimeoutSec    int              `json:"timeout_sec"`
	NetworkMode   string           `json:"network_mode"`   // "offline", "fetch", "full"
	TaskIsolation string           `json:"task_isolation"` // "container", "process"
	Mounts        []WorkspaceMount `json:"mounts,omitempty"`
}

type WorkspaceMount struct {
	HostPath  string `json:"host_path"`
	GuestPath string `json:"guest_path"`
	ReadOnly  bool   `json:"read_only"`
}

type ExecRequest struct {
	Command    []string          `json:"command"`
	CWD        string            `json:"cwd"`
	Env        map[string]string `json:"env"`
	TimeoutSec int               `json:"timeout_sec"`
}

type ExecResult struct {
	ExitCode  int      `json:"exit_code"`
	Stdout    string   `json:"stdout"`
	Stderr    string   `json:"stderr"`
	Artifacts []string `json:"artifacts,omitempty"`
}

type SandboxBackend interface {
	CreateSandbox(spec SandboxSpec) (string, error)
	MountWorkspace(handle string, mount WorkspaceMount) error
	Exec(handle string, req ExecRequest) (ExecResult, error)
	CopyOut(handle string, path string, dest string) error
	ReadFile(handle string, guestPath string) ([]byte, error)
	WriteFile(handle string, guestPath string, data []byte) (int, error)
	ListDir(handle string, guestPath string) ([]DirEntry, error)
	StatPath(handle string, guestPath string) (StatInfo, error)
	DestroySandbox(handle string) error
}

// PTYSession is a live interactive terminal session backed by a PTY inside the
// guest. It bridges a persistent guest connection to a client (e.g. a WebSocket
// handler). Callers must observe a single-writer-per-direction discipline:
// SendStdin/Resize from one goroutine, NextEvent from another. The underlying
// connection tolerates one concurrent reader and one concurrent writer, but not
// two concurrent writers.
type PTYSession interface {
	// SendStdin forwards keystrokes/input to the PTY master.
	SendStdin(data []byte) error
	// Resize updates the terminal window size (triggers SIGWINCH in the guest).
	Resize(cols, rows uint16) error
	// NextEvent returns the next event from the guest (stdout/exit/error). It
	// returns the {event:"exit"} event normally, then io.EOF on the following
	// call once the session has fully ended — mirroring the io.Reader
	// convention so callers loop until errors.Is(err, io.EOF).
	NextEvent() (agentproto.StreamEvent, error)
	// Close tears down the session: closes the connection, which signals the
	// guest to terminate the PTY child and reap it.
	Close() error
}

// PTYBackend is an OPTIONAL capability a SandboxBackend may implement to support
// interactive PTY sessions. Callers type-assert a SandboxBackend to PTYBackend
// and degrade gracefully when it is absent (e.g. backends without PTY support).
type PTYBackend interface {
	StartPTY(handle string, req agentproto.PTYStartRequest) (PTYSession, error)
}

// DirEntry is a directory entry returned by ListDir.
type DirEntry struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Mode    uint32 `json:"mode"`
	IsDir   bool   `json:"is_dir"`
	ModTime int64  `json:"mod_time"`
}

// StatInfo is file/directory metadata returned by StatPath.
type StatInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Mode    uint32 `json:"mode"`
	IsDir   bool   `json:"is_dir"`
	ModTime int64  `json:"mod_time"`
}
