# CI/CD Setup Guide

## Flow Overview

```
Push to prod branch → GitHub Actions → Build & Push to GHCR → SSH vào server → Deploy thủ công
```

## 1. GitHub Repository Settings

### 1.1 Enable GitHub Container Registry

Go to **Settings → Actions → General** → set "Workflow permissions" to **Read and write permissions**.

## 2. Server Setup

### 2.1 Install Docker & Docker Compose

```bash
# Ubuntu/Debian
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# Install Docker Compose plugin
sudo apt-get update
sudo apt-get install docker-compose-plugin
```

### 2.2 Create Deploy Directory

```bash
sudo mkdir -p /opt/tiktok-sync
sudo chown $USER:$USER /opt/tiktok-sync
cd /opt/tiktok-sync
```

### 2.3 Copy Required Files

Copy these files to `/opt/tiktok-sync`:
- `docker-compose.prod.yml` → rename to `docker-compose.yml`
- `init.sql`
- `.env.prod.example` → rename to `.env` and fill in values

```bash
# On server
cd /opt/tiktok-sync
mv docker-compose.prod.yml docker-compose.yml
mv .env.prod.example .env
nano .env  # Edit with your production values
```

### 2.4 Create GitHub Personal Access Token

1. Go to GitHub → Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Generate new token with `read:packages` scope
3. Save token for login

### 2.5 Login to GitHub Container Registry

```bash
# Replace YOUR_GITHUB_USERNAME and YOUR_GITHUB_TOKEN
echo YOUR_GITHUB_TOKEN | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
```

### 2.6 First Deployment (Manual)

```bash
cd /opt/tiktok-sync
docker compose pull
docker compose up -d
docker compose logs -f
```

## 3. Workflow

### Deploy to Production

**Bước 1:** Push code lên branch `prod`
```bash
git checkout prod
git merge main
git push origin prod
```

**Bước 2:** Đợi GitHub Actions build xong (xem tab Actions)

**Bước 3:** SSH vào server và deploy
```bash
# VPN vào mạng nội bộ trước
ssh user@server-ip

# Deploy
cd /opt/tiktok-sync
docker compose pull
docker compose up -d

# Hoặc dùng script
./scripts/deploy.sh
```

### Rollback

```bash
# Rollback về version cụ thể
docker compose pull ghcr.io/nvmau/tiktok:COMMIT_SHA
docker compose up -d
```

## 4. Useful Commands (On Server)

```bash
# View logs
docker compose logs -f api
docker compose logs -f worker

# Restart services
docker compose restart api worker

# Check status
docker compose ps

# Manual deploy
docker compose pull && docker compose up -d
```

## 5. SSL/HTTPS Setup (Optional)

Add Nginx reverse proxy with Let's Encrypt:

```bash
sudo apt install nginx certbot python3-certbot-nginx
sudo certbot --nginx -d your-domain.com
```

Nginx config example at `/etc/nginx/sites-available/tiktok-sync`:

```nginx
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://localhost:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```
