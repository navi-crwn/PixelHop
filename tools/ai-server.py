#!/usr/bin/env python3
"""AI image service for PixelHop (optional).

Answers on localhost only; PixelHop handles sign-in, limits and queueing.

  POST /remove   image bytes                      -> PNG, background removed (rembg)
  POST /faces    image bytes                      -> JSON {"faces": [[x, y, w, h], ...]}
  POST /erase    multipart: image, mask (white = erase) -> PNG, area filled in (LaMa)
  POST /upscale  image bytes                      -> PNG at 2x (Real-ESRGAN x2plus)
  POST /normalize image bytes (HEIC, AVIF, ...)    -> JPEG (PNG if transparent), upright,
                 no metadata; header X-Removed lists what was stripped (gps, camera, date, ...)
  POST /transform?w=&h=&fit=inside|cover&format=jpeg|png|webp|avif&q=  -> resized/re-encoded image
  GET  /health   -> "ok"

Models load the first time they're needed and are released after
AI_IDLE_MINUTES without use, so an idle server doesn't hold gigabytes.
Needs: rembg, onnxruntime, opencv-python-headless, numpy, pillow, pillow-heif.
"""
import gc
import io
import json
import os
import sys
import threading
import time
from email.parser import BytesParser
from email.policy import HTTP
from http.server import BaseHTTPRequestHandler, HTTPServer

import numpy as np
from PIL import Image, ImageOps

try:  # iPhone photos (HEIC/HEIF); AVIF is built into recent Pillow
    import pillow_heif

    pillow_heif.register_heif_opener()
except ImportError:
    pass

HOST = os.environ.get("AI_HOST", os.environ.get("REMBG_HOST", "127.0.0.1"))
PORT = int(os.environ.get("AI_PORT", os.environ.get("REMBG_PORT", "7001")))
MAX_BYTES = int(os.environ.get("AI_MAX_MB", "25")) * 1024 * 1024
MAX_SIDE = int(os.environ.get("AI_MAX_SIDE", "4096"))
UPSCALE_MAX_PIXELS = int(float(os.environ.get("AI_UPSCALE_MAX_MP", "1.0")) * 1_000_000)
IDLE = int(os.environ.get("AI_IDLE_MINUTES", "15")) * 60
REMBG_MODEL = os.environ.get("REMBG_MODEL", "u2net")
MODELS = os.environ.get("AI_MODELS", os.path.join(os.path.dirname(os.path.abspath(__file__)), "models"))
LAMA = os.environ.get("LAMA_MODEL", os.path.join(MODELS, "lama.onnx"))
ESRGAN = os.environ.get("ESRGAN_MODEL", os.path.join(MODELS, "RealESRGAN_x2plus.onnx"))
os.environ.setdefault("U2NET_HOME", MODELS)  # rembg looks for u2net.onnx here
THREADS = int(os.environ.get("AI_THREADS", "3"))


def log(msg):
    sys.stderr.write("ai-server: %s\n" % msg)
    sys.stderr.flush()


# ---------------------------------------------------------------- models
_models = {}
_used = {}
_lock = threading.Lock()


def model(name):
    """Load a model on first use; remember when it was last used."""
    with _lock:
        _used[name] = time.time()
        if name in _models:
            return _models[name]
        started = time.time()
        if name == "rembg":
            from rembg import new_session

            m = new_session(REMBG_MODEL)
        elif name in ("lama", "esrgan"):
            import onnxruntime as ort

            opts = ort.SessionOptions()
            opts.intra_op_num_threads = THREADS
            m = ort.InferenceSession(LAMA if name == "lama" else ESRGAN, opts, providers=["CPUExecutionProvider"])
        elif name == "faces":
            import cv2

            m = [cv2.CascadeClassifier(cv2.data.haarcascades + f) for f in
                 ("haarcascade_frontalface_default.xml", "haarcascade_profileface.xml")]
        else:
            raise KeyError(name)
        _models[name] = m
        log("loaded %s in %.1fs" % (name, time.time() - started))
        return m


