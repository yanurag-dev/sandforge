"""HTTP client for communicating with the Sandforge control plane."""

import json
import secrets
from typing import Dict, Any, Optional
import requests

from .pty import PTYNamespace
from .types import (
    SandboxSpec,
    ExecRequest,
    ExecResult,
    SandboxInfo,
    EntryInfo,
    GitStatus,
    SandforgeException,
    NetworkError,
    SandboxNotFoundError,
)


class Client:
    """Sandforge control plane HTTP client.

    Example:
        client = Client("http://localhost:8080")
        sandbox = client.create_sandbox(SandboxSpec())
        result = client.exec(sandbox.id, ExecRequest(command=["echo", "hello"]))
    """

    def __init__(self, base_url: str, timeout: int = 60):
        """Initialize the Sandforge client.

        Args:
            base_url: The control plane base URL (e.g., "http://localhost:8080").
            timeout: Request timeout in seconds.
        """
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self.session = requests.Session()

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.session.close()
        return False

    def close(self) -> None:
        """Close the underlying HTTP session."""
        self.session.close()

    def create_sandbox(self, spec: Optional[SandboxSpec] = None) -> "SandboxHandle":
        """Create a new sandbox.

        Args:
            spec: SandboxSpec for the sandbox. If None, uses defaults.

        Returns:
            SandboxHandle: A handle to the created sandbox.

        Raises:
            NetworkError: If communication with the control plane fails.
            SandforgeException: If sandbox creation fails.
        """
        if spec is None:
            spec = SandboxSpec()

        sandbox_id = self._generate_id()
        payload = {
            "id": sandbox_id,
            "spec": spec.to_dict(),
        }

        response = self._do("POST", "/v1/sandboxes", payload)
        return SandboxHandle(self, response.get("id", sandbox_id))

    def exec(self, sandbox_id: str, request: ExecRequest) -> ExecResult:
        """Execute a command in a sandbox.

        Args:
            sandbox_id: The sandbox ID.
            request: The execution request.

        Returns:
            ExecResult: The command execution result.

        Raises:
            NetworkError: If communication with the control plane fails.
            SandforgeException: If execution fails.
        """
        payload = request.to_dict()
        response = self._do("POST", f"/v1/sandboxes/{sandbox_id}/exec", payload)
        return ExecResult.from_dict(response)

    def get_status(self, sandbox_id: str) -> str:
        """Get the current state of a sandbox.

        Args:
            sandbox_id: The sandbox ID.

        Returns:
            str: The sandbox state (e.g., "ready", "executing", "destroyed").

        Raises:
            NetworkError: If communication with the control plane fails.
            SandboxNotFoundError: If the sandbox is not found.
        """
        response = self._do("GET", f"/v1/sandboxes/{sandbox_id}", None)
        return response.get("state", "unknown")

    def get_info(self, sandbox_id: str) -> SandboxInfo:
        """Get detailed information about a sandbox.

        Args:
            sandbox_id: The sandbox ID.

        Returns:
            SandboxInfo: Information about the sandbox.

        Raises:
            NetworkError: If communication with the control plane fails.
            SandboxNotFoundError: If the sandbox is not found.
        """
        response = self._do("GET", f"/v1/sandboxes/{sandbox_id}", None)
        return SandboxInfo.from_dict(response)

    def destroy(self, sandbox_id: str) -> None:
        """Destroy a sandbox.

        Args:
            sandbox_id: The sandbox ID.

        Raises:
            NetworkError: If communication with the control plane fails.
            SandforgeException: If destruction fails.
        """
        self._do("DELETE", f"/v1/sandboxes/{sandbox_id}", None)

    # ─── Private Methods ───────────────────────────────────────────────────────

    def _do(self, method: str, path: str, body: Optional[Dict[str, Any]]) -> Dict[str, Any]:
        """Execute an HTTP request to the control plane.

        Args:
            method: HTTP method (GET, POST, DELETE, etc.).
            path: API path (e.g., "/v1/sandboxes").
            body: Request body (or None for GET/DELETE).

        Returns:
            dict: Parsed JSON response.

        Raises:
            NetworkError: If the request fails.
            SandboxNotFoundError: If the resource is not found.
            SandforgeException: If the response indicates an error.
        """
        url = self.base_url + path
        headers = {"Content-Type": "application/json"}

        try:
            if method == "GET":
                resp = self.session.get(url, headers=headers, timeout=self.timeout)
            elif method == "POST":
                resp = self.session.post(
                    url, json=body, headers=headers, timeout=self.timeout
                )
            elif method == "PUT":
                resp = self.session.put(
                    url, json=body, headers=headers, timeout=self.timeout
                )
            elif method == "DELETE":
                resp = self.session.delete(url, headers=headers, timeout=self.timeout)
            else:
                raise ValueError(f"Unsupported HTTP method: {method}")

        except requests.RequestException as e:
            raise NetworkError(f"Request failed: {e}") from e

        # Handle error responses
        if resp.status_code >= 400:
            self._handle_error_response(resp)

        # Parse response body
        if resp.text:
            try:
                return resp.json()
            except json.JSONDecodeError as e:
                raise SandforgeException(f"Invalid JSON response: {e}") from e
        return {}

    def _handle_error_response(self, resp: requests.Response) -> None:
        """Parse and raise an appropriate exception from an error response.

        Args:
            resp: The HTTP response object.

        Raises:
            SandboxNotFoundError: If the resource is not found (404).
            SandforgeException: For other error responses.
        """
        status = resp.status_code
        try:
            error_data = resp.json()
            error_msg = error_data.get("error", "Unknown error")
        except json.JSONDecodeError:
            error_msg = resp.text or f"HTTP {status}"

        if status == 404:
            raise SandboxNotFoundError(f"Sandbox not found: {error_msg}")
        else:
            raise SandforgeException(f"HTTP {status}: {error_msg}")

    @staticmethod
    def _generate_id() -> str:
        """Generate a unique sandbox ID.

        Returns:
            str: A sandbox ID in the form "sbx-<hex>".
        """
        random_bytes = secrets.token_hex(8)
        return f"sbx-{random_bytes}"


