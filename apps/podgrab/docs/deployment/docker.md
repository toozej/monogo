# Docker Deployment Guide

Complete guide for deploying Podgrab using Docker and Docker Compose.

## Quick Start

### Using Docker Run

```bash
docker run -d \
  --name=podgrab \
  -p 8080:8080 \
  -v /path/to/config:/config \
  -v /path/to/data:/assets \
  -e PASSWORD=mypassword \
  akhilrex/podgrab:latest
```

### Using Docker Compose

Create `docker-compose.yml`:

```yaml
version: '3'
services:
  podgrab:
    image: akhilrex/podgrab:latest
    container_name: podgrab
    ports:
      - "8080:8080"
    volumes:
      - /path/to/config:/config
      - /path/to/data:/assets
    environment:
      - PASSWORD=mypassword
      - CHECK_FREQUENCY=30
    restart: unless-stopped
```

Start the service:

```bash
docker-compose up -d
```

## Container Images

### Official Images

Available on Docker Hub:

```
akhilrex/podgrab:latest        # Latest stable release
akhilrex/podgrab:v1.x.x        # Specific version
akhilrex/podgrab:develop       # Development branch
```

### Image Architecture

Multi-architecture support:

- `linux/amd64` - x86_64 (Intel/AMD)
- `linux/arm64` - ARM 64-bit (Raspberry Pi 4, Apple Silicon)
- `linux/arm/v7` - ARM 32-bit (Raspberry Pi 3)

Docker automatically pulls the correct architecture.

## Volume Mounts

### Required Volumes

#### Configuration Volume

```
-v /host/path/config:/config
```

**Contents:**

- `podgrab.db` - SQLite database
- `backups/` - Automatic database backups

**Permissions:** Read/write required

**Backup Recommendation:** Regular backups of this directory

#### Assets Volume

```
-v /host/path/data:/assets
```

**Contents:**

- Downloaded podcast episodes
- Episode artwork
- Podcast cover images

**Permissions:** Read/write required

**Storage Sizing:** Plan for podcast library growth (episodes average 50-100 MB
each)

### Volume Examples

#### Local Storage

```yaml
volumes:
  - ./podgrab/config:/config
  - ./podgrab/data:/assets
```

#### NAS/Network Share

```yaml
volumes:
  - /mnt/nas/podgrab/config:/config
  - /mnt/nas/podgrab/media:/assets
```

#### Named Docker Volumes

```yaml
volumes:
  - podgrab-config:/config
  - podgrab-assets:/assets

volumes:
  podgrab-config:
  podgrab-assets:
```

## Environment Variables

### Authentication

#### PASSWORD

Optional HTTP Basic Authentication password.

```yaml
environment:
  - PASSWORD=mysecurepassword
```

**Default:** None (no authentication)

**Username:** Always `podgrab` (hardcoded)

**Security Note:** Use strong passwords and HTTPS in production.

### Data Directories

#### CONFIG

Configuration and database directory.

```yaml
environment:
  - CONFIG=/config
```

**Default:** `/config`

**Note:** Rarely needs to be changed.

#### DATA

Assets and downloads directory.

```yaml
environment:
  - DATA=/assets
```

**Default:** `/assets`

**Note:** Rarely needs to be changed.

### Background Jobs

#### CHECK_FREQUENCY

Minutes between background job runs.

```yaml
environment:
  - CHECK_FREQUENCY=30
```

**Default:** `30` (minutes)

**Jobs Affected:**

- RSS feed refresh (every N minutes)
- Missing episode downloads (every N minutes)
- File verification (every N minutes)
- File size updates (every N×2 minutes)
- Image downloads (every N minutes)

**Recommendations:**

- **Light usage:** 60 minutes
- **Normal usage:** 30 minutes (default)
- **Heavy usage:** 15 minutes
- **Minimal load:** 120+ minutes

**Trade-offs:**

- Lower values: Faster new episode detection, higher CPU/network usage
- Higher values: Lower resource usage, delayed episode discovery

## Port Configuration

### Default Port

```yaml
ports:
  - "8080:8080"
```

Application listens on port `8080` inside container.

### Custom Host Port

```yaml
ports:
  - "9000:8080"  # Access via http://localhost:9000
```

