---
sidebar_position: 11
title: Production Deployment
description: Deploy Sandforge in cloud infrastructure, configure systemd daemons, and set up reverse proxies.
---

# Production Deployment 🚀

This document outlines best practices for hosting the Sandforge supervisor and scaling sandbox microVMs inside cloud infrastructure environments.

---

## ☁️ 1. Cloud Provider Prerequisites

Since Sandforge utilizes bare-metal virtualization layers (KVM on Linux), you must host the supervisor on instances that support **nested hardware virtualization**.

### Google Cloud Platform (GCP)
* **Family:** N2, N1, or C2 machine types.
* **Configuration:** Enable nested virtualization in your custom image setup:
  ```bash
  gcloud compute instances create sandforge-host \
      --machine-type=n2-standard-4 \
      --image-project=ubuntu-os-cloud \
      --image-family=ubuntu-2204-lts \
      --enable-nested-virtualization
  ```

### Amazon Web Services (AWS)
* **Family:** Metal instances (e.g., `c5.metal`, `m6i.metal`) or instances built on the **Nitro Hypervisor** running vCPU allocations (AWS supports nested virtualization on Nitro vCPU instances like `c6i`).
* **Prerequisite:** KVM must be fully active on the host (`/dev/kvm` must exist).

---

## ⚙️ 2. Systemd Service Daemon

Create a systemd unit file to ensure the Sandforge Supervisor daemon initializes automatically on host system boots, manages logs, and auto-restarts upon crashes.

### Create `/etc/systemd/system/sandforge.service`

```ini
[Unit]
Description=Sandforge Sandbox Supervisor Daemon
After=network.target

[Service]
Type=simple
User=sandforge
Group=kvm
Environment=SANDFORGE_TOKEN=sf_live_a1b2c3d4e5f6g7h8i9j0
ExecStart=/usr/local/bin/sandforge supervisor --port 8585 --host 127.0.0.1 --images /var/lib/sandforge/images
Restart=on-failure
RestartSec=5s
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

### Reload and Start the Daemon
```bash
sudo systemctl daemon-reload
sudo systemctl enable sandforge
sudo systemctl start sandforge
sudo systemctl status sandforge
```

---

## 🔒 3. Secure Reverse Proxy (Nginx)

Never expose the Sandforge HTTP port `8585` directly to the open internet. Set up an Nginx reverse proxy to handle SSL termination, rate-limiting, and client token inspection.

### Configuration (`/etc/nginx/sites-available/sandforge.conf`)

```nginx
limit_req_zone $binary_remote_addr zone=api_limit:10m rate=30r/m;

server {
    listen 443 ssl http2;
    server_name sandbox.my-agent-platform.com;

    ssl_certificate /etc/letsencrypt/live/sandbox.my-agent-platform.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/sandbox.my-agent-platform.com/privkey.pem;

    location /v1/ {
        limit_req zone=api_limit burst=10 delay=5;

        proxy_pass http://127.0.0.1:8585;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Keepalive limits for hypervisor command streams
        proxy_read_timeout 300s;
        proxy_connect_timeout 75s;
      }
}
```
```bash
# Link configuration and test Nginx syntax
sudo ln -s /etc/nginx/sites-available/sandforge.conf /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl restart nginx
```
