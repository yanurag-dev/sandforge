//go:build darwin

package vz

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/Code-Hex/vz/v3"
	"github.com/sandforge/sandforge/pkg/api"
)

// guestAgentPort is the VSOCK port the in-guest agent listens on.
const guestAgentPort uint32 = 2222

// execRequest is the JSON payload sent to the guest agent for command execution.
type execRequest struct {
	Command    []string          `json:"command"`
	CWD        string            `json:"cwd"`
	Env        map[string]string `json:"env"`
	TimeoutSec int               `json:"timeout_sec"`
}

// execResponse is the JSON payload returned by the guest agent.
type execResponse struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

// copyOutRequest asks the guest agent to send a file's contents.
type copyOutRequest struct {
	GuestPath string `json:"guest_path"`
}

// copyOutResponse carries the (base64-encoded) file bytes from the guest.
type copyOutResponse struct {
	Data  []byte `json:"data"`
	Error string `json:"error,omitempty"`
}

// sandboxEntry bundles the VM with its socket device so we can dial VSOCK later.
type sandboxEntry struct {
	vm     *vz.VirtualMachine
	socket *vz.VirtioSocketDevice
}

// VZBackend implements api.SandboxBackend using Apple Virtualization Framework.
type VZBackend struct {
	mu         sync.RWMutex
	sandboxes  map[string]*sandboxEntry
	kernelPath string
	initrdPath string
}

// NewVZBackend creates a backend using default image paths (./images/).
func NewVZBackend() *VZBackend {
	return NewVZBackendWithImages("./images/vmlinuz", "./images/initrd.img")
}

// NewVZBackendWithImages creates a backend with explicit kernel and initrd paths.
func NewVZBackendWithImages(kernelPath, initrdPath string) *VZBackend {
	return &VZBackend{
		sandboxes:  make(map[string]*sandboxEntry),
		kernelPath: kernelPath,
		initrdPath: initrdPath,
	}
}

func (v *VZBackend) CreateSandbox(spec api.SandboxSpec) (string, error) {
	return v.CreateSandboxWithMounts(spec, nil)
}

func (v *VZBackend) CreateSandboxWithMounts(spec api.SandboxSpec, mounts []api.WorkspaceMount) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	kernelPath := v.kernelPath
	initrdPath := v.initrdPath

	if _, err := os.Stat(kernelPath); err != nil {
		return "", fmt.Errorf("kernel not found at %s: %w", kernelPath, err)
	}
	if _, err := os.Stat(initrdPath); err != nil {
		return "", fmt.Errorf("initrd not found at %s: %w", initrdPath, err)
	}

	bootLoader, err := vz.NewLinuxBootLoader(
		kernelPath,
		vz.WithCommandLine("console=hvc0 root=/dev/ram0"),
		vz.WithInitrd(initrdPath),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create bootloader: %w", err)
	}

	attachment, err := vz.NewFileHandleSerialPortAttachment(os.Stdin, os.Stdout)
	if err != nil {
		return "", fmt.Errorf("failed to create serial attachment: %w", err)
	}
	serial, err := vz.NewVirtioConsoleDeviceSerialPortConfiguration(attachment)
	if err != nil {
		return "", fmt.Errorf("failed to create serial port: %w", err)
	}

	entropy, err := vz.NewVirtioEntropyDeviceConfiguration()
	if err != nil {
		return "", fmt.Errorf("failed to create entropy device: %w", err)
	}

	balloon, err := vz.NewVirtioTraditionalMemoryBalloonDeviceConfiguration()
	if err != nil {
		return "", fmt.Errorf("failed to create balloon device: %w", err)
	}

	vsockCfg, err := vz.NewVirtioSocketDeviceConfiguration()
	if err != nil {
		return "", fmt.Errorf("failed to create vsock device: %w", err)
	}

	var fsConfigs []vz.DirectorySharingDeviceConfiguration
	for i, m := range mounts {
		tag := fmt.Sprintf("mount%d", i)
		fsCfg, err := vz.NewVirtioFileSystemDeviceConfiguration(tag)
		if err != nil {
			return "", fmt.Errorf("failed to create fs config for %s: %w", m.HostPath, err)
		}
		dir, err := vz.NewSharedDirectory(m.HostPath, m.ReadOnly)
		if err != nil {
			return "", fmt.Errorf("failed to create shared directory for %s: %w", m.HostPath, err)
		}
		share, err := vz.NewSingleDirectoryShare(dir)
		if err != nil {
			return "", fmt.Errorf("failed to create directory share for %s: %w", m.HostPath, err)
		}
		fsCfg.SetDirectoryShare(share)
		fsConfigs = append(fsConfigs, fsCfg)
	}

	config, err := vz.NewVirtualMachineConfiguration(
		bootLoader,
		uint(spec.CPU),
		uint64(spec.MemoryMb),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create VM config: %w", err)
	}

	config.SetSerialPortsVirtualMachineConfiguration([]*vz.VirtioConsoleDeviceSerialPortConfiguration{serial})
	config.SetEntropyDevicesVirtualMachineConfiguration([]*vz.VirtioEntropyDeviceConfiguration{entropy})
	config.SetMemoryBalloonDevicesVirtualMachineConfiguration([]vz.MemoryBalloonDeviceConfiguration{balloon})
	config.SetSocketDevicesVirtualMachineConfiguration([]vz.SocketDeviceConfiguration{vsockCfg})
	if len(fsConfigs) > 0 {
		config.SetDirectorySharingDevicesVirtualMachineConfiguration(fsConfigs)
	}

	valid, err := config.Validate()
	if !valid || err != nil {
		return "", fmt.Errorf("invalid VM configuration: %w", err)
	}

	vm, err := vz.NewVirtualMachine(config)
	if err != nil {
		return "", fmt.Errorf("failed to initialize VM: %w", err)
	}

	if err := vm.Start(); err != nil {
		return "", fmt.Errorf("failed to start VM: %w", err)
	}

	// Retrieve the live socket device so we can dial VSOCK connections later.
	socketDevices := vm.SocketDevices()
	if len(socketDevices) == 0 {
		_ = vm.Stop()
		return "", fmt.Errorf("VM started but no socket device available")
	}

	handle := fmt.Sprintf("vz-%p", vm)
	v.sandboxes[handle] = &sandboxEntry{vm: vm, socket: socketDevices[0]}

	return handle, nil
}

