# Docker Deployment

## Quick Start (Recommended)

Deploy drip-server using pre-built images from GitHub Container Registry:

```bash
# Pull the latest image
docker pull ghcr.io/gouryella/drip:latest

# Or use docker compose
curl -fsSL https://raw.githubusercontent.com/Gouryella/drip/main/docker-compose.release.yml -o docker-compose.yml

# Create .env file
cat > .env << EOF
DOMAIN=tunnel.example.com
AUTH_TOKEN=your-secret-token
VERSION=latest
EOF

# Place your TLS certificates
mkdir -p certs
cp /path/to/fullchain.pem certs/
cp /path/to/privkey.pem certs/

# Start server
docker compose up -d
```

## Build from Source

If you prefer to build locally:

### Server (Production)

```bash
# Copy and configure environment
cp .env.example .env
nano .env

# Edit server configuration
DOMAIN=tunnel.example.com
AUTH_TOKEN=your-secret-token
TLS_CERT=1
TLS_KEY=1

# Place certificates
mkdir -p certs
cp /path/to/fullchain.pem certs/
cp /path/to/privkey.pem certs/

# Uncomment volume mount in docker-compose.yml
# - ./certs:/app/data/certs:ro

# Start server
docker compose up -d

# View logs
docker compose logs -f
```

### Client (Development/Testing)

```bash
# Copy and configure client environment
cp .env.example .env.client
nano .env.client

# Edit client configuration
SERVER_ADDR=tunnel.example.com:443
AUTH_TOKEN=your-secret-token
TUNNEL_TYPE=http
LOCAL_PORT=3000

# Start client
docker compose -f docker-compose.client.yml --env-file .env.client up -d

# View logs
docker compose -f docker-compose.client.yml logs -f
```

## Configuration

### Environment Variables

Create `.env` from `.env.example`:

```bash
DOMAIN=tunnel.example.com
AUTH_TOKEN=your-secret-token
```

### TLS Certificates

**Option 1: Auto TLS (Let's Encrypt)**

```bash
# Enable in .env
AUTO_TLS=1

# Ensure port 80 is accessible for ACME challenges
```

**Option 2: Manual Certificates**

```bash
# Place certificates in ./certs/
mkdir -p certs
cp fullchain.pem certs/cert.pem
cp privkey.pem certs/key.pem

# Uncomment in docker-compose.yml
# - ./certs:/app/data/certs:ro

# Enable in .env
TLS_CERT=1
TLS_KEY=1
```

## Data Persistence

All data is stored in Docker volumes:

- `drip-data`: Server data and certificates at `/app/data`
- `client-data`: Client configuration at `/app/data`

### Backup

```bash
# Backup server data
docker run --rm -v drip-data:/data -v $(pwd):/backup alpine tar czf /backup/drip-backup.tar.gz -C /data .

# Restore
docker run --rm -v drip-data:/data -v $(pwd):/backup alpine tar xzf /backup/drip-backup.tar.gz -C /data
```

## Port Mapping

| Container Port | Host Port | Purpose |
|---------------|-----------|---------|
| 80 | 80 | HTTP (ACME challenges) |
| 443 | 443 | HTTPS (main service) |
| 8080 | 8080 | HTTP (no TLS) |
| 20000-20100 | 20000-20100 | TCP tunnels |

## Management

### Server

```bash
# Start
docker compose up -d

# Stop
docker compose down

# Restart
docker compose restart

# View logs
docker compose logs -f

# Shell access
docker compose exec server sh

# Update
docker compose pull
docker compose up -d
```

### Client

```bash
# Start
docker compose -f docker-compose.client.yml up -d

# Stop
docker compose -f docker-compose.client.yml down

# View logs
docker compose -f docker-compose.client.yml logs -f

# Different tunnel types
TUNNEL_TYPE=http LOCAL_PORT=3000 docker compose -f docker-compose.client.yml up -d
TUNNEL_TYPE=https LOCAL_PORT=8443 docker compose -f docker-compose.client.yml up -d
TUNNEL_TYPE=tcp LOCAL_PORT=5432 docker compose -f docker-compose.client.yml up -d
```

## Production Deployment

### With Reverse Proxy

If using Nginx/Traefik in front:

```yaml
services:
  server:
    ports:
      - "127.0.0.1:8080:8080"  # Only expose to localhost
    command: >
      server
      --domain tunnel.example.com
      --port 8080
      --token ${AUTH_TOKEN}
```

### Resource Limits

Adjust in `docker-compose.yml`:

```yaml
deploy:
  resources:
    limits:
      cpus: '2'
      memory: 512M
```

## Troubleshooting

**Certificate errors**

```bash
# Check certificate files
docker compose exec server ls -la /app/data/certs

# Check server logs
docker compose logs server | grep -i tls
```

**Connection issues**

```bash
# Verify port accessibility
curl -I https://tunnel.example.com

# Check server status
docker compose exec server /app/drip server --help
```

**Reset everything**

```bash
# Stop and remove everything
docker compose down -v

# Start fresh
docker compose up -d
```
