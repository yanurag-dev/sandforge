//go:build darwin

package vz

import (
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/Code-Hex/vz/v3"
	"github.com/sandforge/sandforge/pkg/agentproto"
	"github.com/sandforge/sandforge/pkg/api"
)

// guestAgentPort is the VSOCK port the in-guest agent listens on.
const guestAgentPort uint32 = 2222

// sandboxEntry bundles the VM with its socket device so we can dial VSOCK later.
type sandboxEntry struct {
	vm       *vz.VirtualMachine
	socket   *vz.VirtioSocketDevice
	consoleR *os.File
	consoleW *os.File
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
	return v.CreateSandboxWithMounts(spec, spec.Mounts)
}

func (v *VZBackend) CreateSandboxWithMounts(spec api.SandboxSpec, mounts []api.WorkspaceMount) (string, error) {
	switch spec.NetworkMode {
	case "", "offline":
		spec.NetworkMode = "offline"
	case "fetch":
	default:
		return "", fmt.Errorf("unsupported network mode: %q (must be offline or fetch)", spec.NetworkMode)
	}

	kernelPath := v.kernelPath
	initrdPath := v.initrdPath

	if _, err := os.Stat(kernelPath); err != nil {
		return "", fmt.Errorf("kernel not found at %s: %w", kernelPath, err)
	}
	if _, err := os.Stat(initrdPath); err != nil {
		return "", fmt.Errorf("initrd not found at %s: %w", initrdPath, err)
	}

	cmdLine := fmt.Sprintf("console=hvc0 root=/dev/ram0 sandforge.network=%s", spec.NetworkMode)
	bootLoader, err := vz.NewLinuxBootLoader(
		kernelPath,
		vz.WithCommandLine(cmdLine),
		vz.WithInitrd(initrdPath),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create bootloader: %w", err)
	}

	// Use a pipe so VZ doesn't require the process to own a terminal.
	pr, pw, err := os.Pipe()
	if err != nil {
		return "", fmt.Errorf("failed to create console pipe: %w", err)
	}
	cleanupConsole := true
	defer func() {
		if cleanupConsole {
			_ = pr.Close()
			_ = pw.Close()
		}
	}()

	attachment, err := vz.NewFileHandleSerialPortAttachment(pr, pw)
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

	if spec.CPU <= 0 {
		return "", fmt.Errorf("invalid CPU count: must be greater than 0")
	}
	if spec.MemoryMb <= 0 {
		return "", fmt.Errorf("invalid memory size: must be greater than 0")
	}

	config, err := vz.NewVirtualMachineConfiguration(
		bootLoader,
		uint(spec.CPU),
		uint64(spec.MemoryMb)*1024*1024,
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

	// Network: offline = no NIC; fetch = NAT NIC (nftables allowlist enforced in guest)
	if spec.NetworkMode == "fetch" {
		netCfg, err := buildNATNetworkConfig()
		if err != nil {
			return "", fmt.Errorf("failed to create network config: %w", err)
		}
		config.SetNetworkDevicesVirtualMachineConfiguration([]*vz.VirtioNetworkDeviceConfiguration{netCfg})
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
	v.mu.Lock()
	v.sandboxes[handle] = &sandboxEntry{
		vm:       vm,
		socket:   socketDevices[0],
		consoleR: pr,
		consoleW: pw,
	}
	v.mu.Unlock()
	cleanupConsole = false

	return handle, nil
}

func (v *VZBackend) MountWorkspace(handle string, mount api.WorkspaceMount) error {
	v.mu.RLock()
	_, exists := v.sandboxes[handle]
	v.mu.RUnlock()

	if !exists {
		return fmt.Errorf("sandbox handle not found: %s", handle)
	}

	return fmt.Errorf(
		"mount hot-plug not supported on VZ backend; provide mounts at sandbox creation (host=%s guest=%s ro=%v)",
		mount.HostPath, mount.GuestPath, mount.ReadOnly,
	)
}

// Exec sends an execRequest JSON message to the in-guest agent over VSOCK and
// returns the decoded execResponse. The guest agent must be listening on
// guestAgentPort and speak the same length-prefixed JSON protocol.
func (v *VZBackend) Exec(handle string, req api.ExecRequest) (api.ExecResult, error) {
	conn, err := v.dialGuest(handle)
	if err != nil {
		return api.ExecResult{}, fmt.Errorf("exec: dial guest: %w", err)
	}
	defer func() { _ = conn.Close() }()

	payload := agentproto.ExecRequest{
		Command:    req.Command,
		CWD:        req.CWD,
		Env:        req.Env,
		TimeoutSec: req.TimeoutSec,
	}

	if err := agentproto.WriteRequest(conn, "exec", payload); err != nil {
		return api.ExecResult{}, fmt.Errorf("exec: write request: %w", err)
	}

	var resp agentproto.ExecResponse
	if err := agentproto.ReadResponse(conn, &resp); err != nil {
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
	defer func() { _ = conn.Close() }()

	payload := agentproto.CopyOutRequest{GuestPath: guestPath}
	if err := agentproto.WriteRequest(conn, "copyout", payload); err != nil {
		return fmt.Errorf("copyout: write request: %w", err)
	}

	var resp agentproto.CopyOutResponse
	if err := agentproto.ReadResponse(conn, &resp); err != nil {
		return fmt.Errorf("copyout: read response: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("copyout: guest error: %s", resp.Error)
	}

	if err := os.WriteFile(dest, resp.Data, 0o600); err != nil {
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

	if entry.consoleR != nil {
		_ = entry.consoleR.Close()
	}
	if entry.consoleW != nil {
		_ = entry.consoleW.Close()
	}

	if entry.vm.CanStop() {
		if _, err := entry.vm.RequestStop(); err != nil {
			fmt.Printf("Warning: failed graceful stop for %s: %v\n", handle, err)
		}
	}

	delete(v.sandboxes, handle)
	return nil
}

// dialGuest opens a VSOCK connection to the guest agent. Retries for up to
// 30s to allow the guest kernel and init to boot before the agent is ready.
func (v *VZBackend) dialGuest(handle string) (net.Conn, error) {
	v.mu.RLock()
	entry, exists := v.sandboxes[handle]
	v.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("sandbox handle not found: %s", handle)
	}

	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := entry.socket.Connect(guestAgentPort)
		if err == nil {
			if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
				_ = conn.Close()
				lastErr = fmt.Errorf("failed to set deadline: %w", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return conn, nil
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}
	return nil, fmt.Errorf("vsock connect to port %d: %w", guestAgentPort, lastErr)
}

// buildNATNetworkConfig creates a Virtio NIC backed by macOS NAT (shared networking).
// Used for "fetch" network mode. "offline" mode attaches no NIC at all.
func buildNATNetworkConfig() (*vz.VirtioNetworkDeviceConfiguration, error) {
	nat, err := vz.NewNATNetworkDeviceAttachment()
	if err != nil {
		return nil, fmt.Errorf("nat attachment: %w", err)
	}
	netCfg, err := vz.NewVirtioNetworkDeviceConfiguration(nat)
	if err != nil {
		return nil, fmt.Errorf("virtio network config: %w", err)
	}
	return netCfg, nil
}
