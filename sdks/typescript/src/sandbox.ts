/**
 * @sandforge/sdk Sandbox class
 * High-level API for sandbox lifecycle and command execution.
 */

import { HTTPClient } from "./client";
import {
  SandboxSpec,
  ExecRequest,
  ExecResult,
  SandboxInfo,
} from "./types";

/**
 * CommandsNamespace groups command execution methods.
 */
export class CommandsNamespace {
  constructor(
    private sandboxID: string,
    private http: HTTPClient,
  ) {}

  /**
   * run executes a command inside the sandbox.
   */
  async run(request: ExecRequest): Promise<ExecResult> {
    return this.http.do<ExecResult>(
      "POST",
      `/v1/sandboxes/${this.sandboxID}/exec`,
      {
        command: request.command,
        cwd: request.cwd || "",
        env: request.env || {},
        timeout_sec: request.timeoutSec || 0,
      },
    );
  }
}

/**
 * FilesNamespace groups file operation methods.
 */
export class FilesNamespace {
  constructor(
    private sandboxID: string,
    private http: HTTPClient,
  ) {}

  /**
   * read reads a file from the sandbox.
   * Note: This is a stub for now. Full implementation requires VSOCK copyout support.
   */
  async read(path: string): Promise<string> {
    throw new Error(
      "files.read() not yet implemented (requires VSOCK copyout support)",
    );
  }
}

/**
 * Sandbox is a handle to a created sandbox instance.
 * It provides methods to execute commands, read files, get status, and destroy the sandbox.
 */
export class Sandbox {
  id: string;
  commands: CommandsNamespace;
  files: FilesNamespace;
  private http: HTTPClient;

  constructor(id: string, http: HTTPClient) {
    this.id = id;
    this.http = http;
    this.commands = new CommandsNamespace(id, http);
    this.files = new FilesNamespace(id, http);
  }

  /**
   * info returns the current status of the sandbox.
   */
  async info(): Promise<SandboxInfo> {
    return this.http.do<SandboxInfo>(
      "GET",
      `/v1/sandboxes/${this.id}`,
    );
  }

  /**
   * kill destroys the sandbox.
   */
  async kill(): Promise<void> {
    await this.http.do<void>(
      "DELETE",
      `/v1/sandboxes/${this.id}`,
    );
  }
}

/**
 * Client is the high-level API for Sandforge.
 */
export class Client {
  private http: HTTPClient;

  constructor(baseURL: string, fetchImpl?: typeof globalThis.fetch) {
    this.http = new HTTPClient(baseURL, fetchImpl);
  }

  /**
   * create provisions a new sandbox from the given spec.
   * If no spec is provided, defaults are used.
   */
  async create(spec?: SandboxSpec): Promise<Sandbox> {
    const id = generateID();

    interface CreateRequest {
      id: string;
      spec: SandboxSpec;
    }

    interface CreateResponse {
      id: string;
    }

    const req: CreateRequest = {
      id,
      spec: spec || {},
    };

    const response = await this.http.do<CreateResponse>(
      "POST",
      "/v1/sandboxes",
      req,
    );

    return new Sandbox(response.id, this.http);
  }
}

/**
 * generateID returns a random hex sandbox identifier prefixed with "sbx-".
 */
function generateID(): string {
  const bytes = new Uint8Array(8);
  // Use crypto.getRandomValues if available (browser + Node 15+)
  if (typeof globalThis !== "undefined" && globalThis.crypto) {
    globalThis.crypto.getRandomValues(bytes);
  } else {
    // Fallback for older Node.js
    const { randomBytes } = require("crypto");
    const buf = randomBytes(8);
    bytes.set(buf);
  }

  // Convert bytes to hex
  let hex = "";
  for (let i = 0; i < bytes.length; i++) {
    hex += bytes[i].toString(16).padStart(2, "0");
  }

  return "sbx-" + hex;
}
