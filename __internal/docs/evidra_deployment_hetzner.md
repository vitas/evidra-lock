# Evidra API — Production Deployment on Hetzner Cloud

**Date:** 2026-02-26
**Stack:** Terraform (IaC) → Hetzner Cloud → Traefik v3 (reverse proxy + SSL) → evidra-api (Go) + PostgreSQL
**Target:** Dedicated Hetzner Cloud VM (separate from samebits.com VM)

---

## 0. Repository Strategy

| Repo | Visibility | Purpose |
|---|---|---|
| `vitas/evidra` | Public | Source code, tests, CI → builds & pushes images to GHCR |
| `vitas/evidra-gitops` | Public | Argo CD UI extension (separate product) |
| `vitas/evidra-infra` | **Private** | Production deployment: docker-compose, .env, Traefik config, deploy scripts |

**Why a separate infra repo:**
- Open source repo should not contain deployment secrets, server IPs, or infrastructure specifics.
- `evidra-infra` holds everything needed to run production — and nothing else.
- CI in `vitas/evidra` pushes image to GHCR. Deploy workflow in `evidra-infra` pulls and restarts.

**Two-VM architecture:**
```
VM1 (ARM, existing)          VM2 (CX22, new)
├── Caddy                    ├── Traefik v3
├── samebits.com site        ├── evidra-api
└── (not touched)            ├── PostgreSQL 17
                             └── Hetzner Volume (/mnt/data)
```

These are fully independent. Different VMs, different IPs, different domains, different reverse proxies.

### evidra-infra repo structure

```
evidra-infra/
├── README.md                           # Setup instructions
├── .gitignore                          # .env, acme.json, *.tfstate, .terraform/
├── terraform/
│   ├── main.tf                         # Hetzner resources: server, volume, firewall, SSH key
│   ├── variables.tf                    # Input variables (location, server_type, domain)
│   ├── outputs.tf                      # Server IP, volume ID
│   ├── terraform.tfvars.example        # Example values (no secrets)
│   └── versions.tf                     # Provider requirements
├── docker-compose.prod.yml             # Traefik + evidra-api + PostgreSQL
├── .env.example                        # Docker Compose env template (no secrets)
├── db/
│   └── migrations/
│       └── 001_init.sql                # Initial schema: api_keys, evaluations
├── scripts/
│   ├── bootstrap-hetzner.sh            # First-time VM setup (post-terraform)
│   ├── deploy.sh                       # Pull & restart evidra-api
│   └── health-check.sh                 # Cron health check
└── .github/
    └── workflows/
        └── deploy.yml                  # SSH deploy on manual trigger or dispatch
```

---

## 1. Infrastructure

### Hetzner Cloud — VM2 (Evidra API)

| Resource | Spec | Price (EU) | Notes |
|---|---|---|---|
| **VM** | CX22 — 2 vCPU (Intel), 4 GB RAM, 40 GB NVMe | ~€3.79/mo | Runs Traefik + API + PostgreSQL. `docker-ce` image. |
| **Volume** | 20 GB ext4, attached to VM | ~€0.88/mo | PostgreSQL data, Traefik certs, evidence logs. |
| **Firewall** | Hetzner Firewall (free) | — | Inbound: 22, 80, 443 only. |
| **IPv4** | Included with VM | — | 1 public IPv4. |
| **Traffic** | 20 TB/mo included (EU) | — | More than enough for years. |
| **Total** | | **~€4.67/mo** | |

**Location:** Nuremberg (nbg1) or Falkenstein (fsn1) — GDPR-compliant, lowest latency to Central Europe.

**Upgrade path:** CX22 → CX32 (4 vCPU, 8 GB, €6.80/mo) if PostgreSQL needs more RAM under load. Volume resize is online — no downtime.

### Volume Strategy

Volume is mounted at `/mnt/data` on the host. All persistent data lives there:

```
/mnt/data/
├── traefik/
│   └── acme.json          # Let's Encrypt certificates (600 permissions)
├── postgres/               # PostgreSQL data directory
│   └── data/
└── evidra/
    └── logs/               # Optional: structured logs
```

**Why Volume instead of root disk:**
- Root disk is tied to the VM. Delete VM = lose data.
- Volume is independent. Can be detached and attached to a new server.
- Volume supports snapshots for backup.
- 40 GB root disk is enough for OS + Docker images. Data goes on Volume.

---

## 2. DNS

Single domain — UI and API served from the same origin. No CORS, one TLS cert.

```
A     evidra.rest       → <VM_PUBLIC_IP>
CNAME www.evidra.rest   → evidra.rest
```

Traefik routes both `evidra.rest` and `www.evidra.rest` to the same container. The Go binary serves the SPA on `/` and API on `/v1/*` — no path-based splitting needed in Traefik.

No `api.` subdomain needed. Curl examples use `https://evidra.rest/v1/validate`.

---

## 3. Docker Compose — Production

