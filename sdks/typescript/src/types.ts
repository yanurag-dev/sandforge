/**
 * @sandforge/sdk types
 * TypeScript interfaces matching the Sandforge control plane API.
 */

/**
 * SandboxSpec defines the configuration for a new sandbox.
 */
export interface SandboxSpec {
  backend?: "linux-kvm" | "linux-firecracker" | "macos-vz";
  cpu?: number;
  memoryMb?: number;
  diskGb?: number;
  timeoutSec?: number;
  networkMode?: "offline" | "fetch" | "full";
  taskIsolation?: "container" | "process";
  mounts?: WorkspaceMount[];
}

/**
 * WorkspaceMount describes a single host-to-guest mount.
 */
export interface WorkspaceMount {
  hostPath: string;
  guestPath: string;
  readOnly?: boolean;
}

/**
 * ExecRequest describes a command to execute inside a sandbox.
 */
export interface ExecRequest {
  command: string[];
  cwd?: string;
  env?: Record<string, string>;
  timeoutSec?: number;
}

/**
 * ExecResult is the outcome of running a command.
 */
export interface ExecResult {
  exitCode: number;
  stdout: string;
  stderr: string;
  artifacts?: string[];
}

/**
 * SandboxInfo is the response from GET /v1/sandboxes/{id}.
 */
export interface SandboxInfo {
  id: string;
  state: string;
}

/**
 * SandboxError wraps API errors with status code and message.
 */
export class SandboxError extends Error {
  constructor(
    public statusCode: number,
    message: string,
  ) {
    super(message);
    this.name = "SandboxError";
  }
}
