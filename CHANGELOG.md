# Changelog

All notable changes to PixelHop will be documented in this file.

## [Unreleased]

## [3.9.0] - 2026-11-10

### Added
- Every footer starts with the HOP ID mark and "Part of Hop Easy Link" (linking to hel.ink), for everyone and in both looks; the Clean-only brand strip is replaced by it.
- HopUI dialogs: questions and notices use the family's shared dialog instead of the browser's own, with the question's title and its explanation said separately.
- A ticket's private note is shown once on the upload result page, to its owner, and stays visible only to the owner and the moderators.
- The trash has its own entry in the dashboard sidebar.
- An optional Clean interface style (flat surfaces, solid bordered cards, quiet tables and tabs), chosen with the `hop_ui` cookie, which the server reads. The HOP look stays the default.

### Fixed
- All five family tiles on the home page assemble: the 3D engine is the hop-assets v7 copy, which keeps drawing every unsettled scene (v6 animated only four).
- The family section on the home page draws every other app, FileHop included, as the same 3D tile (hop-assets v6 engine, WebP before WebGL) with its name and host below; the numbered "10 · HOP" label is gone and the five tiles sit in one row, a 3 + 2 bento on tablets and two columns on phones. The footer's HOP family list shows each app's mark.
- Signing out of a HOP ID account goes through the account server with `return=` set to this app, so the person ends up back on PixelHop.
- The footer no longer repeats the language list (the language menu is in the top bar); the copyright line is right-aligned, centred on phones.
- The changelog page lists every merged pull request again: the snapshot is rebuilt from a freshly fetched main, and `make check-changelog` fails when merged PRs are missing from it.
- A file its owner put in the trash answers 404, not 451. Hidden media is no longer cached publicly or redirected.
- The content gate stays on the host it was posted to.
- The language menu stays on the screen at 375 px and closes with Escape.

### Changed
- The header and footer show the mark and the app name as real text in the app's own typeface (hop-assets v7 brand fonts: name-only subsets, vendored byte-identical, SIL OFL).
- New LinkHop and BioHop marks from hop-assets v6 (the embedded manifest and, where the app carries them, the home art and floating-tile copies).
- Controls follow the HOP control standard: one 8 px radius, 36 px controls (32 px in rows, 40 px on forms), capped field widths.
- HOP family assets move to hop-assets v4: FileHop's new pixel mark in the family list, the app switcher, the footer and the About page (bundled manifest), and the copied home engine with FileHop's matching 3D map. `scripts/sync-hop-assets.sh` copies from v4.

### Security
- `HOP_PICKER_SECRET` (32+ characters) verifies the media picker's calls (`GET /api/v1/me/media` signed by an app in `PIXELHOP_MEDIA_CLIENTS`) with a secret the hub never uses. Until now PixelHop had to hold BioHop's sign-in secret (`HOP_SSO_SECRET_BIOHOP`) for that, so a leak here could forge a hub ticket for BioHop. It is accepted next to the older secrets while the family moves; once BioHop signs with it (navi-crwn/BioHop `fix/picker-secret`), remove `HOP_SSO_SECRET_BIOHOP` from this server.


### HOP family
- FileHop (file.hel.ink), the sixth HOP app, is in the family list: the dashboard sidebar, the app switcher, the footer and the About page (from the bundled asset manifest), the FAQ and features text in all nine languages, and the README family table.
- The HOP family assets move to hop-assets v3, which adds FileHop: the bundled manifest, the copied home engine (`web/static/hop-home/`, with the FileHop 3D mark), `scripts/sync-hop-assets.sh` and the documentation site's font link. In the HOP family section, an app the 3D engine has no block map for keeps its flat mark instead of falling back to the NoteHop logo.
- `GET /api/v1/me/media` lists a member's own pictures (`type=image`) or PDFs (`type=pdf`) with their public address, thumbnail, size and album, newest first, with an opaque `cursor`. It leaves out what a page elsewhere could not show: private, password-protected, expiring, hidden (trash, moderation, suspension) and content-warning files. It also says whether the member blocks embedding on other sites. Besides a session or a full API key, a HOP app named in `PIXELHOP_MEDIA_CLIENTS` (default `biohop`, for BioHop's "Choose from PixelHop" picker) may call it server to server with a HOP App Protocol signature and `X-HOP-Actor` set to the member's HOP ID; the shared SSO secret verifies it, or that app's own secret in `HOP_SSO_SECRET_<APP>` when the family keeps one per app. Media clients are not admin clients: they cannot call `/hop/*`. The file stays here; the other app only keeps the address.

