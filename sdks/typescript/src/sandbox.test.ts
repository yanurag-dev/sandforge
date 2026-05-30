/**
 * @sandforge/sdk integration tests using Node built-in http mock server.
 * Runs with: node --test dist/**\/*.test.js
 */

import { describe, it, before, after } from "node:test";
import assert from "node:assert/strict";
import * as http from "node:http";
import { Client, Sandbox } from "./sandbox";
import { HTTPClient } from "./client";
import { SandboxError } from "./types";
import type { ExecResult, SandboxInfo } from "./types";

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
