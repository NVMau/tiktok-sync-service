# Security Best Practices

## Overview

This document outlines security best practices for the TikTok Sync Service. Please follow these guidelines to ensure the security of your deployment.

## Environment Variables

### Never Hardcode Sensitive Data

**DO NOT** hardcode the following in your source code:
- API keys and secrets
- Database passwords
- Webhook secrets
- Access tokens
- Any credentials

**ALWAYS** use environment variables for sensitive configuration.

### Development Environment

1. Copy `.env.dev.example` to `.env`:
   ```bash
   cp .env.dev.example .env
   ```

2. **IMPORTANT**: Change default passwords immediately:
   ```bash
   # .env file
   POSTGRES_PASSWORD=your_strong_password_here  # Change this!
   CALLBACK_SECRET=your_secret_here             # Change this!
   ```

3. Never commit `.env` files to version control (already in `.gitignore`)

### Production Environment

1. Copy `.env.prod.example` to `.env` on your production server:
   ```bash
   cp .env.prod.example .env
   ```

2. Set strong, unique passwords:
   - Use a password manager or password generator
   - Minimum 20 characters with mixed case, numbers, and symbols
   - Never reuse passwords across environments

3. Protect your `.env` file:
   ```bash
   chmod 600 .env
   ```

## Docker Compose Security

### Default Passwords

All docker-compose files use environment variables with safe fallback defaults:

```yaml
POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-changeme}
```

**ALWAYS** set `POSTGRES_PASSWORD` in your environment before starting services:

```bash
# Development
export POSTGRES_PASSWORD="your_dev_password"
docker-compose -f docker-compose.dev.yml up

# Production
export POSTGRES_PASSWORD="your_prod_password"
docker-compose -f docker-compose.prod.yml up
```

### Network Isolation

In production (`docker-compose.prod.yml`):
- Database and Redis are NOT exposed to the host
- Only the API service is exposed on the configured port
- All services communicate via internal Docker network

## API Security

### TikTok API Credentials

1. Obtain credentials from [TikTok Shop Partners](https://partner.tiktokshop.com/)
2. Store in environment variables:
   ```bash
   TIKTOK_APP_KEY=your_app_key
   TIKTOK_APP_SECRET=your_app_secret
   TIKTOK_SERVICE_ID=your_service_id
   ```

3. Never commit these values to version control

### Webhook Security

Enable webhook signature verification:

```bash
TIKTOK_WEBHOOK_SECRET=your_webhook_secret
```

This ensures incoming webhooks are authentic and from TikTok.

### Callback Security

When configuring callbacks to your services:

```bash
CALLBACK_SECRET=your_callback_secret
```

The service will sign outgoing callbacks with HMAC-SHA256. Verify signatures on the receiving end.

## Database Security

### PostgreSQL

1. **Change default credentials** immediately
2. Use strong passwords (20+ characters)
3. Enable SSL in production:
   ```bash
   DATABASE_URL=postgres://user:pass@host:5432/db?sslmode=require
   ```

4. Regular backups:
   ```bash
   docker exec tiktok_sync_postgres pg_dump -U tiktok_sync tiktok_sync_db > backup.sql
   ```

### Redis

For production Redis:
1. Configure password authentication
2. Use Redis ACL for fine-grained access control
3. Consider Redis Sentinel for high availability

## Secrets Management

### Recommended Approach

For production deployments, consider using:

1. **Docker Secrets** (Docker Swarm):
   ```yaml
   secrets:
     postgres_password:
       external: true
   ```

2. **Kubernetes Secrets**:
   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: tiktok-sync-secrets
   ```

3. **Cloud Provider Secret Managers**:
   - AWS Secrets Manager
   - Google Cloud Secret Manager
   - Azure Key Vault

4. **HashiCorp Vault** for centralized secrets management

### Environment Variable Security

On production servers:
1. Use systemd environment files with restricted permissions
2. Never log environment variables
3. Rotate secrets regularly
4. Use different secrets for each environment

## Logging

### Safe Logging Practices

The service uses the `masker` package to automatically mask sensitive data in logs:

```go
import "github.com/NVMau/tiktok-sync-service/pkg/masker"

// Sensitive fields are automatically masked in logs
log.Info("User authenticated", "access_token", token) // Logs: access_token=***
```

### What Gets Masked

- Access tokens
- API keys
- Passwords
- Webhook signatures
- Any field matching sensitive patterns

## Incident Response

If credentials are compromised:

1. **Immediately** rotate all affected credentials
2. Review access logs for unauthorized access
3. Update all services with new credentials
4. Investigate the compromise source
5. Document the incident

## Security Checklist

Before deploying to production:

- [ ] All default passwords changed
- [ ] Environment variables set (not hardcoded)
- [ ] `.env` files have restricted permissions (600)
- [ ] Database uses SSL/TLS
- [ ] Webhook signature verification enabled
- [ ] Callback signature enabled
- [ ] Database ports not exposed to public
- [ ] Redis not exposed to public
- [ ] Logs reviewed for sensitive data leaks
- [ ] Regular backup schedule configured
- [ ] Monitoring and alerting set up

## Reporting Security Issues

If you discover a security vulnerability, please email security@your-domain.com instead of opening a public issue.

## Additional Resources

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [Docker Security Best Practices](https://docs.docker.com/engine/security/)
- [PostgreSQL Security](https://www.postgresql.org/docs/current/security.html)
- [TikTok Shop API Documentation](https://partner.tiktokshop.com/docv2/page/6507ead7b99d5302be949ba9)