### Documentation
- `.env.example` names every variable the program reads: the free disk limits, read-only mode, the secret fallbacks and rotation, `HELINK_API_ADDR`, `HOP_ADMIN_CLIENTS` and the three the command line uses. A test keeps it that way.
- README rewritten and checked against the code, in all nine languages. It now says exactly which metadata is removed from which file type: photos (JPEG, PNG, WebP) and videos lose their metadata, PDFs are compressed but keep their document properties, GIFs are stored as uploaded, and videos and PDFs are only processed when `ffmpeg` and Ghostscript are installed on the server (the Docker image has neither).
- Fresh screenshots of the home page, image page, dashboard, album page and studio, in dark and light. `scripts/readme-images.mjs` now renders them from a running instance with generated demo pictures (`scripts/demo-images`); the old hero, features and studio pictures, and the unused `docs/media` screenshots (one showed the retired admin pages), are gone.
- `docs/ARCHITECTURE.md` no longer says that sideways photos keep their pixels: a photo with an EXIF rotation is saved again as JPEG at quality 92.
- `web/changelog.json`, the data behind the `/changelog` page, was regenerated with `scripts/build-changelog.py`; it had stopped at the content warning pull request.

### Hardening
- CI and release builds use Go 1.27.3 (`go` line in `go.mod`), which has the fixes the 1.27.2 toolchain lacks.
- A member's storage quota is checked and used up as one step per member: uploads that arrive together can no longer all pass the same reading of "used so far". Uploads sent in pieces are checked again when they finish, so several big files started while there was room cannot all be kept.
- Deleting an account (by the member, or by the control center) now removes the profile photo from storage as well. If storage refuses the delete, the account is kept and the purge can be run again.
- Deleting an account also removes the pictures in its trash. They used to stay, without an owner, until the janitor cleared them up to a week later, although the settings page says every image is deleted.
- An account is no longer deleted when its pictures could not be listed: the purge stops, reports the error and can be run again.
- Private, password-protected and expiring files are no longer redirected to the public storage address (`S3_PUBLIC_URL`); PixelHop serves them itself, behind its own checks.
- Deleting a picture now removes its stored files first and the database row last, and reports a storage error instead of ignoring it. Emptying the trash, deleting an account and deleting a reported picture from the control center stop at the first failure, keep the records, and can be run again.
- A HOP ID event that fails (bad payload, storage error) no longer uses up its id, so the sender's retry is handled. Events that succeed are still handled once.
- The image page no longer says "Metadata: clean" for GIFs, which are stored as uploaded.
- Removed the unused `PIXELHOP_SECRET` setting; `install.sh` no longer writes it to `.env`. An existing value in `.env` is simply ignored.
- A profile website that opens an encrypted NoteHop note (`https://note.hel.ink/ID#k=KEY`) is refused, and the same addresses are cut down to the part before the `#` in report notes, so PixelHop never stores or shows a note key. The rule follows hop-safety's `EncryptedNoteLink`; the hosts come from `PIXELHOP_NOTE_HOSTS` (default `note.hel.ink`, `n.hel.ink`).
- The same rule now covers what the hub sends: a website with a note key in a profile push (`POST /hop/profile`, the `profile.updated` event) or a sign-in ticket is ignored and the earlier address stays, and a key in a name or bio is cut out, there and in the settings form. The form also refuses an escaped `%23k=` and an address hidden inside another one. A key stored earlier stays in the row until its owner changes it, but `/hop/users` and the profile page no longer hand it out.
- A sign-in ticket whose `status` is not `active` is refused before any local account is made or linked.
- The address after sign-in (`next`) is refused when it holds a control character: `/<TAB>/host` was read by browsers as `//host`.
- A build from a clone (`make`, `install.sh`, `update.sh`) and the Docker image of a release report the version without the tag's `v` (`3.8.0`), like the release binaries; before, the same release showed as `v3.8.0` on the status page and in `/hop/health`.
- A mistyped command (`pixelhop servee`) prints the usage and exits before the data folder and database are created.
- The janitor no longer stops at the first picture whose files cannot be deleted: it logs that picture, removes the others and still clears old sessions. The same for the trash.

### Fixes
- `install.sh --dry-run` no longer creates the data folder and `bin/`. It said nothing would be executed, and now nothing is.
- The home page "Batch editing" button opened a page that does not exist (`/batch`); it now opens `/tools/batch`.
- Settings no longer shows "Your profile page" while the profile page is switched off (that address answered 404).
- The image page no longer scrolls sideways on phones: the side column can shrink below the width of its tab row.
- The studio QR tool says so when the server refuses the text (more than 1500 bytes, or too long for the level) instead of saving the error text as `qr-code.png`.

