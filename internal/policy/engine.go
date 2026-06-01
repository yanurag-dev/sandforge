package policy

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/yanurag-dev/sandforge/pkg/api"
)

var (
	ErrForbiddenHostPath     = errors.New("requested host path is forbidden by policy")
	ErrPathNotAbs            = errors.New("host path must be an absolute path")
	ErrResourceLimitExceeded = errors.New("requested resource exceeds policy limits")
	ErrInvalidNetworkMode    = errors.New("requested network mode is not allowed")
	ErrForbiddenCommand      = errors.New("command is not allowed by policy")
	ErrInteractiveDisabled   = errors.New("interactive PTY sessions are not allowed by policy")
)

type Engine struct {
	AllowedHostPrefixes []string
	BlockedHostPatterns []string
	MaxCPU              int
	MaxMemoryMb         int
	MaxDiskGb           int
	AllowedNetworkModes []string
	AllowedCommands     []string
	// AllowInteractive opts in to interactive PTY sessions. It defaults to
	// false because a PTY hands the caller a live shell, which can spawn any
	// program — the AllowedCommands allowlist (which only matches Command[0])
	// cannot constrain it. For interactive sessions the VM itself is the
	// containment boundary, so enabling this must be a deliberate choice.
	AllowInteractive bool
}

func (e *Engine) EvaluateMount(mount api.WorkspaceMount) error {
	return e.EvaluateHostPath(mount.HostPath)
}

func (e *Engine) EvaluateHostPath(path string) error {
	path = filepath.Clean(path)

	if !filepath.IsAbs(path) {
		return ErrPathNotAbs
	}

	// Resolve symlinks to prevent bypasses (e.g., a symlink pointing to /etc)
	// For CopyOut, the file might not exist yet, so we resolve the parent directory if the path itself doesn't exist.
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		// If path doesn't exist, try resolving the directory
		dir := filepath.Dir(path)
		resolvedDir, errDir := filepath.EvalSymlinks(dir)
		if errDir != nil {
			return errDir
		}
		path = filepath.Join(resolvedDir, filepath.Base(path))
	} else {
		path = resolved
	}

	allowed := false
	for _, prefix := range e.AllowedHostPrefixes {
		p, err := filepath.EvalSymlinks(filepath.Clean(prefix))
		if err != nil {
			continue // Skip invalid prefixes
		}
		if path == p || strings.HasPrefix(path, p+string(filepath.Separator)) {
			allowed = true
			break
		}
	}

	if !allowed {
		return ErrForbiddenHostPath
	}

	// Precise segment matching for blocklist to avoid false positives
	segments := strings.Split(path, string(filepath.Separator))
	for _, pattern := range e.BlockedHostPatterns {
		for _, segment := range segments {
			if segment == pattern {
				return ErrForbiddenHostPath
			}
		}
	}
	return nil
}

func (e *Engine) EvaluateSandbox(spec api.SandboxSpec) error {
	if spec.CPU > e.MaxCPU {
		return ErrResourceLimitExceeded
	}
	if spec.MemoryMb > e.MaxMemoryMb {
		return ErrResourceLimitExceeded
	}
	if spec.DiskGb > e.MaxDiskGb {
		return ErrResourceLimitExceeded
	}

	allowed := false
	for _, mode := range e.AllowedNetworkModes {
		if spec.NetworkMode == mode {
			allowed = true
			break
		}
	}

	if !allowed {
		return ErrInvalidNetworkMode
	}
	return nil
}

func (e *Engine) EvaluateExec(req api.ExecRequest) error {
	if len(req.Command) == 0 {
		return errors.New("no command provided")
	}

	binary := req.Command[0]
	allowed := false
	for _, command := range e.AllowedCommands {
		if binary == command {
			allowed = true
			break
		}
	}

	if !allowed {
		return ErrForbiddenCommand
	}
	return nil
}

// EvaluatePTY decides whether an interactive PTY session may start. Unlike
// EvaluateExec there is no command allowlist to apply — an interactive shell
// can run anything — so the gate is the single AllowInteractive opt-in.
func (e *Engine) EvaluatePTY() error {
	if !e.AllowInteractive {
		return ErrInteractiveDisabled
	}
	return nil
}
