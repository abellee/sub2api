"""Offline checks for the split deployment; never contacts a real backend."""
import http.client
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from unittest import mock

import yaml

ROOT = Path(__file__).resolve().parents[2]
SPLIT = ROOT / "deploy/split"
NGINX = os.environ.get("NGINX_BIN") or shutil.which("nginx")


def read(path):
    return (ROOT / path).read_text(encoding="utf-8")


class ConfigurationTests(unittest.TestCase):
    def test_builds_are_independent(self):
        backend = read("Dockerfile.backend")
        frontend = read("Dockerfile.frontend")
        self.assertNotIn("-tags embed", backend)
        self.assertNotIn("frontend-builder", backend)
        self.assertNotIn("COPY frontend", backend)
        for required in ("pg_dump", "psql", "/app/resources", "docker-entrypoint.sh", "HEALTHCHECK"):
            self.assertIn(required, backend)
        self.assertNotIn("go build", frontend)
        self.assertIn("ENV VITE_API_BASE_URL=/api/v1", frontend)
        self.assertIn("/app/backend/internal/web/dist", frontend)
        self.assertIn("NGINX_ENVSUBST_FILTER", frontend)

    def test_build_pins_source_and_does_not_deploy(self):
        text = read(".github/workflows/build-split.yml")
        self.assertIn("github.event.repository.private", text)
        self.assertIn("sha-${GITHUB_SHA}", text)
        self.assertNotRegex(text, r"(?im)^\s*(ssh |scp |.*docker compose .*up)")

    def test_compose_is_two_containers_and_private_backend(self):
        data = yaml.safe_load(read("deploy/docker-compose.split.yml"))
        self.assertEqual(set(data["services"]), {"frontend", "backend"})
        backend, frontend = data["services"]["backend"], data["services"]["frontend"]
        self.assertNotIn("ports", backend)
        self.assertIn("127.0.0.1", frontend["ports"][0])
        self.assertFalse(backend["volumes"][0]["bind"]["create_host_path"])
        self.assertEqual(frontend["depends_on"]["backend"]["condition"], "service_healthy")
        self.assertEqual(backend["environment"]["SECURITY_TRUST_FORWARDED_IP_FOR_API_KEY_ACL"], "false")
        for service in (backend, frontend):
            self.assertIn(":?", service["image"])

    def test_proxy_does_not_buffer_retry_or_trust_arbitrary_ip(self):
        proxy = read("deploy/split/proxy.conf")
        for directive in ("proxy_buffering off;", "proxy_request_buffering off;", "proxy_next_upstream off;",
                          "proxy_set_header X-Forwarded-For $remote_addr;", "proxy_set_header Upgrade $http_upgrade;"):
            self.assertIn(directive, proxy)
        self.assertNotIn("$proxy_add_x_forwarded_for", proxy)



class FakeBackend(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, *_):
        pass

    def do_POST(self):
        self.do_GET()

    def do_GET(self):
        if self.path == "/internal/frontend/check":
            self.send_response(403 if self.headers.get("X-Frontend-Path", "").split("?")[0].rstrip("/") == "/public/transit" else 204)
            self.send_header("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'")
            self.send_header("Content-Length", "0")
            self.end_headers()
            return
        if self.path == "/v1/stream":
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.send_header("Connection", "close")
            self.end_headers()
            self.wfile.write(b"data: first\n\n")
            self.wfile.flush()
            time.sleep(0.8)
            self.wfile.write(b"data: last\n\n")
            self.close_connection = True
            return
        if self.headers.get("Upgrade", "").lower() == "websocket":
            self.send_response(101)
            self.send_header("Upgrade", "websocket")
            self.send_header("Connection", "Upgrade")
            self.end_headers()
            self.wfile.write(b"WS-PASSTHROUGH")
            self.wfile.flush()
            self.close_connection = True
            return
        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length).decode() if length else ""
        output = json.dumps({"path": self.path, "method": self.command, "body": body,
                             "headers": dict(self.headers)}).encode()
        self.send_response(206 if self.headers.get("Range") else 200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(output)))
        self.end_headers()
        self.wfile.write(output)