### Multiple Instances

```yaml
# Instance 1
ports:
  - "8081:8080"

# Instance 2
ports:
  - "8082:8080"
```

**Note:** Each instance needs separate config and data volumes.

## Complete Docker Compose Examples

### Basic Setup

```yaml
version: '3.8'

services:
  podgrab:
    image: akhilrex/podgrab:latest
    container_name: podgrab
    ports:
      - "8080:8080"
    volumes:
      - ./config:/config
      - ./data:/assets
    restart: unless-stopped
```

### With Authentication

```yaml
version: '3.8'

services:
  podgrab:
    image: akhilrex/podgrab:latest
    container_name: podgrab
    ports:
      - "8080:8080"
    volumes:
      - ./config:/config
      - ./data:/assets
    environment:
      - PASSWORD=${PODGRAB_PASSWORD}
      - CHECK_FREQUENCY=30
    restart: unless-stopped
```

**`.env` file:**

```
PODGRAB_PASSWORD=mysecurepassword
```

### With Reverse Proxy (Traefik)

```yaml
version: '3.8'

services:
  podgrab:
    image: akhilrex/podgrab:latest
    container_name: podgrab
    volumes:
      - ./config:/config
      - ./data:/assets
    environment:
      - PASSWORD=${PODGRAB_PASSWORD}
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.podgrab.rule=Host(`podgrab.example.com`)"
      - "traefik.http.routers.podgrab.entrypoints=websecure"
      - "traefik.http.routers.podgrab.tls.certresolver=letsencrypt"
      - "traefik.http.services.podgrab.loadbalancer.server.port=8080"
    networks:
      - traefik
    restart: unless-stopped

networks:
  traefik:
    external: true
```

### With Nginx Reverse Proxy

```yaml
version: '3.8'

services:
  podgrab:
    image: akhilrex/podgrab:latest
    container_name: podgrab
    volumes:
      - ./config:/config
      - ./data:/assets
    environment:
      - PASSWORD=${PODGRAB_PASSWORD}
    networks:
      - internal
    restart: unless-stopped

  nginx:
    image: nginx:alpine
    container_name: podgrab-nginx
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./certs:/etc/nginx/certs:ro
    networks:
      - internal
    depends_on:
      - podgrab
    restart: unless-stopped

networks:
  internal:
```

### With Resource Limits

```yaml
version: '3.8'

services:
  podgrab:
    image: akhilrex/podgrab:latest
    container_name: podgrab
    ports:
      - "8080:8080"
    volumes:
      - ./config:/config
      - ./data:/assets
    environment:
      - PASSWORD=${PODGRAB_PASSWORD}
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 1G
        reservations:
          cpus: '0.5'
          memory: 512M
    restart: unless-stopped
```

## Building from Source

### Clone Repository

```bash
git clone https://github.com/akhilrex/podgrab.git
cd podgrab
```

### Build Image

```bash
docker build -t podgrab:custom .
```

### Multi-Architecture Build

```bash
docker buildx create --use
docker buildx build \
  --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t podgrab:custom \
  --push .
```

## Upgrading

### Stop Container

```bash
docker-compose down
```

### Pull Latest Image

```bash
docker-compose pull
```

### Start Container

```bash
docker-compose up -d
```

### Check Logs

```bash
docker-compose logs -f podgrab
```

### One-Liner Upgrade

```bash
docker-compose pull && docker-compose up -d
```

## Backup and Restore

### Backup

#### Stop Container

```bash
docker-compose down
```

#### Backup Volumes

```bash
# Create backup directory
mkdir -p backups/$(date +%Y%m%d)

# Backup config (database)
cp -r ./config backups/$(date +%Y%m%d)/

# Backup data (optional, can be large)
cp -r ./data backups/$(date +%Y%m%d)/
```

#### Start Container

```bash
docker-compose up -d
```

### Restore

#### Stop Container

```bash
docker-compose down
```

#### Restore Volumes

```bash
# Restore config
cp -r backups/20240115/config/* ./config/

# Restore data (optional)
cp -r backups/20240115/data/* ./data/
```

#### Start Container

```bash
docker-compose up -d
```

### Automated Backups

