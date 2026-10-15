#!/usr/bin/env python3
"""Background-removal service for PixelHop (optional).

Keeps one rembg model loaded and answers on localhost:
  POST /remove   body: image bytes  ->  PNG with a transparent background
  GET  /health   -> "ok"

PixelHop (PIXELHOP_REMBG_URL=http://127.0.0.1:7001) sends one image at a
time and enforces sign-in and daily limits; this service only does the work.

Needs: pip install "rembg[cpu]"   (models download to ~/.u2net on first use)
Run:   REMBG_MODEL=u2net python3 tools/rembg-server.py
Uses only the standard library besides rembg, so no web framework is needed.
"""
import io
import os
import sys
import time
from http.server import BaseHTTPRequestHandler, HTTPServer

from rembg import new_session, remove

HOST = os.environ.get("REMBG_HOST", "127.0.0.1")
PORT = int(os.environ.get("REMBG_PORT", "7001"))
MODEL = os.environ.get("REMBG_MODEL", "u2net")
MAX_BYTES = int(os.environ.get("REMBG_MAX_MB", "25")) * 1024 * 1024
MAX_SIDE = int(os.environ.get("REMBG_MAX_SIDE", "4096"))

print(f"rembg-server: loading model {MODEL}…", flush=True)
SESSION = new_session(MODEL)
print(f"rembg-server: ready on http://{HOST}:{PORT}", flush=True)


class Handler(BaseHTTPRequestHandler):
    server_version = "pixelhop-rembg/1"

    def log_message(self, fmt, *args):  # keep logs short; never log image data
        sys.stderr.write("rembg-server: %s\n" % (fmt % args))

    def _reply(self, code, body, ctype="text/plain"):
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path == "/health":
            self._reply(200, b"ok")
        else:
            self._reply(404, b"not found")

    def do_POST(self):
        if self.path != "/remove":
            return self._reply(404, b"not found")
        try:
            n = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            n = 0
        if n <= 0 or n > MAX_BYTES:
            return self._reply(413, b"image missing or too large")
        data = self.rfile.read(n)
        try:
            from PIL import Image

            img = Image.open(io.BytesIO(data))
            img.load()
            if max(img.size) > MAX_SIDE:  # keep memory and time bounded
                img.thumbnail((MAX_SIDE, MAX_SIDE))
            started = time.time()
            out = remove(img, session=SESSION)
            buf = io.BytesIO()
            out.save(buf, "PNG", optimize=False)
            sys.stderr.write("rembg-server: %dx%d in %.1fs\n" % (img.size[0], img.size[1], time.time() - started))
            self._reply(200, buf.getvalue(), "image/png")
        except Exception as exc:  # bad image or model error
            self._reply(422, ("could not process image: %s" % exc.__class__.__name__).encode())


if __name__ == "__main__":
    # One request at a time on purpose: the model uses all CPU cores already.
    HTTPServer((HOST, PORT), Handler).serve_forever()