```yaml
# docker-compose.prod.yml
# Evidra API — Production deployment with Traefik v3

services:
  # ─── Reverse Proxy ──────────────────────────────────────
  traefik:
    image: traefik:v3.3
    container_name: traefik
    restart: unless-stopped
    security_opt:
      - no-new-privileges:true
    command:
      # API & Dashboard
      - --api=true
      - --api.dashboard=true
      # Entrypoints
      - --entrypoints.web.address=:80
      - --entrypoints.web.http.redirections.entrypoint.to=websecure
      - --entrypoints.web.http.redirections.entrypoint.scheme=https
      - --entrypoints.websecure.address=:443
      - --entrypoints.websecure.http.tls=true
      # Let's Encrypt
      - --certificatesresolvers.letsencrypt.acme.email=${ACME_EMAIL}
      - --certificatesresolvers.letsencrypt.acme.storage=/certs/acme.json
      - --certificatesresolvers.letsencrypt.acme.httpchallenge.entrypoint=web
      # Docker provider
      - --providers.docker=true
      - --providers.docker.exposedbydefault=false
      - --providers.docker.network=evidra-net
      # Logging
      - --log.level=WARN
      - --accesslog=true
      - --accesslog.fields.headers.defaultmode=drop
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ${DATA_DIR:-/mnt/data}/traefik:/certs
    networks:
      - evidra-net
    labels:
      # Dashboard — controlled by TRAEFIK_DASHBOARD_ENABLED in .env
      # P0: enabled with BasicAuth. Later: disable and use SSH port-forward only.
      - traefik.enable=${TRAEFIK_DASHBOARD_ENABLED:-false}
      - traefik.http.routers.traefik-dashboard.rule=Host(`traefik.${DOMAIN}`)
      - traefik.http.routers.traefik-dashboard.entrypoints=websecure
      - traefik.http.routers.traefik-dashboard.tls.certresolver=letsencrypt
      - traefik.http.routers.traefik-dashboard.service=api@internal
      - traefik.http.routers.traefik-dashboard.middlewares=traefik-auth
      - traefik.http.middlewares.traefik-auth.basicauth.users=${TRAEFIK_AUTH}
    healthcheck:
      test: ["CMD", "traefik", "healthcheck"]
      interval: 30s
      timeout: 5s
      retries: 3

  # ─── Evidra API ─────────────────────────────────────────
  evidra-api:
    image: ghcr.io/vitas/evidra/evidra-api:${EVIDRA_VERSION:-latest}
    container_name: evidra-api
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    security_opt:
      - no-new-privileges:true
    read_only: true
    tmpfs:
      - /tmp:noexec,nosuid,size=10m
    environment:
      - EVIDRA_ENV=production
      - EVIDRA_LISTEN_ADDR=:8080
      - EVIDRA_SIGNING_KEY=${EVIDRA_SIGNING_KEY}
      - EVIDRA_DATABASE_URL=postgres://evidra:${PG_PASSWORD}@postgres:5432/evidra?sslmode=disable
      # To migrate DB to external host: change to postgres://user:pass@external-ip:5432/evidra?sslmode=require
      # then remove the postgres service from this file. No code changes needed.
    expose:
      - "8080"
    networks:
      - evidra-net
    labels:
      - traefik.enable=true
      # Main router: evidra.rest + www.evidra.rest → evidra-api:8080
      - traefik.http.routers.evidra.rule=Host(`${DOMAIN}`) || Host(`www.${DOMAIN}`)
      - traefik.http.routers.evidra.entrypoints=websecure
      - traefik.http.routers.evidra.tls.certresolver=letsencrypt
      - traefik.http.routers.evidra.tls.domains[0].main=${DOMAIN}
      - traefik.http.routers.evidra.tls.domains[0].sans=www.${DOMAIN}
      - traefik.http.services.evidra.loadbalancer.server.port=8080
      # Middlewares (single declaration — headers + rate limit)
      - traefik.http.routers.evidra.middlewares=evidra-headers,evidra-ratelimit
      # Security headers
      - traefik.http.middlewares.evidra-headers.headers.stsSeconds=31536000
      - traefik.http.middlewares.evidra-headers.headers.stsIncludeSubdomains=true
      - traefik.http.middlewares.evidra-headers.headers.contentTypeNosniff=true
      - traefik.http.middlewares.evidra-headers.headers.frameDeny=true
      - traefik.http.middlewares.evidra-headers.headers.referrerPolicy=strict-origin-when-cross-origin
      - traefik.http.middlewares.evidra-headers.headers.permissionsPolicy=camera=(), microphone=(), geolocation=()
      - traefik.http.middlewares.evidra-headers.headers.customResponseHeaders.Content-Security-Policy=default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'
      # Rate limiting: 10 req/s sustained (average/period = 10/1s), burst up to 50 concurrent
      # Traefik formula: rate = average ÷ period. Default period = 1s.
      # 10 req/s is generous for P0; tighten per-key in application layer later.
      - traefik.http.middlewares.evidra-ratelimit.ratelimit.average=10
      - traefik.http.middlewares.evidra-ratelimit.ratelimit.burst=50
    # NOTE: No container-level healthcheck — distroless has no wget/curl.
    # Health is verified externally:
    #   - Traefik checks backend via loadbalancer health (TCP connect to :8080)
    #   - Deploy script checks https://evidra.rest/healthz after restart
    #   - Cron health-check.sh checks every 5 min

  # ─── PostgreSQL ─────────────────────────────────────────
  postgres:
    image: postgres:17-alpine
    container_name: evidra-postgres
    restart: unless-stopped
    security_opt:
      - no-new-privileges:true
    environment:
      - POSTGRES_DB=evidra
      - POSTGRES_USER=evidra
      - POSTGRES_PASSWORD=${PG_PASSWORD}
    volumes:
      - ${DATA_DIR:-/mnt/data}/postgres/data:/var/lib/postgresql/data
    networks:
      - evidra-net
    expose:
      - "5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U evidra -d evidra"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 20s

networks:
  evidra-net:
    name: evidra-net
```