class SandboxHandle:
    """A handle to a sandbox, providing convenient command and file operations.

    Example:
        sandbox = client.create_sandbox()
        result = sandbox.commands.run(["echo", "hello"])
        content = sandbox.files.read("/etc/hostname")
        sandbox.kill()
        info = sandbox.info()
    """

    def __init__(self, client: Client, sandbox_id: str):
        """Initialize a sandbox handle.

        Args:
            client: The Sandforge client.
            sandbox_id: The sandbox ID.
        """
        self.id = sandbox_id
        self._client = client
        self.commands = CommandsAPI(self)
        self.files = FilesAPI(self)
        self.git = GitAPI(self)
        self.pty = PTYNamespace(self)

    def kill(self) -> None:
        """Destroy the sandbox.

        Raises:
            NetworkError: If communication with the control plane fails.
        """
        self._client.destroy(self.id)

    def info(self) -> SandboxInfo:
        """Get information about the sandbox.

        Returns:
            SandboxInfo: Current sandbox state and ID.

        Raises:
            NetworkError: If communication with the control plane fails.
        """
        return self._client.get_info(self.id)


class CommandsAPI:
    """Commands API for executing commands in a sandbox."""

    def __init__(self, sandbox: SandboxHandle):
        """Initialize the commands API.

        Args:
            sandbox: The parent SandboxHandle.
        """
        self._sandbox = sandbox

    def run(
        self,
        command: list,
        cwd: str = "/",
        env: Optional[Dict[str, str]] = None,
        timeout_sec: int = 60,
    ) -> ExecResult:
        """Run a command in the sandbox.

        Args:
            command: Command and arguments as a list (e.g., ["echo", "hello"]).
            cwd: Working directory for the command (default: "/").
            env: Environment variables as a dict (default: empty).
            timeout_sec: Command timeout in seconds (default: 60).

        Returns:
            ExecResult: Command execution result with exit code, stdout, stderr.

        Raises:
            NetworkError: If communication with the control plane fails.
            SandforgeException: If execution fails.
        """
        if env is None:
            env = {}

        request = ExecRequest(
            command=command,
            cwd=cwd,
            env=env,
            timeout_sec=timeout_sec,
        )
        return self._sandbox._client.exec(self._sandbox.id, request)


