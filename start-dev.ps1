# Start development environment
Write-Host "Starting TikTok Sync development environment..." -ForegroundColor Green

# Start docker services (postgres, redis, ngrok)
Write-Host "Starting Docker services..." -ForegroundColor Yellow
docker-compose -f docker-compose.dev.yml up -d

# Wait for services
Write-Host "Waiting for services to be ready..." -ForegroundColor Yellow
Start-Sleep -Seconds 3

# Build the app
Write-Host "Building sync-api..." -ForegroundColor Yellow
go build -o sync-api.exe ./cmd/api/

# Get ngrok URL
Write-Host ""
Write-Host "=== Services Status ===" -ForegroundColor Cyan
docker-compose -f docker-compose.dev.yml ps

Write-Host ""
Write-Host "=== Ngrok Tunnel ===" -ForegroundColor Cyan
Write-Host "View ngrok dashboard: http://localhost:4040" -ForegroundColor White
Write-Host "Get public URL from: http://localhost:4040/api/tunnels" -ForegroundColor White

Write-Host ""
Write-Host "=== Webhook URL ===" -ForegroundColor Cyan
try {
    $response = Invoke-RestMethod -Uri "http://localhost:4040/api/tunnels" -ErrorAction SilentlyContinue
    $publicUrl = $response.tunnels[0].public_url
    Write-Host "Webhook URL: $publicUrl/api/v1/webhooks/tiktok" -ForegroundColor Green
} catch {
    Write-Host "Ngrok not ready yet. Check http://localhost:4040 for the URL" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "=== Starting API Server ===" -ForegroundColor Cyan
Write-Host "Press Ctrl+C to stop" -ForegroundColor Gray
.\sync-api.exe