---

## 4. Database Schema

Migrations are applied by `evidra-api` on startup (embedded via `golang-migrate` or similar).
Alternatively, run manually: `docker exec -i evidra-postgres psql -U evidra evidra < db/migrations/001_init.sql`

```sql
-- db/migrations/001_init.sql

-- API keys: created via POST /v1/keys or CLI
CREATE TABLE IF NOT EXISTS api_keys (
    id          TEXT PRIMARY KEY,            -- ulid or uuid
    prefix      TEXT NOT NULL UNIQUE,        -- first 12 chars, for display (ev1_a8Fk...)
    hash        TEXT NOT NULL UNIQUE,        -- SHA-256 of full key
    label       TEXT,                        -- optional human label
    tenant_id   TEXT NOT NULL,               -- tenant/org identifier
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at  TIMESTAMPTZ,                -- NULL = active, set = revoked
    last_used   TIMESTAMPTZ
);

CREATE INDEX idx_api_keys_hash ON api_keys (hash) WHERE revoked_at IS NULL;

-- Evaluation log: optional, for dashboard "recent evaluations" and analytics
CREATE TABLE IF NOT EXISTS evaluations (
    id          TEXT PRIMARY KEY,            -- event_id from evidence record
    api_key_id  TEXT NOT NULL REFERENCES api_keys(id),
    tool        TEXT NOT NULL,
    operation   TEXT NOT NULL,
    environment TEXT,
    allowed     BOOLEAN NOT NULL,
    risk_level  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_evaluations_key_time ON evaluations (api_key_id, created_at DESC);
```

**Key lookup flow:** `POST /v1/validate` with `Authorization: Bearer ev1_...` → SHA-256 hash the token → `SELECT id, tenant_id FROM api_keys WHERE hash = $1 AND revoked_at IS NULL` → constant-time compare already handled by hash equality. One index scan, ~0.1ms.

**Key creation flow:** `POST /v1/keys` → generate 256-bit random key → SHA-256 hash → INSERT into `api_keys` → return plaintext key once. Never stored.

**Evaluations:** Written async (goroutine + channel) so they never slow down the validate response. Optional — can be disabled via `EVIDRA_LOG_EVALUATIONS=false`.

---

## 5. Environment File

```bash
# .env — production environment
# NEVER commit this file. It's in .gitignore.

# ─── Domain ───────────────────────
DOMAIN=evidra.rest

# ─── Let's Encrypt ────────────────
ACME_EMAIL=admin@evidra.rest

# ─── Traefik Dashboard Auth ───────
# Generate: echo $(htpasswd -nB admin) | sed -e s/\\$/\\$\\$/g
TRAEFIK_AUTH=admin:$$2y$$10$$...hashedpassword...
# Set to "true" to expose dashboard on TRAEFIK_DOMAIN with BasicAuth.
# Set to "false" to disable. Access via SSH port-forward: ssh -L 8080:localhost:8080 deploy@<VM2-IP>
TRAEFIK_DASHBOARD_ENABLED=true

# ─── Data directory ───────────────
DATA_DIR=/mnt/data

# ─── Evidra API ───────────────────
EVIDRA_VERSION=latest
EVIDRA_SIGNING_KEY=base64-encoded-ed25519-private-key

# ─── Key Self-Service ────────────
# false = POST /v1/keys disabled (manual issuance only)
# true  = POST /v1/keys enabled (with rate limit + optional invite)
EVIDRA_SELF_SERVE_KEYS=false
# When set, POST /v1/keys requires X-Invite-Token header.
# Leave empty to allow open key creation (with rate limit).
EVIDRA_INVITE_SECRET=

# ─── PostgreSQL ──────────────────
PG_PASSWORD=change-me-to-random-64-chars
```

---

## 6. Server Bootstrap Script

Run once after `terraform apply`. Cloud-init already handles OS hardening (SSH, deploy user, fail2ban, unattended-upgrades). This script only sets up the application layer:

