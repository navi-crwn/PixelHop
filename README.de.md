<p align="center">🌐 <a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a> · <b>Deutsch</b> · <a href="README.id.md">Bahasa Indonesia</a> · <a href="README.ja.md">日本語</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.es.md">Español</a> · <a href="README.fr.md">Français</a> · <a href="README.ru.md">Русский</a></p>

# PixelHop

Open-Source-Bilderhosting in einem einzigen Go-Programm. Es entfernt GPS-Standort und Kameradaten aus Fotos, bevor jemand sie sieht. Das Fotostudio läuft im Browser, das Foto bleibt also auf deinem Gerät, bis du es hochlädst.

Online unter [pixel.hel.ink](https://pixel.hel.ink). Kurzlinks nutzen [p.hel.ink](https://p.hel.ink).

<p align="center">
  <a href="https://pixel.hel.ink"><b>pixel.hel.ink ausprobieren</b></a> ·
  <a href="https://navi-crwn.github.io/PixelHop/">Website</a> ·
  <a href="#install">Installation</a> ·
  <a href="#api">API</a> ·
  <a href="#upgrading">Update von v2</a> ·
  <a href="CHANGELOG.md">Changelog</a> ·
  <a href="SECURITY.md">Sicherheit</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.27-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.27"/>
  <img src="https://img.shields.io/badge/SQLite-built_in-003B57?style=flat-square&logo=sqlite&logoColor=white" alt="SQLite"/>
  <img src="https://img.shields.io/badge/storage-disk_|_S3-a3e635?style=flat-square" alt="Disk or S3"/>
  <img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" alt="MIT License"/>
  <img src="https://img.shields.io/github/actions/workflow/status/navi-crwn/pixelhop/ci.yml?style=flat-square&label=tests" alt="Tests"/>
</p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/home-dark.webp"><img src="docs/readme/home-light.webp" alt="Die PixelHop-Startseite nach einem Gastupload: Der Link ist fertig, ein Hinweis nennt entfernten GPS-Standort, Kameradaten und Aufnahmedatum" width="100%"></picture></p>

## Was es kann

- Nimmt Bilder, Videos und PDFs per Drag and Drop, Einfügen, Dateiauswahl oder Link an und gibt jeder Datei einen kurzen, lesbaren Link wie `beach-sunset-x7Kp2Q`.
- Entfernt versteckte Metadaten aus Fotos und Videos (was genau bei welchem Dateityp passiert, steht unter [Was entfernt wird](#what-gets-removed)).
- Fasst mehrere Dateien in einem Album mit einem Link zusammen.
- Enthält ein Fotostudio, das im Browser komprimiert, skaliert, zuschneidet, konvertiert, unkenntlich macht, Text erkennt und Hintergründe entfernt.
- Läuft als einzelne statische Datei mit eingebautem SQLite. PHP, Python, MySQL oder Cronjobs sind nicht nötig. Bilder liegen auf der Festplatte oder in einem beliebigen S3-kompatiblen Bucket.

Gedacht für alle, die Screenshots und Fotos in Chats, Foren oder auf GitHub teilen, ein [ShareX](https://getsharex.com)-Ziel unter eigener Kontrolle wollen oder darauf achten, was ein Foto verrät.

## Screenshots

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/image-dark.webp"><img src="docs/readme/image-light.webp" alt="Eine Bildseite: das Foto, Kopierknöpfe für direkten Link, Markdown, HTML und BBCode, Dateidetails und ein QR-Code" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/dashboard-dark.webp"><img src="docs/readme/dashboard-light.webp" alt="Das Dashboard für Mitglieder: Seitenleiste mit der HOP-Familie, Schnellzugriffe und ein Raster hochgeladener Dateien mit Tag-Filtern" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/album-dark.webp"><img src="docs/readme/album-light.webp" alt="Eine Albumseite mit vier Fotos und Knöpfen zum Kopieren des Albumlinks, des Einbettungscodes und aller Links in einem Format" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/studio-dark.webp"><img src="docs/readme/studio-light.webp" alt="Das Fotostudio mit geöffnetem Komprimieren-Werkzeug an einem Foto: Format, Qualität, längste Seite und resultierende Dateigröße" width="100%"></picture></p>

Die Screenshots stammen aus einer lokalen Demo-Instanz mit generierten Bildern. [`scripts/readme-images.mjs`](scripts/readme-images.mjs) erzeugt sie neu.

<a id="install"></a>

## Installation

Das Installationsskript prüft deinen Rechner, stellt ein paar Fragen und baut PixelHop aus dem Quellcode. Es braucht Go 1.27 oder neuer, oder Docker.

```bash
curl -fsSL https://raw.githubusercontent.com/navi-crwn/PixelHop/main/install.sh | bash
```

Aus einem Klon startest du `./install.sh`. Später holt `./update.sh` die neue Version, kopiert die Datenbank nach `backups/` und baut neu. `./uninstall.sh` entfernt das Programm, behält aber deine Daten, außer du gibst `--purge` an. Jedes Skript kennt `--yes` (alle Vorgaben übernehmen), `--dry-run` (Schritte nur anzeigen) und `--help`. Das Installationsskript kann PixelHop im Vordergrund laufen lassen, eine systemd-Unit oder eine Docker-Compose-Datei schreiben und auf Wunsch einen [HOP-ID](https://github.com/navi-crwn/HEL.ink)-Hub daneben installieren.

Du kannst auch von Hand installieren.

**Binärdatei** (Linux, macOS und Windows, amd64 und arm64, auf der Seite [Releases](../../releases)):

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

Das Image läuft als Nicht-root-Benutzer 65532, das Datenvolume muss also für diesen Benutzer beschreibbar sein. Im Repository liegt außerdem eine [`docker-compose.yml`](docker-compose.yml), die eine `.env`-Datei liest (kopiere `.env.example`) und die Daten in `./data` ablegt.

**Aus dem Quellcode** (Go 1.27 oder neuer):

```bash
git clone https://github.com/navi-crwn/pixelhop.git && cd pixelhop
make build && ./pixelhop
```

Öffne `http://localhost:8080` und lege ein Konto an. Das erste Konto wird zum Admin. Auf der Kommandozeile geht es auch: `pixelhop user add du@example.com --admin`.

Videos und PDFs werden nur verarbeitet, wenn `ffmpeg`, `ffprobe` und Ghostscript (`gs`) auf dem Server installiert sind. Ohne sie werden diese Dateien so gespeichert, wie sie hochgeladen wurden. Das Docker-Image enthält keines davon. Installiere sie auf dem Host oder baue ein eigenes Image, wenn du Videos oder PDFs hostest. HEIC- und AVIF-Fotos, WebP- und AVIF-Ausgabe über die Größen-URL und die serverseitigen Studio-Werkzeuge brauchen das optionale [`tools/ai-server.py`](tools/ai-server.py).

<a id="what-gets-removed"></a>

## Was entfernt wird

PixelHop arbeitet an der Dateistruktur und kopiert die komprimierten Bilddaten unverändert, die Qualität bleibt also gleich.

| Datei | Was passiert |
|---|---|
| JPEG | EXIF (GPS, Kameramarke und -modell, Seriennummern, Aufnahmedatum), XMP, IPTC und Kommentare werden entfernt. Farbprofil und andere Segmente bleiben. Ein Foto, dessen EXIF eine Drehung angibt, wird aufgerichtet und mit Qualität 92 neu als JPEG gespeichert, ohne Farbprofil. Das ist der einzige Fall, in dem Pixel neu kodiert werden. |
| PNG | Textblöcke, der EXIF-Block und die Änderungszeit werden entfernt. Das Farbprofil bleibt. |
| WebP | EXIF und XMP werden aus erweiterten Dateien entfernt. Einfache WebP-Dateien enthalten keine Metadaten. |
| GIF | Wird so gespeichert, wie es hochgeladen wurde. Es wird nichts entfernt. |
| BMP | Wird in PNG umgewandelt. |
| HEIC, AVIF | Wird bei eingerichtetem KI-Dienst in ein aufgerichtetes JPEG oder PNG ohne Metadaten umgewandelt. Sonst wird der Upload abgelehnt. |
| Video (MP4, MOV, 3GP, WebM, MKV) | Mit installiertem `ffmpeg` werden Standort, Datum, Kamera und Encoder-Tags ohne Neukodierung entfernt. Nur die erste Video- und die erste Tonspur bleiben. Danach verkleinert ein Hintergrundjob das Video auf H.264 und AAC, höchstens 1280 px an der langen Seite und 30 Bilder pro Sekunde. Ohne `ffmpeg` wird die Datei so gespeichert, wie sie hochgeladen wurde. |
| PDF | Mit installiertem Ghostscript wird die Datei neu geschrieben, um sie kleiner zu machen. Die kleinere Kopie bleibt, wenn sie mindestens 10 % spart. Dokumenteigenschaften wie Titel und Autor werden nicht entfernt. Ohne Ghostscript wird die Datei so gespeichert, wie sie hochgeladen wurde. |

Nach einem Upload nennt die Seite, welche Arten von Daten entfernt wurden. Geprüft wird nach Art (GPS-Standort, Kameradaten, Aufnahmedatum, Bearbeitungssoftware, XMP, IPTC, Kommentar, eingebetteter Text). Die Werte selbst werden nie ausgelesen.

## Funktionen

Teilen:
- Für jede Datei gibt es einen direkten Link, einen Seitenlink, Markdown, HTML, BBCode und einen QR-Code sowie Open-Graph-Tags, damit Vorschauen in Discord, X und WhatsApp funktionieren.
- Alben aus mehreren Dateien, mit Titel, Beschreibung, Titelbild und Reihenfolge, die du später ändern kannst. Ein Album lässt sich per iframe in eine andere Website einbetten (`/a/{id}/embed`).
- Alle Links eines Albums oder eines Mehrfach-Uploads in einem Format mit einem Klick kopieren.
- Ablauf nach 10 Minuten, 1 Stunde, 1 Tag, 7 Tagen, 30 Tagen oder 90 Tagen.
- Jede Datei ist nicht gelistet (Standard), nur für dich sichtbar oder passwortgeschützt. Eine Datei kann eine Inhaltswarnung tragen (sensibel oder nur für Erwachsene); Besucher bestätigen, bevor sie sie sehen. Besitzer können verbieten, dass ihre Bilder auf anderen Websites eingebettet werden.
- Tags, Suche und ein Tag-Filter im Dashboard. Gelöschte Dateien liegen 7 Tage im Papierkorb.
- Aufrufe pro Tag und die Websites, die auf eine Datei verlinken. Referrer werden nur als Hostname gespeichert.
- Skalierte Kopien auf Abruf, zum Beispiel `/i/photo.jpg?w=800`, einmal erzeugt und dann zwischengespeichert.
- Öffentliche Profile unter `/u/username`, oEmbed und eine Sitemap.

Konten:
- Gäste können hochladen (standardmäßig bis 20 MB pro Datei, 25 Dateien am Tag pro Adresse und 10 Bilder pro Album) und bekommen für jede Datei einen privaten Löschlink, den sich auch der Browser merkt.
- Mitglieder bekommen eine Galerie mit Suche, Sortierung, Umbenennen und Sammelaktionen, ein Speicherkontingent (standardmäßig 128 MB), eine Einstiegskarte und einen ZIP-Export aller hochgeladenen Dateien.
- Die Einstellungen zeigen, wo du angemeldet bist, melden dich überall sonst ab und verwalten API-Schlüssel, Webhooks, dein Profil und die Kontolöschung.
- Anmeldung mit E-Mail und Passwort, mit Google oder mit einem HEL.ink-Konto (dem HOP-ID-Hub), alles optional. Ohne `HELINK_URL` nimmt PixelHop nie Kontakt zu HEL.ink auf. Cloudflare Turnstile kann Anmelde- und Registrierungsformulare schützen.

Studio:
- Im Browser: komprimieren, skalieren, zuschneiden, konvertieren (JPG, PNG, WebP und AVIF, wo der Browser es kodieren kann), anpassen und drehen, weichzeichnen und verpixeln, Wasserzeichen, Farbpalette, QR-Code, Text aus Bildern (OCR, 30 Sprachen, bis zu fünf gleichzeitig) und Hintergrund entfernen. Bis du „Ergebnis hochladen“ drückst, wird nichts hochgeladen.
- Stapel: eine Bildgruppe skalieren, konvertieren, mit Wasserzeichen versehen oder die KI-Schritte darauf anwenden und dann ein ZIP herunterladen oder ein Album hochladen.
- Auf dem Server, für angemeldete Nutzer, wenn du `tools/ai-server.py` betreibst: Hintergrund entfernen, automatisches Weichzeichnen von Gesichtern, Zauberradierer und 2-fach-Hochskalierung. Ein Bild nach dem anderen, mit Tageslimit, und nichts wird gespeichert.

Für Entwickler:
- Eine REST-API mit Bearer-Schlüsseln. Ein Schlüssel lässt sich auf reines Hochladen beschränken.
- Eine ShareX-Konfigurationsdatei, die du in den Einstellungen erzeugst. ShareX-Konfigurationen aus PixelHop 2 funktionieren weiter.
- Webhooks für `image.uploaded` und `image.deleted`, mit HMAC-SHA256 im Header `X-PixelHop-Signature` signiert.
- Ein Kommandozeilen-Uploader im selben Programm: `pixelhop push *.png --format markdown`.
- Eine [OpenAPI-Beschreibung](web/static/openapi.json) unter `/static/openapi.json` sowie `/healthz`, `/metrics` und `/status` fürs Monitoring.
- Als App auf dem Handy installierbar, mit Teilen-Ziel für Fotos, Videos und PDFs.

Betrieb:
- Eine einzelne statische Datei, ein Docker-Image oder eine systemd-Unit. Dateien liegen auf der Festplatte oder in S3-kompatiblem Speicher (AWS S3, Cloudflare R2, MinIO, Backblaze B2, Wasabi, Contabo).
- Uploads pausieren von selbst, wenn der freie Speicherplatz knapp wird, und `PIXELHOP_READ_ONLY=true` pausiert sie für Wartungsarbeiten. Bereits online stehende Dateien bleiben erreichbar. Siehe [docs/UPLOAD-STORAGE.md](docs/UPLOAD-STORAGE.md).
- Die Oberfläche gibt es in neun Sprachen: English, 简体中文, Deutsch, Bahasa Indonesia, 日本語, Português (Brasil), Español, Français und Русский. Mit `?lang=de` wechselst du die Sprache; die Wahl bleibt in einem Cookie gespeichert, die Spracheinstellung des Browsers wird nicht verwendet.
- Geteilte Links können auf einem anderen Host liegen als die App (`PIXELHOP_LINK_URL`). Bei pixel.hel.ink liegt die App unter pixel.hel.ink, und die Links bleiben `p.hel.ink/...`.

Sicherheit:
- Passwörter nutzen Argon2id. API-Schlüssel und Sitzungstoken werden als Hashes gespeichert.
- Formulare sind gegen seitenübergreifende Posts geschützt, und es wird eine strikte Content Security Policy gesendet.
- „Von Link hochladen“ prüft die aufgelöste Adresse jeder Verbindung und lässt sich so nicht auf private Netzwerke richten. Sehr große Bilder werden anhand ihres Headers abgelehnt, bevor sie dekodiert werden. Uploads sind ratenbegrenzt, und Client-IPs werden nur von Proxys übernommen, die du angibst.
- Von Moderatoren entfernte Dateien lassen sich nicht erneut hochladen, weil ihr SHA-256 gespeichert bleibt.
- Wie ein Upload verarbeitet wird und wie der Server unter Last stabil bleibt: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). Die Vorgeschichte von Version 3: [AUDIT.md](AUDIT.md). Schwachstellen meldest du nach [SECURITY.md](SECURITY.md).

Moderation: Die eingebauten Admin-Seiten wurden in 3.7.0 abgeschafft. Jede Seite hat einen Melden-Knopf. Meldewarteschlange, Ausblenden und Löschen von Inhalten, IP-Sperren, Kontoaktionen, Laufzeiteinstellungen und die Ankündigungsleiste gehören zum HOP-Kontrollzentrum, das über die signierte `/hop/*`-API (das HOP App Protocol) mit PixelHop spricht. Es wird mit der HEL.ink-Anmeldung benutzt, mit `HELINK_SSO_SECRET` signiert, und die erlaubten Aufrufer stehen in `HOP_ADMIN_CLIENTS` (Standard `imperium,helink`). Jede so vorgenommene Änderung wird in der Tabelle `hop_audit` festgehalten. Das Kontrollzentrum selbst ist privat beim HOP-Projekt. Eine eigenständige Installation hat den Melden-Knopf, die `pixelhop user`-Befehle (`add`, `passwd`, `admin`, `merge`) und die zusätzlichen Rechte der Admin-Rolle über die API, aber keine Oberfläche für die Meldewarteschlange.

## Einstellungen

Einstellungen sind Umgebungsvariablen, alle optional. [`.env.example`](.env.example) listet die üblichen mit ihren Vorgaben auf, die übrigen stehen in [`internal/config/config.go`](internal/config/config.go). Die Tabelle zeigt die, die du am ehesten änderst. Die vollständige Liste steht in der englischen [README](README.md).

| Variable | Standard | Wirkung |
|---|---|---|
| `PIXELHOP_BASE_URL` | `http://localhost:8080` | Öffentliche Adresse der App (Anmeldung, Upload, Dashboard). Nutze in Produktion `https://`, das schaltet auch sichere Cookies ein |
| `PIXELHOP_LINK_URL` | `PIXELHOP_BASE_URL` | Ein anderer Host für geteilte Links (Bild- und Albumseiten, direkte Dateien). Er muss ein anderer Host sein und ebenfalls PixelHop erreichen |
| `PIXELHOP_DATA` | `./data` | Datenbank und, bei lokalem Speicher, die Bilder |
| `PIXELHOP_MAX_UPLOAD_MB` | `20` | Pro Bild |
| `PIXELHOP_MAX_MEDIA_MB` | `100` | Pro Video oder PDF, für angemeldete Mitglieder |
| `PIXELHOP_GUEST_MAX_MB` | `20` | Pro Datei beliebigen Typs, für Gäste |
| `PIXELHOP_GUEST_UPLOADS` | `true` | Uploads ohne Konto erlauben |
| `PIXELHOP_GUEST_ALBUM_MAX` | `10` | Bilder in einem Album eines Gastes (0 = keine Grenze); auch im Kontrollzentrum einstellbar |
| `PIXELHOP_GUEST_DAILY_MAX` | `25` | Dateien, die eine Gast-Adresse in 24 Stunden hochladen darf (0 = keine Grenze); auch im Kontrollzentrum einstellbar |
| `PIXELHOP_GUEST_TTL_DAYS` | `30` | Tage, die ein neuer Gast-Upload online bleibt (0 = bis zum Löschen); ältere Dateien bleiben unverändert; auch im Kontrollzentrum einstellbar |
| `PIXELHOP_SHARE_STORAGE` | `true` | Eine gespeicherte Datei, wenn verschiedene Personen dasselbe Bild hochladen; jede behält ihren eigenen Link, Besitzer und Ablauf (false = jeder Upload speichert eine eigene Kopie); bereits gespeicherte Dateien bleiben unverändert; auch im Kontrollzentrum einstellbar |
| `PIXELHOP_DEFAULT_QUOTA_MB` | `128` | Speicher für jedes neue Konto; bestehende Konten behalten ihres |
| `PIXELHOP_REGISTRATION` | `true` | Neue Konten erlauben |
| `PIXELHOP_READ_ONLY` | `false` | Uploads für Wartungsarbeiten pausieren |
| `PIXELHOP_TRUSTED_PROXIES` | - | `cloudflare` oder die IPs oder CIDRs deines Reverse-Proxys |
| `PIXELHOP_STORAGE`, `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY` | `local` | `local` oder `s3`; für S3-Speicher werden die übrigen vier gebraucht |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | - | Mit Google anmelden |
| `HELINK_URL`, `HELINK_SSO_SECRET` | - | Mit einem HEL.ink-Konto anmelden. Das Geheimnis hat mindestens 32 Zeichen und entspricht `PIXELHOP_SSO_SECRET` in der `.env` von HEL.ink |
| `PIXELHOP_CONTROL_URL` | - | Adresse des HOP-Kontrollzentrums; Admins, die `/admin` öffnen, werden dorthin geschickt |
| `TURNSTILE_SITE_KEY`, `TURNSTILE_SECRET_KEY` | - | Cloudflare Turnstile bei Anmeldung und Registrierung. Bei aktiven HEL.ink-Konten lehnen diese Formulare ohne die Schlüssel jeden Versuch ab |
| `PIXELHOP_REMBG_URL` | - | Adresse von `tools/ai-server.py`, zum Beispiel `http://127.0.0.1:7001` |

Das HEL.ink-Geheimnis kann auch `HOP_SSO_SECRET_PIXELHOP` oder `HOP_SSO_SECRET` heißen. Beim Rotieren gilt das alte Geheimnis über `HELINK_SSO_SECRET_PREV` (oder `HOP_SSO_SECRET_PREV`) weiter.

### Reverse-Proxy, Dienst und Backups

- [`deploy/nginx.conf`](deploy/nginx.conf) ist ein Beispiel für einen Reverse-Proxy. Setze `PIXELHOP_TRUSTED_PROXIES=127.0.0.1`, damit PixelHop dem `X-Forwarded-For` von nginx vertraut, und ergänze `cloudflare`, wenn Cloudflare davorsteht.
- [`deploy/pixelhop.service`](deploy/pixelhop.service) ist eine gehärtete systemd-Unit. [`deploy/pixelhop-ai.service`](deploy/pixelhop-ai.service) startet den optionalen KI-Dienst, der `rembg`, `opencv-python-headless` und `onnxruntime` braucht (siehe Kopf von [`tools/ai-server.py`](tools/ai-server.py)).
- Für ein Backup kopierst du den Datenordner oder führst bei laufendem PixelHop `sqlite3 data/pixelhop.db ".backup copy.db"` aus. `./update.sh` legt vor jedem Update eine solche Kopie in `backups/` an.

## So benutzt du es

1. Öffne die Website und ziehe ein Bild hinein, füge es ein (<kbd>Strg</kbd>+<kbd>V</kbd>) oder gib einen Link an.
2. Optional: Wähle „Löschen nach“ und ob der Dateiname in den Link kommt.
3. Der Link wird für dich kopiert. Öffne ihn für den direkten Link, die Einbettungscodes und den QR-Code.

Mit `?` zeigt jede Seite die Tastenkürzel.

**ShareX.** Geh zu Einstellungen, API-Schlüssel, erstelle einen Schlüssel, wähle „ShareX-Konfiguration mit diesem Schlüssel herunterladen“ und öffne die Datei.

**Kommandozeile.** Dasselbe Programm `pixelhop` lädt auf jeden PixelHop-Server hoch. `pixelhop login` speichert Adresse und Schlüssel in `~/.config/pixelhop/client.json`; `PIXELHOP_SERVER` und `PIXELHOP_KEY` haben Vorrang.

```bash
pixelhop login --server https://pixel.hel.ink --key ph_yourkey
pixelhop push screenshot.png photos/*.jpg --expire 7d --format markdown
```

**Handy.** Öffne die Website und wähle „Zum Home-Bildschirm“. PixelHop erscheint dann im Teilen-Menü.

<a id="api"></a>

## API

```bash
curl -H "Authorization: Bearer ph_yourkey" -F "file=@holiday.jpg" https://pixel.hel.ink/api/v1/upload
```

| Endpunkt | Wirkung |
|---|---|
| `POST /api/v1/upload` | Dateien (`file`, wiederholbar) oder Links (`url`) hochladen. Optionen: `expire` (`10m`, `1h`, `1d`, `7d`, `30d`, `90d`), `slug` (`0` hält den Dateinamen aus dem Link), `title`, `alt`, `album_id`, `album` (`force` oder `0`), `listed`, `content_warning` (`sensitive` oder `adult`), `warning_reason` |
| `POST /api/v1/uploads`, `PUT /api/v1/uploads/{id}`, `POST /api/v1/uploads/{id}/finish` | Eine große Datei in 8-MB-Stücken hochladen |
| `GET /api/v1/images` | Deine Bilder (`page`, `limit`, `q`, `sort`) |
| `GET`, `PATCH`, `DELETE /api/v1/images/{id}` | Eins lesen, ändern oder löschen. `PATCH` nimmt `title`, `alt`, `listed`, `visibility` (`unlisted`, `private` oder `password` mit `password`), `tags`, `content_warning` und `warning_reason`. `DELETE` verschiebt deine eigene Datei in den Papierkorb; mit `?permanent=1` wird sie endgültig entfernt |
| `POST /api/v1/images/{id}/restore` | Eine Datei aus dem Papierkorb zurückholen |
| `GET /api/v1/images/{id}/stats` | Aufrufe pro Tag und verweisende Websites |
| `GET`, `POST /api/v1/albums` / `GET`, `PATCH`, `DELETE /api/v1/albums/{id}` | Alben auflisten, anlegen, lesen, bearbeiten und löschen |
| `GET /api/v1/users/{username}/images` | Öffentlich: die Bilder im Profil einer Person |
| `GET /api/v1/me` | Dein Konto und der Speicherverbrauch |
| `GET /api/v1/me/media` | Deine eigenen Bilder (`type=image`) oder PDFs (`type=pdf`), die eine andere Seite zeigen darf: keine privaten, passwortgeschützten, ablaufenden oder mit Inhaltswarnung versehenen Dateien (`cursor`, `limit`). Auch für die in `PIXELHOP_MEDIA_CLIENTS` genannten HOP-Apps (BioHops Medienauswahl), serverseitig signiert für ein Mitglied |
| `GET /api/v1/stats` | Öffentliche Summen und die Serverversion |
| `GET /api/v1/upload-status` | Ob gerade Uploads angenommen werden |
| `POST /api/v1/tools/{remove-background,faces,erase,upscale}` | Server-KI-Werkzeuge (angemeldet; `file`, bei erase zusätzlich `mask`) |
| `GET /i/{id}.jpg?w=&h=&fit=&format=&q=` | Skalierte Kopien, einmal erzeugt und dann zwischengespeichert. `format=webp` und `avif` brauchen den KI-Dienst |
| `GET /oembed?url=...` | oEmbed für Bildseiten |

Jede Antwort sieht aus wie `{"success": true, "data": ...}` oder `{"success": false, "error": "..."}`. Die vollständige Referenz liegt auf jedem PixelHop-Server unter [`/docs`](https://pixel.hel.ink/docs).

<a id="upgrading"></a>

## Update von PixelHop 2

PixelHop 2 war eine PHP-App mit MySQL, einer Python-venv und Cronjobs. Der Code bleibt im Tag [`v2.3.0-php`](../../tree/v2.3.0-php) erhalten. Ein Befehl übernimmt die Daten:

```bash
pixelhop import --images /var/www/pichost/data/images.json \
  --mysql "user:pass@tcp(127.0.0.1:3306)/pixelhop" --copy
```

- Nutzer behalten ihre IDs, Rollen, Kontingente und Passwörter. Die argon2id-Hashes aus PHP werden bei der nächsten Anmeldung geprüft und aktualisiert. Google-Konten bleiben verknüpft.
- Bilder behalten ihre IDs. Mit `--copy` werden sie in den Speicher von PixelHop 3 geladen und bekommen neue Vorschaubilder; ohne den Schalter werden sie weiter aus den alten Buckets ausgeliefert.
- Alte Links funktionieren weiter: `/{id}`, `/i/2026/08/27/{id}_original.webp`, `/auth/google-callback` (setze `GOOGLE_REDIRECT_URI`) und ShareX-Konfigurationen, die an `/api/upload.php` senden.

Warum es neu geschrieben wurde, steht in [AUDIT.md](AUDIT.md).

## Entwicklung

```
cmd/pixelhop/     das Kommando: serve, user, import, push, login
internal/app/     Webserver: Seiten, API, Uploads, Moderation, HOP App Protocol
internal/db/      SQLite-Schema, Migrationen und Abfragen
internal/imaging/ Typerkennung, Metadaten entfernen, Vorschaubilder
internal/storage/ lokale Festplatte, S3 (eigener SigV4-Signierer), alte URLs
internal/netsafe/ SSRF-sicheres Abrufen für „Von Link hochladen“
web/              Vorlagen, Skripte und die neun Sprachdateien, ins Programm eingebettet
docs/             die GitHub-Pages-Website und die README-Bilder
scripts/          Hilfsskripte (README-Bilder, Changelog-Daten, HOP-Familien-Check)
tools/            der optionale KI-Dienst
deploy/           nginx- und systemd-Beispiele
```

`make test` führt `go vet` und die Tests aus, `make build` erzeugt `./pixelhop`. `scripts/readme-images.mjs` rendert die README-Screenshots aus einer laufenden Instanz neu (siehe Kommentar am Dateianfang). `python3 scripts/build-changelog.py --repo PixelHop` baut `web/changelog.json`, die Daten hinter der Seite `/changelog`, aus `CHANGELOG.md` neu (verständliche Sprache, ein Eintrag je Version); es braucht weder Netz noch `gh`, und `make check-changelog` schlägt fehl, wenn die Datei nicht verständlich oder nicht aktuell ist.

## FAQ

<details><summary>Ist das Studio wirklich privat?</summary>

Die Bearbeitung läuft auf deinem Gerät mit Canvas und WebAssembly. Wenn du Texterkennung oder Hintergrundentfernung zum ersten Mal benutzt, lädt der Browser [tesseract.js](https://github.com/naptha/tesseract.js) und [@imgly/background-removal](https://github.com/imgly/background-removal-js) von jsDelivr und deren Sprachdaten und Modelle von tessdata.projectnaptha.com und staticimgly.com. Dein Bild wird nirgendwohin gesendet, solange du nicht „Ergebnis hochladen“ drückst. Ein selbst gehosteter Server, der eine strengere Content Security Policy setzt, muss diese Adressen erlauben.
</details>

<details><summary>Kann ich Gast-Uploads oder Registrierungen abschalten?</summary>

Ja. Setze `PIXELHOP_GUEST_UPLOADS=false` oder `PIXELHOP_REGISTRATION=false`. Mit dem HOP-Kontrollzentrum kannst du beides auch ändern, während der Server läuft.
</details>

<details><summary>Was braucht es an Ressourcen?</summary>

Ein kleiner Server genügt. Ein frisch gestarteter `pixelhop`-Prozess brauchte bei einem Test unter macOS etwa 16 MB Arbeitsspeicher, und die Release-Binärdateien sind 16 bis 17 MB groß. Große Bilder werden nur wenige gleichzeitig dekodiert, ein Schwung Uploads kann also nicht den ganzen Speicher aufbrauchen.
</details>

## Selbst hosten

<!-- self-hosting:start -->
Alles, was PixelHop braucht, liegt in öffentlichen Repositorys: kein privates Repository, kein Token und kein hel.ink-Konto. Details stehen in der Familienanleitung: [docs/self-hosting.md](https://github.com/navi-crwn/HEL.ink/blob/main/docs/self-hosting.md) (Englisch).

| Baustein | Für PixelHop | Wozu |
| --- | --- | --- |
| [HEL.ink](https://github.com/navi-crwn/HEL.ink) (Hub, HOP ID) | Optional: ergänzt „Mit HOP ID anmelden“ | Konten und Anmeldung für alle Apps |
| [Imperium](https://github.com/navi-crwn/Imperium) | Optional, aber die Admin-Seiten von PixelHop liegen dort | Admin-Panel für die ganze Familie; braucht den Hub |
| Familien-Schriften und -Kacheln | Nichts zu installieren: kommen von `hel.ink/hop-assets` | Ohne hel.ink: Systemschriften, keine schwebenden Kacheln |

- **Reihenfolge**: PixelHop allein mit `./install.sh`. Für die Anmeldung mit HOP ID zuerst der [Hub](https://github.com/navi-crwn/HEL.ink): `./install.sh --hub new` installiert einen daneben.
- **Mindest-Einstellungen**: `PIXELHOP_BASE_URL`; mit Hub: `HELINK_URL`, `HELINK_SSO_SECRET`; mit Imperium: `PIXELHOP_CONTROL_URL`.
- **Allein oder in der Familie**: PixelHop läuft allein mit eigenen Konten. Mit einem Hub kann man sich zusätzlich mit HOP ID anmelden und überall zugleich abmelden.
- **Ohne Imperium**: `/admin` zeigt einen Hinweis. Dafür gibt es `pixelhop user add` und die signierte `/hop/*`-API (das erste Konto ist Admin).
<!-- self-hosting:end -->

## Die HOP-Familie

PixelHop gehört zur HOP-Familie datenschutzfreundlicher, selbst hostbarer Open-Source-Apps.

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

## Danksagung und Lizenz

- [MIT](LICENSE.md), Copyright Ivan Novskies (navi-crwn). Teile von Drittanbietern stehen in [CREDITS.md](CREDITS.md).
- Gebaut mit [Go](https://go.dev), [modernc.org/sqlite](https://gitlab.com/cznic/sqlite), [golang.org/x/image](https://pkg.go.dev/golang.org/x/image) und [rsc.io/qr](https://pkg.go.dev/rsc.io/qr).
- Das Studio lädt bei Bedarf [tesseract.js](https://github.com/naptha/tesseract.js) (Apache-2.0) und [@imgly/background-removal](https://github.com/imgly/background-removal-js) (AGPL-3.0, unverändert, von jsDelivr).

Mehr vom selben Autor: [Amnesia](https://github.com/navi-crwn/amnesia), ein Mac, der beim Abmelden alles vergisst.
