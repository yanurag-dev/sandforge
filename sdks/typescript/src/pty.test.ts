/**
 * sandforge-sdk PTY session tests.
 * Runs with: node --test dist/**\/*.test.js
 *
 * Uses an injected fake WebSocket (the same seam HTTPClient exposes for fetch)
 * so tests run without a WebSocket server or any dependency.
 */

import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { PTYSession } from "./pty";
import type { PTYEvent } from "./pty";

// Minimal fake WebSocket: records sent frames, lets the test drive onmessage /
// onclose, and resolves the open promise synchronously.
class FakeWebSocket {
  static OPEN = 1;
  binaryType = "blob";
  sent: string[] = [];
  closed = false;
  onopen: (() => void) | null = null;
  onmessage: ((ev: { data: unknown }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;

  constructor(public url: string) {
    // Fire open on the next tick so handlers are wired first.
    queueMicrotask(() => this.onopen?.());
  }
  send(data: string): void {
    this.sent.push(data);
  }
  close(): void {
    this.closed = true;
  }
  emit(text: string): void {
    this.onmessage?.({ data: text });
  }
  fireClose(): void {
    this.onclose?.();
  }
}

function frame(event: string, data?: string, code?: number): string {
  const o: Record<string, unknown> = { event };
  if (data !== undefined) o.data = Buffer.from(data).toString("base64");
  if (code !== undefined) o.code = code;
  return JSON.stringify(o);
}

describe("PTYSession", () => {
  it("encodes stdin as base64 over the socket", () => {
    let ws!: FakeWebSocket;
    const Impl = class extends FakeWebSocket {
      constructor(url: string) {
        super(url);
        ws = this;
      }
    } as unknown as typeof WebSocket;

    const session = new PTYSession("ws://x/pty", Impl);
    session.send("ls\n");

    assert.equal(ws.sent.length, 1);
    const sent = JSON.parse(ws.sent[0]);
    assert.equal(sent.event, "stdin");
    assert.equal(Buffer.from(sent.data, "base64").toString(), "ls\n");
  });

  it("sends resize with cols/rows", () => {
    let ws!: FakeWebSocket;
    const Impl = class extends FakeWebSocket {
      constructor(url: string) {
        super(url);
        ws = this;
      }
    } as unknown as typeof WebSocket;

    const session = new PTYSession("ws://x/pty", Impl);
    session.resize(120, 40);
    const sent = JSON.parse(ws.sent[0]);
    assert.deepEqual([sent.event, sent.cols, sent.rows], ["resize", 120, 40]);
  });

  it("async-iterates events then completes on close", async () => {
    let ws!: FakeWebSocket;
    const Impl = class extends FakeWebSocket {
      constructor(url: string) {
        super(url);
        ws = this;
      }
    } as unknown as typeof WebSocket;

    const session = new PTYSession("ws://x/pty", Impl);

    // Queue some events, then close.
    ws.emit(frame("stdout", "hello"));
    ws.emit(frame("exit", undefined, 0));
    ws.fireClose();

    const events: PTYEvent[] = [];
    for await (const ev of session) {
      events.push(ev);
    }

    assert.equal(events.length, 2);
    assert.equal(events[0].event, "stdout");
    assert.equal(Buffer.from(events[0].data!).toString(), "hello");
    assert.equal(events[1].event, "exit");
    assert.equal(events[1].code, 0);
  });

  it("delivers a message that arrives while a consumer is awaiting", async () => {
    let ws!: FakeWebSocket;
    const Impl = class extends FakeWebSocket {
      constructor(url: string) {
        super(url);
        ws = this;
      }
    } as unknown as typeof WebSocket;

    const session = new PTYSession("ws://x/pty", Impl);
    const iter = session[Symbol.asyncIterator]();

    const pending = iter.next(); // awaiting before any message arrives
    ws.emit(frame("stdout", "later"));
    const result = await pending;

    assert.equal(result.done, false);
    assert.equal(Buffer.from(result.value.data!).toString(), "later");
  });

  it("close() ends iteration", async () => {
    let ws!: FakeWebSocket;
    const Impl = class extends FakeWebSocket {
      constructor(url: string) {
        super(url);
        ws = this;
      }
    } as unknown as typeof WebSocket;

    const session = new PTYSession("ws://x/pty", Impl);
    session.close();
    assert.equal(ws.closed, true);

    const iter = session[Symbol.asyncIterator]();
    const r = await iter.next();
    assert.equal(r.done, true);
  });
});
