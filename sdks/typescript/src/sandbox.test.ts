/**
 * sandforge-sdk integration tests using Node built-in http mock server.
 * Runs with: node --test dist/**\/*.test.js
 */

import { describe, it, before, after } from "node:test";
import assert from "node:assert/strict";
import * as http from "node:http";
import { Client, Sandbox } from "./sandbox";
import { HTTPClient } from "./client";
import { SandboxError } from "./types";
import type { ExecResult, SandboxInfo, EntryInfo } from "./types";

// ---------------------------------------------------------------------------
// Minimal mock HTTP server
// ---------------------------------------------------------------------------

interface MockRequest {
  method: string;
  url: string;
  body: string;
}

interface MockResponse {
  status: number;
  body: string;
}

type Handler = (req: MockRequest) => MockResponse;

function startMockServer(handler: Handler): Promise<{ server: http.Server; baseURL: string }> {
  return new Promise((resolve) => {
    const server = http.createServer((req, res) => {
      let body = "";
      req.on("data", (chunk) => { body += chunk; });
      req.on("end", () => {
        const result = handler({ method: req.method ?? "", url: req.url ?? "", body });
        res.writeHead(result.status, { "Content-Type": "application/json" });
        res.end(result.body);
      });
    });

    server.listen(0, "127.0.0.1", () => {
      const addr = server.address() as { port: number };
      resolve({ server, baseURL: `http://127.0.0.1:${addr.port}` });
    });
  });
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("Client.create()", () => {
  let server: http.Server;
  let baseURL: string;

  before(async () => {
    ({ server, baseURL } = await startMockServer((req) => {
      if (req.method === "POST" && req.url === "/v1/sandboxes") {
        const parsed = JSON.parse(req.body) as { id: string };
        return { status: 200, body: JSON.stringify({ id: parsed.id }) };
      }
      return { status: 404, body: JSON.stringify({ error: "not found" }) };
    }));
  });

  after(() => { server.close(); });

  it("makes POST /v1/sandboxes and returns a Sandbox with the right ID", async () => {
    const client = new Client(baseURL);
    const sandbox = await client.create();
    assert.ok(sandbox.id.startsWith("sbx-"), `expected id to start with 'sbx-', got: ${sandbox.id}`);
  });
});

describe("sandbox.commands.run()", () => {
  let server: http.Server;
  let baseURL: string;
  const fixedID = "sbx-testid01";

  before(async () => {
    ({ server, baseURL } = await startMockServer((req) => {
      if (req.method === "POST" && req.url === `/v1/sandboxes/${fixedID}/exec`) {
        const result: ExecResult = { exitCode: 0, stdout: "hello\n", stderr: "" };
        return { status: 200, body: JSON.stringify(result) };
      }
      if (req.method === "POST" && req.url === "/v1/sandboxes") {
        return { status: 200, body: JSON.stringify({ id: fixedID }) };
      }
      return { status: 404, body: JSON.stringify({ error: "not found" }) };
    }));
  });

  after(() => { server.close(); });

  it("posts to /v1/sandboxes/{id}/exec and returns ExecResult", async () => {
    const client = new Client(baseURL);
    const sandbox = await client.create();
    assert.equal(sandbox.id, fixedID);

    const result = await sandbox.commands.run({ command: ["echo", "hello"] });
    assert.equal(result.exitCode, 0);
    assert.equal(result.stdout, "hello\n");
    assert.equal(result.stderr, "");
  });
});

describe("sandbox.kill()", () => {
  let server: http.Server;
  let baseURL: string;
  const fixedID = "sbx-killtest";
  let deleteCalled = false;

  before(async () => {
    ({ server, baseURL } = await startMockServer((req) => {
      if (req.method === "POST" && req.url === "/v1/sandboxes") {
        return { status: 200, body: JSON.stringify({ id: fixedID }) };
      }
      if (req.method === "DELETE" && req.url === `/v1/sandboxes/${fixedID}`) {
        deleteCalled = true;
        return { status: 200, body: "" };
      }
      return { status: 404, body: JSON.stringify({ error: "not found" }) };
    }));
  });

  after(() => { server.close(); });

  it("sends DELETE /v1/sandboxes/{id}", async () => {
    const client = new Client(baseURL);
    const sandbox = await client.create();
    await sandbox.kill();
    assert.ok(deleteCalled, "expected DELETE to have been called");
  });
});

describe("sandbox.info()", () => {
  let server: http.Server;
  let baseURL: string;
  const fixedID = "sbx-infotest";

  before(async () => {
    ({ server, baseURL } = await startMockServer((req) => {
      if (req.method === "POST" && req.url === "/v1/sandboxes") {
        return { status: 200, body: JSON.stringify({ id: fixedID }) };
      }
      if (req.method === "GET" && req.url === `/v1/sandboxes/${fixedID}`) {
        const info: SandboxInfo = { id: fixedID, state: "ready" };
        return { status: 200, body: JSON.stringify(info) };
      }
      return { status: 404, body: JSON.stringify({ error: "not found" }) };
    }));
  });

  after(() => { server.close(); });

  it("sends GET /v1/sandboxes/{id} and returns SandboxInfo", async () => {
    const client = new Client(baseURL);
    const sandbox = await client.create();
    const info = await sandbox.info();
    assert.equal(info.id, fixedID);
    assert.equal(info.state, "ready");
  });
});

describe("sandbox.files.read()", () => {
  let server: http.Server;
  let baseURL: string;
  const fixedID = "sbx-fsread";

  before(async () => {
    ({ server, baseURL } = await startMockServer((req) => {
      if (req.method === "POST" && req.url === "/v1/sandboxes") {
        return { status: 200, body: JSON.stringify({ id: fixedID }) };
      }
      if (req.method === "GET" && req.url?.startsWith(`/v1/sandboxes/${fixedID}/files/read`)) {
        const bytes = Array.from(new TextEncoder().encode("hello"));
        return { status: 200, body: JSON.stringify({ data: bytes }) };
      }
      return { status: 404, body: JSON.stringify({ error: "not found" }) };
    }));
  });

  after(() => { server.close(); });

  it("returns text by default", async () => {
    const client = new Client(baseURL);
    const sandbox = await client.create();
    const content = await sandbox.files.read("/tmp/hello.txt");
    assert.equal(content, "hello");
  });

  it("returns Uint8Array when format=bytes", async () => {
    const client = new Client(baseURL);
    const sandbox = await client.create();
    const content = await sandbox.files.read("/tmp/hello.txt", { format: "bytes" });
    assert.ok(content instanceof Uint8Array);
    assert.equal(content.length, 5);
  });
});

describe("sandbox.files.write()", () => {
  let server: http.Server;
  let baseURL: string;
  const fixedID = "sbx-fswrite";

  before(async () => {
    ({ server, baseURL } = await startMockServer((req) => {
      if (req.method === "POST" && req.url === "/v1/sandboxes") {
        return { status: 200, body: JSON.stringify({ id: fixedID }) };
      }
      if (req.method === "PUT" && req.url === `/v1/sandboxes/${fixedID}/files`) {
        const parsed = JSON.parse(req.body) as { guest_path: string; data: number[] };
        return { status: 200, body: JSON.stringify({ size: parsed.data.length }) };
      }
      return { status: 404, body: JSON.stringify({ error: "not found" }) };
    }));
  });

  after(() => { server.close(); });

  it("PUTs to /v1/sandboxes/{id}/files and returns write response", async () => {
    const client = new Client(baseURL);
    const sandbox = await client.create();
    const resp = await sandbox.files.write("/tmp/hello.txt", "hello");
    assert.equal(resp.size, 5);
  });
});

describe("sandbox.files.list()", () => {
  let server: http.Server;
  let baseURL: string;
  const fixedID = "sbx-fslist";
  const fakeEntry: EntryInfo = {
    name: "a.txt", path: "/tmp/a.txt", size: 3, isDir: false, modTime: "2025-01-01T00:00:00Z",
  };

  before(async () => {
    ({ server, baseURL } = await startMockServer((req) => {
      if (req.method === "POST" && req.url === "/v1/sandboxes") {
        return { status: 200, body: JSON.stringify({ id: fixedID }) };
      }
      if (req.method === "GET" && req.url?.startsWith(`/v1/sandboxes/${fixedID}/files`)) {
        return { status: 200, body: JSON.stringify({ entries: [fakeEntry] }) };
      }
      return { status: 404, body: JSON.stringify({ error: "not found" }) };
    }));
  });

  after(() => { server.close(); });

  it("GETs /v1/sandboxes/{id}/files?path=... and returns entries", async () => {
    const client = new Client(baseURL);
    const sandbox = await client.create();
    const entries = await sandbox.files.list("/tmp");
    assert.equal(entries.length, 1);
    assert.equal(entries[0].name, "a.txt");
  });
});

describe("sandbox.files.stat()", () => {
  let server: http.Server;
  let baseURL: string;
  const fixedID = "sbx-fsstat";
  const fakeEntry: EntryInfo = {
    name: "a.txt", path: "/tmp/a.txt", size: 3, isDir: false, modTime: "2025-01-01T00:00:00Z",
  };

  before(async () => {
    ({ server, baseURL } = await startMockServer((req) => {
      if (req.method === "POST" && req.url === "/v1/sandboxes") {
        return { status: 200, body: JSON.stringify({ id: fixedID }) };
      }
      if (req.method === "GET" && req.url?.startsWith(`/v1/sandboxes/${fixedID}/stat`)) {
        return { status: 200, body: JSON.stringify(fakeEntry) };
      }
      return { status: 404, body: JSON.stringify({ error: "not found" }) };
    }));
  });

  after(() => { server.close(); });

  it("GETs /v1/sandboxes/{id}/stat?path=... and returns EntryInfo", async () => {
    const client = new Client(baseURL);
    const sandbox = await client.create();
    const info = await sandbox.files.stat("/tmp/a.txt");
    assert.equal(info.name, "a.txt");
    assert.equal(info.size, 3);
    assert.equal(info.isDir, false);
  });
});

describe("sandbox.files.exists()", () => {
  let server: http.Server;
  let baseURL: string;
  const fixedID = "sbx-fsexists";

  before(async () => {
    ({ server, baseURL } = await startMockServer((req) => {
      if (req.method === "POST" && req.url === "/v1/sandboxes") {
        return { status: 200, body: JSON.stringify({ id: fixedID }) };
      }
      if (req.method === "GET" && req.url?.includes("existing")) {
        const entry: EntryInfo = { name: "x", path: "/x", size: 1, isDir: false, modTime: "" };
        return { status: 200, body: JSON.stringify(entry) };
      }
      if (req.method === "GET" && req.url?.includes("missing")) {
        return { status: 404, body: JSON.stringify({ error: "not found" }) };
      }
      return { status: 404, body: JSON.stringify({ error: "not found" }) };
    }));
  });

  after(() => { server.close(); });

  it("returns true when stat succeeds", async () => {
    const client = new Client(baseURL);
    const sandbox = await client.create();
    const result = await sandbox.files.exists("/existing");
    assert.equal(result, true);
  });

  it("returns false when stat throws", async () => {
    const client = new Client(baseURL);
    const sandbox = await client.create();
    const result = await sandbox.files.exists("/missing");
    assert.equal(result, false);
  });
});

describe("sandbox.git.clone()", () => {
  let server: http.Server;
  let baseURL: string;
  const fixedID = "sbx-gitclone";
  let lastBody: string = "";

  before(async () => {
    ({ server, baseURL } = await startMockServer((req) => {
      if (req.method === "POST" && req.url === "/v1/sandboxes") {
        return { status: 200, body: JSON.stringify({ id: fixedID }) };
      }
      if (req.method === "POST" && req.url === `/v1/sandboxes/${fixedID}/exec`) {
        lastBody = req.body;
        const result: ExecResult = { exitCode: 0, stdout: "", stderr: "" };
        return { status: 200, body: JSON.stringify(result) };
      }
      return { status: 404, body: JSON.stringify({ error: "not found" }) };
    }));
  });

  after(() => { server.close(); });

  it("runs git clone via exec endpoint", async () => {
    const client = new Client(baseURL);
    const sandbox = await client.create();
    const result = await sandbox.git.clone("https://github.com/example/repo.git");
    assert.equal(result.exitCode, 0);
    const parsed = JSON.parse(lastBody) as { command: string[] };
    assert.equal(parsed.command[0], "git");
    assert.equal(parsed.command[1], "clone");
    assert.ok(parsed.command.includes("https://github.com/example/repo.git"));
  });
});

describe("sandbox.git.status()", () => {
  let server: http.Server;
  let baseURL: string;
  const fixedID = "sbx-gitstatus";

  before(async () => {
    ({ server, baseURL } = await startMockServer((req) => {
      if (req.method === "POST" && req.url === "/v1/sandboxes") {
        return { status: 200, body: JSON.stringify({ id: fixedID }) };
      }
      if (req.method === "POST" && req.url === `/v1/sandboxes/${fixedID}/exec`) {
        // Single call: git status --branch --porcelain
        const result: ExecResult = { exitCode: 0, stdout: "## main...origin/main\n", stderr: "" };
        return { status: 200, body: JSON.stringify(result) };
      }
      return { status: 404, body: JSON.stringify({ error: "not found" }) };
    }));
  });

  after(() => { server.close(); });

  it("returns GitStatus with branch and clean flag from a single exec call", async () => {
    const client = new Client(baseURL);
    const sandbox = await client.create();
    const status = await sandbox.git.status("/workspace");
    assert.equal(status.branch, "main");
    assert.equal(status.clean, true);
  });
});

describe("error handling", () => {
  let server: http.Server;
  let baseURL: string;

  before(async () => {
    ({ server, baseURL } = await startMockServer((req) => {
      if (req.url === "/v1/sandboxes" && req.method === "POST") {
        return { status: 500, body: JSON.stringify({ error: "internal server error" }) };
      }
      if (req.url?.startsWith("/v1/sandboxes/") && req.method === "GET") {
        return { status: 404, body: JSON.stringify({ error: "not found" }) };
      }
      return { status: 500, body: JSON.stringify({ error: "unexpected" }) };
    }));
  });

  after(async () => {
    await new Promise<void>((resolve) => server.close(() => resolve()));
  });

  it("create() throws SandboxError on 500", async () => {
    const client = new Client(baseURL);
    await assert.rejects(
      () => client.create(),
      (err: unknown) => {
        assert.ok(err instanceof SandboxError);
        assert.strictEqual(err.statusCode, 500);
        return true;
      },
    );
  });

  it("info() throws SandboxError on 404", async () => {
    const sandbox = new Sandbox("sbx-test", new HTTPClient(baseURL));
    await assert.rejects(
      () => sandbox.info(),
      (err: unknown) => {
        assert.ok(err instanceof SandboxError);
        assert.strictEqual(err.statusCode, 404);
        return true;
      },
    );
  });
});
