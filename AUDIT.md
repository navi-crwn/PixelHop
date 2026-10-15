# PixelHop audit (October 2026)

This is the audit that led to PixelHop 3. It covers two code bases that had drifted apart:

- **Repo**: `main` at v2.3.0 (`3b7119a`), the code people clone from GitHub. Kept unchanged in [`legacy/`](legacy/) and tagged `v2.3.0-php`.
- **Production**: the PHP install serving p.hel.ink, read-only.

## Summary

| # | Finding | Repo | Production | PixelHop 3 |
|---|---|---|---|---|
| 1 | Repo and production have drifted apart | — | 40+ files differ; features and security fixes exist only on the server | One code base; releases are built from Git |
| 2 | Server-side request forgery in "upload from URL" | **Vulnerable** | Fixed (`ImageHandler::assertPublicUrl`) | Fixed at connect time, after DNS ([`internal/netsafe`](internal/netsafe/netsafe.go)) |
| 3 | Client IP taken from spoofable headers | **Vulnerable** (18 places trust `X-Forwarded-For` / `CF-Connecting-IP`) | Mostly fixed (`ClientIp.php`); takes the left-most XFF entry | Trusted-proxy list, right-most untrusted hop |
| 4 | All image records in one JSON file | No locking | `flock` added, plus a drift-check cron and `.bak` copies | SQLite (WAL) with indexes |
| 5 | TLS verification disabled for remote fetches | `CURLOPT_SSL_VERIFYPEER => false` | Fixed | Go's default TLS verification |
| 6 | Unbounded remote download | Whole body read, then size checked | — | `LimitReader`, size cap, timeouts, max 3 redirects |
| 7 | Debug log of every upload URL and IP | `temp/upload_debug.log` | Present (≈40 KB and growing) | No request bodies or URLs logged |
| 8 | Descriptive links (v2.3.0) break `/i/` links | `i.php` regex rejects `_` in ids | Fixed | Ids allow `-` and `_`; old `/i/YYYY/MM/DD/id_size.ext` still served |
| 9 | Heavy AI on the server | Python + rembg/PaddleOCR per request | Same, plus upscale, erase, face blur | Runs in the visitor's browser (WASM) |
| 10 | Thumbnail domain no longer resolves | — | `r2.p.hel.ink` has no DNS; R2 is reached through signed URLs | Importer prefers public sizes, or `--copy` re-hosts everything |
| 11 | Hard-coded paths | `/var/www/pichost/python/venv` in `AiService.php` | Same | Nothing machine-specific |
| 12 | Moving parts | nginx + PHP-FPM + MySQL + Python venv + 4 cron jobs | Same | One binary (SQLite inside, background janitor) |

## Details

### 1. Repo ≠ production
`diff -rq` between the two lists over 40 changed files. Production also has files that are not in Git at all: `api/erase.php`, `api/faceblur.php`, `api/palette.php`, `api/upscale.php`, `core/SafeGuard.php`, `includes/ClientIp.php`, `includes/UploadService.php`, `cron/daily_backup.php`, `cron/images_drift_check.php`, and more.

This means:
- Anyone self-hosting from GitHub gets the vulnerable version.
- A server rebuild from Git would silently drop the fixes.

### 2. SSRF (repo)
`api/upload.php` passes any `http(s)` URL to cURL with `FOLLOWLOCATION`. Nothing stops `http://169.254.169.254/` (cloud metadata), `http://127.0.0.1:6379/` or a redirect into the private network.

PixelHop 3 checks the **resolved address of every connection**, redirects included, in the dialer's `Control` hook. A check on the hostname alone can be beaten by DNS rebinding; this one can't. It blocks:
- loopback, private, link-local and CGNAT ranges;
- NAT64 and 6to4 prefixes (both can map to private IPv4);
- non-web ports.

### 3. IP spoofing
When forwarding headers are believed from any client, an attacker rotates `X-Forwarded-For` and dodges per-IP rate limits, bans and abuse counters.

PixelHop 3 believes those headers only from addresses in `PIXELHOP_TRUSTED_PROXIES` (`cloudflare` expands to Cloudflare's published ranges). It then walks `X-Forwarded-For` from the right, skipping its own proxies.

### 4. JSON as a database
`data/images.json` is read whole, edited and rewritten on uploads, views and deletes (13 files touch it). The repo version has no locking, so two concurrent uploads can lose one record. Production added `flock` and a drift-check cron, which treats the symptom.

PixelHop 3 uses SQLite in WAL mode:
- indexed lookups by id, owner, album, hash and expiry;
- atomic writes;
- no separate database server.

### 9. AI on the server
Background removal and OCR start a Python process per request. Each one loads hundreds of MB of model weights, so a handful of visitors can exhaust a small VPS (hence the "Gatekeeper" CPU checks).

In PixelHop 3 these tools run in the browser:
- **OCR:** tesseract.js (Apache-2.0).
- **Background removal:** @imgly/background-removal (AGPL-3.0, loaded unmodified from jsDelivr).

The server only ever hosts the results, and private images never have to be uploaded to be edited.

Upscaling and object erasing were dropped: they need a GPU-class model to be worth offering.

## What else changed in PixelHop 3

- **Privacy by default**
  - GPS/EXIF, XMP and IPTC are stripped by rewriting the file's container (JPEG segments, PNG chunks, WebP RIFF chunks), so the pixels are not re-compressed.
  - EXIF rotation is applied first, so photos stay upright.
- **Decompression-bomb guard**
  - Pixel count is checked before decoding.
  - Concurrent decodes are capped by CPU count.
- **CSRF**
  - Go's `http.CrossOriginProtection` (Fetch Metadata) on every state-changing request.
  - API-key requests are exempt; they carry no cookies.
- **Content Security Policy**
  - No inline scripts on any page.
  - Images are served with `sandbox` and `default-src 'none'`.
  - The studio page alone allows `unsafe-eval`, which the background-removal runtime needs.
- **Passwords**
  - Argon2id.
  - PixelHop 2's PHP hashes still verify and are upgraded on the next sign-in.
  - Logins are rate-limited, and checks take the same time for unknown accounts.
- **API keys and sessions**
  - API keys are random 192-bit tokens stored only as SHA-256 hashes, and shown once.
  - Session tokens are hashed the same way.
- **Uploads**
  - Guests get a secret delete link; only its hash is stored.
- **Migration**
  - `pixelhop import` moves users (passwords included) and images.
  - Old page links, `/i/` links, the Google callback URL and ShareX configs pointing at `/api/upload.php` keep working.

## Not carried over (yet)

- **Email** verification and password-reset mails. Admins can reset a password in the panel or with `pixelhop user passwd`.
- **Google Safe Browsing scans** (SafeGuard), the automatic abuse scoring (AbuseGuard), and the daily summary mail.
- **Server-side upscale, erase and face blur.** The studio has manual blur/pixelate instead.
