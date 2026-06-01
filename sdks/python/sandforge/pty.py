"""Interactive PTY session support for the Sandforge Python SDK.

A PTY session is a long-lived, full-duplex WebSocket to the control plane. It
requires the optional ``websocket-client`` dependency:

    pip install sandforge-sdk[pty]

Example:
    sandbox = client.create_sandbox(SandboxSpec(network_mode="fetch"))
    session = sandbox.pty.open(cols=120, rows=40)
    session.send("ls\\n")
    for event in session:          # blocks until each event; ends on exit
        if event.event == "stdout":
            print(event.data.decode(errors="replace"), end="")
        elif event.event == "exit":
            print(f"\\nexited {event.code}")
    session.close()
"""

import base64
import json
from dataclasses import dataclass
from typing import List, Optional


@dataclass
class PTYEvent:
    """A single event from a PTY session.

    event: one of "stdout", "exit", "error".
    data:  raw terminal bytes (for "stdout").
    code:  process exit code (for "exit").
    msg:   error detail (for "error").
    """

    event: str
    data: bytes = b""
    code: int = 0
    msg: str = ""

    @staticmethod
    def from_dict(d: dict) -> "PTYEvent":
        raw = d.get("data")
        # The wire encodes []byte as base64 (Go json); decode back to bytes.
        data = base64.b64decode(raw) if raw else b""
        return PTYEvent(
            event=d.get("event", ""),
            data=data,
            code=d.get("code", 0),
            msg=d.get("msg", ""),
        )


class PTYSession:
    """A live interactive terminal session.

    The session is iterable: iterating blocks until each event arrives and stops
    (raises StopIteration) once the session ends — the Python equivalent of the
    NextEvent()/io.EOF contract used by the other SDKs.
    """

    def __init__(self, ws):
        self._ws = ws

    def send(self, data) -> None:
        """Send input (str or bytes) to the terminal."""
        if isinstance(data, str):
            data = data.encode()
        self._send_event({"event": "stdin", "data": _b64(data)})

    def resize(self, cols: int, rows: int) -> None:
        """Resize the terminal window."""
        self._send_event({"event": "resize", "cols": cols, "rows": rows})

    def __iter__(self) -> "PTYSession":
        return self

    def __next__(self) -> PTYEvent:
        from websocket import WebSocketConnectionClosedException

        try:
            raw = self._ws.recv()
        except WebSocketConnectionClosedException:
            raise StopIteration
        if raw is None or raw == "":
            # A clean close surfaces as an empty frame in some versions.
            raise StopIteration
        return PTYEvent.from_dict(json.loads(raw))

    def close(self) -> None:
        """End the session and close the connection."""
        self._ws.close()

    def __enter__(self) -> "PTYSession":
        return self

    def __exit__(self, exc_type, exc_val, exc_tb) -> bool:
        self.close()
        return False

    def _send_event(self, payload: dict) -> None:
        self._ws.send(json.dumps(payload))


class PTYNamespace:
    """Opens interactive PTY sessions for a sandbox."""

    def __init__(self, sandbox):
        self._sandbox = sandbox

    def open(
        self,
        cols: int = 80,
        rows: int = 24,
        command: Optional[List[str]] = None,
    ) -> PTYSession:
        """Open an interactive PTY session.

        Args:
            cols: Initial terminal width.
            rows: Initial terminal height.
            command: Command to run (default: the guest login shell).

        Returns:
            PTYSession: A live, iterable session.

        Raises:
            ImportError: If the optional ``websocket-client`` dependency is
                missing. Install with ``pip install sandforge-sdk[pty]``.
        """
        try:
            from websocket import create_connection
        except ImportError as e:  # pragma: no cover - exercised via message only
            raise ImportError(
                "Interactive PTY sessions require the 'websocket-client' package. "
                "Install it with: pip install sandforge-sdk[pty]"
            ) from e

        base = self._sandbox._client.base_url
        ws_base = base.replace("https://", "wss://", 1).replace("http://", "ws://", 1)
        url = f"{ws_base}/v1/sandboxes/{self._sandbox.id}/pty?cols={cols}&rows={rows}"
        for c in command or []:
            url += f"&cmd={c}"

        ws = create_connection(url)
        return PTYSession(ws)


def _b64(data: bytes) -> str:
    return base64.b64encode(data).decode("ascii")
