# Evidra API — Production Deployment on Hetzner Cloud

**Date:** 2026-02-26
**Stack:** Traefik v3 (reverse proxy + SSL) → evidra-api (Go) + PostgreSQL
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
VM1 (ARM CAX11, existing)   VM2 (CX22, new)
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
├── .gitignore                          # .env, acme.json
├── .env.example                        # Template (no secrets)
├── docker-compose.prod.yml             # Traefik + evidra-api + PostgreSQL
├── db/
│   └── migrations/
│       └── 001_init.sql                # Initial schema: api_keys, evaluations
├── scripts/
│   ├── bootstrap-hetzner.sh            # First-time VM setup
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

Assuming the domain `api.evidra.rest`.

```
A     api.evidra.rest       → <VM_PUBLIC_IP>
A     traefik.evidra.rest   → <VM_PUBLIC_IP>    # optional: dashboard
```

DNS is configured at the domain registrar. Traefik uses HTTP challenge — no DNS provider API token needed (no Cloudflare setup required).

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
      - traefik.http.routers.traefik-dashboard.rule=Host(`${TRAEFIK_DOMAIN}`)
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
      # Main API + UI
      - traefik.http.routers.evidra.rule=Host(`${API_DOMAIN}`)
      - traefik.http.routers.evidra.entrypoints=websecure
      - traefik.http.routers.evidra.tls.certresolver=letsencrypt
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
      # Rate limiting
      - traefik.http.middlewares.evidra-ratelimit.ratelimit.average=100
      - traefik.http.middlewares.evidra-ratelimit.ratelimit.burst=50
      - traefik.http.middlewares.evidra-ratelimit.ratelimit.period=1m
    # NOTE: No container-level healthcheck — distroless has no wget/curl.
    # Health is verified externally:
    #   - Traefik checks backend via loadbalancer health (TCP connect to :8080)
    #   - Deploy script checks https://api.evidra.rest/healthz after restart
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
API_DOMAIN=api.evidra.rest
TRAEFIK_DOMAIN=traefik.evidra.rest

# ─── Let's Encrypt ────────────────
ACME_EMAIL=admin@evidra.rest

# ─── Traefik Dashboard Auth ───────
# Generate: echo $(htpasswd -nB admin) | sed -e s/\\$/\\$\\$/g
TRAEFIK_AUTH=admin:$$2y$$10$$...hashedpassword...
# Set to "true" to expose dashboard on TRAEFIK_DOMAIN with BasicAuth.
# Set to "false" to disable. Access via SSH port-forward: ssh -L 8080:localhost:8080 root@<VM2-IP>
TRAEFIK_DASHBOARD_ENABLED=true

# ─── Data directory ───────────────
DATA_DIR=/mnt/data

# ─── Evidra API ───────────────────
EVIDRA_VERSION=latest
EVIDRA_SIGNING_KEY=base64-encoded-ed25519-private-key

# ─── PostgreSQL ──────────────────
PG_PASSWORD=change-me-to-random-64-chars
```

---

## 6. Server Bootstrap Script

Run once on first VM boot:

```bash
#!/bin/bash
# scripts/bootstrap-hetzner.sh
# Run as root on fresh CX22 with docker-ce image.

set -euo pipefail

echo "=== Evidra API — Hetzner Bootstrap ==="

# ─── 1. Mount Volume ────────────────────────────────
# Hetzner volume auto-mounted if selected "automatic" during creation.
# Verify it's mounted:
MOUNT_POINT="/mnt/data"
if ! mountpoint -q "$MOUNT_POINT"; then
    echo "ERROR: Volume not mounted at $MOUNT_POINT"
    echo "Mount manually:"
    echo "  mkfs.ext4 /dev/disk/by-id/scsi-0HC_Volume_<ID>"
    echo "  mkdir -p $MOUNT_POINT"
    echo "  mount -o discard,defaults /dev/disk/by-id/scsi-0HC_Volume_<ID> $MOUNT_POINT"
    echo "  echo '/dev/disk/by-id/scsi-0HC_Volume_<ID> $MOUNT_POINT ext4 discard,nofail,defaults 0 0' >> /etc/fstab"
    exit 1
fi
echo "✓ Volume mounted at $MOUNT_POINT"

# ─── 2. Create directory structure ──────────────────
mkdir -p "$MOUNT_POINT/traefik"
mkdir -p "$MOUNT_POINT/postgres/data"
mkdir -p "$MOUNT_POINT/evidra/logs"

# acme.json needs strict permissions
touch "$MOUNT_POINT/traefik/acme.json"
chmod 600 "$MOUNT_POINT/traefik/acme.json"

echo "✓ Directory structure created"

# ─── 3. Firewall reminder ──────────────────────────
echo ""
echo "IMPORTANT: Configure Hetzner Firewall in console:"
echo "  Inbound rules:"
echo "    TCP 22   — SSH (restrict to your IP if possible)"
echo "    TCP 80   — HTTP (Traefik / Let's Encrypt challenge)"
echo "    TCP 443  — HTTPS (Traefik)"
echo "  Drop all other inbound traffic."
echo ""

# ─── 4. Docker check ───────────────────────────────
if ! command -v docker &> /dev/null; then
    echo "Installing Docker..."
    curl -fsSL https://get.docker.com | sh
fi
echo "✓ Docker $(docker --version | awk '{print $3}')"

# ─── 5. Docker Compose check ───────────────────────
if ! docker compose version &> /dev/null; then
    echo "ERROR: Docker Compose plugin not found."
    echo "  apt-get install docker-compose-plugin"
    exit 1
