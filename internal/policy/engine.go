package policy

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/sandforge/sandforge/pkg/api"
)

var (
	ErrForbiddenHostPath = errors.New("requested host path is forbidden by policy")
	ErrPathNotAbs        = errors.New("host path must be an absolute path")
)

type Engine struct {
	AllowedHostPrefixes []string
	BlockedHostPatterns []string
}

func (e *Engine) EvaluateMount(mount api.WorkspaceMount) error {
	path := filepath.Clean(mount.HostPath)

	if !filepath.IsAbs(path) {
		return ErrPathNotAbs
	}

	allowed := false
	for _, prefix := range e.AllowedHostPrefixes {
		p := filepath.Clean(prefix)
		if path == p || strings.HasPrefix(path, p+string(filepath.Separator)) {
			allowed = true
			break
		}
	}

	if !allowed {
		return ErrForbiddenHostPath
	}

	for _, pattern := range e.BlockedHostPatterns {
		if strings.Contains(path, pattern) {
			return ErrForbiddenHostPath
		}
	}
	return nil
}