def janitor():
    while True:
        time.sleep(60)
        with _lock:
            for name in [n for n, t in _used.items() if n in _models and time.time() - t > IDLE]:
                del _models[name]
                log("released %s (idle)" % name)
            gc.collect()


# ---------------------------------------------------------------- helpers
def open_image(data):
    img = Image.open(io.BytesIO(data))
    img = ImageOps.exif_transpose(img)
    img.load()
    if max(img.size) > MAX_SIDE:
        img.thumbnail((MAX_SIDE, MAX_SIDE), Image.LANCZOS)
    return img


def png(img):
    buf = io.BytesIO()
    img.save(buf, "PNG", optimize=False)
    return buf.getvalue()


def split_alpha(img):
    """RGB float array 0..1 and the alpha channel (or None)."""
    alpha = img.getchannel("A") if img.mode in ("RGBA", "LA") or "transparency" in img.info else None
    rgb = np.asarray(img.convert("RGB"), dtype=np.float32) / 255.0
    return rgb, alpha


# ---------------------------------------------------------------- tools
def remove_bg(data):
    from rembg import remove

    return png(remove(open_image(data), session=model("rembg")))


def faces(data):
    import cv2

    img = open_image(data).convert("L")
    w, h = img.size
    k = min(1.0, 1600 / max(w, h))  # detect on a smaller copy, scale boxes back
    small = img.resize((max(1, int(w * k)), max(1, int(h * k)))) if k < 1 else img
    gray = cv2.equalizeHist(np.asarray(small))
    found = []
    frontal, profile = model("faces")
    min_side = max(24, int(min(small.size) * 0.03))
    for cascade, flip in ((frontal, False), (profile, False), (profile, True)):
        g = gray[:, ::-1] if flip else gray
        for (x, y, fw, fh) in cascade.detectMultiScale(g, scaleFactor=1.1, minNeighbors=5, minSize=(min_side, min_side)):
            if flip:
                x = g.shape[1] - x - fw
            found.append([int(x / k), int(y / k), int(fw / k), int(fh / k)])
    # merge overlapping boxes from the different detectors
    found.sort(key=lambda b: -b[2] * b[3])
    kept = []
    for b in found:
        if all(iou(b, o) < 0.3 for o in kept):
            kept.append(b)
    return json.dumps({"faces": kept, "width": w, "height": h}).encode()


def iou(a, b):
    ax2, ay2, bx2, by2 = a[0] + a[2], a[1] + a[3], b[0] + b[2], b[1] + b[3]
    iw = max(0, min(ax2, bx2) - max(a[0], b[0]))
    ih = max(0, min(ay2, by2) - max(a[1], b[1]))
    inter = iw * ih
    return inter / float(a[2] * a[3] + b[2] * b[3] - inter or 1)


