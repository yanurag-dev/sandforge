---
sidebar_position: 4
title: Authentication & Secrets
description: Learn how to manage credentials, access keys, and inject secure environment tokens.
---

# Authentication & Secrets 🔐

Autonomous coding agents often require access tokens (such as `GITHUB_TOKEN`, `npm_token`, or API keys) to download private packages or interact with version control. Sandforge provides secure, isolated methods to pass these credentials into the guest VM without exposing them to other tasks or leaving artifacts in the guest image.

---

## 🔑 Token-Based Supervisor Authentication

When interacting with the Sandforge Supervisor daemon via the REST API or SDK, you must authenticate using a cryptographically secure token.

### Setting the Host Token
Set the `SANDFORGE_TOKEN` environment variable on the host where the Sandforge Supervisor is running:

```bash
export SANDFORGE_TOKEN="sf_live_a1b2c3d4e5f6g7h8i9j0"
```

### Passing Authentication in API Requests
Include the token in the HTTP request header:

```bash
curl -H "Authorization: Bearer sf_live_a1b2c3d4e5f6g7h8i9j0" \
     https://localhost:8585/v1/sandboxes
```

---

## 🔒 Secure Secret Injection (Transient Env)

> [!CAUTION]
> **Security Hazard:**
> Never write API secrets or private key files directly to disk volumes or embed them into custom container images. If an untrusted package has write access, it could scan the disk and leak your credentials.

Sandforge solves this by injecting secrets into **transient environment memory** when launching a task. Secrets are held strictly in memory and are discarded immediately when the container process exits.

### How to Inject Secrets via CLI

Pass secrets using the `--env` or `-e` flags:

```bash
./sandforge run \
  -e GITHUB_TOKEN="ghp_securepassword" \
  -e NPM_TOKEN="npm_secret" \
  "git clone https://x-access-token:\$GITHUB_TOKEN@github.com/my-org/private-repo.git"
```

### How to Inject Secrets via Go SDK

```go
config := &api.Config{
    CPU:      2,
    MemoryMB: 2048,
    // Inject secrets into transient environment maps
    Env: map[string]string{
        "OPENAI_API_KEY": "sk-proj-xxxxxx",
    },
}
```

---

## 🏛️ Cryptographic Integrity Checks

Sandforge validates that the execution payload matches the authorized instruction before invoking the guest process.

```mermaid
sequenceDiagram
    participant Host CLI/SDK
    participant Policy Engine
    participant Supervisor Daemon
    participant Guest VM

    Host CLI/SDK->>Policy Engine: Submit command + env secrets
    Note over Policy Engine: Resolves environment paths & signs command payload hash
    Policy Engine->>Supervisor Daemon: Forward signed instruction
    Supervisor Daemon->>Guest VM: Dispatch process execute request
    Note over Guest VM: Verifies hash signature matches instruction before execution
    Guest VM-->>Supervisor Daemon: Stream stdout/stderr (Secrets scrubbed)
```

By hashing and signing the task request, Sandforge guarantees that the command cannot be modified by third-party processes on the host.
