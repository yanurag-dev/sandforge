/**
 * sandforge-sdk Sandbox class
 * High-level API for sandbox lifecycle and command execution.
 */

import { HTTPClient } from "./client";
import {
  SandboxSpec,
  ExecRequest,
  ExecResult,
  SandboxInfo,
  EntryInfo,
  WriteFileResponse,
  GitStatus,
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
   * read reads a file from the sandbox, returning text or raw bytes.
   */
  async read(path: string, opts?: { format: "text" | "bytes" }): Promise<string | Uint8Array> {
    const resp = await this.http.do<{ data: number[] }>(
      "GET",
      `/v1/sandboxes/${this.sandboxID}/files/read?path=${encodeURIComponent(path)}`,
    );
    const bytes = new Uint8Array(resp.data ?? []);
    return opts?.format === "bytes" ? bytes : new TextDecoder().decode(bytes);
  }

  /**
   * write writes data to a file inside the sandbox.
   */
  async write(path: string, data: string | Uint8Array): Promise<WriteFileResponse> {
    const bytes: Uint8Array =
      typeof data === "string" ? new TextEncoder().encode(data) : data;
    return this.http.do<WriteFileResponse>(
      "PUT",
      `/v1/sandboxes/${this.sandboxID}/files`,
      { guest_path: path, data: Array.from(bytes) },
    );
  }

  /**
   * list lists directory contents inside the sandbox.
   */
  async list(path: string): Promise<EntryInfo[]> {
    const resp = await this.http.do<{ entries: EntryInfo[] }>(
      "GET",
      `/v1/sandboxes/${this.sandboxID}/files?path=${encodeURIComponent(path)}`,
    );
    return resp.entries ?? [];
  }

  /**
   * stat returns metadata for a path inside the sandbox.
   */
  async stat(path: string): Promise<EntryInfo> {
    return this.http.do<EntryInfo>(
      "GET",
      `/v1/sandboxes/${this.sandboxID}/stat?path=${encodeURIComponent(path)}`,
    );
  }

  /**
   * exists returns true if the path exists in the sandbox.
   */
  async exists(path: string): Promise<boolean> {
    try {
      await this.stat(path);
      return true;
    } catch {
      return false;
    }
  }

  /**
   * remove deletes a file or directory inside the sandbox via `rm -rf`.
   */
  async remove(path: string): Promise<ExecResult> {
    // Delegate to the exec endpoint — no dedicated delete op needed.
    return this.http.do<ExecResult>(
      "POST",
      `/v1/sandboxes/${this.sandboxID}/exec`,
      { command: ["rm", "-rf", path], cwd: "/", env: {}, timeout_sec: 30 },
    );
  }
}

/**
 * GitNamespace is a shell facade for common git operations.
 * Every method runs `git` inside the sandbox via `commands.run()`.
 */
export class GitNamespace {
  constructor(
    private sandboxID: string,
    private http: HTTPClient,
  ) {}

  private exec(args: string[], cwd = "/"): Promise<ExecResult> {
    return this.http.do<ExecResult>(
      "POST",
      `/v1/sandboxes/${this.sandboxID}/exec`,
      { command: ["git", ...args], cwd, env: {}, timeout_sec: 120 },
    );
  }

  clone(url: string, dest = ".", opts?: { depth?: number }): Promise<ExecResult> {
    const args = ["clone"];
    if (opts?.depth) args.push("--depth", String(opts.depth));
    args.push(url, dest);
    return this.exec(args);
  }

  init(cwd: string): Promise<ExecResult> {
    return this.exec(["init"], cwd);
  }

  add(paths: string | string[], cwd: string): Promise<ExecResult> {
    const targets = Array.isArray(paths) ? paths : [paths];
    return this.exec(["add", ...targets], cwd);
  }

  commit(message: string, cwd: string): Promise<ExecResult> {
    return this.exec(["commit", "-m", message], cwd);
  }

  push(cwd: string, remote = "origin", branch = "HEAD"): Promise<ExecResult> {
    return this.exec(["push", remote, branch], cwd);
  }

  pull(cwd: string, remote = "origin"): Promise<ExecResult> {
    return this.exec(["pull", remote], cwd);
  }

  async status(cwd: string): Promise<GitStatus> {
    const result = await this.exec(["status", "--branch", "--porcelain"], cwd);
    const lines = result.stdout.split("\n").filter(Boolean);
    const branchLine = lines[0] ?? "";
    const branch = branchLine.startsWith("## ")
      ? branchLine.slice(3).split("...")[0]
      : branchLine;
    const fileLines = lines.slice(1);
    return {
      branch,
      clean: fileLines.length === 0,
      stdout: result.stdout,
    };
  }

  async branches(cwd: string): Promise<string[]> {
    const result = await this.exec(["branch", "--list"], cwd);
    return result.stdout
      .split("\n")
      .map((b) => b.replace(/^\*?\s+/, "").trim())
      .filter(Boolean);
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
  git: GitNamespace;
  private http: HTTPClient;

  constructor(id: string, http: HTTPClient) {
    this.id = id;
    this.http = http;
    this.commands = new CommandsNamespace(id, http);
    this.files = new FilesNamespace(id, http);
    this.git = new GitNamespace(id, http);
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