```bash
#!/bin/bash
# scripts/bootstrap-hetzner.sh
# Run as deploy user after terraform provisions the VM.
# Cloud-init has already: created deploy user, hardened SSH, installed packages.
#
# Usage (from local machine):
#   VOLUME_DEVICE=$(cd terraform && terraform output -raw volume_linux_device)
#   ssh deploy@<IP> "VOLUME_DEVICE=$VOLUME_DEVICE bash -s" < scripts/bootstrap-hetzner.sh

set -euo pipefail

echo "=== Evidra API — Post-Terraform Bootstrap ==="

# ─── 1. Volume Mount (fstab, not symlink) ─────────
# Terraform output provides the deterministic device path:
#   /dev/disk/by-id/scsi-0HC_Volume_<ID>
# We mount it at /mnt/data via fstab for a stable, reboot-safe path.
MOUNT_POINT="/mnt/data"
VOLUME_DEVICE="${VOLUME_DEVICE:-}"

if [ -z "$VOLUME_DEVICE" ]; then
    # Fallback: detect Hetzner volume device automatically
    VOLUME_DEVICE=$(ls /dev/disk/by-id/scsi-0HC_Volume_* 2>/dev/null | head -1)
fi

if [ -z "$VOLUME_DEVICE" ]; then
    echo "ERROR: No volume device found."
    echo "Pass VOLUME_DEVICE env var from: terraform output -raw volume_linux_device"
    exit 1
fi

# Remove any Hetzner automount entries (they use /mnt/HC_Volume_* paths)
sudo sed -i '/HC_Volume/d' /etc/fstab

# Add stable fstab entry
if ! grep -q "$MOUNT_POINT" /etc/fstab; then
    echo "$VOLUME_DEVICE $MOUNT_POINT ext4 discard,nofail,defaults 0 0" | sudo tee -a /etc/fstab
fi

sudo mkdir -p "$MOUNT_POINT"
sudo mount -a
echo "✓ Volume: $VOLUME_DEVICE → $MOUNT_POINT (fstab)"

# ─── 2. Directory Structure ──────────────────────
sudo mkdir -p "$MOUNT_POINT/traefik"
sudo mkdir -p "$MOUNT_POINT/postgres/data"
sudo mkdir -p "$MOUNT_POINT/evidra/logs"

# acme.json needs strict permissions
sudo touch "$MOUNT_POINT/traefik/acme.json"
sudo chmod 600 "$MOUNT_POINT/traefik/acme.json"

echo "✓ Directory structure created"

# ─── 3. Docker Check ─────────────────────────────
# docker-ce image should have Docker pre-installed
if ! command -v docker &> /dev/null; then
    echo "Installing Docker..."
    curl -fsSL https://get.docker.com | sudo sh
    sudo usermod -aG docker deploy
    echo "Re-login required for docker group. Run: newgrp docker"
fi
echo "✓ Docker $(docker --version | awk '{print $3}')"

if ! docker compose version &> /dev/null; then
    echo "ERROR: Docker Compose plugin not found."
    echo "  sudo apt-get install docker-compose-plugin"
    exit 1
fi
echo "✓ Docker Compose $(docker compose version --short)"

# ─── 4. App Directory ────────────────────────────
APP_DIR="/opt/evidra"
sudo mkdir -p "$APP_DIR"
sudo chown deploy:deploy "$APP_DIR"
echo "✓ App directory: $APP_DIR"

echo ""
echo "Next steps:"
echo "  1. scp docker-compose.prod.yml .env db/ deploy@<IP>:$APP_DIR/"
echo "  2. Edit .env with domains, signing key, DB password"
echo "  3. cd $APP_DIR && docker compose -f docker-compose.prod.yml up -d"
echo "  4. curl https://evidra.rest/healthz"
echo ""
echo "=== Bootstrap complete ==="
```


## 7. Deployment Workflow

### First Deploy (manual)

```bash
# After terraform apply — cloud-init has already:
#   ✓ Created deploy user with docker group
#   ✓ Hardened SSH (key-only, no root password)
#   ✓ Installed unattended-upgrades, jq, fail2ban

# SSH in (use output from terraform)
ssh deploy@$(cd terraform && terraform output -raw server_ip)

# On VM — run bootstrap script (mounts volume via fstab, creates directories):
# From local machine:
VOLUME_DEVICE=$(cd terraform && terraform output -raw volume_linux_device)
IP=$(cd terraform && terraform output -raw server_ip)
ssh deploy@$IP "VOLUME_DEVICE=$VOLUME_DEVICE bash -s" < scripts/bootstrap-hetzner.sh

# Copy files from local machine:
# scp docker-compose.prod.yml .env db/ deploy@<IP>:/opt/evidra/

# Generate Traefik dashboard password
sudo apt-get install -y apache2-utils
echo $(htpasswd -nB admin) | sed -e 's/\$/\$\$/g'
# → paste into .env as TRAEFIK_AUTH

# Generate Evidra signing key
openssl genpkey -algorithm ed25519 -outform DER | base64 -w0
# → paste into .env as EVIDRA_SIGNING_KEY

# Generate PostgreSQL password
openssl rand -base64 48
# → paste into .env as PG_PASSWORD

# Start (PostgreSQL starts first, API waits for healthy check)
cd /opt/evidra
docker compose -f docker-compose.prod.yml up -d

# Verify
docker compose -f docker-compose.prod.yml ps
# Should show: traefik (running), evidra-postgres (healthy), evidra-api (running)
curl -I https://evidra.rest/healthz
# Should return 200 with HSTS header
```

### Rolling Update (via CI)

CI pushes new image to GHCR → SSH into VM → pull & restart:

```bash
# scripts/deploy.sh — called from CI or manually
#!/bin/bash
set -euo pipefail

ssh deploy@<VM_IP> << 'EOF'
cd /opt/evidra
docker compose -f docker-compose.prod.yml pull evidra-api
docker compose -f docker-compose.prod.yml up -d evidra-api
docker image prune -f
echo "Deployed: $(docker inspect --format='{{.Image}}' evidra-api | head -c 20)"
EOF
```