### Translations
- The 404 page and the error pages (titles and the 500 text) are translated; the profile website refusal and the QR error are new messages in all nine languages.
- The interface, the sign-in and member pages, the home page, the documentation site and the READMEs come in nine languages: English, 简体中文, Deutsch, Bahasa Indonesia, 日本語, Português (Brasil), Español, Français and Русский. German and Russian are new; Arabic and right-to-left layout are gone. The old codes `zh` and `pt` still work in `?lang=` and in the cookie, and a retired language falls back to English. The browser's language setting is not used.
- Copy edits in Chinese and German: dash connectors removed from the text.

### Installers
- `install.sh` checks the computer (Go 1.27 or newer, or Docker, git, curl, disk space), asks a few questions and builds PixelHop. It can run it in the foreground (`start.sh`), write a systemd unit or a Docker Compose file, connect to an existing HOP ID hub or install one next to it, and ends with a self-test against `/healthz`. It works from a clone or through `curl | bash`.
- `update.sh` copies the database to `backups/` (with `sqlite3 .backup` when available), pulls and rebuilds (it restarts the Docker container, and tells you how to restart anything else); `uninstall.sh` removes the program and keeps your data unless you pass `--purge`. All three take `--yes` and `--dry-run`.

### Safety, speed and ownership
- Views are counted in memory and written every ten seconds in one transaction, so a viral image no longer means three database writes per visitor.
- API keys can be limited to **Upload only** (they cannot list, rename or delete).
- Owners can refuse embedding of their images on other websites (Settings → Embedding); links you share and direct visits still work.
- Settings lists where you are signed in and can sign out everywhere else, and exports everything (every file in original quality plus `account.json`) as one ZIP.
- Files taken down by the moderators (and files of banned uploaders) cannot be uploaded again: their SHA-256 is remembered.
- Maintenance mode (`PIXELHOP_READ_ONLY=true`, or the control center) pauses uploads while everything online keeps working.
- An album can be embedded on another website with an iframe (`/a/{id}/embed`); the album page copies the code.
- Keyboard shortcuts (press `?`): upload, studio, dashboard, search, copy link. After a multi-file upload the album link is copied for you.
- An OpenAPI 3 description of the API (`/static/openapi.json`) and an architecture page (`docs/ARCHITECTURE.md`).
- Deleted files go to a **trash** for 7 days (restore them, delete for good, or empty it); the API follows (`DELETE` trashes, `POST /api/v1/images/{id}/restore`, `?permanent=1`). Files can have **tags** (search and a tag filter on the dashboard; `tags` in the API).
- The terminal boxes on the home page keep their dark panel in light mode (the site-wide `code` and `pre` styles painted them pale).

### Home v2
- The home page below the upload box is rebuilt on the shared HOP scene engine (`hop-scenes`, HOP assets v2): a running line and numbers, then pinned scenes where the scroll plays an upload, the removal of a photo's location and camera data, an album turning into five link formats, and a photo that comes apart when its time runs out; a studio section with a live mini tool in each card; the API and self-host terminals; question cards; and the other HOP apps as four marks you can turn.
- Everything is complete without script or with "reduce motion"; nothing loads from a CDN.
- The shared copy of the HOP assets is now v2 (`scripts/sync-hop-assets.sh` follows it); new text is English in all catalogs until translated (`docs/pending-translation-home2.json`).

### Dashboard
- A "Get the most out of PixelHop" card shows four steps (first upload, an album, a public profile, an API key) with a progress ring, and goes away when all four are done.

### Media addresses
- The profile photo from HOP ID or Google is made absolute before it is shown: a bare path (`/storage/...`) was asked of PixelHop and gave a 404, an `http://` address was blocked on the https pages.
- Photos are also served on the link host (`p.hel.ink/avatar/...`) instead of bouncing to the app, and carry `Cross-Origin-Resource-Policy: cross-origin` so the gallery API's `avatar` can be used on other sites.
- Someone who hides their profile now sees their own photo in the top bar and settings (it answered 404 for them).
- A profile's link preview uses a video's or PDF's poster instead of the file, and no picture when the first image has a content warning.
- A video opened on its own is allowed to play (`media-src 'self'` in its policy); pages allow video from a storage CDN (`media-src ... https:`).

