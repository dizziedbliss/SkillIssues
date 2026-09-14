# SkillIssues Production Deployment Guide

This guide covers deploying **SkillIssues** in production using Docker Compose, Nginx, and cloud hosting providers.

---

## Quickstart (1-Command Docker Deployment)

### 1. Clone & Configure
```bash
git clone https://github.com/dizziedbliss/SkillIssues.git
cd SkillIssues
cp .env.example .env
nano .env
```

Ensure your `.env` contains:
- A strong `POSTGRES_PASSWORD`
- `DEMO_MODE=false`
- A unique `ADMIN_SYNC_TOKEN`
- Valid `GITHUB_TOKEN` and `GITHUB_WEBHOOK_SECRET`

### 2. Launch All Services in Production Mode
```bash
docker compose -f docker-compose.prod.yml up -d --build
```

The production stack manages:
- **Port 80**: Nginx reverse proxy serving optimized static React build & routing `/api/` traffic
- **Port 8000**: FastAPI High-Performance Control Plane
- **Port 5432**: PostgreSQL Database with persistent volume storage
- **Internal Network**: Recommendation Service, Contribution Service, Ingestion Pipeline, and Worker Queue

---

## Cloud Deployment Options

### Option A: VPS (DigitalOcean Droplet, Hetzner, AWS EC2, Linode)
1. Provision an **Ubuntu 24.04 LTS** server (2GB+ RAM recommended).
2. Install Docker & Docker Compose:
   ```bash
   curl -fsSL https://get.docker.com | sh
   ```
3. Clone repository, configure `.env`, and launch:
   ```bash
   docker compose -f docker-compose.prod.yml up -d --build
   ```
4. Point your domain (e.g. `skillissues.com`) to your server IP.
5. Enable SSL/HTTPS with Certbot:
   ```bash
   sudo apt install -y certbot python3-certbot-nginx
   sudo certbot --nginx -d skillissues.com
   ```

### Option B: Render / Railway / Fly.io
- Deploy `services/api` as a Python Web Service (`uvicorn app.main:app --host 0.0.0.0 --port 8000`).
- Deploy `postgres` as a Managed PostgreSQL database.
- Deploy `frontend` as a Static Site build (`npm run build` -> `dist/`).
- Set internal environment variables to link services.

---

## Security Checklist Before Going Live

- [ ] Set `DEMO_MODE=false` in `.env`.
- [ ] Generate a secure `POSTGRES_PASSWORD` and `ADMIN_SYNC_TOKEN`.
- [ ] Register your production domain in your GitHub OAuth App settings.
- [ ] Ensure server firewall (UFW) blocks public access to port `5432` (database).
- [ ] Enforce HTTPS for all web traffic via Nginx / Cloudflare.
