"""Unit tests for the Sandforge Python SDK PTY session.

Follows the existing suite's approach: stub the optional ``websocket`` module so
tests run without the dependency or a real server, exercising the session's
encode/iterate/close logic in isolation.
"""

import base64
import json
import sys
import os
import unittest
from unittest.mock import MagicMock

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
sys.modules.setdefault("requests", MagicMock())

# Stub the optional websocket-client dependency before importing the SDK.
_ws_stub = MagicMock()


class _WSClosed(Exception):
    pass


_ws_stub.WebSocketConnectionClosedException = _WSClosed
sys.modules["websocket"] = _ws_stub

from sandforge.pty import PTYEvent, PTYSession  # noqa: E402


class FakeWS:
    """A fake websocket connection: queues outbound frames, replays inbound."""

    def __init__(self, inbound):
        self.inbound = list(inbound)
        self.sent = []
        self.closed = False

    def send(self, data):
        self.sent.append(data)

    def recv(self):
        if not self.inbound:
            raise _WSClosed()
        return self.inbound.pop(0)

    def close(self):
        self.closed = True


def _frame(event, data=b"", code=0):
    payload = {"event": event}
    if data:
        payload["data"] = base64.b64encode(data).decode("ascii")
    if code:
        payload["code"] = code
    return json.dumps(payload)


class TestPTYSession(unittest.TestCase):
    def test_send_encodes_stdin_as_base64(self):
        ws = FakeWS([])
        PTYSession(ws).send("ls\n")
        self.assertEqual(len(ws.sent), 1)
        sent = json.loads(ws.sent[0])
        self.assertEqual(sent["event"], "stdin")
        self.assertEqual(base64.b64decode(sent["data"]), b"ls\n")

    def test_resize_sends_cols_rows(self):
        ws = FakeWS([])
        PTYSession(ws).resize(120, 40)
        sent = json.loads(ws.sent[0])
        self.assertEqual((sent["event"], sent["cols"], sent["rows"]), ("resize", 120, 40))

    def test_iteration_yields_events_then_stops(self):
        ws = FakeWS([_frame("stdout", b"hello"), _frame("exit", code=0)])
        session = PTYSession(ws)

        events = list(session)  # iterates until StopIteration (clean close)

        self.assertEqual(len(events), 2)
        self.assertIsInstance(events[0], PTYEvent)
        self.assertEqual(events[0].event, "stdout")
        self.assertEqual(events[0].data, b"hello")
        self.assertEqual(events[1].event, "exit")
        self.assertEqual(events[1].code, 0)

    def test_close_closes_connection(self):
        ws = FakeWS([])
        session = PTYSession(ws)
        session.close()
        self.assertTrue(ws.closed)

    def test_context_manager_closes(self):
        ws = FakeWS([])
        with PTYSession(ws):
            pass
        self.assertTrue(ws.closed)


if __name__ == "__main__":
    unittest.main()
