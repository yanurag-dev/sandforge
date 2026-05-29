#!/usr/bin/env python3
"""Quick start example for the Sandforge Python SDK.

This example demonstrates:
1. Creating a sandbox
2. Running commands
3. Checking results
4. Destroying the sandbox
"""

import sys
from pathlib import Path

# Add the parent directory to the path to import sandforge
sys.path.insert(0, str(Path(__file__).parent.parent))

from sandforge import Client, SandboxSpec, NetworkError, SandforgeException


def main():
    """Run the quick start example."""
    # Create a client pointing to the local control plane
    # Make sure the server is running: make run
    client = Client("http://localhost:8080")

    print("Sandforge Python SDK Quick Start Example")
    print("=" * 50)

    try:
        # Create a sandbox
        print("\n1. Creating a sandbox...")
        sandbox = client.create_sandbox()
        print(f"   Created sandbox: {sandbox.id}")

        # Get sandbox info
        print("\n2. Getting sandbox info...")
        info = sandbox.info()
        print(f"   State: {info.state}")

        # Run a simple echo command
        print("\n3. Running command: echo 'Hello, Sandforge!'")
        result = sandbox.commands.run(["echo", "Hello, Sandforge!"])
        print(f"   Exit code: {result.exit_code}")
        print(f"   Output: {result.stdout.strip()}")

        # Run a command that uses environment variables
        print("\n4. Running command with environment variables")
        result = sandbox.commands.run(
            command=["sh", "-c", "echo $MY_VAR"],
            env={"MY_VAR": "Environment variable value"},
        )
        print(f"   Exit code: {result.exit_code}")
        print(f"   Output: {result.stdout.strip()}")

        # Run a command in a custom working directory
        print("\n5. Running command in /tmp directory")
        result = sandbox.commands.run(
            ["pwd"],
            cwd="/tmp",
        )
        print(f"   Exit code: {result.exit_code}")
        print(f"   Working dir: {result.stdout.strip()}")

        # Run a command that outputs to stderr
        print("\n6. Running command that outputs to stderr")
        result = sandbox.commands.run(["sh", "-c", "echo 'Error message' >&2"])
        print(f"   Exit code: {result.exit_code}")
        print(f"   Stderr: {result.stderr.strip()}")

        # Run a command with a non-zero exit code
        print("\n7. Running command that fails")
        result = sandbox.commands.run(["sh", "-c", "exit 42"])
        print(f"   Exit code: {result.exit_code}")

        # Run a more complex shell command
        print("\n8. Running a more complex shell command")
        result = sandbox.commands.run(
            [
                "sh",
                "-c",
                "for i in 1 2 3; do echo 'Line '$i; done",
            ]
        )
        print(f"   Exit code: {result.exit_code}")
        print(f"   Output:\n{result.stdout}")

        # Clean up
        print("\n9. Destroying sandbox...")
        sandbox.kill()
        print("   Sandbox destroyed")

        print("\n" + "=" * 50)
        print("Example completed successfully!")

    except NetworkError as e:
        print(f"\nError: Network error - {e}")
        print("Make sure the Sandforge control plane is running:")
        print("  cd /path/to/sandforge && make run")
        sys.exit(1)
    except SandforgeException as e:
        print(f"\nError: Sandforge error - {e}")
        sys.exit(1)
    except Exception as e:
        print(f"\nError: Unexpected error - {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