func (v *VZBackend) MountWorkspace(handle string, mount api.WorkspaceMount) error {
	v.mu.RLock()
	_, exists := v.sandboxes[handle]
	v.mu.RUnlock()

	if !exists {
		return fmt.Errorf("sandbox handle not found: %s", handle)
	}

	// Virtio-fs devices cannot be hot-plugged after boot with Apple VZ.
	// Mounts must be provided to CreateSandboxWithMounts before VM start.
	fmt.Printf("VZBackend: hot-plug not supported; mount %s→%s (RO:%v) must be pre-configured\n",
		mount.HostPath, mount.GuestPath, mount.ReadOnly)
	return nil
}

// Exec sends an execRequest JSON message to the in-guest agent over VSOCK and
// returns the decoded execResponse. The guest agent must be listening on
// guestAgentPort and speak the same length-prefixed JSON protocol.
func (v *VZBackend) Exec(handle string, req api.ExecRequest) (api.ExecResult, error) {
	conn, err := v.dialGuest(handle)
	if err != nil {
		return api.ExecResult{}, fmt.Errorf("exec: dial guest: %w", err)
	}
	defer conn.Close()

	payload := execRequest{
		Command:    req.Command,
		CWD:        req.CWD,
		Env:        req.Env,
		TimeoutSec: req.TimeoutSec,
	}

	if err := writeJSON(conn, "exec", payload); err != nil {
		return api.ExecResult{}, fmt.Errorf("exec: write request: %w", err)
	}

	var resp execResponse
	if err := readJSON(conn, &resp); err != nil {
		return api.ExecResult{}, fmt.Errorf("exec: read response: %w", err)
	}

	return api.ExecResult{
		ExitCode: resp.ExitCode,
		Stdout:   resp.Stdout,
		Stderr:   resp.Stderr,
	}, nil
}

// CopyOut retrieves a file from the guest at guestPath and writes it to dest
// on the host. The guest agent streams the file contents over VSOCK.
func (v *VZBackend) CopyOut(handle string, guestPath string, dest string) error {
	conn, err := v.dialGuest(handle)
	if err != nil {
		return fmt.Errorf("copyout: dial guest: %w", err)
	}
	defer conn.Close()

	payload := copyOutRequest{GuestPath: guestPath}
	if err := writeJSON(conn, "copyout", payload); err != nil {
		return fmt.Errorf("copyout: write request: %w", err)
	}

	var resp copyOutResponse
	if err := readJSON(conn, &resp); err != nil {
		return fmt.Errorf("copyout: read response: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("copyout: guest error: %s", resp.Error)
	}

	if err := os.WriteFile(dest, resp.Data, 0o644); err != nil {
		return fmt.Errorf("copyout: write dest %s: %w", dest, err)
	}
	return nil
}

func (v *VZBackend) DestroySandbox(handle string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	entry, exists := v.sandboxes[handle]
	if !exists {
		return fmt.Errorf("sandbox handle not found: %s", handle)
	}

	if entry.vm.CanStop() {
		if _, err := entry.vm.RequestStop(); err != nil {
			fmt.Printf("Warning: failed graceful stop for %s: %v\n", handle, err)
		}
	}

	delete(v.sandboxes, handle)
	return nil
}

// dialGuest opens a VSOCK connection to the guest agent running on the VM
// identified by handle.
func (v *VZBackend) dialGuest(handle string) (net.Conn, error) {
	v.mu.RLock()
	entry, exists := v.sandboxes[handle]
	v.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("sandbox handle not found: %s", handle)
	}

	conn, err := entry.socket.Connect(guestAgentPort)
	if err != nil {
		return nil, fmt.Errorf("vsock connect to port %d: %w", guestAgentPort, err)
	}
	conn.SetDeadline(time.Now().Add(30 * time.Second))
	return conn, nil
}

// agentEnvelope is the top-level wrapper sent to the guest agent.
type agentEnvelope struct {
	Op      string          `json:"op"`
	Payload json.RawMessage `json:"payload"`
}

// writeJSON encodes op+payload as a newline-delimited JSON envelope.
func writeJSON(w io.Writer, op string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	env := agentEnvelope{Op: op, Payload: json.RawMessage(raw)}
	return json.NewEncoder(w).Encode(env)
}

// readJSON decodes a single newline-delimited JSON value from r into v.
func readJSON(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}