### GitHub Actions — Deploy Job (in evidra-infra repo)

This workflow lives in `vitas/evidra-infra`, NOT in the open source repo.

```yaml
# evidra-infra/.github/workflows/deploy.yml
name: Deploy Evidra API
on:
  # Manual trigger from GitHub UI
  workflow_dispatch:
    inputs:
      image_tag:
        description: "Image tag to deploy (default: latest)"
        default: "latest"
  # Cross-repo trigger: evidra CI sends dispatch after image push
  repository_dispatch:
    types: [deploy-api]

jobs:
  deploy:
    runs-on: ubuntu-latest
    environment: production
    steps:
      - name: Deploy to Hetzner VM2
        uses: appleboy/ssh-action@v1
        with:
          host: ${{ secrets.HETZNER_HOST }}
          username: deploy
          key: ${{ secrets.HETZNER_SSH_KEY }}
          script: |
            cd /opt/evidra
            docker compose -f docker-compose.prod.yml pull evidra-api
            docker compose -f docker-compose.prod.yml up -d evidra-api
            docker image prune -f
            # Health check
            sleep 5
            curl -sf https://evidra.rest/healthz || exit 1
```

**Cross-repo trigger (optional):** Add to `vitas/evidra` CI after image push:

```yaml
# In vitas/evidra/.github/workflows/ci.yml, after build-and-push job:
  notify-deploy:
    needs: build-and-push
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    steps:
      - name: Trigger deploy in evidra-infra
        uses: peter-evans/repository-dispatch@v3
        with:
          token: ${{ secrets.INFRA_DISPATCH_PAT }}
          repository: vitas/evidra-infra
          event-type: deploy-api
```

**Required secrets in evidra-infra:**
- `HETZNER_HOST` — VM2 public IP
- `HETZNER_SSH_KEY` — SSH private key (Ed25519) for `deploy` user

**Required secret in vitas/evidra (for cross-repo dispatch):**
- `INFRA_DISPATCH_PAT` — GitHub PAT with `repo` scope on `vitas/evidra-infra`

---

## 8. Traefik Dashboard

Controlled by a single env var:

```bash
# .env
TRAEFIK_DASHBOARD_ENABLED=true   # P0: enabled, BasicAuth protected
TRAEFIK_DASHBOARD_ENABLED=false  # Later: disabled, SSH port-forward only
```

**When enabled (`true`):** Accessible at `https://traefik.evidra.rest` with BasicAuth. Shows routers, entrypoints, certs, middleware chain, service health. Useful for P0 to see traffic and debug routing.

**When disabled (`false`):** Labels are ignored by Traefik. Access dashboard via SSH port-forward:

```bash
ssh -L 8080:localhost:8080 deploy@<VM2-IP>
# Open http://localhost:8080/dashboard/
```

For this to work with `--api.insecure=false` (default), temporarily add `--api.insecure=true` to Traefik command. Never expose port 8080 in Hetzner Firewall.

---

## 9. Security Checklist

### Traefik
- [x] HTTP → HTTPS redirect (forced)
- [x] HSTS header (1 year, includeSubdomains)
- [x] X-Content-Type-Options: nosniff
- [x] X-Frame-Options: DENY
- [x] Content-Security-Policy (default-src 'self', frame-ancestors 'none')
- [x] Referrer-Policy: strict-origin-when-cross-origin
- [x] Permissions-Policy: camera=(), microphone=(), geolocation=()
- [x] Rate limiting (10 req/s per source IP, burst 50)
- [x] Dashboard toggleable via env var (P0: enabled + BasicAuth, later: disabled)
- [x] Docker socket mounted read-only
- [x] no-new-privileges security opt
- [x] acme.json permissions 600

### Hetzner (via Terraform)
- [x] Firewall: only 22, 80, 443 inbound (hcloud_firewall resource)
- [x] SSH: key-only auth (cloud-init 99-hardening.conf)
- [x] SSH: `deploy` user with docker group (cloud-init)
- [x] SSH: restrict to specific IP in Hetzner Firewall if possible (edit firewall rule source_ips)
- [x] Automatic security updates: unattended-upgrades (cloud-init)
- [x] fail2ban installed and enabled (cloud-init)
- [ ] Volume snapshots scheduled (weekly)

### Evidra API
- [x] read_only container filesystem + tmpfs /tmp
- [x] no-new-privileges
- [x] Runs as non-root (distroless)
- [x] No ports exposed directly (only via Traefik network)
- [x] Signing key in env var, never on disk
- [x] Multiple API key hashes for zero-downtime rotation
- [x] No in-container healthcheck (distroless — verified externally)
- [x] `/v1/keys` endpoint abuse protection (see §9.1)

### 9.1 `/v1/keys` Abuse Protection

`POST /v1/keys` is the most attractive endpoint for bots — it mints credentials with no authentication. Traefik's blanket rate limit (100 req/min) is not enough; a single IP can create 100 keys/minute, or a botnet can use thousands of IPs.

**Three layers of defense, all in application code:**

**Layer 1: Feature gate (P0, immediate)**

```bash
# .env
EVIDRA_SELF_SERVE_KEYS=false   # P0 default: endpoint disabled
```

