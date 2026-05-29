"""Unit tests for the Sandforge Python SDK client."""

import unittest
from unittest.mock import MagicMock

import sys
import os
from unittest.mock import MagicMock

# Allow running tests from the sdks/python directory without installing the package.
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

# Stub the `requests` module so tests run without installing it.
_requests_stub = MagicMock()
sys.modules.setdefault("requests", _requests_stub)

from sandforge import Client, SandboxHandle, Sandbox
from sandforge.types import ExecResult, SandboxInfo, SandboxSpec


class TestClientCreateSandbox(unittest.TestCase):
    """Tests for Client.create_sandbox()."""

    def _make_client(self):
        client = Client("http://localhost:8080")
        client.session = MagicMock()
        return client

    def test_create_sandbox_posts_to_v1_sandboxes(self):
        """create_sandbox() should POST to /v1/sandboxes and return a SandboxHandle."""
        client = self._make_client()

        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.text = '{"id": "sbx-abc123"}'
        mock_response.json.return_value = {"id": "sbx-abc123"}
        client.session.post.return_value = mock_response

        handle = client.create_sandbox(SandboxSpec())

        # Verify POST was called with the right URL
        call_args = client.session.post.call_args
        self.assertIn("/v1/sandboxes", call_args[0][0])

        # Verify the returned handle has the right ID
        self.assertIsInstance(handle, SandboxHandle)
        self.assertEqual(handle.id, "sbx-abc123")

    def test_create_sandbox_uses_default_spec_when_none(self):
        """create_sandbox(None) should use a default SandboxSpec."""
        client = self._make_client()

        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.text = '{"id": "sbx-def456"}'
        mock_response.json.return_value = {"id": "sbx-def456"}
        client.session.post.return_value = mock_response

        handle = client.create_sandbox()

        self.assertIsInstance(handle, SandboxHandle)
        self.assertEqual(handle.id, "sbx-def456")

    def test_sandbox_alias_equals_sandbox_handle(self):
        """The Sandbox alias should be the same class as SandboxHandle."""
        self.assertIs(Sandbox, SandboxHandle)


class TestCommandsAPIRun(unittest.TestCase):
    """Tests for sandbox.commands.run()."""

    def _make_sandbox(self):
        client = Client("http://localhost:8080")
        client.session = MagicMock()
        return SandboxHandle(client, "sbx-test01"), client

    def test_run_posts_to_exec_endpoint(self):
        """commands.run() should POST to /v1/sandboxes/{id}/exec."""
        sandbox, client = self._make_sandbox()

        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.text = '{"exit_code": 0, "stdout": "hi\\n", "stderr": ""}'
        mock_response.json.return_value = {
            "exit_code": 0,
            "stdout": "hi\n",
            "stderr": "",
        }
        client.session.post.return_value = mock_response

        result = sandbox.commands.run(["echo", "hi"])

        call_args = client.session.post.call_args
        self.assertIn("/v1/sandboxes/sbx-test01/exec", call_args[0][0])

        self.assertIsInstance(result, ExecResult)
        self.assertEqual(result.exit_code, 0)
        self.assertEqual(result.stdout, "hi\n")
        self.assertEqual(result.stderr, "")

    def test_run_returns_exec_result_fields(self):
        """commands.run() should correctly populate all ExecResult fields."""
        sandbox, client = self._make_sandbox()

        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.text = '{"exit_code": 1, "stdout": "out", "stderr": "err", "artifacts": ["a.txt"]}'
        mock_response.json.return_value = {
            "exit_code": 1,
            "stdout": "out",
            "stderr": "err",
            "artifacts": ["a.txt"],
        }
        client.session.post.return_value = mock_response

        result = sandbox.commands.run(["false"])

        self.assertEqual(result.exit_code, 1)
        self.assertEqual(result.stdout, "out")
        self.assertEqual(result.stderr, "err")
        self.assertEqual(result.artifacts, ["a.txt"])


class TestSandboxKill(unittest.TestCase):
    """Tests for sandbox.kill()."""

    def test_kill_calls_delete(self):
        """sandbox.kill() should send DELETE to /v1/sandboxes/{id}."""
        client = Client("http://localhost:8080")
        client.session = MagicMock()
        sandbox = SandboxHandle(client, "sbx-kill01")

        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.text = ""
        client.session.delete.return_value = mock_response

        sandbox.kill()

        call_args = client.session.delete.call_args
        self.assertIn("/v1/sandboxes/sbx-kill01", call_args[0][0])


class TestSandboxInfo(unittest.TestCase):
    """Tests for sandbox.info()."""

    def test_info_calls_get_and_returns_sandbox_info(self):
        """sandbox.info() should GET /v1/sandboxes/{id} and return SandboxInfo."""
        client = Client("http://localhost:8080")
        client.session = MagicMock()
        sandbox = SandboxHandle(client, "sbx-info01")

        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.text = '{"id": "sbx-info01", "state": "ready"}'
        mock_response.json.return_value = {"id": "sbx-info01", "state": "ready"}
        client.session.get.return_value = mock_response

        info = sandbox.info()

        call_args = client.session.get.call_args
        self.assertIn("/v1/sandboxes/sbx-info01", call_args[0][0])

        self.assertIsInstance(info, SandboxInfo)
        self.assertEqual(info.id, "sbx-info01")
        self.assertEqual(info.state, "ready")


if __name__ == "__main__":
    unittest.main()