fi
echo "✓ Docker Compose $(docker compose version --short)"

# ─── 6. Create app directory ───────────────────────
APP_DIR="/opt/evidra"
mkdir -p "$APP_DIR"
echo "✓ App directory: $APP_DIR"
echo ""
echo "Next steps:"
echo "  1. Copy docker-compose.prod.yml and .env to $APP_DIR"
echo "  2. Edit .env with your domains, keys, passwords"
echo "  3. cd $APP_DIR && docker compose -f docker-compose.prod.yml up -d"
echo "  4. Verify: curl https://api.evidra.rest/healthz"
echo ""
echo "=== Bootstrap complete ==="
```

---

## 7. Deployment Workflow

### First Deploy (manual)

```bash
# On local machine:
ssh root@<VM_IP>

# On VM:
bash /path/to/bootstrap-hetzner.sh

# Copy files
cd /opt/evidra
# (scp docker-compose.prod.yml and .env from local)

# Generate Traefik dashboard password
apt-get install -y apache2-utils
echo $(htpasswd -nB admin) | sed -e 's/\$/\$\$/g'
# → paste into .env as TRAEFIK_AUTH

# Generate Evidra signing key
openssl genpkey -algorithm ed25519 -outform DER | base64 -w0
# → paste into .env as EVIDRA_SIGNING_KEY

# Generate PostgreSQL password
openssl rand -base64 48
# → paste into .env as PG_PASSWORD

# Start (PostgreSQL starts first, API waits for healthy check)
docker compose -f docker-compose.prod.yml up -d

# Verify
docker compose -f docker-compose.prod.yml ps
# Should show: traefik (running), evidra-postgres (healthy), evidra-api (running)
curl -I https://api.evidra.rest/healthz
# Should return 200 with HSTS header
```

### Rolling Update (via CI)

CI pushes new image to GHCR → SSH into VM → pull & restart:

```bash
# scripts/deploy.sh — called from CI or manually
#!/bin/bash
set -euo pipefail

ssh root@<VM_IP> << 'EOF'
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
          username: root
          key: ${{ secrets.HETZNER_SSH_KEY }}
          script: |
            cd /opt/evidra
            docker compose -f docker-compose.prod.yml pull evidra-api
            docker compose -f docker-compose.prod.yml up -d evidra-api
            docker image prune -f
            # Health check
            sleep 5
            curl -sf https://api.evidra.rest/healthz || exit 1
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
- `HETZNER_SSH_KEY` — SSH private key (Ed25519)

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
ssh -L 8080:localhost:8080 root@<VM2-IP>
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
- [x] Rate limiting (100 req/min, burst 50)
- [x] Dashboard toggleable via env var (P0: enabled + BasicAuth, later: disabled)
- [x] Docker socket mounted read-only
- [x] no-new-privileges security opt
- [x] acme.json permissions 600

### Hetzner
- [ ] Firewall: only 22, 80, 443 inbound
- [ ] SSH: key-only auth (`PasswordAuthentication no` in sshd_config)
- [ ] SSH: disable root login, create `deploy` user with docker group
- [ ] SSH: restrict to specific IP in Hetzner Firewall if possible
- [ ] Automatic security updates: `apt install unattended-upgrades`
- [ ] Volume snapshots scheduled (weekly)

### Evidra API
- [x] read_only container filesystem + tmpfs /tmp
- [x] no-new-privileges
- [x] Runs as non-root (distroless)
- [x] No ports exposed directly (only via Traefik network)
- [x] Signing key in env var, never on disk
- [x] Multiple API key hashes for zero-downtime rotation
- [x] No in-container healthcheck (distroless — verified externally)

---

## 10. Monitoring (Minimal)

Phase 0 — no Prometheus, no Grafana. Just:

```bash
# scripts/health-check.sh — cron every 5 min
#!/bin/bash
if ! curl -sf --max-time 10 https://api.evidra.rest/healthz > /dev/null; then
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

## 13. Hetzner Setup — Step by Step

```
1. Hetzner Console → existing Project (or new: "evidra")

2. Create Server (VM2 — separate from samebits.com ARM VM):
   - Location: Nuremberg (nbg1)
   - Image: Apps → Docker CE (Ubuntu 24.04)
   - Type: Shared vCPU → CX22 (Intel)
   - SSH Key: add your public key
   - Firewall: create "evidra-fw"
     - Inbound: TCP 22, TCP 80, TCP 443
   - Name: evidra-api-01

3. Create Volume:
   - Size: 20 GB
   - Location: same as server (nbg1)
   - Server: evidra-api-01
   - Mount: Automatic
   - Filesystem: ext4
   - Name: evidra-data

4. DNS: add A record → api.evidra.rest → <VM2 IP>
   (optional: traefik.evidra.rest → same IP)

5. Get evidra-infra on VM2:
   ssh root@<VM2-IP>
   # Option A: clone with deploy key
   git clone git@github.com:vitas/evidra-infra.git /opt/evidra
   # Option B: scp from local
   scp -r ./evidra-infra root@<VM2-IP>:/opt/evidra

6. Run bootstrap:
   cd /opt/evidra
   bash scripts/bootstrap-hetzner.sh

7. Configure:
   cp .env.example .env
   # Edit .env: domains, signing key, API key hash, ACME email

8. Deploy:
   docker compose -f docker-compose.prod.yml up -d
   
9. Verify:
   curl https://api.evidra.rest/healthz
```

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