Create `backup.sh`:

```bash
#!/bin/bash
BACKUP_DIR="/path/to/backups"
DATE=$(date +%Y%m%d_%H%M%S)

# Create backup directory
mkdir -p "$BACKUP_DIR/$DATE"

# Backup database only (lightweight)
cp /path/to/config/podgrab.db "$BACKUP_DIR/$DATE/"

# Compress
cd "$BACKUP_DIR"
tar -czf "$DATE.tar.gz" "$DATE"
rm -rf "$DATE"

# Keep only last 30 days
find "$BACKUP_DIR" -name "*.tar.gz" -mtime +30 -delete
```

Add to crontab:

```bash
# Backup daily at 2 AM
0 2 * * * /path/to/backup.sh
```

## Troubleshooting

### Container Won't Start

**Check logs:**

```bash
docker-compose logs podgrab
```

**Common issues:**

- Port 8080 already in use
- Volume permissions incorrect
- Invalid environment variables

### Permission Errors

**Fix volume permissions:**

```bash
sudo chown -R 1000:1000 ./config ./data
```

**Note:** Podgrab runs as UID 1000 by default.

### Database Locked

**Symptoms:** "Database locked" errors in logs

**Cause:** Multiple instances accessing same database

**Solution:**

- Ensure only one container per config volume
- Check for stale lock files: `rm ./config/*.db-shm ./config/*.db-wal`

### Out of Disk Space

**Check usage:**

```bash
docker system df
```

**Clean up:**

```bash
# Remove unused images
docker image prune -a

# Remove unused volumes
docker volume prune
```

### High Memory Usage

**Monitor resources:**

```bash
docker stats podgrab
```

**Reduce memory:**

- Lower `CHECK_FREQUENCY` to reduce concurrent operations
- Decrease `MaxDownloadConcurrency` in settings
- Add memory limits to docker-compose.yml

### Network Issues

**Test connectivity:**

```bash
docker exec podgrab ping -c 3 google.com
```

**DNS issues:**

```yaml
dns:
  - 8.8.8.8
  - 8.8.4.4
```

### WebSocket Connection Failed

**Check reverse proxy configuration:**

- Ensure WebSocket upgrade headers are forwarded
- Verify `/ws` path is not blocked

**Nginx example:**

```nginx
location /ws {
    proxy_pass http://podgrab:8080/ws;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
}
```

## Performance Optimization

### Use SSD Storage

Mount volumes on SSD for better database performance:

```yaml
volumes:
  - /mnt/ssd/podgrab/config:/config
  - /mnt/hdd/podgrab/data:/assets  # Episodes can stay on HDD
```

### Adjust Download Concurrency

In Podgrab settings, set `MaxDownloadConcurrency`:

- **Low power devices:** 2-3
- **Normal systems:** 5 (default)
- **High performance:** 10+

### Resource Limits

```yaml
deploy:
  resources:
    limits:
      cpus: '4.0'      # Adjust based on your system
      memory: 2G
```

## Security Best Practices

### Use HTTPS

Always deploy with HTTPS in production:

- Use reverse proxy (Nginx, Traefik, Caddy)
- Obtain SSL certificate (Let's Encrypt)
- Redirect HTTP to HTTPS

### Strong Passwords

```yaml
environment:
  - PASSWORD=$(openssl rand -base64 32)
```

### Network Isolation

```yaml
networks:
  podgrab-internal:
    internal: true  # No external access
  proxy:
    external: true  # Reverse proxy only
```

### Regular Updates

```bash
# Weekly update check
0 0 * * 0 cd /path/to/podgrab && docker-compose pull && docker-compose up -d
```

## Monitoring

### Health Checks

Add to docker-compose.yml:

```yaml
healthcheck:
  test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8080"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 40s
```

### Log Management

**Limit log size:**

```yaml
logging:
  driver: "json-file"
  options:
    max-size: "10m"
    max-file: "3"
```

**View logs:**

```bash
docker-compose logs -f --tail=100 podgrab
```

## Related Documentation

- [Production Deployment](production.md) - Production best practices
- [Configuration Guide](../guides/configuration.md) - Application settings
- [REST API](../api/rest-api.md) - API reference
