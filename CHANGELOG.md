# Changelog

All notable changes to PixelHop will be documented in this file.

## [3.0.0] - 2026-10-15

PixelHop rebuilt from scratch in Go. See [AUDIT.md](AUDIT.md) for why.

### Added after the first preview
- **One account with HEL.ink**: optional sign-in with a HEL.ink account (signed, single-use tickets); sign-up, password resets, Google and 2FA handled by HEL.ink; `pixelhop user merge` folds old accounts in.
- **Server AI tools** (optional `tools/ai-server.py`): background removal, automatic face blur, magic eraser (LaMa) and HD upscale 2x (Real-ESRGAN), sign-in only with daily limits.
- **Batch page**: the same steps for many images, then one ZIP or one album.
- **Copy all links** after bulk uploads, on albums and on old `/result` links; share buttons; thumbnail links; gallery select all, copy links and ZIP download.
- **QR code generator** for any text, PNG or SVG.
- Cloudflare **Turnstile**, a **Help** page, admin tool usage and disk space, local-only `/metrics`, and redirects for every PixelHop 2 page and tool address.

### Added
- **One binary**: web server, SQLite database and cleanup janitor in a ~16 MB executable. Docker image, systemd unit and nginx example included.
- **Lossless metadata stripping**: GPS/EXIF, XMP and IPTC removed from JPEG, PNG and WebP without re-encoding; EXIF rotation applied first.
- **In-browser studio**: compress, resize, crop, convert (incl. AVIF), adjust/rotate, blur & pixelate, color palette, OCR and background removal, all client-side.
- **Albums** for multi-file uploads, **QR codes**, **Markdown/HTML/BBCode** embeds, and readable links (`beach-sunset-x7Kp2Q`).
- **Self-destruct** timers (10 minutes to 90 days) and private **delete links** for guests, plus a per-device upload history.
- **API keys** (hashed, shown once) with a one-click **ShareX** config; JSON API for upload, list, rename, delete.
- **Storage backends**: local disk or any S3-compatible bucket, with a built-in SigV4 signer and optional CDN redirects.
- **Admin**: overview with a 30-day chart, images, users (lock/suspend/role/quota/password/warnings), reports, IP bans, site settings, SEO title/description, announcement bar.
- **`pixelhop import`**: moves PixelHop 2 users (with their passwords) and images; `--copy` re-hosts the files.
- GitHub Pages website with EN/ID and light/dark, and README images rendered from it.

### Security
- SSRF-proof "upload from link" (checks every resolved address, including redirects).
- Forwarding headers are trusted only from configured proxies (`PIXELHOP_TRUSTED_PROXIES=cloudflare`).
- CSRF protection via Fetch Metadata, a strict CSP without inline scripts, and sandboxed image responses.
- Argon2id passwords, rate-limited sign-in, hashed session tokens, and a decompression-bomb guard.

### Changed
- Server-side Python AI (rembg, PaddleOCR, upscale, erase, face blur) replaced by in-browser tools; upscale and erase are not offered.
- MySQL + `images.json` replaced by SQLite.
- The PHP code moved to `legacy/` (tag `v2.3.0-php`).

### Kept working
- Image pages `/{id}`, PixelHop 2 image links `/i/YYYY/MM/DD/{id}_{size}.{ext}`, `POST /api/upload.php` (ShareX), and `/auth/google-callback`.

## [2.3.0] - 2026-01-03

### Added
- **Descriptive Image Links**: Uploaded image links now include a slug from the original filename (e.g. `rumah-baru_a3x9K2`), making it easier to identify images from the link alone.
- **Smart Slug Truncation**: For long filenames, only the first ~15 characters are used for the slug, cut at word boundaries for readability.
- **Bulk Upload Friendly**: Copying multiple links now shows which image is which, even for guests.

### Changed
- **Backward Compatibility**: All old image links (e.g. `akqxW3`) remain valid and accessible. Only new uploads use the new descriptive format.
- **View, Result, and Gallery Pages**: All pages now support both old and new link formats seamlessly.

### Fixed
- Improved filename sanitization for URL slugs.

## [2.2.0] - 2025-12-18

### Added
- **View Count Tracking**: Track views for all images
  - View count increments once per hour per IP
  - View count and last viewed date in Image Info
  - "Most Views" sort option in galleries
  - Total Views stat in galleries
- **Report Image Feature**: Users can report inappropriate images
  - Report button on image view page
  - Multiple report reasons (NSFW, Illegal, Privacy, Copyright, Other)
  - Report queue in Admin Security panel
  - Dismiss or Delete actions for admins
- **User Account Status System**:
  - **Active**: Normal access
  - **Locked**: Cannot login, images still viewable, must contact support
  - **Suspended**: No access, images inaccessible, auto-deleted in 30 days
- **Admin Warning System**: Send warnings that popup on user's next login
- **Image Expiration System**: Auto-delete images after 90 days of inactivity
  - Daily cron job at 2 AM
  - Based on last_viewed or created_at date
  - Logged to data/expiration_log.json

### Changed
- **Admin Gallery Stats**: Compact layout (5 stats in 1 row)
- **Image View Page**: Added view count, last viewed, and report button
- **Admin Users Page**: Added status management (Lock/Suspend/Warning)
- **Login Flow**: Checks account status before allowing login

### Fixed
- Modal styling for light mode in admin panel
- Report dismiss now properly disables row
- PHP 8.5 compatibility improvements

## [2.1.0] - 2025-12-18

### Added
- **Hybrid Storage System**: R2 + Contabo S3 integration
  - Cloudflare R2 for thumbnails (9.5GB free tier)
  - Contabo S3 for original files (250GB)
  - Automatic failover between providers
- **R2StorageManager**: Smart storage routing with rate limiting
- **Storage Stats Dashboard**: Real-time monitoring for R2, Contabo, and Temp storage
- **Security Firewall**: IP blocking, rate limiting, abuse detection
- **Swap Memory**: 4GB swap with swappiness=10 for better performance

### Changed
- **Admin Dashboard**: Consolidated from 10 menus to 5 (Dashboard, Users, Gallery, Security, Settings)
- **Admin UI**: New shared header, CSS, and JS components
- **Storage Display**: Progress bars for R2, Contabo, and Temp (10GB allocation)
- **Gauge Charts**: Larger donut charts (130px) with better centering

### Fixed
- Member settings page layout and styling
- Admin panel centering with visible background
- Storage stats now show actual provider usage

## [2.0.0] - 2025-12-17

### Added
- Admin panel v2.0 with glassmorphism design
- Theme toggle (dark/light mode)
- Real-time server health monitoring
- Python process management for OCR/RemBG

### Changed
- Complete admin UI overhaul
- Responsive design improvements

## [1.0.0] - 2025-12-16

### Added
- Initial release
- Image upload with multiple formats
- Image tools: resize, crop, compress, convert
- OCR text extraction
- Background removal (RemBG)
- User authentication with Google OAuth
- Rate limiting and abuse protection
