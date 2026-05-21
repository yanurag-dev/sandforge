---
sidebar_position: 5
title: API Reference
description: Complete REST API documentation for the Sandforge Supervisor daemon.
---

# API Reference 🔌

The Sandforge Supervisor runs a secure local HTTP API (by default on port `8585`) allowing control planes to provision, monitor, and destroy hypervisor-level sandboxes.

---

## 1. Create a Sandbox

Provision a new isolated guest microVM.

* **Endpoint:** `POST /v1/sandboxes`
* **Content-Type:** `application/json`

### Request Body
```json
{
  "cpu": 2,
  "memory_mb": 2048,
  "network": "offline",
  "mounts": [
    {
      "host_path": "/Users/developer/project",
      "guest_path": "/workspace",
      "read_only": false
    }
  ]
}
```

### Example Request
```bash
curl -X POST http://localhost:8585/v1/sandboxes \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer sf_live_token" \
     -d '{
       "cpu": 2,
       "memory_mb": 2048,
       "network": "offline",
       "mounts": [{"host_path": "/Users/anurag/app", "guest_path": "/workspace", "read_only": false}]
     }'
```

### Example Response
```json
{
  "id": "sb_7f8g9h0i",
  "status": "starting",
  "cpu": 2,
  "memory_mb": 2048,
  "network": "offline",
  "created_at": "2026-05-22T00:25:11Z"
}
```

---

## 2. List Active Sandboxes

Retrieve metadata for all running and preparing sandboxes.

* **Endpoint:** `GET /v1/sandboxes`

### Example Request
```bash
curl http://localhost:8585/v1/sandboxes \
     -H "Authorization: Bearer sf_live_token"
```

### Example Response
```json
[
  {
    "id": "sb_7f8g9h0i",
    "status": "running",
    "cpu": 2,
    "memory_mb": 2048,
    "network": "offline"
  }
]
```

---

## 3. Execute a Command

Run a shell command or script inside an active guest VM container.

* **Endpoint:** `POST /v1/sandboxes/{id}/execute`
* **Content-Type:** `application/json`

### Request Body
```json
{
  "command": "npm run test",
  "timeout_seconds": 60,
  "env": {
    "NODE_ENV": "test"
  }
}
```

### Example Request
```bash
curl -X POST http://localhost:8585/v1/sandboxes/sb_7f8g9h0i/execute \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer sf_live_token" \
     -d '{
       "command": "npm run test",
       "timeout_seconds": 60,
       "env": {"NODE_ENV": "test"}
     }'
```

### Example Response
```json
{
  "exit_code": 0,
  "stdout": "✓ All tests passed (12 tests completed)\n",
  "stderr": "",
  "timed_out": false
}
```

---

## 4. Destroy a Sandbox

Gracefully terminate a microVM guest kernel and reclaim CPU/RAM host allocations.

* **Endpoint:** `DELETE /v1/sandboxes/{id}`

### Example Request
```bash
curl -X DELETE http://localhost:8585/v1/sandboxes/sb_7f8g9h0i \
     -H "Authorization: Bearer sf_live_token"
```

### Example Response
```json
{
  "id": "sb_7f8g9h0i",
  "status": "terminated",
  "message": "Sandbox resources reclaimed successfully."
}
```
