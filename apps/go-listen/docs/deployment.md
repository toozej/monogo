# Deployment Guide

This guide covers different deployment options for the go-listen application.

## Prerequisites

1. **Spotify Developer Account**: Required for API access
   - Create an app at [Spotify Developer Dashboard](https://developer.spotify.com/dashboard)
   - Note your Client ID and Client Secret
   - Configure redirect URI: `http://your-domain:port/callback`

2. **System Requirements**:
   - Linux/macOS/Windows
   - Network access to Spotify API
   - Available port for HTTP server (default: 8080)

## Deployment Options

### 1. Binary Deployment

#### Build using Make

```bash
# Build the binary
make local-build 

# Create configuration
cp .env.sample .env
# Edit .env with your Spotify credentials

# Run the application
make local-run
```

#### Build from Source

```bash
# Clone the repository
git clone https://github.com/toozej/go-listen.git
cd go-listen

# Build the binary
go build -o go-listen .

# Create configuration
cp .env.sample .env
# Edit .env with your Spotify credentials

# Run the application
./go-listen serve
```

#### Download Pre-built Binary

```bash
# Download latest release (replace with actual URL)
wget https://github.com/toozej/go-listen/releases/latest/download/go-listen-linux-amd64
chmod +x go-listen-linux-amd64
mv go-listen-linux-amd64 go-listen

# Set up configuration
export SPOTIFY_CLIENT_ID=your_client_id
export SPOTIFY_CLIENT_SECRET=your_client_secret

# Run the application
./go-listen serve
```

### 2. Docker Deployment

#### Using Docker Compose (Recommended)

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  go-listen:
    image: go-listen:latest
    container_name: go-listen
    ports:
      - "8080:8080"
    environment:
      # Required Spotify configuration
      - SPOTIFY_CLIENT_ID=${SPOTIFY_CLIENT_ID}
      - SPOTIFY_CLIENT_SECRET=${SPOTIFY_CLIENT_SECRET}
      - SPOTIFY_REDIRECT_URL=http://localhost:8080/callback
      
      # Server configuration
      - SERVER_HOST=0.0.0.0
      - SERVER_PORT=8080
      
      # Scraper configuration
      - SCRAPER_TIMEOUT=30s
      - SCRAPER_MAX_RETRIES=3
      - SCRAPER_RETRY_BACKOFF=2s
      - SCRAPER_USER_AGENT=go-listen/1.0
      - SCRAPER_MAX_CONTENT_SIZE=10485760
      
      # Security configuration
      - SECURITY_RATE_LIMIT_REQUESTS_PER_SECOND=10
      - SECURITY_RATE_LIMIT_BURST=20
      
      # Logging configuration
      - LOGGING_LEVEL=info
      - LOGGING_FORMAT=json
      - LOGGING_ENABLE_HTTP=true
    
    restart: unless-stopped
    
    # Optional: Mount logs directory
    volumes:
      - ./logs:/app/logs
    
    # Optional: Health check
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

# Optional: Add reverse proxy
  nginx:
    image: nginx:alpine
    container_name: go-listen-proxy
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./ssl:/etc/nginx/ssl:ro
    depends_on:
      - go-listen
    restart: unless-stopped
```

Create `.env` file for Docker Compose:

```bash
# Spotify API credentials
SPOTIFY_CLIENT_ID=your_actual_client_id
SPOTIFY_CLIENT_SECRET=your_actual_client_secret
```

Deploy:

```bash
# Start the services
docker-compose up -d

# View logs
docker-compose logs -f go-listen

# Stop the services
docker-compose down
```

#### Using Docker Run

```bash
# Run with environment variables
docker run -d \
  --name go-listen \
  -p 8080:8080 \
  -e SPOTIFY_CLIENT_ID=your_client_id \
  -e SPOTIFY_CLIENT_SECRET=your_client_secret \
  -e SERVER_HOST=0.0.0.0 \
  --restart unless-stopped \
  go-listen:latest
```

### 3. Systemd Service (Linux)

Create `/etc/systemd/system/go-listen.service`:

```ini
[Unit]
Description=go-listen Spotify Playlist Manager
After=network.target
Wants=network.target

[Service]
Type=simple
User=go-listen
Group=go-listen
WorkingDirectory=/opt/go-listen
ExecStart=/opt/go-listen/go-listen serve
Restart=always
RestartSec=5

# Environment variables
Environment=SPOTIFY_CLIENT_ID=your_client_id
Environment=SPOTIFY_CLIENT_SECRET=your_client_secret
Environment=SERVER_HOST=0.0.0.0
Environment=SERVER_PORT=8080
Environment=SCRAPER_TIMEOUT=30s
Environment=SCRAPER_MAX_RETRIES=3
Environment=SCRAPER_RETRY_BACKOFF=2s
Environment=LOGGING_LEVEL=info
Environment=LOGGING_FORMAT=json
Environment=LOGGING_ENABLE_HTTP=true

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/go-listen/logs

[Install]
WantedBy=multi-user.target
```

Set up the service:

```bash
# Create user and directory
sudo useradd --system --shell /bin/false go-listen
sudo mkdir -p /opt/go-listen/logs
sudo chown go-listen:go-listen /opt/go-listen/logs

# Copy binary
sudo cp go-listen /opt/go-listen/
sudo chown go-listen:go-listen /opt/go-listen/go-listen
sudo chmod +x /opt/go-listen/go-listen

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable go-listen
sudo systemctl start go-listen

# Check status
sudo systemctl status go-listen
sudo journalctl -u go-listen -f
```

### 4. Kubernetes Deployment

Create `k8s-deployment.yaml`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: go-listen-secrets
type: Opaque
stringData:
  spotify-client-id: "your_client_id"
  spotify-client-secret: "your_client_secret"

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: go-listen-config
data:
  SERVER_HOST: "0.0.0.0"
  SERVER_PORT: "8080"
  SCRAPER_TIMEOUT: "30s"
  SCRAPER_MAX_RETRIES: "3"
  SCRAPER_RETRY_BACKOFF: "2s"
  SCRAPER_USER_AGENT: "go-listen/1.0"
  SCRAPER_MAX_CONTENT_SIZE: "10485760"
  LOGGING_LEVEL: "info"
  LOGGING_FORMAT: "json"
  LOGGING_ENABLE_HTTP: "true"
  SECURITY_RATE_LIMIT_REQUESTS_PER_SECOND: "10"
  SECURITY_RATE_LIMIT_BURST: "20"

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: go-listen
  labels:
    app: go-listen
spec:
  replicas: 2
  selector:
    matchLabels:
      app: go-listen
  template:
    metadata:
      labels:
        app: go-listen
    spec:
      containers:
      - name: go-listen
        image: go-listen:latest
        ports:
        - containerPort: 8080
        env:
        - name: SPOTIFY_CLIENT_ID
          valueFrom:
            secretKeyRef:
              name: go-listen-secrets
              key: spotify-client-id
        - name: SPOTIFY_CLIENT_SECRET
          valueFrom:
            secretKeyRef:
              name: go-listen-secrets
              key: spotify-client-secret
        envFrom:
        - configMapRef:
            name: go-listen-config
        livenessProbe:
          httpGet:
            path: /
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "200m"

---
apiVersion: v1
kind: Service
metadata:
  name: go-listen-service
spec:
  selector:
    app: go-listen
  ports:
  - protocol: TCP
    port: 80
    targetPort: 8080
  type: ClusterIP

---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: go-listen-ingress
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  rules:
  - host: go-listen.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: go-listen-service
            port:
              number: 80
```

Deploy to Kubernetes:

```bash
# Apply the configuration
kubectl apply -f k8s-deployment.yaml

# Check deployment status
kubectl get pods -l app=go-listen
kubectl get services
kubectl get ingress

# View logs
kubectl logs -l app=go-listen -f
```

## Reverse Proxy Configuration

### Nginx Configuration

Create `nginx.conf`:

```nginx
events {
    worker_connections 1024;
}

http {
    upstream go-listen {
        server go-listen:8080;
        # For multiple instances:
        # server go-listen-1:8080;
        # server go-listen-2:8080;
    }

    server {
        listen 80;
        server_name your-domain.com;
        
        # Redirect HTTP to HTTPS
        return 301 https://$server_name$request_uri;
    }

    server {
        listen 443 ssl http2;
        server_name your-domain.com;

        # SSL configuration
        ssl_certificate /etc/nginx/ssl/cert.pem;
        ssl_certificate_key /etc/nginx/ssl/key.pem;
        ssl_protocols TLSv1.2 TLSv1.3;
        ssl_ciphers ECDHE-RSA-AES256-GCM-SHA512:DHE-RSA-AES256-GCM-SHA512:ECDHE-RSA-AES256-GCM-SHA384:DHE-RSA-AES256-GCM-SHA384;
        ssl_prefer_server_ciphers off;

        # Security headers
        add_header X-Frame-Options DENY;
        add_header X-Content-Type-Options nosniff;
        add_header X-XSS-Protection "1; mode=block";
        add_header Strict-Transport-Security "max-age=63072000; includeSubDomains; preload";

        # Proxy configuration
        location / {
            proxy_pass http://go-listen;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            
            # WebSocket support (if needed in future)
            proxy_http_version 1.1;
            proxy_set_header Upgrade $http_upgrade;
            proxy_set_header Connection "upgrade";
        }

        # Static files caching
        location /static/ {
            proxy_pass http://go-listen;
            expires 1y;
            add_header Cache-Control "public, immutable";
        }
    }
}
```

### Caddy Configuration

Create `Caddyfile`:

```caddy
your-domain.com {
    reverse_proxy go-listen:8080
    
    # Automatic HTTPS
    tls your-email@domain.com
    
    # Security headers
    header {
        X-Frame-Options DENY
        X-Content-Type-Options nosniff
        X-XSS-Protection "1; mode=block"
        Strict-Transport-Security "max-age=63072000; includeSubDomains; preload"
    }
    
    # Static file caching
    @static path /static/*
    header @static Cache-Control "public, max-age=31536000, immutable"
}
```

## Monitoring and Maintenance

### Health Checks

The application provides a simple health check endpoint:

```bash
# Check if the application is running
curl -f http://localhost:8080/ || echo "Service is down"
```

### Log Monitoring

Monitor application logs for issues:

```bash
# For systemd service
sudo journalctl -u go-listen -f

# For Docker
docker logs -f go-listen

# For Docker Compose
docker-compose logs -f go-listen
```

### Backup and Recovery

The application is stateless, so backup considerations:

1. **Configuration**: Backup your environment variables/config files
2. **Spotify Integration**: No local data to backup (uses Spotify API)
3. **Logs**: Consider log rotation and archival if needed

### Updates

To update the application:

1. **Binary deployment**: Replace binary and restart service
2. **Docker deployment**: Update image tag and restart containers
3. **Kubernetes**: Update deployment with new image

```bash
# Example Docker update
docker-compose pull
docker-compose up -d

# Example Kubernetes update
kubectl set image deployment/go-listen go-listen=go-listen:new-version
```

## Troubleshooting

### Common Issues

1. **Port already in use**: Change `SERVER_PORT` or stop conflicting service
2. **Spotify API errors**: Verify credentials and network connectivity
3. **Permission denied**: Check file permissions and user privileges
4. **High memory usage**: Monitor and adjust resource limits

### Debug Mode

Enable debug logging for troubleshooting:

```bash
# Environment variable
LOGGING_LEVEL=debug

# Command line flag
./go-listen serve --debug
```

### Performance Monitoring

Monitor key metrics:
- HTTP response times
- Rate limiting hits
- Memory and CPU usage
- Spotify API response times
- Error rates

Consider using monitoring tools like Prometheus, Grafana, or application performance monitoring (APM) solutions.