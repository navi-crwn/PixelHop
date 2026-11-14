<p align="center">🌐 <a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.de.md">Deutsch</a> · <b>Bahasa Indonesia</b> · <a href="README.ja.md">日本語</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.es.md">Español</a> · <a href="README.fr.md">Français</a> · <a href="README.ru.md">Русский</a></p>

# PixelHop

Hosting gambar open source dalam satu program Go. PixelHop menghapus lokasi GPS dan detail kamera dari foto sebelum ada yang melihatnya. Studio fotonya berjalan di browser, jadi foto tetap ada di perangkatmu sampai kamu mengunggahnya.

Tersedia di [pixel.hel.ink](https://pixel.hel.ink). Tautan pendek memakai [p.hel.ink](https://p.hel.ink).

<p align="center">
  <a href="https://pixel.hel.ink"><b>Coba pixel.hel.ink</b></a> ·
  <a href="https://navi-crwn.github.io/PixelHop/">Situs</a> ·
  <a href="#install">Instal</a> ·
  <a href="#api">API</a> ·
  <a href="#upgrading">Naik dari v2</a> ·
  <a href="CHANGELOG.md">Changelog</a> ·
  <a href="SECURITY.md">Keamanan</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.27-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.27"/>
  <img src="https://img.shields.io/badge/SQLite-built_in-003B57?style=flat-square&logo=sqlite&logoColor=white" alt="SQLite"/>
  <img src="https://img.shields.io/badge/storage-disk_|_S3-a3e635?style=flat-square" alt="Disk or S3"/>
  <img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" alt="MIT License"/>
  <img src="https://img.shields.io/github/actions/workflow/status/navi-crwn/pixelhop/ci.yml?style=flat-square&label=tests" alt="Tests"/>
</p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/home-dark.webp"><img src="docs/readme/home-light.webp" alt="Halaman unggah PixelHop setelah tamu mengunggah foto: tautan sudah siap, dan ada catatan bahwa lokasi GPS, detail kamera, dan tanggal pengambilan dihapus" width="100%"></picture></p>

## Apa yang dilakukannya

- Menerima gambar, video, dan PDF lewat seret dan lepas, tempel, pilih file, atau tautan, lalu memberi setiap file tautan pendek yang mudah dibaca, misalnya `beach-sunset-x7Kp2Q`.
- Menghapus metadata tersembunyi dari foto dan video (apa tepatnya untuk tiap jenis file ada di [Apa yang dihapus](#what-gets-removed)).
- Menggabungkan beberapa file menjadi album dengan satu tautan.
- Punya studio foto yang mengompres, mengubah ukuran, memotong, mengonversi, memburamkan, membaca teks, dan menghapus latar belakang di browser.
- Berjalan sebagai satu file statis dengan SQLite bawaan. Tidak perlu PHP, Python, MySQL, atau cron. Gambar disimpan di disk atau di bucket apa pun yang kompatibel dengan S3.

Cocok untuk orang yang membagikan tangkapan layar dan foto di obrolan, forum, atau GitHub, yang ingin tujuan [ShareX](https://getsharex.com) yang dikendalikan sendiri, atau yang peduli pada apa yang dibocorkan sebuah foto.

## Tangkapan layar

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/image-dark.webp"><img src="docs/readme/image-light.webp" alt="Halaman gambar: foto, tombol salin untuk tautan langsung, Markdown, HTML, dan BBCode, detail file, dan kode QR" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/dashboard-dark.webp"><img src="docs/readme/dashboard-light.webp" alt="Dasbor anggota: bilah samping berisi keluarga HOP, pintasan, dan kisi file yang diunggah dengan filter tag" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/album-dark.webp"><img src="docs/readme/album-light.webp" alt="Halaman album dengan empat foto dan tombol untuk menyalin tautan album, kode sematan, dan semua tautan dalam satu format" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/studio-dark.webp"><img src="docs/readme/studio-light.webp" alt="Studio foto dengan alat kompres terbuka pada sebuah foto: format, kualitas, sisi terpanjang, dan ukuran file hasilnya" width="100%"></picture></p>

Tangkapan layar ini berasal dari instans demo lokal dengan gambar buatan. [`scripts/readme-images.mjs`](scripts/readme-images.mjs) membuatnya ulang.

<a id="install"></a>

## Instal

Skrip instalasi memeriksa komputermu, mengajukan beberapa pertanyaan, lalu membangun PixelHop dari kode sumber. Skrip ini membutuhkan Go 1.27 atau lebih baru, atau Docker.

```bash
curl -fsSL https://raw.githubusercontent.com/navi-crwn/PixelHop/main/install.sh | bash
```

Dari hasil clone, jalankan `./install.sh`. Nanti, `./update.sh` menarik versi baru, menyalin database ke `backups/`, dan membangun ulang, sedangkan `./uninstall.sh` menghapus program tetapi menyimpan datamu kecuali kamu menambahkan `--purge`. Setiap skrip menerima `--yes` (terima semua nilai bawaan), `--dry-run` (tampilkan langkah tanpa menjalankannya), dan `--help`. Skrip instalasi bisa menjalankan PixelHop di latar depan, menulis unit systemd atau file Docker Compose, dan, kalau mau, memasang hub [HOP ID](https://github.com/navi-crwn/HEL.ink) di sebelahnya.

Kamu juga bisa memasang secara manual.

**Biner** (Linux, macOS, dan Windows, amd64 dan arm64, di halaman [Releases](../../releases)):

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

Image ini berjalan sebagai pengguna non-root 65532, jadi volume data harus bisa ditulis oleh pengguna itu. Repositori juga punya [`docker-compose.yml`](docker-compose.yml) yang membaca file `.env` (salin `.env.example`) dan menyimpan data di `./data`.

**Dari kode sumber** (Go 1.27 atau lebih baru):

```bash
git clone https://github.com/navi-crwn/pixelhop.git && cd pixelhop
make build && ./pixelhop
```

Buka `http://localhost:8080` dan buat akun. Akun pertama menjadi admin. Kamu juga bisa membuatnya lewat baris perintah: `pixelhop user add kamu@example.com --admin`.

Video dan PDF hanya diproses jika `ffmpeg`, `ffprobe`, dan Ghostscript (`gs`) terpasang di server. Tanpa semuanya itu, file disimpan apa adanya. Image Docker tidak berisi satu pun dari alat itu, jadi pasang di host atau buat image sendiri jika kamu menyimpan video atau PDF. Foto HEIC dan AVIF, keluaran WebP dan AVIF dari URL ubah ukuran, dan alat studio di sisi server membutuhkan [`tools/ai-server.py`](tools/ai-server.py) yang opsional.

<a id="what-gets-removed"></a>

## Apa yang dihapus

PixelHop bekerja pada struktur file dan menyalin data gambar terkompresi apa adanya, sehingga kualitasnya tidak berubah.

| File | Yang terjadi |
|---|---|
| JPEG | EXIF (GPS, merek dan model kamera, nomor seri, tanggal pengambilan), XMP, IPTC, dan komentar dihapus. Profil warna dan segmen lain tetap ada. Foto yang EXIF-nya menyatakan terputar akan ditegakkan dan disimpan ulang sebagai JPEG dengan kualitas 92, tanpa profil warna. Ini satu-satunya kasus ketika piksel dikodekan ulang. |
| PNG | Potongan teks, potongan EXIF, dan waktu modifikasi dihapus. Profil warna tetap ada. |
| WebP | EXIF dan XMP dihapus dari file yang diperluas. File WebP sederhana tidak membawa metadata. |
| GIF | Disimpan apa adanya. Tidak ada yang dihapus. |
| BMP | Dikonversi ke PNG. |
| HEIC, AVIF | Jika layanan AI disiapkan, dikonversi menjadi JPEG atau PNG tegak tanpa metadata. Jika tidak, unggahan ditolak. |
| Video (MP4, MOV, 3GP, WebM, MKV) | Jika `ffmpeg` terpasang, lokasi, tanggal, kamera, dan tag encoder dihapus tanpa pengodean ulang. Hanya trek video pertama dan trek audio pertama yang dipertahankan. Setelah itu tugas latar belakang mengecilkan video ke H.264 dan AAC, paling besar 1280 piksel pada sisi panjang dan 30 frame per detik. Tanpa `ffmpeg`, file disimpan apa adanya. |
| PDF | Jika Ghostscript terpasang, file ditulis ulang agar lebih kecil, dan salinan yang lebih kecil dipakai bila menghemat minimal 10%. Properti dokumen seperti judul dan penulis tidak dihapus. Tanpa Ghostscript, file disimpan apa adanya. |

Setelah unggahan, halaman menyebutkan jenis data apa yang dihapus. Pemeriksaannya berdasarkan jenis (lokasi GPS, detail kamera, tanggal pengambilan, perangkat lunak penyunting, XMP, IPTC, komentar, teks tersemat). Nilainya tidak pernah dibaca.

## Fitur

Berbagi:
- Setiap file mendapat tautan langsung, tautan halaman, Markdown, HTML, BBCode, dan kode QR, serta tag Open Graph agar pratinjau tampil benar di Discord, X, dan WhatsApp.
- Album dari beberapa file, dengan judul, deskripsi, sampul, dan urutan yang bisa diubah kemudian. Album bisa disematkan di situs lain dengan iframe (`/a/{id}/embed`).
- Salin semua tautan sebuah album atau unggahan massal dalam satu format dengan satu klik.
- Kedaluwarsa setelah 10 menit, 1 jam, 1 hari, 7 hari, 30 hari, atau 90 hari.
- Setiap file bisa tidak terdaftar (bawaan), hanya untukmu, atau dilindungi kata sandi. File bisa diberi peringatan konten (sensitif atau dewasa); pengunjung mengonfirmasi dulu sebelum melihatnya. Pemilik bisa melarang gambarnya disematkan di situs lain.
- Tag, pencarian, dan filter tag di dasbor. File yang dihapus menunggu di tempat sampah selama 7 hari.
- Tayangan per hari dan situs yang menautkan ke sebuah file. Perujuk hanya disimpan sebagai nama host.
- Salinan berukuran lain sesuai permintaan, misalnya `/i/photo.jpg?w=800`, dibuat sekali lalu disimpan di cache.
- Profil publik di `/u/username`, oEmbed, dan sitemap.

Akun:
- Tamu bisa mengunggah (bawaan sampai 20 MB per file, 25 file per hari per alamat, dan 10 gambar per album) dan mendapat tautan hapus pribadi untuk setiap file, yang juga diingat browser.
- Anggota mendapat galeri dengan pencarian, pengurutan, penggantian nama, dan aksi massal, kuota penyimpanan (bawaan 128 MB), kartu panduan awal, dan ekspor ZIP semua file yang diunggah.
- Pengaturan menampilkan tempat kamu masuk, bisa mengeluarkanmu dari semua tempat lain, dan mengelola kunci API, webhook, profil, dan penghapusan akun.
- Masuk dengan email dan kata sandi, Google, atau akun HEL.ink (hub HOP ID), semuanya opsional. Tanpa `HELINK_URL`, PixelHop tidak pernah menghubungi HEL.ink. Cloudflare Turnstile bisa melindungi formulir masuk dan daftar.

Studio:
- Di browser: kompres, ubah ukuran, potong, konversi (JPG, PNG, WebP, dan AVIF jika browser bisa mengodekannya), atur dan putar, buramkan dan pikselkan, tanda air, palet warna, kode QR, teks dari gambar (OCR, 30 bahasa, sampai lima sekaligus), dan hapus latar belakang. Tidak ada yang diunggah sampai kamu menekan Unggah hasil.
- Batch: ubah ukuran, konversi, beri tanda air, atau jalankan langkah AI pada sekumpulan gambar, lalu unduh satu ZIP atau unggah sebagai album.
- Di server, untuk pengguna yang masuk, jika kamu menjalankan `tools/ai-server.py`: hapus latar belakang, buramkan wajah otomatis, penghapus ajaib, dan pembesaran 2x. Satu gambar per kali, dengan batas harian, dan tidak ada yang disimpan.

Untuk pengembang:
- REST API dengan kunci bearer. Kunci bisa dibatasi hanya untuk mengunggah.
- File konfigurasi ShareX yang dibuat di Pengaturan, dan konfigurasi ShareX dari PixelHop 2 tetap berfungsi.
- Webhook untuk `image.uploaded` dan `image.deleted`, ditandatangani dengan HMAC-SHA256 pada header `X-PixelHop-Signature`.
- Pengunggah baris perintah di dalam program yang sama: `pixelhop push *.png --format markdown`.
- [Deskripsi OpenAPI](web/static/openapi.json) di `/static/openapi.json`, serta `/healthz`, `/metrics`, dan `/status` untuk pemantauan.
- Bisa dipasang sebagai aplikasi di ponsel, dengan target berbagi untuk foto, video, dan PDF.

Menjalankannya:
- Satu biner statis, image Docker, atau unit systemd. Simpan file di disk atau di penyimpanan yang kompatibel dengan S3 (AWS S3, Cloudflare R2, MinIO, Backblaze B2, Wasabi, Contabo).
- Unggahan berhenti sendiri saat ruang disk menipis, dan `PIXELHOP_READ_ONLY=true` menghentikannya untuk pemeliharaan. File yang sudah online tetap berfungsi. Lihat [docs/UPLOAD-STORAGE.md](docs/UPLOAD-STORAGE.md).
- Antarmukanya tersedia dalam sembilan bahasa: English, 简体中文, Deutsch, Bahasa Indonesia, 日本語, Português (Brasil), Español, Français, dan Русский. Gunakan `?lang=de` untuk berganti; pilihan disimpan dalam cookie, dan pengaturan bahasa browser tidak dipakai.
- Tautan yang dibagikan bisa berada di host lain daripada aplikasinya (`PIXELHOP_LINK_URL`). Di pixel.hel.ink, aplikasi berada di pixel.hel.ink dan tautan tetap `p.hel.ink/...`.

Keamanan:
- Kata sandi memakai Argon2id. Kunci API dan token sesi disimpan sebagai hash.
- Formulir dilindungi dari kiriman lintas situs, dan Content Security Policy yang ketat dikirim.
- "Unggah dari tautan" memeriksa alamat hasil resolusi untuk setiap koneksi, sehingga tidak bisa diarahkan ke jaringan privat. Gambar yang sangat besar ditolak berdasarkan headernya, sebelum didekode. Unggahan dibatasi lajunya, dan IP klien hanya diambil dari proxy yang kamu daftarkan.
- File yang diturunkan moderator tidak bisa diunggah lagi, karena SHA-256-nya diingat.
- Cara unggahan diproses dan bagaimana server tetap stabil di bawah beban: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). Sejarah di balik versi 3: [AUDIT.md](AUDIT.md). Untuk melaporkan kerentanan, baca [SECURITY.md](SECURITY.md).

Moderasi: halaman admin bawaan dihapus di 3.7.0. Setiap halaman punya tombol Laporkan. Antrean laporan, menyembunyikan dan menghapus konten, blokir IP, aksi akun, pengaturan saat berjalan, dan bilah pengumuman ditangani pusat kendali HOP, yang berbicara dengan PixelHop lewat API `/hop/*` bertanda tangan (HOP App Protocol). Pusat kendali dipakai bersama masuk lewat HEL.ink, ditandatangani dengan `HELINK_SSO_SECRET`, dan pemanggil yang diizinkan tercantum di `HOP_ADMIN_CLIENTS` (bawaan `imperium,helink`). Setiap perubahan yang dibuat dengan cara itu dicatat di tabel `hop_audit`. Pusat kendali itu sendiri bersifat privat milik proyek HOP. Instalasi mandiri punya tombol Laporkan, perintah `pixelhop user` (`add`, `passwd`, `admin`, `merge`), dan hak tambahan peran admin lewat API, tetapi tidak punya layar untuk antrean laporan.

## Pengaturan

Pengaturan berupa variabel lingkungan, semuanya opsional. [`.env.example`](.env.example) mendaftar variabel yang umum beserta nilai bawaannya; sisanya dibaca di [`internal/config/config.go`](internal/config/config.go). Tabel ini berisi yang paling sering diubah. Daftar lengkapnya ada di [README](README.md) berbahasa Inggris.

| Variabel | Bawaan | Fungsi |
|---|---|---|
| `PIXELHOP_BASE_URL` | `http://localhost:8080` | Alamat publik aplikasi (masuk, unggah, dasbor). Pakai `https://` di produksi, yang juga mengaktifkan cookie aman |
| `PIXELHOP_LINK_URL` | `PIXELHOP_BASE_URL` | Host lain untuk tautan yang dibagikan (halaman gambar dan album, file langsung). Harus host yang berbeda dan juga harus menjangkau PixelHop |
| `PIXELHOP_DATA` | `./data` | Database dan, pada penyimpanan lokal, gambar |
| `PIXELHOP_MAX_UPLOAD_MB` | `20` | Per gambar |
| `PIXELHOP_MAX_MEDIA_MB` | `100` | Per video atau PDF, untuk anggota yang masuk |
| `PIXELHOP_GUEST_MAX_MB` | `20` | Per file jenis apa pun, untuk tamu |
| `PIXELHOP_GUEST_UPLOADS` | `true` | Izinkan unggahan tanpa akun |
| `PIXELHOP_GUEST_ALBUM_MAX` | `10` | Gambar dalam album buatan tamu (0 = tanpa batas); juga bisa diatur di pusat kendali |
| `PIXELHOP_GUEST_DAILY_MAX` | `25` | File yang boleh diunggah satu alamat tamu per 24 jam (0 = tanpa batas); juga bisa diatur di pusat kendali |
| `PIXELHOP_GUEST_TTL_DAYS` | `30` | Hari sebuah unggahan tamu baru tetap daring (0 = sampai dihapus); file lama tidak berubah; juga bisa diatur di pusat kendali |
| `PIXELHOP_DEFAULT_QUOTA_MB` | `128` | Penyimpanan untuk setiap akun baru; akun yang sudah ada tetap dengan kuotanya |
| `PIXELHOP_REGISTRATION` | `true` | Izinkan akun baru |
| `PIXELHOP_READ_ONLY` | `false` | Hentikan unggahan untuk pemeliharaan |
| `PIXELHOP_TRUSTED_PROXIES` | - | `cloudflare`, atau IP atau CIDR reverse proxy-mu |
| `PIXELHOP_STORAGE`, `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY` | `local` | `local` atau `s3`; penyimpanan S3 membutuhkan empat lainnya |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | - | Masuk dengan Google |
| `HELINK_URL`, `HELINK_SSO_SECRET` | - | Masuk dengan akun HEL.ink. Rahasianya minimal 32 karakter dan sama dengan `PIXELHOP_SSO_SECRET` di `.env` HEL.ink |
| `PIXELHOP_CONTROL_URL` | - | Alamat pusat kendali HOP; admin yang membuka `/admin` diarahkan ke sana |
| `TURNSTILE_SITE_KEY`, `TURNSTILE_SECRET_KEY` | - | Cloudflare Turnstile pada formulir masuk dan daftar. Saat akun HEL.ink aktif, formulir ini menolak setiap percobaan tanpa kunci tersebut |
| `PIXELHOP_REMBG_URL` | - | Alamat `tools/ai-server.py`, misalnya `http://127.0.0.1:7001` |

Rahasia HEL.ink juga bisa bernama `HOP_SSO_SECRET_PIXELHOP` atau `HOP_SSO_SECRET`. Saat merotasi, rahasia lama tetap diterima lewat `HELINK_SSO_SECRET_PREV` (atau `HOP_SSO_SECRET_PREV`).

### Reverse proxy, layanan, dan cadangan

- [`deploy/nginx.conf`](deploy/nginx.conf) adalah contoh reverse proxy. Setel `PIXELHOP_TRUSTED_PROXIES=127.0.0.1` agar PixelHop memercayai `X-Forwarded-For` dari nginx, dan tambahkan `cloudflare` jika Cloudflare ada di depan.
- [`deploy/pixelhop.service`](deploy/pixelhop.service) adalah unit systemd yang diperkeras. [`deploy/pixelhop-ai.service`](deploy/pixelhop-ai.service) menjalankan layanan AI opsional, yang membutuhkan `rembg`, `opencv-python-headless`, dan `onnxruntime` (lihat bagian atas [`tools/ai-server.py`](tools/ai-server.py)).
- Untuk mencadangkan, salin folder data, atau jalankan `sqlite3 data/pixelhop.db ".backup copy.db"` saat PixelHop berjalan. `./update.sh` membuat salinan seperti itu di `backups/` sebelum setiap pembaruan.

## Cara memakainya

1. Buka situs, lalu seret, tempel (<kbd>Ctrl</kbd>+<kbd>V</kbd>), atau tautkan sebuah gambar.
2. Opsional: pilih Hapus setelah, dan apakah nama file ikut masuk ke tautan.
3. Tautan disalin untukmu. Buka untuk melihat tautan langsung, kode sematan, dan kode QR.

Tekan `?` di halaman mana pun untuk melihat pintasan keyboard.

**ShareX.** Buka Pengaturan, Kunci API, buat kunci, lalu tekan Unduh konfigurasi ShareX dengan kunci ini dan buka filenya.

**Baris perintah.** Program `pixelhop` yang sama mengunggah ke server PixelHop mana pun. `pixelhop login` menyimpan alamat dan kunci di `~/.config/pixelhop/client.json`; `PIXELHOP_SERVER` dan `PIXELHOP_KEY` menimpanya.

```bash
pixelhop login --server https://pixel.hel.ink --key ph_yourkey
pixelhop push screenshot.png photos/*.jpg --expire 7d --format markdown
```

**Ponsel.** Buka situs dan pilih Tambahkan ke Layar Utama. PixelHop lalu muncul di lembar berbagi.

<a id="api"></a>

## API

```bash
curl -H "Authorization: Bearer ph_yourkey" -F "file=@holiday.jpg" https://pixel.hel.ink/api/v1/upload
```

| Endpoint | Fungsi |
|---|---|
| `POST /api/v1/upload` | Unggah file (`file`, bisa berulang) atau tautan (`url`). Opsi: `expire` (`10m`, `1h`, `1d`, `7d`, `30d`, `90d`), `slug` (`0` menjaga nama file tetap di luar tautan), `title`, `alt`, `album_id`, `album` (`force` atau `0`), `listed`, `content_warning` (`sensitive` atau `adult`), `warning_reason` |
| `POST /api/v1/uploads`, `PUT /api/v1/uploads/{id}`, `POST /api/v1/uploads/{id}/finish` | Mengunggah file besar dalam potongan 8 MB |
| `GET /api/v1/images` | Gambarmu (`page`, `limit`, `q`, `sort`) |
| `GET`, `PATCH`, `DELETE /api/v1/images/{id}` | Membaca, mengubah, atau menghapus satu gambar. `PATCH` menerima `title`, `alt`, `listed`, `visibility` (`unlisted`, `private`, atau `password` beserta `password`), `tags`, `content_warning`, dan `warning_reason`. `DELETE` memindahkan file milikmu ke tempat sampah; tambahkan `?permanent=1` untuk menghapusnya permanen |
| `POST /api/v1/images/{id}/restore` | Mengembalikan file dari tempat sampah |
| `GET /api/v1/images/{id}/stats` | Tayangan per hari dan situs perujuk |
| `GET`, `POST /api/v1/albums` / `GET`, `PATCH`, `DELETE /api/v1/albums/{id}` | Mendaftar, membuat, membaca, mengedit, dan menghapus album |
| `GET /api/v1/users/{username}/images` | Publik: gambar di profil seseorang |
| `GET /api/v1/me` | Akunmu dan pemakaian penyimpanan |
| `GET /api/v1/me/media` | Gambar (`type=image`) atau PDF (`type=pdf`) milikmu sendiri yang boleh ditampilkan halaman lain: tanpa berkas privat, berkata sandi, kedaluwarsa, atau berperingatan konten (`cursor`, `limit`). Juga terbuka untuk app HOP yang disebut di `PIXELHOP_MEDIA_CLIENTS` (pemilih media BioHop), ditandatangani server-ke-server untuk satu anggota |
| `GET /api/v1/stats` | Total publik dan versi server |
| `GET /api/v1/upload-status` | Apakah unggahan sedang diterima |
| `POST /api/v1/tools/{remove-background,faces,erase,upscale}` | Alat AI server (harus masuk; `file`, ditambah `mask` untuk erase) |
| `GET /i/{id}.jpg?w=&h=&fit=&format=&q=` | Salinan berukuran lain, dibuat sekali lalu disimpan di cache. `format=webp` dan `avif` membutuhkan layanan AI |
| `GET /oembed?url=...` | oEmbed untuk halaman gambar |

Setiap jawaban berbentuk `{"success": true, "data": ...}` atau `{"success": false, "error": "..."}`. Referensi lengkap ada di [`/docs`](https://pixel.hel.ink/docs) pada server PixelHop mana pun.

<a id="upgrading"></a>

## Naik dari PixelHop 2

PixelHop 2 adalah aplikasi PHP dengan MySQL, venv Python, dan tugas cron. Kodenya tetap ada di tag [`v2.3.0-php`](../../tree/v2.3.0-php). Satu perintah memindahkan datanya:

```bash
pixelhop import --images /var/www/pichost/data/images.json \
  --mysql "user:pass@tcp(127.0.0.1:3306)/pixelhop" --copy
```

- Pengguna mempertahankan id, peran, kuota, dan kata sandinya. Hash argon2id dari PHP diverifikasi dan ditingkatkan saat masuk berikutnya. Akun Google tetap terhubung.
- Gambar mempertahankan id-nya. Dengan `--copy`, gambar diunduh ke penyimpanan PixelHop 3 dengan thumbnail baru; tanpa itu, gambar tetap dilayani dari bucket lama.
- Tautan lama tetap berfungsi: `/{id}`, `/i/2026/08/27/{id}_original.webp`, `/auth/google-callback` (setel `GOOGLE_REDIRECT_URI`), dan konfigurasi ShareX yang mengirim ke `/api/upload.php`.

[AUDIT.md](AUDIT.md) menjelaskan mengapa program ini ditulis ulang.

## Pengembangan

```
cmd/pixelhop/     perintahnya: serve, user, import, push, login
internal/app/     server web: halaman, API, unggahan, moderasi, HOP App Protocol
internal/db/      skema SQLite, migrasi, dan kueri
internal/imaging/ pengenalan jenis, penghapusan metadata, thumbnail
internal/storage/ disk lokal, S3 (penanda tangan SigV4 sendiri), URL lama
internal/netsafe/ pengambilan aman dari SSRF untuk "Unggah dari tautan"
web/              templat, skrip, dan sembilan file bahasa, tertanam di dalam biner
docs/             situs GitHub Pages dan gambar README
scripts/          skrip bantu (gambar README, data changelog, pemeriksaan keluarga HOP)
tools/            layanan AI opsional
deploy/           contoh nginx dan systemd
```

`make test` menjalankan `go vet` dan pengujian, dan `make build` membuat `./pixelhop`. `scripts/readme-images.mjs` merender ulang tangkapan layar README dari instans yang berjalan (lihat komentar di awal file). `python3 scripts/build-changelog.py --repo PixelHop` membangun ulang `web/changelog.json`, data di balik halaman `/changelog`, dari `CHANGELOG.md` (bahasa yang mudah dipahami, satu entri per versi); perintah ini tidak butuh jaringan maupun `gh`, dan `make check-changelog` gagal bila berkasnya tidak mudah dipahami atau tidak terbaru.

## FAQ

<details><summary>Apakah studio benar-benar privat?</summary>

Penyuntingan berlangsung di perangkatmu dengan Canvas dan WebAssembly. Saat pertama kali memakai pengenalan teks atau penghapusan latar belakang, browser mengunduh [tesseract.js](https://github.com/naptha/tesseract.js) dan [@imgly/background-removal](https://github.com/imgly/background-removal-js) dari jsDelivr, serta data bahasa dan modelnya dari tessdata.projectnaptha.com dan staticimgly.com. Gambarmu tidak dikirim ke mana pun kecuali kamu menekan Unggah hasil. Server yang di-host sendiri dan menetapkan Content Security Policy yang lebih ketat perlu mengizinkan alamat-alamat itu.
</details>

<details><summary>Bisakah saya mematikan unggahan tamu atau pendaftaran?</summary>

Bisa. Setel `PIXELHOP_GUEST_UPLOADS=false` atau `PIXELHOP_REGISTRATION=false`. Dengan pusat kendali HOP, keduanya juga bisa diubah saat server berjalan.
</details>

<details><summary>Seberapa besar kebutuhannya?</summary>

Server kecil sudah cukup. Proses `pixelhop` yang baru dimulai memakai sekitar 16 MB memori dalam pengujian di macOS, dan biner rilisnya berukuran 16 sampai 17 MB. Gambar besar didekode beberapa saja sekaligus, sehingga lonjakan unggahan tidak bisa menghabiskan seluruh memori.
</details>

## Hosting sendiri

<!-- self-hosting:start -->
Semua yang dibutuhkan PixelHop ada di repositori publik: tanpa repositori privat, tanpa token, dan tanpa akun hel.ink. Rinciannya ada di panduan keluarga: [docs/self-hosting.md](https://github.com/navi-crwn/HEL.ink/blob/main/docs/self-hosting.md) (bahasa Inggris).

| Komponen | Untuk PixelHop | Fungsinya |
| --- | --- | --- |
| [HEL.ink](https://github.com/navi-crwn/HEL.ink) (hub, HOP ID) | Opsional: menambahkan “Masuk dengan HOP ID” | Akun dan proses masuk untuk semua aplikasi |
| [Imperium](https://github.com/navi-crwn/Imperium) | Opsional, tetapi halaman admin PixelHop ada di sana | Panel admin untuk seluruh keluarga; butuh hub |
| Font dan ubin keluarga | Tidak perlu dipasang: dimuat dari `hel.ink/hop-assets` | Tanpa hel.ink: font sistem, tanpa ubin melayang |

- **Urutan pemasangan**: PixelHop sendiri dengan `./install.sh`. Untuk masuk dengan HOP ID, pasang [hub](https://github.com/navi-crwn/HEL.ink) dulu: `./install.sh --hub new` memasangnya di sebelahnya.
- **Pengaturan minimum**: `PIXELHOP_BASE_URL`; dengan hub: `HELINK_URL`, `HELINK_SSO_SECRET`; dengan Imperium: `PIXELHOP_CONTROL_URL`.
- **Sendiri atau dalam keluarga**: PixelHop berjalan sendiri dengan akunnya sendiri. Dengan hub, orang juga bisa masuk dengan HOP ID dan keluar dari semua aplikasi sekaligus.
- **Tanpa Imperium**: `/admin` hanya menampilkan pemberitahuan. Gunakan `pixelhop user add` dan API bertanda tangan `/hop/*` (akun pertama menjadi admin).
<!-- self-hosting:end -->

## Keluarga HOP

PixelHop adalah bagian dari keluarga HOP, aplikasi open source yang menghargai privasi dan bisa di-host sendiri.

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

## Kredit dan lisensi

- [MIT](LICENSE.md), hak cipta Ivan Novskies (navi-crwn). Komponen pihak ketiga tercantum di [CREDITS.md](CREDITS.md).
- Dibangun dengan [Go](https://go.dev), [modernc.org/sqlite](https://gitlab.com/cznic/sqlite), [golang.org/x/image](https://pkg.go.dev/golang.org/x/image), dan [rsc.io/qr](https://pkg.go.dev/rsc.io/qr).
- Studio memuat [tesseract.js](https://github.com/naptha/tesseract.js) (Apache-2.0) dan [@imgly/background-removal](https://github.com/imgly/background-removal-js) (AGPL-3.0, tanpa modifikasi, dari jsDelivr) saat dibutuhkan.

Karya lain dari penulis yang sama: [Amnesia](https://github.com/navi-crwn/amnesia), Mac yang lupa semuanya saat keluar.
