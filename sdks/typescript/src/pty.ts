/**
 * sandforge-sdk interactive PTY sessions.
 *
 * A PTY session is a long-lived, full-duplex WebSocket to the control plane.
 * It uses the platform-native WebSocket (browser, or Node 22+ global) so the
 * SDK stays dependency-free.
 *
 * Example:
 *   const session = sandbox.pty.connect({ cols: 120, rows: 40 });
 *   session.send("ls\n");
 *   for await (const ev of session) {
 *     if (ev.event === "stdout") process.stdout.write(Buffer.from(ev.data ?? []));
 *     else if (ev.event === "exit") console.log("exit", ev.code);
 *   }
 */

/**
 * PTYEvent is a single event from a session. `data` carries raw terminal bytes.
 */
export interface PTYEvent {
  event: "stdout" | "exit" | "error";
  data?: Uint8Array;
  code?: number;
  msg?: string;
}

export interface PTYConnectOptions {
  cols?: number;
  rows?: number;
  command?: string[];
}

/**
 * PTYSession is a live interactive terminal session.
 *
 * It is an async-iterable: `for await (const ev of session)` blocks until each
 * event arrives and completes when the session ends — the JS equivalent of the
 * NextEvent()/io.EOF contract used by the other SDKs.
 */
export class PTYSession implements AsyncIterable<PTYEvent> {
  private ws: WebSocket;
  // Bridge the event-based WebSocket to a pull-based async iterator: incoming
  // messages either resolve a waiting next() or buffer in the queue.
  private queue: PTYEvent[] = [];
  private pending: ((r: IteratorResult<PTYEvent>) => void) | null = null;
  private done = false;
  private opened: Promise<void>;

  constructor(url: string, wsImpl?: typeof WebSocket) {
    const WS = wsImpl ?? (globalThis as { WebSocket?: typeof WebSocket }).WebSocket;
    if (!WS) {
      throw new Error(
        "No WebSocket implementation available. Use Node 22+, a browser, or pass one to connect().",
      );
    }
    this.ws = new WS(url);
    this.ws.binaryType = "arraybuffer";

    this.opened = new Promise<void>((resolve, reject) => {
      this.ws.onopen = () => resolve();
      this.ws.onerror = () => reject(new Error("pty websocket error"));
    });

    this.ws.onmessage = (ev: MessageEvent) => this.push(decodeEvent(ev.data));
    this.ws.onclose = () => this.finish();
  }

  /** send forwards input (string or bytes) to the terminal. */
  send(data: string | Uint8Array): void {
    const bytes = typeof data === "string" ? new TextEncoder().encode(data) : data;
    this.ws.send(JSON.stringify({ event: "stdin", data: toBase64(bytes) }));
  }

  /** resize updates the terminal window size. */
  resize(cols: number, rows: number): void {
    this.ws.send(JSON.stringify({ event: "resize", cols, rows }));
  }

  /** close ends the session. */
  close(): void {
    this.ws.close();
    this.finish();
  }

  [Symbol.asyncIterator](): AsyncIterator<PTYEvent> {
    return {
      next: (): Promise<IteratorResult<PTYEvent>> => {
        if (this.queue.length > 0) {
          return Promise.resolve({ value: this.queue.shift()!, done: false });
        }
        if (this.done) {
          return Promise.resolve({ value: undefined, done: true });
        }
        return new Promise((resolve) => {
          this.pending = resolve;
        });
      },
    };
  }

  private push(ev: PTYEvent): void {
    if (this.pending) {
      const resolve = this.pending;
      this.pending = null;
      resolve({ value: ev, done: false });
    } else {
      this.queue.push(ev);
    }
  }

  private finish(): void {
    if (this.done) return;
    this.done = true;
    if (this.pending) {
      const resolve = this.pending;
      this.pending = null;
      resolve({ value: undefined, done: true });
    }
  }
}

/**
 * PTYNamespace opens interactive PTY sessions for a sandbox.
 */
export class PTYNamespace {
  constructor(
    private sandboxID: string,
    private baseURL: string,
    private wsImpl?: typeof WebSocket,
  ) {}

  /** connect opens an interactive PTY session over a WebSocket. */
  connect(opts?: PTYConnectOptions): PTYSession {
    const wsBase = this.baseURL.replace(/^http/, "ws");
    const params = new URLSearchParams();
    if (opts?.cols) params.set("cols", String(opts.cols));
    if (opts?.rows) params.set("rows", String(opts.rows));
    for (const c of opts?.command ?? []) params.append("cmd", c);
    const qs = params.toString();
    const url = `${wsBase}/v1/sandboxes/${this.sandboxID}/pty${qs ? "?" + qs : ""}`;
    return new PTYSession(url, this.wsImpl);
  }
}

function toBase64(bytes: Uint8Array): string {
  if (typeof Buffer !== "undefined") {
    return Buffer.from(bytes).toString("base64");
  }
  let bin = "";
  for (const b of bytes) bin += String.fromCharCode(b);
  return btoa(bin);
}

function fromBase64(s: string): Uint8Array {
  if (typeof Buffer !== "undefined") {
    return new Uint8Array(Buffer.from(s, "base64"));
  }
  const bin = atob(s);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

function decodeEvent(raw: unknown): PTYEvent {
  const text =
    typeof raw === "string"
      ? raw
      : new TextDecoder().decode(raw as ArrayBuffer);
  const obj = JSON.parse(text) as {
    event: PTYEvent["event"];
    data?: string;
    code?: number;
    msg?: string;
  };
  return {
    event: obj.event,
    data: obj.data ? fromBase64(obj.data) : undefined,
    code: obj.code,
    msg: obj.msg,
  };
}
