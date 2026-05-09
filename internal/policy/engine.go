package policy

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/sandforge/sandforge/pkg/api"
)

var (
	ErrForbiddenHostPath     = errors.New("requested host path is forbidden by policy")
	ErrPathNotAbs            = errors.New("host path must be an absolute path")
	ErrResourceLimitExceeded = errors.New("requested resource exceeds policy limits")
	ErrInvalidNetworkMode    = errors.New("requested network mode is not allowed")
	ErrForbiddenCommand      = errors.New("command is not allowed by policy")
)

type Engine struct {
	AllowedHostPrefixes []string
	BlockedHostPatterns []string
	MaxCPU              int
	MaxMemoryMb         int
	MaxDiskGb           int
	AllowedNetworkModes []string
	AllowedCommands     []string
}

func (e *Engine) EvaluateMount(mount api.WorkspaceMount) error {
	path := filepath.Clean(mount.HostPath)

	if !filepath.IsAbs(path) {
		return ErrPathNotAbs
	}

	// Resolve symlinks to prevent bypasses (e.g., a symlink pointing to /etc)
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err // Path must exist to be validated
	}
	path = resolved

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