@unittest.skipUnless(NGINX, "Nginx not installed; live proxy checks not verified")
class ProxyTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory(prefix="sub2api-split-")
        cls.addClassCleanup(cls.temp.cleanup)
        root = Path(cls.temp.name)
        # Nginx workers need access even when the test runner is root.
        root.chmod(0o755)
        html = root / "html"
        html.mkdir()
        (html / "index.html").write_text("<html>SPLIT-SPA</html>")
        (html / "assets").mkdir()
        (html / "assets/app-123.js").write_text("console.log('split')")
        previous = root / "previous"
        (previous / "assets").mkdir(parents=True)
        (previous / "assets/old-123.js").write_text("console.log('old')")
        cls.backend = ThreadingHTTPServer(("127.0.0.1", 0), FakeBackend)
        cls.addClassCleanup(cls.backend.server_close)
        cls.addClassCleanup(cls.backend.shutdown)
        threading.Thread(target=cls.backend.serve_forever, daemon=True).start()
        with socket.socket() as reservation:
            reservation.bind(("127.0.0.1", 0))
            cls.port = reservation.getsockname()[1]
        proxy = root / "proxy.conf"
        proxy.write_text(read("deploy/split/proxy.conf"))
        config = read("deploy/split/default.conf.template")
        for key, value in {"BACKEND_UPSTREAM": f"127.0.0.1:{cls.backend.server_port}",
                           "FRONTEND_LISTEN": "127.0.0.1:8080", "PUBLIC_SCHEME": "https", "TRUSTED_PROXY_CIDR": "192.0.2.1/32"}.items():
            config = config.replace("${" + key + "}", value)
        config = config.replace("listen 127.0.0.1:8080;", f"listen 127.0.0.1:{cls.port};")
        config = config.replace("/usr/share/nginx/html", str(html))
        config = config.replace("/usr/share/nginx/previous", str(previous))
        config = config.replace("/etc/nginx/snippets/sub2api-proxy.conf", str(proxy))
        conf = root / "nginx.conf"
        conf.write_text(f"pid {root}/nginx.pid;\nerror_log {root}/error.log;\nevents {{}}\nhttp {{\n"
                        f"access_log off; client_body_temp_path {root}/body; proxy_temp_path {root}/proxy;\n"
                        f"fastcgi_temp_path {root}/fastcgi; uwsgi_temp_path {root}/uwsgi; scgi_temp_path {root}/scgi;\n"
                        + config + "\n}\n")
        checked = subprocess.run([NGINX, "-p", str(root), "-c", str(conf), "-t"], capture_output=True, text=True)
        if checked.returncode:
            raise RuntimeError(checked.stderr)
        cls.process = subprocess.Popen([NGINX, "-p", str(root), "-c", str(conf), "-g", "daemon off;"],
                                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        cls.addClassCleanup(cls.stop_nginx)
        for _ in range(100):
            try:
                with socket.create_connection(("127.0.0.1", cls.port), timeout=0.1):
                    return
            except OSError:
                time.sleep(0.03)
        raise RuntimeError((root / "error.log").read_text())

    @classmethod
    def stop_nginx(cls):
        cls.process.terminate()
        try:
            cls.process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            cls.process.kill()
            cls.process.wait()

    def request(self, path, method="GET", body=None, headers=None):
        conn = http.client.HTTPConnection("127.0.0.1", self.port, timeout=4)
        try:
            conn.request(method, path, body=body, headers=headers or {})
            response = conn.getresponse()
            return response.status, dict(response.getheaders()), response.read()
        finally:
            conn.close()

    def test_spa_refresh_models_and_static_caching(self):
        for path in ("/", "/admin/users", "/setup", "/oauth/mcp/authorize", "/models"):
            with self.subTest(path=path):
                status, headers, body = self.request(path, headers={"Accept": "text/html"})
                self.assertEqual(status, 200)
                self.assertIn(b"SPLIT-SPA", body)
                self.assertIn("no-store", headers["Cache-Control"])
                self.assertEqual(headers["Content-Security-Policy"], "default-src 'self'; frame-ancestors 'none'")
        status, headers, _ = self.request("/assets/app-123.js")
        self.assertEqual(status, 200)
        self.assertIn("immutable", headers["Cache-Control"])
        for path in ("/assets/missing.js", "/missing.css"):
            self.assertEqual(self.request(path)[0], 404)
        self.assertEqual(self.request("/frontend-health")[2], b"ok\n")
        status, headers, body = self.request("/assets/old-123.js")
        self.assertEqual(status, 200)
        self.assertIn(b"console.log('old')", body)
        self.assertIn("immutable", headers["Cache-Control"])

    def test_all_existing_embed_bypass_routes_and_media(self):
        source = read("backend/internal/web/embed_on.go").split("func shouldBypassEmbeddedFrontend", 1)[1].split("func serveIndexHTML", 1)[0]
        paths = re.findall(r'trimmed == "([^"]+)"', source)
        paths += [path + "sample" for path in re.findall(r'strings.HasPrefix\(trimmed, "([^"]+)"\)', source)]
        paths += ["/media/video", "/internal/media/settle", "/m/video", "/cdn/recorded.mp4",
                  "/api/v1/images/content.png", "/api/v1/admin/ops/live?token=test", "/.well-known/ai-transit.json"]
        for path in set(paths):
            with self.subTest(path=path):
                status, _, body = self.request(path)
                self.assertEqual(status, 200)
                self.assertEqual(json.loads(body)["path"], path)

    def test_disabled_public_page_and_internal_subrequest(self):
        for path in ("/public/transit", "/public/transit/", "/public/transit?x=1"):
            self.assertEqual(self.request(path)[0], 404)
        self.assertEqual(self.request("/_frontend_check")[0], 404)

    def test_request_body_range_and_untrusted_headers(self):
        status, _, body = self.request("/v1/responses?x=1", "POST", '{"model":"fake"}',
                                       {"X-Forwarded-For": "6.6.6.6", "CF-Connecting-IP": "7.7.7.7"})
        output = json.loads(body)
        self.assertEqual(status, 200)
        self.assertEqual(output["body"], '{"model":"fake"}')
        self.assertEqual(output["method"], "POST")
        self.assertEqual(output["headers"]["X-Forwarded-For"], "127.0.0.1")
        self.assertEqual(output["headers"]["X-Forwarded-Proto"], "https")
        self.assertNotIn("CF-Connecting-IP", output["headers"])
        status, _, body = self.request("/m/video", headers={"Range": "bytes=0-99"})
        self.assertEqual(status, 206)
        self.assertEqual(json.loads(body)["headers"]["Range"], "bytes=0-99")

    def test_stream_is_delivered_before_completion(self):
        conn = http.client.HTTPConnection("127.0.0.1", self.port, timeout=4)
        try:
            start = time.monotonic()
            conn.request("GET", "/v1/stream")
            response = conn.getresponse()
            self.assertEqual(response.readline(), b"data: first\n")
            self.assertLess(time.monotonic() - start, 0.7)
            self.process.send_signal(signal.SIGHUP)
            self.assertEqual(self.request("/health")[0], 200)
            self.assertIn(b"data: last", response.read())
        finally:
            conn.close()

    def test_websocket_upgrade_and_bytes(self):
        with socket.create_connection(("127.0.0.1", self.port), timeout=4) as conn:
            conn.sendall(b"GET /responses HTTP/1.1\r\nHost: localhost\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
            response = b""
            while b"WS-PASSTHROUGH" not in response:
                chunk = conn.recv(4096)
                if not chunk:
                    break
                response += chunk
            self.assertIn(b"101 Switching Protocols", response)
            self.assertIn(b"WS-PASSTHROUGH", response)


if __name__ == "__main__":
    unittest.main()
