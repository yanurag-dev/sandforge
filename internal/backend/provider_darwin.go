//go:build darwin

package backend

import (
	"os"

	"github.com/yanurag-dev/sandforge/internal/backend/vz"
	"github.com/yanurag-dev/sandforge/pkg/api"
)

func New() api.SandboxBackend {
	kernel := envOr("SANDFORGE_KERNEL_PATH", "./images/vmlinuz")
	initrd := envOr("SANDFORGE_INITRD_PATH", "./images/initrd.img")
	return vz.NewVZBackendWithImages(kernel, initrd)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
