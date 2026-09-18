# SkillIssues Production Deployment Guide

This guide details deploying **SkillIssues** in production using Docker Compose, Kubernetes, or cloud VPS providers with SSL/TLS, Google Distroless containers, PostgreSQL, and Redis.

---

## 🏗️ Production Architecture Overview

The production architecture consists of:
- **`nginx` / Reverse Proxy (Port 80/443)**: TLS termination, static asset compression, and rate-buffered reverse proxying to `:8000` and `:5173`.
- **`go-api` (Port 8000)**: Pure Go API Gateway running on Google Distroless Debian 12 non-root container.
- **`syncd` (Port 8002)**: Go GitHub GraphQL Sync Coordinator with distributed worker pools.
- **`worker`**: Background priority catalog refresher.
- **`postgres` (Port 5432)**: PostgreSQL 16 relational database with persistent storage.
- **`redis` (Port 6379)**: In-memory session store, distributed locks, and sliding-window rate limiting.
- **`frontend` (Port 5173)**: React 18 + Vite production build served through static Web server.

---

## 🚀 Option 1: Docker Compose Production Stack

### 1. Clone & Configure Environment
```bash
git clone https://github.com/dizziedbliss/SkillIssues.git
cd SkillIssues
cp .env.example .env
```

Ensure production `.env` contains:
```ini
POSTGRES_DB=skillissues
POSTGRES_USER=skillissues
POSTGRES_PASSWORD=your_strong_generated_password
DATABASE_URL=postgresql://skillissues:your_strong_generated_password@postgres:5432/skillissues

REDIS_URL=redis://redis:6379/0

PORT=8000
FRONTEND_URL=https://skillissues.com
VITE_API_URL=https://skillissues.com/api

GITHUB_CLIENT_ID=your_github_oauth_client_id
GITHUB_CLIENT_SECRET=your_github_oauth_client_secret
GITHUB_REDIRECT_URI=https://skillissues.com/auth/github/callback

GITHUB_TOKEN=ghp_your_github_pat_token
GITHUB_WEBHOOK_SECRET=your_strong_webhook_secret
ADMIN_SYNC_TOKEN=your_strong_admin_token

UNAUTH_RATE_LIMIT=180
AUTH_RATE_LIMIT=1000
SYNC_INTERVAL_SECONDS=900
```

### 2. Launch Production Containers
```bash
docker compose -f docker-compose.prod.yml up -d --build
```

### 3. Verify Container Health
```bash
docker compose -f docker-compose.prod.yml ps
```

---

## ☸️ Option 2: Kubernetes Cluster Deployment

Production Kubernetes manifests are organized in `k8s/`:

### 1. Apply Namespace & Secrets
```bash
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secrets.yaml
```

### 2. Deploy Microservices & Ingress
```bash
kubectl apply -f k8s/api/
kubectl apply -f k8s/syncd/
kubectl apply -f k8s/worker/
kubectl apply -f k8s/frontend/
kubectl apply -f k8s/ingress.yaml
```

### 3. Verify Cluster Pods & Probes
```bash
kubectl get pods -n skillissues
kubectl get ingress -n skillissues
```

All Go microservice pods include static `/healthcheck` liveness and readiness probes.

---

## ☁️ Option 3: VPS Deployment (Ubuntu 24.04 LTS / Debian 12)

### 1. Server Setup
```bash
# Update and install Docker
sudo apt update && sudo apt upgrade -y
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
```

### 2. Clone & Launch Stack
```bash
git clone https://github.com/dizziedbliss/SkillIssues.git
cd SkillIssues
cp .env.example .env
nano .env
docker compose -f docker-compose.prod.yml up -d --build
```

### 3. Configure SSL/TLS with Certbot
```bash
sudo apt install -y certbot python3-certbot-nginx
sudo certbot --nginx -d skillissues.com -d www.skillissues.com
```

---

## 🔒 Production Security Checklist

- [ ] **Strong Passwords**: Generate cryptographically random strings for `POSTGRES_PASSWORD` and `ADMIN_SYNC_TOKEN`.
- [ ] **Firewall Hardening (UFW)**: Block external access to port `5432` (PostgreSQL) and `6379` (Redis). Only expose ports `80` and `443`.
- [ ] **Distroless Non-Root Containers**: Ensure all Go containers run as user `nonroot:nonroot`.
- [ ] **GitHub OAuth URLs**: Ensure your GitHub OAuth Application callback matches your production domain: `https://your-domain.com/auth/github/callback`.
- [ ] **Rate Limiting Active**: Verify Redis is healthy so sliding-window rate limiting is enforced on all API requests.
- [ ] **Observability**: Scrape `/metrics` on port 8000 using Prometheus and configure alerts on high HTTP error rates or sync job failures.