When `false`, `POST /v1/keys` returns `403 {"error": "Key self-service is disabled", "hint": "Contact admin@evidra.rest"}`. Keys are issued manually via CLI or direct DB insert. This is the safest option for private beta — zero attack surface.

When `true`, layers 2 and 3 activate.

**Layer 2: Per-IP rate limit on `/v1/keys` only (P0)**

Application-level rate limiter, separate from Traefik's blanket limit:

```
Limit: 3 keys per IP per hour (token bucket, burst 1)
Storage: in-memory map[ip]bucket with cleanup goroutine
Response: 429 {"error": "Too many key requests", "retry_after": 1800}
```

This is stricter than Traefik's global limit. Implementation lives in `internal/ratelimit/limiter.go` — the same rate limiter used for `/v1/validate`, but with a separate bucket config for `/v1/keys`.

**Layer 3: Invite secret for private beta (P0, optional)**

```bash
# .env
EVIDRA_INVITE_SECRET=change-me-random-token   # set to enable
# Leave empty to disable invite requirement
```

When set, `POST /v1/keys` requires an `X-Invite-Token` header matching the secret. Without it → `403 {"error": "Invite token required"}`. This transforms `/v1/keys` from a public endpoint into a gated one — share the invite token with early users, rotate when needed.

```bash
# User creates key with invite token:
curl -X POST https://evidra.rest/v1/keys \
  -H "X-Invite-Token: $INVITE_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"label": "my-project"}'
```

UI Console checks: if invite is required, shows an "Invite token" input field before the API key form.

**Phase 2 (later):** CAPTCHA (hCaptcha/Turnstile), email verification, or GitHub OAuth as the key creation gate. These replace the invite secret when going public.

**Decision matrix:**

| Phase | `SELF_SERVE_KEYS` | `INVITE_SECRET` | Behavior |
|---|---|---|---|
| P0 private | `false` | — | Endpoint disabled. Manual key issuance only. |
| P0 beta | `true` | set | Endpoint active, requires invite token + IP rate limit. |
| P1 public | `true` | empty | Endpoint open, IP rate limit only. Add CAPTCHA. |

---

## 10. Monitoring (Minimal)

Phase 0 — no Prometheus, no Grafana. Just:

```bash
# scripts/health-check.sh — cron every 5 min
#!/bin/bash
if ! curl -sf --max-time 10 https://evidra.rest/healthz > /dev/null; then
    echo "$(date): evidra-api DOWN" >> /mnt/data/evidra/logs/health.log
    # Optional: send notification
    # curl -X POST https://hooks.slack.com/... -d '{"text":"⚠️ evidra-api is DOWN"}'
fi
```

```bash
# Add to crontab:
*/5 * * * * /opt/evidra/scripts/health-check.sh
```

Traefik access logs available in container stdout → `docker logs traefik --tail 100`.

---

## 11. Backup

### Volume Snapshot (Hetzner API)

```bash
# scripts/backup-snapshot.sh
# Prereqs: hcloud CLI + jq installed on VM or run from local machine.
#   apt-get install -y jq
#   curl -sL https://github.com/hetznercloud/cli/releases/latest/download/hcloud-linux-amd64.tar.gz | tar xz -C /usr/local/bin
#   hcloud context create evidra  # interactive: paste API token
#!/bin/bash
# Requires: hcloud CLI installed and authenticated

VOLUME_ID=$(hcloud volume list -o json | jq '.[0].id')
TIMESTAMP=$(date +%Y%m%d-%H%M)

# Stop writes for consistent snapshot
docker compose -f /opt/evidra/docker-compose.prod.yml stop postgres 2>/dev/null || true
sync

hcloud volume create-snapshot "$VOLUME_ID" --description "evidra-backup-$TIMESTAMP"

# Resume
docker compose -f /opt/evidra/docker-compose.prod.yml start postgres 2>/dev/null || true

echo "Snapshot created: evidra-backup-$TIMESTAMP"
```

**Note:** Snapshot includes PostgreSQL data, Traefik certs, and logs. The script stops PostgreSQL briefly for a consistent snapshot.

---

## 12. Cost Summary

| Phase | Components | Monthly Cost |
|---|---|---|
| **Phase 0** | CX22 + 20 GB Volume (Traefik + API + PostgreSQL) | ~€4.67 |
| **Phase 1** (growth) | CX32 + 50 GB Volume | ~€9.20 |
| **Phase 2** (+ monitoring) | CX32 + 100 GB Volume | ~€11.20 |

Domain cost varies by registrar (~€10-15/year for `.dev`).

---

## 13. Infrastructure as Code — Terraform

All Hetzner infrastructure is provisioned via Terraform. No manual clicks in Console.

**Dogfooding opportunity:** once Evidra API is live, wire `terraform plan -json` output through `POST /v1/validate` in a CI step — Evidra validates its own infrastructure changes.

### 13.1 Prerequisites

```bash
# Install Terraform (or OpenTofu)
brew install terraform    # macOS
# or: sudo apt install terraform

# Hetzner API token
# Console → Project → Security → API Tokens → Generate (Read+Write)
export HCLOUD_TOKEN="your-token-here"

# Your SSH public key (will be uploaded to Hetzner)
cat ~/.ssh/id_ed25519.pub
```

