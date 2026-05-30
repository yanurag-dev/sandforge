//go:build !darwin

package backend

import "github.com/yanurag-dev/sandforge/pkg/api"

func New() api.SandboxBackend {
	return NewMockBackend()
}