class FilesAPI:
    """Files API for filesystem operations inside a sandbox."""

    def __init__(self, sandbox: "SandboxHandle"):
        self._sandbox = sandbox

    def read(self, path: str, as_bytes: bool = False):
        """Read a file from the sandbox.

        Args:
            path: Path to the file inside the sandbox.
            as_bytes: If True, return raw bytes. Default returns str.

        Returns:
            str or bytes: File contents.
        """
        resp = self._sandbox._client._do(
            "GET", f"/v1/sandboxes/{self._sandbox.id}/files/read?path={path}", None
        )
        data = bytes(resp.get("data", []))
        return data if as_bytes else data.decode()

    def write(self, path: str, data) -> int:
        """Write data to a file inside the sandbox.

        Args:
            path: Destination path inside the sandbox.
            data: str or bytes to write.

        Returns:
            int: Number of bytes written.
        """
        if isinstance(data, str):
            data = data.encode()
        payload = {"guest_path": path, "data": list(data)}
        resp = self._sandbox._client._do(
            "PUT", f"/v1/sandboxes/{self._sandbox.id}/files", payload
        )
        return resp.get("size", 0)

    def list(self, path: str) -> list:
        """List directory contents inside the sandbox.

        Args:
            path: Directory path inside the sandbox.

        Returns:
            List[EntryInfo]: Directory entries.
        """
        resp = self._sandbox._client._do(
            "GET", f"/v1/sandboxes/{self._sandbox.id}/files?path={path}", None
        )
        return [EntryInfo.from_dict(e) for e in resp.get("entries", [])]

    def stat(self, path: str) -> EntryInfo:
        """Return metadata for a path inside the sandbox.

        Args:
            path: Path inside the sandbox.

        Returns:
            EntryInfo: Metadata for the path.
        """
        resp = self._sandbox._client._do(
            "GET", f"/v1/sandboxes/{self._sandbox.id}/stat?path={path}", None
        )
        return EntryInfo.from_dict(resp)

    def exists(self, path: str) -> bool:
        """Return True if the path exists inside the sandbox."""
        try:
            self.stat(path)
            return True
        except SandforgeException:
            return False

    def remove(self, path: str) -> ExecResult:
        """Delete a file or directory inside the sandbox via `rm -rf`."""
        return self._sandbox._client.exec(
            self._sandbox.id,
            ExecRequest(command=["rm", "-rf", path], cwd="/", timeout_sec=30),
        )


class GitAPI:
    """Git API — shell facade over `commands.run()` for common git operations."""

    def __init__(self, sandbox: "SandboxHandle"):
        self._sandbox = sandbox

    def _exec(self, args: list, cwd: str = "/") -> ExecResult:
        return self._sandbox._client.exec(
            self._sandbox.id,
            ExecRequest(command=["git"] + args, cwd=cwd, timeout_sec=120),
        )

    def clone(self, url: str, dest: str = ".", depth: Optional[int] = None) -> ExecResult:
        args = ["clone"]
        if depth:
            args += ["--depth", str(depth)]
        args += [url, dest]
        return self._exec(args)

    def init(self, cwd: str) -> ExecResult:
        return self._exec(["init"], cwd)

    def add(self, paths, cwd: str) -> ExecResult:
        if isinstance(paths, str):
            paths = [paths]
        return self._exec(["add"] + paths, cwd)

    def commit(self, message: str, cwd: str) -> ExecResult:
        return self._exec(["commit", "-m", message], cwd)

    def push(self, cwd: str, remote: str = "origin", branch: str = "HEAD") -> ExecResult:
        return self._exec(["push", remote, branch], cwd)

    def pull(self, cwd: str, remote: str = "origin") -> ExecResult:
        return self._exec(["pull", remote], cwd)

    def status(self, cwd: str) -> GitStatus:
        branch_result = self._exec(["rev-parse", "--abbrev-ref", "HEAD"], cwd)
        status_result = self._exec(["status", "--porcelain"], cwd)
        return GitStatus(
            branch=branch_result.stdout.strip(),
            clean=status_result.stdout.strip() == "",
            stdout=status_result.stdout,
        )

    def branches(self, cwd: str) -> list:
        result = self._exec(["branch", "--list"], cwd)
        return [
            b.lstrip("* ").strip()
            for b in result.stdout.splitlines()
            if b.strip()
        ]
