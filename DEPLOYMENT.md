# SkillIssues Production Deployment Guide

This guide covers deploying **SkillIssues** in production using Docker Compose, Nginx, and cloud providers.

---

## Quickstart (1-Command Docker Deployment)

### 1. Clone & Configure
`ash
git clone https://github.com/dizziedbliss/SkillIssues.git
cd SkillIssues
cp .env.example .env
nano .env
`

### 2. Launch All Services in Production Mode
`ash
docker compose -f docker-compose.prod.yml up -d --build
`

The stack will start and automatically manage:
- **Port 80**: Nginx reverse proxy serving optimized static React build & proxying /api/
- **Port 8000**: FastAPI High-Performance Control Plane
- **Port 5432**: PostgreSQL Database with persistent volume storage
- **Internal Network**: Recommendation Service, Contribution Service, Ingestion Pipeline, and Worker Queue

---

## Cloud Deployment Options

### Option A: VPS (DigitalOcean Droplet, Hetzner, AWS EC2, Linode)
1. Provision an **Ubuntu 24.04 LTS** server (2GB+ RAM recommended).
2. Install Docker & Docker Compose:
   `ash
   curl -fsSL https://get.docker.com | sh
   `
3. Clone repository and run docker compose -f docker-compose.prod.yml up -d.
4. Point your domain (e.g. skillissues.com) to your server IP.
5. (Optional) Enable HTTPS with Certbot:
   `ash
   sudo apt install -y certbot python3-certbot-nginx
   sudo certbot --nginx -d skillissues.com
   `

### Option B: Render / Railway / Fly.io
- Deploy services/api as a Web Service.
- Deploy postgres as a Managed PostgreSQL database.
- Deploy rontend as a Static Site or Docker Web Service.

---

## Security Checklist Before Going Live
- [ ] Set DEMO_MODE=false in .env.
- [ ] Generate a secure POSTGRES_PASSWORD and ADMIN_SYNC_TOKEN.
- [ ] Register your production domain in your GitHub OAuth App settings.
- [ ] Ensure firewall blocks public access to port 5432.
