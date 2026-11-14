<p align="center">🌐 <b>English</b> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.de.md">Deutsch</a> · <a href="README.id.md">Bahasa Indonesia</a> · <a href="README.ja.md">日本語</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.es.md">Español</a> · <a href="README.fr.md">Français</a> · <a href="README.ru.md">Русский</a></p>

# PixelHop

Open-source image hosting in one Go program. It removes GPS location and camera details from photos before anyone can see them, and it has a photo studio that runs in the browser, so the photo stays on your device until you upload it.

Live at [pixel.hel.ink](https://pixel.hel.ink). Short links use [p.hel.ink](https://p.hel.ink).

<p align="center">
  <a href="https://pixel.hel.ink"><b>Try pixel.hel.ink</b></a> ·
  <a href="https://navi-crwn.github.io/PixelHop/">Website</a> ·
  <a href="#install">Install</a> ·
  <a href="#api">API</a> ·
  <a href="#upgrading-from-pixelhop-2">Upgrade from v2</a> ·
  <a href="CHANGELOG.md">Changelog</a> ·
  <a href="SECURITY.md">Security</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.27-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.27"/>
  <img src="https://img.shields.io/badge/SQLite-built_in-003B57?style=flat-square&logo=sqlite&logoColor=white" alt="SQLite"/>
  <img src="https://img.shields.io/badge/storage-disk_|_S3-a3e635?style=flat-square" alt="Disk or S3"/>
  <img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" alt="MIT License"/>
  <img src="https://img.shields.io/github/actions/workflow/status/navi-crwn/pixelhop/ci.yml?style=flat-square&label=tests" alt="Tests"/>
</p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/home-dark.webp"><img src="docs/readme/home-light.webp" alt="The PixelHop upload page after a guest uploaded a photo: the link is ready and a note says GPS location, camera details and date taken were removed" width="100%"></picture></p>

## What it does

- Takes images, videos and PDFs by drop, paste, file picker or link, and gives each one a short readable link such as `beach-sunset-x7Kp2Q`.
- Removes hidden metadata from photos and videos (see [What gets removed](#what-gets-removed) for exactly what, per file type).
- Groups several files into an album with one link.
- Includes a photo studio that compresses, resizes, crops, converts, blurs, reads text and removes backgrounds in the browser.
- Runs as one static binary with SQLite built in. No PHP, Python, MySQL or cron jobs are needed. Images live on disk or in any S3-compatible bucket.

It suits people who share screenshots and photos in chats, forums or on GitHub, who want a [ShareX](https://getsharex.com) destination they control, or who care about what a photo gives away.

## Screenshots

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/image-dark.webp"><img src="docs/readme/image-light.webp" alt="An image page: the photo, copy buttons for direct, Markdown, HTML and BBCode links, file details and a QR code" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/dashboard-dark.webp"><img src="docs/readme/dashboard-light.webp" alt="The member dashboard: sidebar with the HOP family, quick links, and a grid of uploaded files with tag filters" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/album-dark.webp"><img src="docs/readme/album-light.webp" alt="An album page with four photos and buttons to copy the album link, the embed code and every link in one format" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/studio-dark.webp"><img src="docs/readme/studio-light.webp" alt="The photo studio with the compress tool open on a photo: format, quality, longest side and the resulting file size" width="100%"></picture></p>

These screenshots come from a local demo instance with generated pictures. [`scripts/readme-images.mjs`](scripts/readme-images.mjs) recreates them.

## Install

The installer checks your computer, asks a few questions and builds PixelHop from source. It needs Go 1.27 or newer, or Docker.

```bash
curl -fsSL https://raw.githubusercontent.com/navi-crwn/PixelHop/main/install.sh | bash
```

From a clone, run `./install.sh`. Later, `./update.sh` pulls the new version, copies the database to `backups/` and rebuilds, and `./uninstall.sh` removes the program but keeps your data unless you add `--purge`. Every script takes `--yes` to accept the defaults, `--dry-run` to show the steps without running them, and `--help` for the rest. The installer can run PixelHop in the foreground, write a systemd unit or a Docker Compose file, and, if you like, install a [HOP ID](https://github.com/navi-crwn/HEL.ink) hub next to it.

You can also install by hand.

**Binary** (Linux, macOS and Windows, amd64 and arm64, on the [Releases](../../releases) page):

```bash
curl -L -o pixelhop https://github.com/navi-crwn/pixelhop/releases/latest/download/pixelhop-linux-amd64
chmod +x pixelhop
PIXELHOP_BASE_URL=https://img.example.com ./pixelhop
```

**Docker:**

```bash
docker run -d --name pixelhop -p 8080:8080 \
  -e PIXELHOP_BASE_URL=https://img.example.com \
  -v pixelhop:/data ghcr.io/navi-crwn/pixelhop:latest
```

The image runs as the non-root user 65532, so the data volume must be writable by that user. The repository also has a [`docker-compose.yml`](docker-compose.yml) that reads a `.env` file (copy `.env.example`) and keeps data in `./data`.

**From source** (Go 1.27 or newer):

```bash
git clone https://github.com/navi-crwn/pixelhop.git && cd pixelhop
make build && ./pixelhop
```

Open `http://localhost:8080` and create an account. The first account becomes the admin. You can also create one on the command line with `pixelhop user add you@example.com --admin`.

Video and PDF processing uses `ffmpeg`, `ffprobe` and Ghostscript (`gs`) when they are installed on the server. Without them those files are stored as uploaded. The Docker image contains none of them, so install them on the host or build your own image if you host videos or PDFs. HEIC and AVIF photos, WebP and AVIF output from the resize URL, and the server-side studio tools need the optional [`tools/ai-server.py`](tools/ai-server.py).

## What gets removed

PixelHop works on the file's structure and copies the compressed picture data unchanged, so quality stays the same.

| File | What happens |
|---|---|
| JPEG | EXIF (GPS, camera make and model, serial numbers, date taken), XMP, IPTC and comments are removed. The colour profile and other segments stay. A photo whose EXIF says it is rotated is turned upright and saved again as JPEG at quality 92, without a colour profile. This is the one case where pixels are re-encoded. |
| PNG | Text chunks, the EXIF chunk and the modification time are removed. The colour profile stays. |
| WebP | EXIF and XMP are removed from extended files. Simple WebP files carry no metadata. |
| GIF | Stored as uploaded. Nothing is removed. |
| BMP | Converted to PNG. |
| HEIC, AVIF | Converted to upright JPEG or PNG without metadata, when the AI service is set up. Otherwise the upload is refused. |
| Video (MP4, MOV, 3GP, WebM, MKV) | With `ffmpeg` installed, location, dates, camera and encoder tags are removed without re-encoding. Only the first video and first audio track are kept. A background job then shrinks the video to H.264 and AAC, at most 1280 px on the long side and 30 frames per second. Without `ffmpeg` the file is stored as uploaded. |
| PDF | With Ghostscript installed, the file is rewritten to make it smaller, and the smaller copy is kept if it saves at least 10%. Document properties such as title and author are not removed. Without Ghostscript the file is stored as uploaded. |

After an upload, the page says which kinds of data were removed. The check is by type (GPS location, camera details, date taken, editing software, XMP, IPTC, comment, embedded text). The values are never read out.

## Features

Sharing:
- Direct link, page link, Markdown, HTML, BBCode and a QR code for every file, and Open Graph tags so previews work in Discord, X and WhatsApp.
- Albums from several files, with a title, description, cover and order you can edit later. An album can be embedded on another site with an iframe (`/a/{id}/embed`).
- Copy every link of an album or a bulk upload in one format.
- Expiry after 10 minutes, 1 hour, 1 day, 7 days, 30 days or 90 days.
- Each file is unlisted (the default), private to you, or behind a password. A file can carry a content warning (sensitive or adult); visitors confirm before they see it. Owners can refuse embedding of their images on other websites.
- Tags, search and a tag filter on the dashboard. Deleted files wait in the trash for 7 days.
- Views per day and the sites that link to a file. Referrers are stored by host name only.
- Resized copies on the fly, for example `/i/photo.jpg?w=800`, made once and cached.
- Public profiles at `/u/username`, oEmbed and a sitemap.

Accounts:
- Guests can upload (up to 20 MB per file, 25 files a day per address and 10 images per album by default) and get a private delete link for each file, which the browser also remembers.
- Members get a gallery with search, sorting, renaming and bulk actions, a storage quota (128 MB by default), a getting-started card, and a ZIP export of everything they uploaded.
- Settings lists where you are signed in, lets you sign out everywhere else, and manages API keys, webhooks, your profile and account deletion.
- Sign-in with email and password, Google, or a HEL.ink account (the HOP ID hub), all optional. Without `HELINK_URL` PixelHop never contacts HEL.ink. Cloudflare Turnstile can protect the sign-in and sign-up forms.

Studio:
- In the browser: compress, resize, crop, convert (JPG, PNG, WebP, and AVIF where the browser can encode it), adjust and rotate, blur and pixelate, watermark, colour palette, QR code, text from images (OCR, 30 languages, up to five at once) and background removal. Nothing is uploaded until you press Upload result.
- Batch: resize, convert, watermark or run the AI steps on a set of images, then download one ZIP or upload an album.
- On the server, for signed-in users, if you run `tools/ai-server.py`: background removal, automatic face blur, magic eraser and 2x upscale. One image at a time, with a daily limit, and nothing is stored.

Tools for developers:
- A REST API with bearer keys. A key can be limited to upload only.
- A ShareX configuration file made in Settings, and ShareX configs from PixelHop 2 keep working.
- Webhooks for `image.uploaded` and `image.deleted`, signed with HMAC-SHA256 in the `X-PixelHop-Signature` header.
- A command-line uploader in the same binary: `pixelhop push *.png --format markdown`.
- An [OpenAPI description](web/static/openapi.json), served at `/static/openapi.json`, and `/healthz`, `/metrics` and `/status` for monitoring.
- Installable as an app on phones, with a share target for photos, videos and PDFs.

Running it:
- One static binary, a Docker image, or a systemd unit. Store files on disk or in S3-compatible storage (AWS S3, Cloudflare R2, MinIO, Backblaze B2, Wasabi, Contabo).
- Uploads pause by themselves when free disk space gets low, and `PIXELHOP_READ_ONLY=true` pauses them for maintenance. Files already online keep working. See [docs/UPLOAD-STORAGE.md](docs/UPLOAD-STORAGE.md).
- The interface is in nine languages: English, 简体中文, Deutsch, Bahasa Indonesia, 日本語, Português (Brasil), Español, Français and Русский. Use `?lang=de` to switch; the choice is kept in a cookie, and the browser's language setting is not used.
- Shared links can live on another host than the app (`PIXELHOP_LINK_URL`). On pixel.hel.ink the app lives at pixel.hel.ink and links stay `p.hel.ink/...`.

Safety:
- Passwords use Argon2id. API keys and session tokens are stored as hashes.
- Forms are protected against cross-site posts, and a strict Content Security Policy is sent.
- "Upload from link" checks the resolved address of every connection, so it cannot be pointed at private networks. Large images are refused from their header, before decoding. Uploads are rate limited, and client IPs are only taken from proxies you list.
- Files taken down by moderators cannot be uploaded again, because their SHA-256 is remembered.
- How an upload is processed, and how the server stays steady under load: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). The history behind version 3: [AUDIT.md](AUDIT.md). To report a vulnerability, read [SECURITY.md](SECURITY.md).

Moderation: the built-in admin pages were retired in 3.7.0. Every page has a Report button. The report queue, hiding and deleting content, IP bans, account actions, runtime settings and the announcement bar belong to the HOP control center, which talks to PixelHop through the signed `/hop/*` API (the HOP App Protocol). It is used with HEL.ink sign-in, signed with `HELINK_SSO_SECRET`, and the callers are listed in `HOP_ADMIN_CLIENTS` (default `imperium,helink`). Every change made that way is written to the `hop_audit` table. The control center itself is private to the HOP project. A standalone install has the Report button, the `pixelhop user` commands (`add`, `passwd`, `admin`, `merge`) and the admin role's extra rights through the API, but no screen for the report queue.

## Settings

Settings are environment variables, and all of them are optional. [`.env.example`](.env.example) lists the usual ones with their defaults; the rest are read in [`internal/config/config.go`](internal/config/config.go). The ones you are most likely to change:

| Variable | Default | What it does |
|---|---|---|
| `PIXELHOP_ADDR` | `:8080` | Address to listen on |
| `PIXELHOP_BASE_URL` | `http://localhost:8080` | Public address of the app (sign-in, upload, dashboard). Use `https://` in production, which also turns on secure cookies |
| `PIXELHOP_LINK_URL` | `PIXELHOP_BASE_URL` | Another host for shared links (image and album pages, direct files). It must be a different host and must also reach PixelHop |
| `PIXELHOP_SITE_NAME` | `PixelHop` | Name shown in pages |
| `PIXELHOP_DATA` | `./data` | Database and, with local storage, the images |
| `PIXELHOP_MAX_UPLOAD_MB` | `20` | Per image |
| `PIXELHOP_MAX_MEDIA_MB` | `100` | Per video or PDF, for signed-in members |
| `PIXELHOP_GUEST_MAX_MB` | `20` | Per file of any kind, for guests |
| `PIXELHOP_GUEST_ALBUM_MAX` | `10` | Images in an album made by a guest (0 = no limit); also a control center setting |
| `PIXELHOP_GUEST_DAILY_MAX` | `25` | Files one guest address may upload per 24 hours (0 = no limit); also a control center setting |
| `PIXELHOP_GUEST_TTL_DAYS` | `30` | Days a new guest upload stays online (0 = until deleted); older files are not changed; also a control center setting |
| `PIXELHOP_SHARE_STORAGE` | `true` | One stored file when different people upload the same picture; each keeps their own link, owner and expiry (false = every upload stores its own copy); files already stored are not changed; also a control center setting |
| `PIXELHOP_MAX_BATCH` | `20` | Files per upload request |
| `PIXELHOP_MAX_MEGAPIXELS` | `80` | Largest image accepted, by width times height |
| `PIXELHOP_UPLOADS_PER_MINUTE` | `30` | Rate limit per address |
| `PIXELHOP_DEFAULT_QUOTA_MB` | `128` | Storage for each new account; accounts that exist keep theirs |
| `PIXELHOP_GUEST_UPLOADS` | `true` | Allow uploads without an account |
| `PIXELHOP_REGISTRATION` | `true` | Allow new accounts |
| `PIXELHOP_INACTIVE_DAYS` | `0` | Delete images nobody viewed for this many days (0 keeps everything) |
| `PIXELHOP_MIN_FREE_MB`, `PIXELHOP_MIN_FREE_PERCENT` | `1024`, `5` | Free-space reserve; uploads pause below the larger of the two |
| `PIXELHOP_READ_ONLY` | `false` | Pause uploads for maintenance |
| `PIXELHOP_CONTACT_EMAIL` | none | Shown on the Terms page for copyright notices |
| `PIXELHOP_TRUSTED_PROXIES` | none | `cloudflare`, or the IPs or CIDRs of your reverse proxy |
| `PIXELHOP_STORAGE` | `local` | `local` or `s3` |
| `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY` | none | Needed for S3 storage; `S3_REGION` (`auto`) and `S3_PATH_STYLE` (`true`) are optional |
| `S3_PUBLIC_URL` | none | CDN address of the bucket; file links redirect there |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | none | Sign in with Google. `GOOGLE_REDIRECT_URI` defaults to `PIXELHOP_BASE_URL/auth/google/callback` |
| `HELINK_URL`, `HELINK_SSO_SECRET` | none | Sign in with a HEL.ink account. The secret is at least 32 characters and equals `PIXELHOP_SSO_SECRET` in HEL.ink's `.env` |
| `HELINK_SSO_SECRET_PREV` | none | The previous secret, accepted while you rotate it |
| `HOP_ADMIN_CLIENTS` | `imperium,helink` | App names allowed to call the signed `/hop/*` API |
| `PIXELHOP_CONTROL_URL` | none | Address of the HOP control center; admins who open `/admin` are sent there |
| `TURNSTILE_SITE_KEY`, `TURNSTILE_SECRET_KEY` | none | Cloudflare Turnstile on sign-in and sign-up. With HEL.ink accounts on, these forms refuse every attempt without the keys |
| `PIXELHOP_REMBG_URL` | none | Address of `tools/ai-server.py`, for example `http://127.0.0.1:7001` |
| `PIXELHOP_REMBG_PER_DAY` | `30` | Server AI uses per member per day |

The names `HOP_SSO_SECRET_PIXELHOP` and `HOP_SSO_SECRET` also work for the HEL.ink secret, and `HOP_SSO_SECRET_PREV` for the previous one.

### Reverse proxy, service and backups

- [`deploy/nginx.conf`](deploy/nginx.conf) is a reverse-proxy example. Set `PIXELHOP_TRUSTED_PROXIES=127.0.0.1` so PixelHop trusts nginx's `X-Forwarded-For`, and add `cloudflare` if Cloudflare is in front.
- [`deploy/pixelhop.service`](deploy/pixelhop.service) is a hardened systemd unit. [`deploy/pixelhop-ai.service`](deploy/pixelhop-ai.service) runs the optional AI service, which needs `rembg`, `opencv-python-headless` and `onnxruntime` (see the header of [`tools/ai-server.py`](tools/ai-server.py)).
- To back up, copy the data folder, or run `sqlite3 data/pixelhop.db ".backup copy.db"` while PixelHop is running. `./update.sh` makes such a copy in `backups/` before every update.

## How to use it

1. Open the site, then drop, paste (<kbd>Ctrl</kbd>+<kbd>V</kbd>) or link an image.
2. Optional: pick Delete after, and whether the file name goes in the link.
3. The link is copied for you. Open it for the direct link, embed codes and QR code.

Press `?` on any page for the keyboard shortcuts.

**ShareX.** Go to Settings, API keys, create a key, then press Download ShareX config with this key and open the file.

**Command line.** The same `pixelhop` binary uploads to any PixelHop server. `pixelhop login` stores the address and key in `~/.config/pixelhop/client.json`; `PIXELHOP_SERVER` and `PIXELHOP_KEY` override them.

```bash
pixelhop login --server https://pixel.hel.ink --key ph_yourkey
pixelhop push screenshot.png photos/*.jpg --expire 7d --format markdown
```

**Phone.** Open the site and choose Add to Home Screen. PixelHop then appears in the share sheet.

## API

```bash
curl -H "Authorization: Bearer ph_yourkey" -F "file=@holiday.jpg" https://pixel.hel.ink/api/v1/upload
```

| Endpoint | What it does |
|---|---|
| `POST /api/v1/upload` | Upload files (`file`, repeatable) or links (`url`). Options: `expire` (`10m`, `1h`, `1d`, `7d`, `30d`, `90d`), `slug` (`0` keeps the file name out of the link), `title`, `alt`, `album_id`, `album` (`force` or `0`), `listed`, `content_warning` (`sensitive` or `adult`), `warning_reason` |
| `POST /api/v1/uploads`, `PUT /api/v1/uploads/{id}`, `POST /api/v1/uploads/{id}/finish` | Upload a big file in 8 MB pieces |
| `GET /api/v1/images` | Your images (`page`, `limit`, `q`, `sort`) |
| `GET`, `PATCH`, `DELETE /api/v1/images/{id}` | Read, change or delete one. `PATCH` takes `title`, `alt`, `listed`, `visibility` (`unlisted`, `private`, or `password` with `password`), `tags`, `content_warning` and `warning_reason`. `DELETE` moves your own file to the trash; add `?permanent=1` to remove it for good |
| `POST /api/v1/images/{id}/restore` | Bring a file back from the trash |
| `GET /api/v1/images/{id}/stats` | Views per day and referring sites |
| `GET`, `POST /api/v1/albums` and `GET`, `PATCH`, `DELETE /api/v1/albums/{id}` | List, create, read, edit and delete albums |
| `GET /api/v1/users/{username}/images` | Public: the images on someone's profile |
| `GET /api/v1/me` | Your account and storage use |
| `GET /api/v1/me/media` | Your own pictures (`type=image`) or PDFs (`type=pdf`) that another page may show: no private, password-protected, expiring or content-warning files (`cursor`, `limit`). Also open to the HOP apps named in `PIXELHOP_MEDIA_CLIENTS` (BioHop's media picker), signed server to server for one member |
| `GET /api/v1/stats` | Public totals and the server version |
| `GET /api/v1/upload-status` | Whether uploads are accepted right now |
| `POST /api/v1/tools/{remove-background,faces,erase,upscale}` | Server AI tools (signed in; `file`, plus `mask` for erase) |
| `GET /i/{id}.jpg?w=&h=&fit=&format=&q=` | Resized copies, made once and cached. `format=webp` and `avif` need the AI service |
| `GET /oembed?url=...` | oEmbed for image pages |

Every answer looks like `{"success": true, "data": ...}` or `{"success": false, "error": "..."}`. The full reference is at [`/docs`](https://pixel.hel.ink/docs) on any PixelHop server.

## Upgrading from PixelHop 2

PixelHop 2 was a PHP app with MySQL, a Python venv and cron jobs. Its code stays in the [`v2.3.0-php`](../../tree/v2.3.0-php) tag. One command brings the data over:

```bash
pixelhop import --images /var/www/pichost/data/images.json \
  --mysql "user:pass@tcp(127.0.0.1:3306)/pixelhop" --copy
```

- Users keep their ids, roles, quotas and passwords. PHP's argon2id hashes are verified and upgraded at the next sign-in. Google accounts stay linked.
- Images keep their ids. With `--copy` they are downloaded into PixelHop 3's storage with fresh thumbnails; without it they are served from the old buckets.
- Old links keep working: `/{id}`, `/i/2026/08/27/{id}_original.webp`, `/auth/google-callback` (set `GOOGLE_REDIRECT_URI`), and ShareX configs that post to `/api/upload.php`.

[AUDIT.md](AUDIT.md) explains why it was rewritten.

## Development

```
cmd/pixelhop/     the command: serve, user, import, push, login
internal/app/     web server: pages, API, uploads, moderation, HOP App Protocol
internal/db/      SQLite schema, migrations and queries
internal/imaging/ type sniffing, metadata stripping, thumbnails
internal/storage/ local disk, S3 (own SigV4 signer), legacy URLs
internal/netsafe/ SSRF-safe fetching for "upload from link"
web/              templates, scripts and the nine language files, embedded in the binary
docs/             the GitHub Pages website and the README images
scripts/          helper scripts (README images, changelog data, HOP family check)
tools/            the optional AI service
deploy/           nginx and systemd examples
```

`make test` runs `go vet` and the tests, and `make build` makes `./pixelhop`. `scripts/readme-images.mjs` re-renders the README screenshots from a running instance (see the comment at the top of the file). `python3 scripts/build-changelog.py --repo PixelHop` rebuilds `web/changelog.json`, the data behind the `/changelog` page, from `CHANGELOG.md` (plain language, one entry per release); it needs no network and no `gh`, and `make check-changelog` fails when the file is not plain or not current.

## FAQ

<details><summary>Is the studio really private?</summary>

Editing happens on your device with Canvas and WebAssembly. The first time you use text recognition or background removal, the browser downloads [tesseract.js](https://github.com/naptha/tesseract.js) and [@imgly/background-removal](https://github.com/imgly/background-removal-js) from jsDelivr, and their language data and models from tessdata.projectnaptha.com and staticimgly.com. Your image is not sent anywhere unless you press Upload result. A self-hosted server that sets a stricter Content Security Policy needs to allow those addresses.
</details>

<details><summary>Can I turn off guest uploads or sign-ups?</summary>

Yes. Set `PIXELHOP_GUEST_UPLOADS=false` or `PIXELHOP_REGISTRATION=false`. With the HOP control center you can also change both while the server runs.
</details>

<details><summary>How much does it need?</summary>

A small server is enough. A fresh `pixelhop` process used about 16 MB of memory in a test on macOS, and the release binaries are 16 to 17 MB. Large images are decoded a few at a time, so a burst of uploads cannot use all memory.
</details>

## Self-hosting

<!-- self-hosting:start -->
Everything PixelHop needs is in public repositories: no private repository, no token and no hel.ink account. The family guide has the details: [docs/self-hosting.md](https://github.com/navi-crwn/HEL.ink/blob/main/docs/self-hosting.md).

| Piece | For PixelHop | What it does |
| --- | --- | --- |
| [HEL.ink](https://github.com/navi-crwn/HEL.ink) (hub, HOP ID) | Optional: adds “Sign in with HOP ID” | Accounts and sign-in for every app |
| [Imperium](https://github.com/navi-crwn/Imperium) | Optional, but the admin pages of PixelHop live there | Admin panel for the whole family; needs the hub |
| Family fonts and tiles | Nothing to install: loaded from `hel.ink/hop-assets` | Without hel.ink: system fonts, no floating tiles |

- **Install order**: PixelHop alone with `./install.sh`. For HOP ID sign-in, the [hub](https://github.com/navi-crwn/HEL.ink) first: `./install.sh --hub new` installs one next to it.
- **Minimum settings**: `PIXELHOP_BASE_URL`; with the hub: `HELINK_URL`, `HELINK_SSO_SECRET`; with Imperium: `PIXELHOP_CONTROL_URL`.
- **Alone or in the family**: PixelHop runs alone with its own accounts. With a hub, people can also sign in with HOP ID and sign out of every app at once.
- **Without Imperium**: `/admin` shows a notice. Use `pixelhop user add` and the signed `/hop/*` API (the first account is the admin).
<!-- self-hosting:end -->

## The HOP family

PixelHop is part of the HOP family of privacy-respecting, self-hostable open-source apps.

<!-- hop-family:start -->
<!-- Generated from HEL.ink resources/hop-assets/apps.source.json; edit there, then run scripts/hop_assets.py build. -->
| App | Address | Repository | What it does |
| --- | --- | --- | --- |
| **HOP ID** | [hel.ink](https://hel.ink) · accounts at `hub.hel.ink` | [navi-crwn/HEL.ink](https://github.com/navi-crwn/HEL.ink) | One account for every HOP app. |
| **LinkHop** | [link.hel.ink](https://link.hel.ink) · `hel.ink/l/…` | [navi-crwn/LinkHop](https://github.com/navi-crwn/LinkHop) | Short links, QR codes and click analytics. |
| **BioHop** | [bio.hel.ink](https://bio.hel.ink) · `hel.ink/b/…` | [navi-crwn/BioHop](https://github.com/navi-crwn/BioHop) | One link-in-bio page for everything you do. |
| **PixelHop** | [pixel.hel.ink](https://pixel.hel.ink) · `p.hel.ink/…` | [navi-crwn/PixelHop](https://github.com/navi-crwn/PixelHop) | Image hosting with albums and direct links. |
| **NoteHop** | [note.hel.ink](https://note.hel.ink) · `n.hel.ink/…` | [navi-crwn/NoteHop](https://github.com/navi-crwn/NoteHop) | Encrypted notes you share with a link. |
| **FileHop** | [file.hel.ink](https://file.hel.ink) | [navi-crwn/FileHop](https://github.com/navi-crwn/FileHop) | Convert files in your browser; nothing is uploaded. |
| **PayHop** | [pay.hel.ink](https://pay.hel.ink) | [navi-crwn/PayHop](https://github.com/navi-crwn/PayHop) | Make a payment QR and split a bill; nothing leaves your browser. |
| **Imperium** | not public-facing: optional, for admins | [navi-crwn/Imperium](https://github.com/navi-crwn/Imperium) | The HOP control panel. |
<!-- hop-family:end -->

## Credits and license

- [MIT](LICENSE.md), copyright Ivan Novskies (navi-crwn). Third-party parts are listed in [CREDITS.md](CREDITS.md).
- Built with [Go](https://go.dev), [modernc.org/sqlite](https://gitlab.com/cznic/sqlite), [golang.org/x/image](https://pkg.go.dev/golang.org/x/image) and [rsc.io/qr](https://pkg.go.dev/rsc.io/qr).
- The studio loads [tesseract.js](https://github.com/naptha/tesseract.js) (Apache-2.0) and [@imgly/background-removal](https://github.com/imgly/background-removal-js) (AGPL-3.0, unmodified, from jsDelivr) on demand.

More by the same author: [Amnesia](https://github.com/navi-crwn/amnesia), a Mac that forgets at logout.
