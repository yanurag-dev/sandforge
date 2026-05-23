package api

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
	DestroySandbox(handle string) error
}