### 13.2 terraform/versions.tf

```hcl
terraform {
  required_version = ">= 1.5"

  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.49"
    }
  }
}

provider "hcloud" {
  token = var.hcloud_token
}
```

### 13.3 terraform/variables.tf

```hcl
variable "hcloud_token" {
  description = "Hetzner Cloud API token"
  type        = string
  sensitive   = true
}

variable "server_name" {
  description = "Server hostname"
  type        = string
  default     = "evidra-api-01"
}

variable "server_type" {
  description = "Hetzner server type"
  type        = string
  default     = "cx22"   # 2 vCPU Intel, 4 GB RAM, 40 GB NVMe — ~€3.79/mo
}

variable "location" {
  description = "Hetzner location"
  type        = string
  default     = "nbg1"   # Nuremberg — closest to Munich
}

variable "image" {
  description = "OS image (Docker CE pre-installed)"
  type        = string
  default     = "docker-ce"   # Ubuntu 24.04 + Docker
}

variable "ssh_public_key_path" {
  description = "Path to SSH public key"
  type        = string
  default     = "~/.ssh/id_ed25519.pub"
}

variable "volume_size" {
  description = "Block storage volume size in GB"
  type        = number
  default     = 20
}

variable "domain" {
  description = "Primary domain for the API"
  type        = string
  default     = "evidra.rest"
}
```

### 13.4 terraform/main.tf

```hcl
# --- SSH Key ---

resource "hcloud_ssh_key" "default" {
  name       = "evidra-deploy"
  public_key = file(var.ssh_public_key_path)
}

# --- Firewall ---

resource "hcloud_firewall" "evidra" {
  name = "evidra-fw"

  # SSH
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = ["0.0.0.0/0", "::/0"]
    description = "SSH"
  }

  # HTTP (redirect to HTTPS + Let's Encrypt challenge)
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "80"
    source_ips = ["0.0.0.0/0", "::/0"]
    description = "HTTP"
  }

  # HTTPS
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "443"
    source_ips = ["0.0.0.0/0", "::/0"]
    description = "HTTPS"
  }
}

# --- Server ---

resource "hcloud_server" "evidra" {
  name        = var.server_name
  server_type = var.server_type
  location    = var.location
  image       = var.image
  ssh_keys    = [hcloud_ssh_key.default.id]
  firewall_ids = [hcloud_firewall.evidra.id]

  labels = {
    project = "evidra"
    role    = "api"
  }

  # cloud-init: create deploy user, harden SSH, install basics, prepare volume mount
  user_data = <<-CLOUDINIT
    #cloud-config
    users:
      - name: deploy
        groups: docker, sudo
        sudo: ALL=(ALL) NOPASSWD:ALL
        shell: /bin/bash
        ssh_authorized_keys:
          - ${file(var.ssh_public_key_path)}

    package_update: true
    package_upgrade: true
    packages:
      - unattended-upgrades
      - jq
      - fail2ban

    write_files:
      - path: /etc/ssh/sshd_config.d/99-hardening.conf
        content: |
          PasswordAuthentication no
          PermitRootLogin no
          MaxAuthTries 3

    runcmd:
      - systemctl restart sshd
      - systemctl enable fail2ban
      - systemctl start fail2ban
      - mkdir -p /opt/evidra
      # Mount volume at /mnt/data via fstab (Hetzner automount creates /mnt/HC_Volume_*).
      # We add our own fstab entry for a stable /mnt/data path using the volume's
      # device ID. The device path is deterministic: /dev/disk/by-id/scsi-0HC_Volume_<ID>.
      # After terraform apply, run: scripts/bootstrap-hetzner.sh to finalize.
      - mkdir -p /mnt/data
  CLOUDINIT
}

# --- Volume ---

resource "hcloud_volume" "data" {
  name      = "evidra-data"
  size      = var.volume_size
  location  = var.location
  format    = "ext4"
  labels = {
    project = "evidra"
  }
}

resource "hcloud_volume_attachment" "data" {
  volume_id = hcloud_volume.data.id
  server_id = hcloud_server.evidra.id
  automount = true
}
```

### 13.5 terraform/outputs.tf

```hcl
output "server_ip" {
  description = "Public IPv4 of evidra-api-01"
  value       = hcloud_server.evidra.ipv4_address
}

output "server_ipv6" {
  description = "Public IPv6 of evidra-api-01"
  value       = hcloud_server.evidra.ipv6_address
}

output "volume_id" {
  description = "Hetzner Volume ID (for snapshots)"
  value       = hcloud_volume.data.id
}

output "volume_linux_device" {
  description = "Linux device path for the volume"
  value       = hcloud_volume.data.linux_device
}

output "volume_fstab_entry" {
  description = "fstab line for mounting volume at /mnt/data"
  value       = "${hcloud_volume.data.linux_device} /mnt/data ext4 discard,nofail,defaults 0 0"
}

output "ssh_command" {
  description = "SSH into the server"
  value       = "ssh deploy@${hcloud_server.evidra.ipv4_address}"
}

output "dns_records" {
  description = "DNS records to create at your registrar"
  value       = <<-DNS
    A     ${var.domain}         → ${hcloud_server.evidra.ipv4_address}
    CNAME www.${var.domain}     → ${var.domain}
    AAAA  ${var.domain}         → ${hcloud_server.evidra.ipv6_address}
  DNS
}
```