### Dashboard chrome
- The signed-in dashboard (dashboard, albums, settings) has the same chrome as the other HOP dashboards: a sidebar with this app's menu and the HOP family (HOP ID, LinkHop, BioHop, NoteHop, with their marks), a top bar of settings-type controls (upload, language, theme and an account menu with your HOP ID, settings and sign out), and a compact footer.
- The footer on every page spreads its five columns evenly; a flex rule had packed them to the left.

### Home page and storage guard
- The home page was rebuilt as one long page: the uploader first, then what happens to a file during its life, on the shared HOP home frame with the 3D pixel logo.
- Uploads pause with HTTP 507 and `Retry-After` while the data or temporary folder is below the free-space reserve (`PIXELHOP_MIN_FREE_MB`, default 1024, and `PIXELHOP_MIN_FREE_PERCENT`, default 5). Upload pages show the state and check again every 30 seconds; `GET /api/v1/upload-status` reports it. See `docs/UPLOAD-STORAGE.md`.

### Content warnings
- An upload or an album can carry a content warning: sensitive or adult. Visitors see the file blurred and confirm before it opens (adult files ask them to confirm they are 18 or older). Set it when uploading (`content_warning`, `warning_reason`) or afterwards through the API.

### Pages on PixelHop's own domain
- New `/about`, `/features`, `/faq`, `/contact` and a live `/status` page, all in the sitemap. `/contact` links to GitHub issues and private security advisories. `SECURITY.md` and issue templates were added to the repository.
- On a link host (`PIXELHOP_LINK_URL`), app pages now answer with a permanent redirect (301, or 308 for other methods) to the app. Shared links, files, thumbnails, QR codes, the API and `/hop/` are served on both hosts.
- The HOP family table in the README is checked against the one in HEL.ink by a CI job (`scripts/check-hop-family.sh`), and the app has a family section with the shared typography.

### Sign-in and storage security
- `GET /auth/helink/logout` ends the PixelHop session when someone signs out at HOP ID, LinkHop or BioHop. Before, only `POST /logout` existed, the session survived and the next visit signed the person straight back in. The `?next=` address is followed only when it is an https address on the HOP family's domain.
- The SQLite database file is created with mode 0600, and a test checks it.
- Two SSO secrets can be valid at once so the shared secret can be rotated without downtime: `HELINK_SSO_SECRET_PREV` (also `HOP_SSO_SECRET_PREV`) is accepted for tickets and signed `/hop/*` calls next to the current one. `HOP_SSO_SECRET_PIXELHOP` and `HOP_SSO_SECRET` work as names for the current secret.

## [3.7.0] - 2026-10-28

### Links on their own host
- **`PIXELHOP_LINK_URL`: shared links can live on another host than the app.** On p.hel.ink this moves the app to **pixel.hel.ink** (like the other HOP apps) while every link stays `p.hel.ink/slug`. New links (page, direct, thumbnail, album, QR, Markdown/HTML/BBCode, API answers) use the link host.
- On the link host PixelHop serves only what a shared link needs: image and album pages, `/i/` and `/t/` files, QR codes, the password and report forms, static files, the API (so ShareX configs and iPhone Shortcuts keep working) and the signed `/hop/*` calls. Everything else (home, sign-in, dashboard, studio) is sent to the app; forms keep their method (307). Nobody is signed in on the link host.
- A visitor who opens a shared page at the app's address is sent to the link host. Signed-in people stay on the app, where they see their own private files; a private file's page opened on the link host goes to the app for the same reason.
- The link host stops offering the app for install and replaces the service worker with one that removes itself, so phones reinstall the app from its new address.
- Shared pages name the link host as canonical (`<link rel="canonical">`, `og:url`).


### HOP App Protocol
- **Signed admin API under `/hop/*`.** A control center can read health, manifest, stats, a user's profile and counts, content (images and albums), the report queue, settings, search, metrics and the audit trail, and can act: suspend/lock/activate, revoke sessions or API keys, set quotas, warn, purge; hide/unhide/delete/ban-IP/transfer content; dismiss/resolve/delete reports; change the runtime settings. HEL.ink pushes profile changes and account events (`profile.updated`, `account.status`, `account.deleted`, `session.revoked`) to `/hop/events`, handled once per event id. Requests are signed with `HELINK_SSO_SECRET`; `HOP_ADMIN_CLIENTS` names the callers (default `imperium,helink`). Everything that changes something is written to the new `hop_audit` table.
- **SSO ticket v2.** A ticket's `roles` now decide the local role: `owner`, `admin` or `admin:pixelhop` make an admin, anything else makes a plain user (demotion included); the old `admin` flag only counts when `roles` is absent. A username, bio, website and "show owner" sent in the ticket are copied to the profile. A ticket for another app (`aud`) is refused.
- Images now remember why they are hidden (`suspended` or `moderation`), so lifting a suspension brings back only the images the suspension hid.
- **The built-in admin pages are retired.** Moderation (images, users, reports, IP bans) and site settings now live in the HOP control center, which talks to PixelHop through the signed `/hop/*` API; the CLI `user` commands still work. `/admin` sends a signed-in admin to `PIXELHOP_CONTROL_URL` when it is set and otherwise shows a short note saying where things went. The `admin` role keeps its other abilities: seeing hidden images, changing or deleting any image or album through the API, and ZIP downloads. The "Admin" links in the menu are replaced by a "Control center" link when `PIXELHOP_CONTROL_URL` is set.

