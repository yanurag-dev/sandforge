# Sandforge - Agent Sandbox

## Project Overview
Sandforge is a portable sandbox architecture designed to run coding agents (like Codex, Claude Code, etc.) in a restricted, isolated workspace. It ensures that generated commands, third-party build tools, and untrusted repository code do not compromise the host machine.

### Key Technologies
- **Language:** Go (Golang)
- **Host Isolation:** Apple Virtualization Framework (macOS), KVM (Linux)
- **Guest OS:** Minimal Linux Worker Image
- **Task Isolation:** Rootless containers (e.g., Podman/Docker) inside the VM

### Architecture Summary
Following the "Control Plane Outside, Execution Inside" principle:
1.  **Control Plane:** Trusted environment managing sessions, routing tools, and enforcing policy.
2.  **Policy Engine:** Decision point for filesystem, network, and resource access.
3.  **Sandbox Supervisor:** Orchestrates the lifecycle of worker VMs and tasks.
4.  **Backend Driver:** Host-specific implementation (VZ on macOS, KVM on Linux).
5.  **Task Runtime:** Per-task isolation boundary inside the Linux guest.

## Building and Running

### Development Commands
- **Initialize/Sync Dependencies:** `go mod tidy`
- **Build the CLI:** `go build -o sandforge ./cmd/sandforge`
- **Run Tests:** `go test ./...`
- **Linting:** (TODO: Add linting configuration, e.g., golangci-lint)

### Running the Sandbox
(TODO: Add instructions once the first backend driver is functional)

## Development Conventions

### Code Structure
- `cmd/`: CLI entry points.
- `internal/`: Private implementation details (Control Plane, Policy Engine, etc.).
- `pkg/`: Publicly importable packages and shared interfaces (API contracts).

### Principles
- **Deny by Default:** Network and filesystem access must be explicitly granted via policy.
- **Interface-Driven:** Use interfaces in `pkg/api` to abstract host-specific drivers.
- **Ephemeral:** Design for clean teardown and minimal state persistence in the worker.
- **Idiomatic Go:** Follow standard Go formatting (`go fmt`) and naming conventions.

## Documentation
- Refer to `ARCHITECTURE.md` for the full technical design and security model.
