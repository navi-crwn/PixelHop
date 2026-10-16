# Changelog

All notable changes to PixelHop will be documented in this file.

## [3.4.1] - 2026-10-16

### Fixed
- The guest note sat on the upload box's colored border and was hard to read; it is now below the box. Phones no longer see the Ctrl+V hint.

### Changed
- PixelHop is fully standalone: HEL.ink sign-in is an optional add-on that stays off unless `HELINK_URL` and `HELINK_SSO_SECRET` are set, and examples no longer point at HEL.ink.

## [3.4.0] - 2026-10-16

### Changed
- **New upload page layout.** A shorter headline and one clear upload box. Options fold into an "Options" chip that shows what's set (e.g. "Delete after 7 days"), and the ticket into a "Ticket & note" chip.
- **Results beside the upload box** on desktop (no scrolling), below it on phones (scrolled into view). Up to 3 files show there; bigger batches open the results page.
- **"Install app"** moved to the top bar; "Rebuilt in Go" moved down to the features.
- **Uploaded from this device** is folded by default (count and a few thumbnails), grouped by day inside, a dozen at a time.

## [3.3.2] - 2026-10-16

### Added
- **Paste button** for phones (no Ctrl+V there): uploads the image on the clipboard, for example one copied inside a work chat app that has no Share button. Chrome asks once for permission; where reading the clipboard isn't allowed, a box appears that takes press-and-hold → Paste. A copied link is fetched like "From a link".

## [3.3.1] - 2026-10-16

### Added
- **Results page for phones**: after uploading several files on a phone you land on `/uploaded`, a page with just those files, their links, "Copy for chat", "Copy all direct links" and (for guests) the delete links. Desktop keeps showing results on the upload page.
- **Install app** button on the upload page when the browser offers it (Android/desktop Chrome), and a hint for iPhone.

### Changed
- The **Ticket / case** box is always visible for signed-in people (it was folded away), with a sign-in hint for guests.

## [3.3.0] - 2026-10-16

### Added
- **Videos and PDFs.** MP4, MOV, WebM, 3GP and PDF files get a page, a poster/first-page preview and a link, like images. Up to 500 MB per file (`PIXELHOP_MAX_MEDIA_MB`).
- **Automatic compression.** Videos lose their location and other metadata at once, then are shrunk in the background to 720p H.264 (plays on every phone; a 53 MB screen recording became 2 MB). PDFs are compressed with Ghostscript on upload. Both need `ffmpeg` / `gs` on the server and are skipped without them.
- **Uploads in pieces** (`/api/v1/uploads`), resumable after a dropped connection, so big phone videos get past proxy limits.
- **Reports / tickets.** Give an upload a ticket number and a note: every file goes into one album with that title and note.
- **Copy for chat**: one tap copies the ticket, note, album link and a numbered list of every file's link, on the upload page, albums, `/result` and the share page. "Copy all direct links" is now a primary button.
- The share target accepts videos and PDFs; an **iPhone Shortcut** guide in the docs.

### Changed
- Upload rows stack their buttons on phones so the link stays readable.

## [3.2.0] - 2026-10-16

### Added
- **Private and password-protected images.** Only you, or anyone with the password; the file itself is locked too, so hotlinks stop working. Unlocking is remembered for 30 days and resets when the password changes.
- **Edit details** on an image page: title, alt text (used in embed codes), visibility and profile listing.
- **View stats** for owners: views per day for 30 days and the sites that sent visitors.
- **Resize on the fly**: `/i/{id}.jpg?w=800&h=600&fit=cover&format=webp&q=75`, cached per image.
- **Album editor**: title, description, cover, drag-to-reorder, add and remove images; **My albums** page; **Make album** from a selection in My images; albums API.
- **Explore** page with images people chose to show publicly, plus `sitemap.xml`, **oEmbed** and a public gallery API (`/api/v1/users/{username}/images`) for HEL.ink bio pages and other sites.
- **Webhooks** for `image.uploaded` and `image.deleted`, signed with HMAC-SHA256.
- **iPhone HEIC and AVIF uploads** are converted on the server; upload results list the hidden data that was removed (GPS location, camera details…).
- **Installable app (PWA)** with a share target: share photos from your phone's share sheet straight to PixelHop.
- **Watermark** tool in the studio and in batch (text or logo, position, size, opacity, tiled), and **hold to compare** with the original.
- **Command-line uploader** in the same binary: `pixelhop login` and `pixelhop push`.

### Changed
- The AI sidecar keeps its models next to it and loads them only when needed.
- The old `/result?id=…` page hides private and password-protected images.

## [3.1.0] - 2026-10-16

### Added
- **Public profiles**: display name, @username, profile photo (square, metadata stripped), bio and website, edited in Settings. Every account gets a username automatically.
- **Image ownership**: image and album pages show who shared them, with photo and a link to their profile; link previews say "Shared by …". Owners can switch this off.
- **Profile pages** at `/u/username` showing only the images their owner chooses ("Show on my profile" on an image, in bulk from My images, or `listed` when uploading / via the API).

### Changed
- Signing in with HEL.ink no longer overwrites a name you edited here; HEL.ink's name and photo only fill in empty ones.

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
