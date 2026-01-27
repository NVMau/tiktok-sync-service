#!/bin/bash
# Deploy script - chạy trên server sau khi SSH vào

set -e

DEPLOY_PATH="${DEPLOY_PATH:-/opt/tiktok-sync}"

echo "🚀 Deploying TikTok Sync..."
cd "$DEPLOY_PATH"

echo "📦 Pulling latest images..."
docker compose pull

echo "🔄 Restarting services..."
docker compose up -d

echo "🧹 Cleaning up old images..."
docker image prune -f --filter "dangling=true"

echo "✅ Deploy completed!"
echo ""
docker compose ps