### 13.6 terraform/terraform.tfvars.example

```hcl
# Copy to terraform.tfvars and fill in:
hcloud_token        = ""                        # Hetzner API token
ssh_public_key_path = "~/.ssh/id_ed25519.pub"   # Your SSH key
server_type         = "cx22"                     # cx22 for Phase 0, cx32 for Phase 1
volume_size         = 20                         # 20 GB Phase 0, 50 GB Phase 1
location            = "nbg1"                     # Nuremberg
domain              = "evidra.rest"
```

### 13.7 Provisioning workflow

```bash
cd evidra-infra/terraform

# 1. Init (one-time)
terraform init

# 2. Plan — review before apply
terraform plan -out=tfplan

# 3. Apply
terraform apply tfplan

# Outputs:
#   server_ip    = "49.13.x.x"
#   ssh_command  = "ssh deploy@49.13.x.x"
#   dns_records  = "A evidra.rest → 49.13.x.x ..."
#   volume_linux_device = "/dev/disk/by-id/scsi-0HC_Volume_12345678"
#   volume_fstab_entry  = "/dev/disk/by-id/scsi-0HC_Volume_... /mnt/data ext4 ..."

# 4. Set DNS records at your registrar (from dns_records output)

# 5. Bootstrap (from local machine — runs script on server)
cd ..  # back to evidra-infra root
VOLUME_DEVICE=$(cd terraform && terraform output -raw volume_linux_device)
IP=$(cd terraform && terraform output -raw server_ip)
ssh deploy@$IP "VOLUME_DEVICE=$VOLUME_DEVICE bash -s" < scripts/bootstrap-hetzner.sh

# 6. Copy docker-compose + .env
scp docker-compose.prod.yml .env deploy@$IP:/opt/evidra/
scp -r db/ deploy@$IP:/opt/evidra/

# 7. Start services
ssh deploy@$IP "cd /opt/evidra && docker compose -f docker-compose.prod.yml up -d"

# 8. Verify
curl https://evidra.rest/healthz
```

### 13.8 State management

For Phase 0 (single developer), local state is fine:

```gitignore
# In .gitignore
*.tfstate
*.tfstate.backup
.terraform/
terraform.tfvars     # contains HCLOUD_TOKEN
```

Phase 2 migration to remote state (optional):

```hcl
# Add to versions.tf when needed
terraform {
  backend "s3" {
    bucket   = "evidra-tfstate"
    key      = "prod/terraform.tfstate"
    region   = "eu-central-1"
    encrypt  = true
  }
}
```

### 13.9 Lifecycle operations

```bash
# Scale up (Phase 1): edit terraform.tfvars
server_type = "cx32"
volume_size = 50
terraform apply    # Server recreated, volume resized in-place

# Destroy everything (careful!)
terraform destroy

# Import existing resource (if created manually before)
terraform import hcloud_server.evidra <server-id>
terraform import hcloud_volume.data <volume-id>
terraform import hcloud_firewall.evidra <firewall-id>
```

### 13.10 Dogfooding — Evidra validates Evidra (Phase 2)

Once the API is live, add to CI:

```yaml
# .github/workflows/infra-validate.yml (in evidra-infra)
name: Validate Infrastructure Change
on:
  pull_request:
    paths: ['terraform/**']

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: hashicorp/setup-terraform@v3

      - name: Terraform Plan
        run: |
          cd terraform
          terraform init
          terraform plan -out=tfplan
          terraform show -json tfplan > plan.json

      - name: Evidra Policy Check
        run: |
          # Extract resource changes and validate each
          jq -c '.resource_changes[]' plan.json | while read change; do
            TOOL=$(echo "$change" | jq -r '.type')
            ACTION=$(echo "$change" | jq -r '.change.actions[0]')
            
            curl -s -X POST https://evidra.rest/v1/evaluate \
              -H "Authorization: Bearer ${{ secrets.EVIDRA_API_KEY }}" \
              -H "Content-Type: application/json" \
              -d "{
                \"tool\": \"terraform.${TOOL}\",
                \"operation\": \"${ACTION}\",
                \"resource\": $(echo "$change" | jq '.change.after // {}'),
                \"actor\": {
                  \"id\": \"github-actions\",
                  \"origin\": \"evidra-infra\"
                }
              }" | jq .
          done
```

This validates infrastructure changes through Evidra's own policy engine before apply — true dogfooding.

---

## 14. Local Development vs Production

| Aspect | Local (docker-compose.yml) | Production (docker-compose.prod.yml) |
|---|---|---|
| Reverse proxy | None, direct :8080 | Traefik v3 + TLS |
| TLS | No | Let's Encrypt auto |
| API key | Static env var | Hashed, rotatable |
| Signing key | Ephemeral (auto-generated) | Persistent in .env |
| PostgreSQL | Docker volume | Hetzner Volume-backed |
| Volume | Docker volumes | Hetzner Block Storage |
| Container registry | Local build | ghcr.io |
| Deployment | `docker compose up` | CI → SSH → pull + restart |
