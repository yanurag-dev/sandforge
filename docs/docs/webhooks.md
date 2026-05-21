---
sidebar_position: 7
title: Webhooks & Events
description: Listen to real-time sandbox status, timeout events, and security violations.
---

# Webhooks & Events 📡

The Sandforge Supervisor can dispatch HTTP POST requests (webhooks) to notify your agent control plane about microVM state transitions, resource exhaustions, and policy violation attempts.

---

## ⚙️ Configuring Webhook Subscriptions

To receive webhooks, register your endpoint in the supervisor configuration or pass a `--webhook` parameter to the supervisor command:

```bash
./sandforge supervisor --webhook-url="https://api.my-agent.com/webhooks" --webhook-secret="sf_wh_secret_xyz123"
```

---

## 🏷️ Webhook Event Types

| Event Name | Description |
| :--- | :--- |
| `sandbox.created` | The guest VM metadata has been created and resource allocation is verified. |
| `sandbox.started` | The guest VM has successfully booted and the VSOCK connection is established. |
| `task.timeout` | A command inside the guest VM exceeded its `timeout_seconds` allotment and was terminated. |
| `security.violation` | The Policy Engine intercepted an unapproved filesystem write or network access. |
| `sandbox.destroyed` | The guest VM was fully stopped and its CPU/RAM allocations returned to the host. |

---

## 📦 Webhook Payload Examples

### Security Violation Alert (`security.violation`)
Dispatched when a command attempts to read or write a blocked host directory:

```json
{
  "event": "security.violation",
  "id": "evt_9a8b7c6d",
  "timestamp": "2026-05-22T00:25:11Z",
  "data": {
    "sandbox_id": "sb_7f8g9h0i",
    "violation_type": "filesystem_escape",
    "attempted_command": "rm -rf /host/var/log",
    "blocked_path": "/host/var/log",
    "action": "terminated_sandbox"
  }
}
```

### Task Timeout Alert (`task.timeout`)
Dispatched when an agent command runs longer than its allowed limit:

```json
{
  "event": "task.timeout",
  "id": "evt_1a2b3c4d",
  "timestamp": "2026-05-22T00:27:00Z",
  "data": {
    "sandbox_id": "sb_7f8g9h0i",
    "command": "npm install heavy-dependency-package",
    "limit_seconds": 60,
    "cpu_consumed": 12.4
  }
}
```

---

## 🔒 Cryptographic Verification

To ensure that incoming webhook payloads originate from your trusted Sandforge Supervisor daemon and were not tampered with during transit, verify the `X-Sandforge-Signature` header.

The signature is computed using **HMAC-SHA256** with your registered webhook secret:

```text
X-Sandforge-Signature: t=1779435911,v1=a5c8e3c6f8a4...
```

### Verification Example (Node.js/Express)

```javascript
import crypto from 'crypto';

function verifyWebhook(req, res, next) {
  const signatureHeader = req.headers['x-sandforge-signature'];
  const secret = process.env.SANDFORGE_WEBHOOK_SECRET;

  const [tPart, v1Part] = signatureHeader.split(',');
  const timestamp = tPart.split('=')[1];
  const signature = v1Part.split('=')[1];

  // Prevent replay attacks
  if (Math.abs(Date.now() / 1000 - Number(timestamp)) > 300) {
    return res.status(400).send('Replay attack suspected.');
  }

  const signedPayload = `${timestamp}.${JSON.stringify(req.body)}`;
  const expectedSignature = crypto
    .createHmac('sha256', secret)
    .update(signedPayload)
    .digest('hex');

  if (crypto.timingSafeEqual(Buffer.from(signature), Buffer.from(expectedSignature))) {
    next(); // Authenticated successfully
  } else {
    res.status(403).send('Invalid signature.');
  }
}
```
