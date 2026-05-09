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
