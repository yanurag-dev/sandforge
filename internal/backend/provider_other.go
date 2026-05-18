//go:build !darwin

package backend

import "github.com/sandforge/sandforge/pkg/api"

func New() api.SandboxBackend {
	return NewMockBackend()
}