def erase(image_data, mask_data):
    """Fill the white parts of the mask. Only a square around the mask is sent
    through the 512x512 model, and only masked pixels are pasted back, so the
    rest of the photo keeps its full resolution."""
    img = open_image(image_data)
    rgb_img = img.convert("RGB")
    W, H = rgb_img.size
    mask = Image.open(io.BytesIO(mask_data)).convert("L").resize((W, H), Image.BILINEAR)
    m = np.asarray(mask) > 127
    if not m.any():
        return png(img)
    ys, xs = np.where(m)
    x0, x1, y0, y1 = xs.min(), xs.max() + 1, ys.min(), ys.max() + 1
    side = int(max(x1 - x0, y1 - y0) * 2.2) + 32  # plenty of context around the hole
    side = max(side, 256)
    cx, cy = (x0 + x1) // 2, (y0 + y1) // 2
    bx0 = int(min(max(0, cx - side // 2), max(0, W - side)))
    by0 = int(min(max(0, cy - side // 2), max(0, H - side)))
    bx1, by1 = min(W, bx0 + side), min(H, by0 + side)
    crop = rgb_img.crop((bx0, by0, bx1, by1))
    cmask = mask.crop((bx0, by0, bx1, by1))
    # grow the mask a little so edges of the object don't leave a halo
    import cv2

    cm = (np.asarray(cmask) > 127).astype(np.uint8)
    cm = cv2.dilate(cm, np.ones((9, 9), np.uint8), iterations=1)
    cw, ch = crop.size
    im512 = np.asarray(crop.resize((512, 512), Image.LANCZOS), dtype=np.float32) / 255.0
    m512 = cv2.resize(cm.astype(np.float32), (512, 512), interpolation=cv2.INTER_NEAREST)
    sess = model("lama")
    out = sess.run(None, {
        sess.get_inputs()[0].name: im512.transpose(2, 0, 1)[None],
        sess.get_inputs()[1].name: (m512 > 0.5).astype(np.float32)[None, None],
    })[0][0].transpose(1, 2, 0)
    if out.max() > 2.0:  # some exports return 0..255
        out = out / 255.0
    filled = Image.fromarray((np.clip(out, 0, 1) * 255).astype(np.uint8)).resize((cw, ch), Image.LANCZOS)
    # feather the paste so the seam disappears
    soft = cv2.GaussianBlur(cm.astype(np.float32), (0, 0), 3)
    soft = np.clip(np.maximum(soft, cm), 0, 1)[..., None]
    merged = np.asarray(filled, np.float32) * soft + np.asarray(crop, np.float32) * (1 - soft)
    result = rgb_img.copy()
    result.paste(Image.fromarray(merged.astype(np.uint8)), (bx0, by0))
    if img.mode == "RGBA":
        result.putalpha(img.getchannel("A"))
    return png(result)


def upscale(data):
    """Real-ESRGAN 2x, in overlapping tiles so memory stays flat."""
    img = open_image(data)
    w, h = img.size
    if w * h > UPSCALE_MAX_PIXELS:
        raise ValueError("too large to upscale: %dx%d (limit %.1f MP)" % (w, h, UPSCALE_MAX_PIXELS / 1e6))
    rgb, alpha = split_alpha(img)
    sess = model("esrgan")
    name = sess.get_inputs()[0].name
    tile, pad, s = 256, 12, 2
    out = np.zeros((h * s, w * s, 3), np.float32)
    for y in range(0, h, tile):
        for x in range(0, w, tile):
            x0, y0 = max(0, x - pad), max(0, y - pad)
            x1, y1 = min(w, x + tile + pad), min(h, y + tile + pad)
            part = rgb[y0:y1, x0:x1].transpose(2, 0, 1)[None]
            # x2plus packs pixels in 2x2 blocks, so tiles must have even sides
            eh, ew = part.shape[2] % 2, part.shape[3] % 2
            if eh or ew:
                part = np.pad(part, ((0, 0), (0, 0), (0, eh), (0, ew)), mode="edge")
            res = sess.run(None, {name: part})[0][0].transpose(1, 2, 0)
            # keep only the centre of each tile (drop the padded border)
            cy0, cx0 = (y - y0) * s, (x - x0) * s
            th, tw = (min(y + tile, h) - y) * s, (min(x + tile, w) - x) * s
            out[y * s:y * s + th, x * s:x * s + tw] = res[cy0:cy0 + th, cx0:cx0 + tw]
    result = Image.fromarray((np.clip(out, 0, 1) * 255).round().astype(np.uint8))
    if alpha is not None:
        result.putalpha(alpha.resize(result.size, Image.LANCZOS))
    return png(result)


def removed_metadata(img):
    """Which kinds of private data the file carried (names only, never values)."""
    found = []
    try:
        exif = img.getexif()
        if exif.get(0x8825) or exif.get_ifd(0x8825):
            found.append("gps")
        if exif.get(0x010F) or exif.get(0x0110):
            found.append("camera")
        if exif.get(0x0132) or exif.get_ifd(0x8769).get(0x9003):
            found.append("date")
        if exif.get(0x0131):
            found.append("software")
    except Exception:
        pass
    if img.info.get("xmp") or img.info.get("XML:com.adobe.xmp"):
        found.append("xmp")
    return found


def normalize(data):
    img = Image.open(io.BytesIO(data))
    found = removed_metadata(img)
    img = ImageOps.exif_transpose(img)
    if max(img.size) > 12000:
        raise ValueError("image is too large")
    buf = io.BytesIO()
    if img.mode in ("RGBA", "LA") or "transparency" in img.info:
        img.convert("RGBA").save(buf, "PNG")
        ctype = "image/png"
    else:
        img.convert("RGB").save(buf, "JPEG", quality=92, optimize=True)
        ctype = "image/jpeg"
    return buf.getvalue(), ctype, found


FORMATS = {"jpeg": ("JPEG", "image/jpeg"), "png": ("PNG", "image/png"), "webp": ("WEBP", "image/webp"), "avif": ("AVIF", "image/avif")}


def transform(data, q):
    w, h = int(q.get("w", "0") or 0), int(q.get("h", "0") or 0)
    fmt = q.get("format", "jpeg")
    quality = max(30, min(95, int(q.get("q", "82") or 82)))
    if fmt not in FORMATS or w < 0 or h < 0 or w > 4096 or h > 4096:
        raise ValueError("bad transform")
    img = ImageOps.exif_transpose(Image.open(io.BytesIO(data)))
    if w or h:
        if q.get("fit") == "cover" and w and h:
            img = ImageOps.fit(img, (w, h), Image.LANCZOS)
        else:
            img.thumbnail((w or img.width, h or img.height), Image.LANCZOS)
    name, ctype = FORMATS[fmt]
    if name == "JPEG":
        img = img.convert("RGB")
    elif img.mode not in ("RGB", "RGBA"):
        img = img.convert("RGBA")
    buf = io.BytesIO()
    img.save(buf, name, quality=quality, **({"method": 4} if name == "WEBP" else {}))
    return buf.getvalue(), ctype


# ---------------------------------------------------------------- HTTP
class Handler(BaseHTTPRequestHandler):
    server_version = "pixelhop-ai/1"

    def log_message(self, fmt, *args):
        log(fmt % args)

    def reply(self, code, body, ctype="text/plain", extra=None):
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        for k, v in (extra or {}).items():
            self.send_header(k, v)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path == "/health":
            self.reply(200, b"ok")
        else:
            self.reply(404, b"not found")

    def do_POST(self):
        try:
            n = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            n = 0
        if n <= 0 or n > MAX_BYTES * 2:
            return self.reply(413, b"image missing or too large")
        body = self.rfile.read(n)
        started = time.time()
        extra = {}
        path, _, query = self.path.partition("?")
        try:
            if path == "/normalize":
                out, ctype, found = normalize(body)
                extra["X-Removed"] = ",".join(found)
            elif path == "/transform":
                from urllib.parse import parse_qsl

                out, ctype = transform(body, dict(parse_qsl(query)))
            elif self.path == "/remove":
                out, ctype = remove_bg(body), "image/png"
            elif self.path == "/faces":
                out, ctype = faces(body), "application/json"
            elif self.path == "/upscale":
                out, ctype = upscale(body), "image/png"
            elif self.path == "/erase":
                parts = parse_multipart(self.headers.get("Content-Type", ""), body)
                if "image" not in parts or "mask" not in parts:
                    return self.reply(400, b"send image and mask")
                out, ctype = erase(parts["image"], parts["mask"]), "image/png"
            else:
                return self.reply(404, b"not found")
        except ValueError as exc:
            return self.reply(422, str(exc).encode())
        except Exception as exc:
            log("%s failed: %r" % (self.path, exc))
            return self.reply(500, ("could not process image: %s" % exc.__class__.__name__).encode())
        log("%s done in %.1fs" % (path, time.time() - started))
        self.reply(200, out, ctype, extra)


def parse_multipart(ctype, body):
    msg = BytesParser(policy=HTTP).parsebytes(b"Content-Type: " + ctype.encode() + b"\r\n\r\n" + body)
    out = {}
    for part in msg.iter_parts():
        name = part.get_param("name", header="content-disposition")
        if name:
            out[name] = part.get_payload(decode=True)
    return out


if __name__ == "__main__":
    threading.Thread(target=janitor, daemon=True).start()
    log("ready on http://%s:%d (models load on first use)" % (HOST, PORT))
    # One request at a time on purpose: every model already uses several cores.
    HTTPServer((HOST, PORT), Handler).serve_forever()
