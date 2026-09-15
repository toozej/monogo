#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../../.."
task_tmp=$(mktemp -d)
backend=""
proxy=""
cleanup() {
    if [[ -n "$proxy" ]]; then docker rm -f "$proxy" >/dev/null; fi
    if [[ -n "$backend" ]]; then docker rm -f "$backend" >/dev/null; fi
    rm -rf "$task_tmp"
}
trap cleanup EXIT
openssl req -x509 -newkey rsa:2048 -nodes -days 1 \
    -subj /CN=gocicle.example.com \
    -addext subjectAltName=DNS:gocicle.example.com,IP:127.0.0.1 \
    -keyout "$task_tmp/key.pem" -out "$task_tmp/cert.pem" >/dev/null 2>&1
python3 - "$task_tmp" <<'PY'
from pathlib import Path
import sys

target = Path(sys.argv[1])
nginx = Path("apps/gocicle/deploy/nginx.conf").read_text()
nginx = nginx.replace("/etc/letsencrypt/live/gocicle.example.com/fullchain.pem", "/fixtures/cert.pem")
nginx = nginx.replace("/etc/letsencrypt/live/gocicle.example.com/privkey.pem", "/fixtures/key.pem")
(target / "nginx.conf").write_text("events {}\nhttp {\n" + nginx + "\n}\n")
caddy = Path("apps/gocicle/deploy/Caddyfile").read_text()
caddy = caddy.replace("gocicle.example.com {", "gocicle.example.com {\n    tls /fixtures/cert.pem /fixtures/key.pem", 1)
(target / "Caddyfile").write_text("{\n    auto_https off\n}\n" + caddy)
PY
cat > "$task_tmp/backend.py" <<'PY'
from http.server import BaseHTTPRequestHandler, HTTPServer
import time

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("X-Observed-Proto", self.headers.get("X-Forwarded-Proto", "missing"))
        self.end_headers()
        self.wfile.write(b"data: first\n\n")
        self.wfile.flush()
        if self.path == "/events":
            time.sleep(2)
            self.wfile.write(b"data: second\n\n")
            self.wfile.flush()

HTTPServer(("127.0.0.1", 8080), Handler).serve_forever()
PY
for kind in nginx caddy; do
    backend=$(docker run -d --rm -p 127.0.0.1::443 -v "$task_tmp:/fixtures:ro" python:3.13-alpine python /fixtures/backend.py)
    port=$(docker port "$backend" 443/tcp | cut -d: -f2)
    if [[ "$kind" == nginx ]]; then
        proxy=$(docker run -d --rm --network "container:$backend" -v "$task_tmp:/fixtures:ro" nginx:stable-alpine nginx -c /fixtures/nginx.conf -g 'daemon off;')
    else
        proxy=$(docker run -d --rm --network "container:$backend" -v "$task_tmp:/fixtures:ro" caddy:2-alpine caddy run --config /fixtures/Caddyfile --adapter caddyfile)
    fi
    python3 - "$port" "$task_tmp/cert.pem" "$kind" <<'PY'
import http.client
import socket
import ssl
import sys
import time

port, certificate, kind = sys.argv[1:]
context = ssl.create_default_context(cafile=certificate)
def connect():
    connection = http.client.HTTPSConnection("gocicle.example.com", int(port), context=context, timeout=5)
    connection.sock = context.wrap_socket(socket.create_connection(("127.0.0.1", int(port)), timeout=5), server_hostname="gocicle.example.com")
    return connection

for attempt in range(60):
    try:
        connection = connect()
        connection.request("GET", "/healthz", headers={"Host": "gocicle.example.com"})
        response = connection.getresponse()
        assert response.status == 200
        response.read()
        connection.close()
        break
    except (OSError, http.client.HTTPException, AssertionError):
        time.sleep(0.2)
else:
    raise AssertionError(kind + " proxy did not start")
connection = connect()
start = time.monotonic()
connection.request("GET", "/events", headers={"Host": "gocicle.example.com"})
response = connection.getresponse()
assert response.status == 200
assert response.getheader("X-Observed-Proto") == "https"
assert response.readline() == b"data: first\n"
assert time.monotonic() - start < 1.5, "proxy buffered the first event"
assert b"data: second" in response.read()
assert time.monotonic() - start >= 2
connection.close()
print(kind + ": HTTPS and incremental streaming passed")
PY
    docker rm -f "$proxy" >/dev/null
    proxy=""
    docker rm -f "$backend" >/dev/null
    backend=""
done
