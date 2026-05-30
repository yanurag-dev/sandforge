"""Unit tests for the Sandforge Python SDK client."""

import unittest
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


class TestFilesAPI(unittest.TestCase):
    """Tests for sandbox.files.*"""

    def _make_sandbox(self):
        client = Client("http://localhost:8080")
        client.session = MagicMock()
        return SandboxHandle(client, "sbx-fs01"), client

    def _mock_put(self, client, body):
        resp = MagicMock()
        resp.status_code = 200
        resp.text = body
        resp.json.return_value = __import__("json").loads(body)
        client.session.put = MagicMock(return_value=resp)

    def _mock_get(self, client, body):
        resp = MagicMock()
        resp.status_code = 200
        resp.text = body
        resp.json.return_value = __import__("json").loads(body)
        client.session.get = MagicMock(return_value=resp)

    def test_read_returns_text_by_default(self):
        sandbox, client = self._make_sandbox()
        payload = __import__("json").dumps({"data": list(b"hello")})
        self._mock_get(client, payload)
        content = sandbox.files.read("/tmp/hello.txt")
        self.assertIsInstance(content, str)
        self.assertEqual(content, "hello")

    def test_read_returns_bytes_when_requested(self):
        sandbox, client = self._make_sandbox()
        payload = __import__("json").dumps({"data": list(b"hello")})
        self._mock_get(client, payload)
        content = sandbox.files.read("/tmp/hello.txt", as_bytes=True)
        self.assertIsInstance(content, bytes)
        self.assertEqual(content, b"hello")

    def test_write_puts_to_files_endpoint(self):
        sandbox, client = self._make_sandbox()
        self._mock_put(client, '{"size": 5}')
        n = sandbox.files.write("/tmp/hello.txt", "hello")
        url = client.session.put.call_args[0][0]
        self.assertIn(f"/v1/sandboxes/{sandbox.id}/files", url)
        self.assertEqual(n, 5)

    def test_list_returns_entry_infos(self):
        from sandforge.types import EntryInfo
        sandbox, client = self._make_sandbox()
        payload = '{"entries": [{"name": "a.txt", "path": "/tmp/a.txt", "size": 3, "isDir": false, "modTime": "2025-01-01T00:00:00Z"}]}'
        self._mock_get(client, payload)
        entries = sandbox.files.list("/tmp")
        self.assertEqual(len(entries), 1)
        self.assertIsInstance(entries[0], EntryInfo)
        self.assertEqual(entries[0].name, "a.txt")

    def test_stat_returns_entry_info(self):
        from sandforge.types import EntryInfo
        sandbox, client = self._make_sandbox()
        payload = '{"name": "a.txt", "path": "/tmp/a.txt", "size": 3, "isDir": false, "modTime": "2025-01-01T00:00:00Z"}'
        self._mock_get(client, payload)
        info = sandbox.files.stat("/tmp/a.txt")
        self.assertIsInstance(info, EntryInfo)
        self.assertEqual(info.size, 3)

    def test_exists_true_on_success(self):
        sandbox, client = self._make_sandbox()
        payload = '{"name": "a.txt", "path": "/tmp/a.txt", "size": 3, "isDir": false, "modTime": "2025-01-01T00:00:00Z"}'
        self._mock_get(client, payload)
        self.assertTrue(sandbox.files.exists("/tmp/a.txt"))

    def test_exists_false_on_error(self):
        sandbox, client = self._make_sandbox()
        resp = MagicMock()
        resp.status_code = 422
        resp.text = '{"error": "not found"}'
        resp.json.return_value = {"error": "not found"}
        client.session.get = MagicMock(return_value=resp)
        self.assertFalse(sandbox.files.exists("/tmp/missing.txt"))


class TestGitAPI(unittest.TestCase):
    """Tests for sandbox.git.*"""

    def _make_sandbox(self):
        client = Client("http://localhost:8080")
        client.session = MagicMock()
        return SandboxHandle(client, "sbx-git01"), client

    def _mock_exec(self, client, stdout="", exit_code=0):
        resp = MagicMock()
        resp.status_code = 200
        body = __import__("json").dumps({"exit_code": exit_code, "stdout": stdout, "stderr": ""})
        resp.text = body
        resp.json.return_value = __import__("json").loads(body)
        client.session.post = MagicMock(return_value=resp)

    def test_clone_runs_git_clone(self):
        sandbox, client = self._make_sandbox()
        self._mock_exec(client)
        sandbox.git.clone("https://github.com/example/repo.git")
        payload = client.session.post.call_args[1]["json"]
        self.assertEqual(payload["command"][0], "git")
        self.assertIn("clone", payload["command"])

    def test_init_runs_git_init(self):
        sandbox, client = self._make_sandbox()
        self._mock_exec(client)
        sandbox.git.init("/workspace")
        payload = client.session.post.call_args[1]["json"]
        self.assertEqual(payload["command"], ["git", "init"])
        self.assertEqual(payload["cwd"], "/workspace")

    def test_status_returns_git_status(self):
        from sandforge.types import GitStatus
        sandbox, client = self._make_sandbox()
        call_count = [0]
        responses = [
            {"exit_code": 0, "stdout": "main\n", "stderr": ""},
            {"exit_code": 0, "stdout": "", "stderr": ""},
        ]
        def side_effect(url, **kwargs):
            resp = MagicMock()
            resp.status_code = 200
            body = __import__("json").dumps(responses[call_count[0]])
            resp.text = body
            resp.json.return_value = responses[call_count[0]]
            call_count[0] += 1
            return resp
        client.session.post.side_effect = side_effect

        status = sandbox.git.status("/workspace")
        self.assertIsInstance(status, GitStatus)
        self.assertEqual(status.branch, "main")
        self.assertTrue(status.clean)

    def test_branches_parses_output(self):
        sandbox, client = self._make_sandbox()
        self._mock_exec(client, stdout="* main\n  dev\n  feature/x\n")
        result = sandbox.git.branches("/workspace")
        self.assertIn("main", result)
        self.assertIn("dev", result)
        self.assertIn("feature/x", result)


if __name__ == "__main__":
    unittest.main()