## [3.6.0] - 2026-10-19

### Added
- **Multi-language OCR.** "Image to text" reads 30 languages and up to five at once, for images that mix languages (for example an English receipt with Japanese or Spanish lines). Pick them as chips; the eight most common show first and "More languages" opens the rest. The choice is remembered, and the first time it starts with English plus your browser's language.

### Changed
- Chinese and Japanese text from OCR no longer has spaces between every character.
- The project website is available in English, 简体中文, Deutsch, Bahasa Indonesia, 日本語, Português (Brasil) and Español.

## [3.5.3] - 2026-10-16

### Changed
- **Upload limits per person.** Guests can upload any file (picture, video or PDF) up to 20 MB (`PIXELHOP_GUEST_MAX_MB`). Signed-in members keep 20 MB per picture and get up to 100 MB per video or PDF (`PIXELHOP_MAX_MEDIA_MB`, default lowered from 500 to 100).
- The upload box shows each person their own limits, and a guest whose file is too big is told how far signing in goes.

## [3.5.2] - 2026-10-16

### Fixed
- **Admin → Users: quota changes no longer get lost.** The quota is typed in GB now (10, 2.5, "2,5" and "10 GB" all work). A value that can't be read is refused with a message instead of being skipped without a word, which is what happened in 3.5.1 when the MB field got a number like "10.240".
- Every admin action on a user (quota, status, role, warning, password, delete) says what happened, or why nothing changed, and keeps your search and page.
- Dashboard: showing, hiding or deleting many files at once reports how many worked. Files that failed, and private or password-protected files that can't go on a profile, are no longer skipped silently.
- Clearer messages when the server answers with an error page instead of JSON (for example "too large").
- Settings → Webhooks: "Send test" with no webhook explains that one is needed first, instead of saying a test was sent.
- IP bans and admin settings confirm the change and report database errors.
- Video and PDF pages no longer advertise an oEmbed link that answered 404.

### Security
- Confirmation messages on Settings, Admin → Users and IP bans come from short codes, so a crafted link can no longer put arbitrary text on those pages.

## [3.5.1] - 2026-10-16

### Added
- **Member dashboard.** Upload straight from the dashboard: links show beside the box and new files appear at the top of "My files" right away. Number cards for files (images, videos, PDFs), views, albums and storage; shortcuts to the studio, albums, public profile, ShareX/API and HEL.ink; a filter by file type.
- One member menu (Dashboard, Albums, Public profile, Studio, Settings) on the dashboard, albums, settings and your own public profile, so every member page links back to the others.

### Fixed
- The upload box fills its column again, as in 3.0. Since 3.4.0 it had shrunk to the width of its content. "From a link" has its label back, and on phones the three buttons fit on one row.

### Removed
- The PixelHop 2 PHP code (`legacy/`). It stays available in the `v2.3.0-php` tag; old links and `pixelhop import` keep working.

## [3.5.0] - 2026-10-16

### Added
- **Own sign-in and sign-up forms with HEL.ink accounts on.** With `HELINK_URL` set, the email and password form on the sign-in page now also accepts HEL.ink accounts (checked with HEL.ink server to server, signed with the shared secret), and signing up here creates the account on HEL.ink too, so one account works in both apps without visiting HEL.ink. "Continue with HEL.ink" stays for Google and two-factor accounts. Needs HEL.ink with `POST /api/pixelhop/login|register`.
- Optional `HELINK_API_ADDR` (e.g. `127.0.0.1:443`) to reach HEL.ink on the same machine for those checks.

### Security
- An unconfirmed HEL.ink address never takes over an existing PixelHop profile with the same email.

## [3.4.2] - 2026-10-16

### Changed
- Image page: the QR code is a compact row inside the details card (with a "Download QR" link) instead of a big card of its own, so the side column no longer runs far below the image.

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
