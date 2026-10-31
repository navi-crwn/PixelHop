<p align="center">🌐 <a href="README.md">English</a> · <a href="README.zh.md">简体中文</a> · <a href="README.es.md">Español</a> · <a href="README.ar.md">العربية</a> · <a href="README.pt.md">Português</a> · <a href="README.fr.md">Français</a> · <a href="README.ja.md">日本語</a> · <b>Bahasa Indonesia</b></p>

# PixelHop

<p align="center">
  <a href="LICENSE.md"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License MIT"></a>
  <a href="https://github.com/navi-crwn/PixelHop/actions"><img src="https://img.shields.io/github/actions/workflow/status/navi-crwn/PixelHop/ci.yml?branch=main&style=flat-square&label=tests" alt="Tests"></a>
</p>

<p align="center"><picture><source media="(prefers-color-scheme: light)" srcset="docs/readme/hero-en-light.webp"><img src="docs/readme/hero-en-dark.webp" alt="PixelHop: drop an image, get a link" width="100%"></picture></p>

<p align="center">
  <b>Hosting gambar open source yang membersihkan lokasi GPS Anda sebelum dilihat orang lain, dengan studio foto privat langsung di peramban. Satu biner kecil Go.</b>
</p>

<p align="center">
  <a href="https://pixel.hel.ink"><b>🐰 pixel.hel.ink</b></a> ·
  <a href="https://navi-crwn.github.io/PixelHop/">Website</a> ·
  <a href="#install">Install</a> ·
  <a href="#api">API</a> ·
  <a href="AUDIT.md">Audit</a> ·
  <a href="CHANGELOG.md">Changelog</a>
</p>

---

## Apa itu PixelHop?

### Berbagi foto
- Tarik, tempel, atau tautkan gambar dan Anda mendapatkan tautan pendek yang rapi.
- Foto ponsel menyimpan koordinat GPS dan data kamera. PixelHop **menghapus data tersembunyi tersebut** tanpa mengompresi ulang piksel asli.
- Unggah beberapa foto sekaligus menjadi **album**.

### Mengedit foto
- **Studio** bawaan mengompresi, mengubah ukuran, memotong, dan mengonversi format.
- Dapat menyamarkan wajah, mengenali teks (OCR), dan menghapus latar belakang.
- Seluruhnya berjalan **di peramban Anda**, sehingga foto tetap berada di perangkat Anda.

### Jalankan sendiri
- PixelHop adalah satu program mandiri ~16 MB dengan basis data terintegrasi.
- Pasang di server, jalankan, selesai. Tanpa PHP, Python, MySQL, atau cron job.

<p align="center"><picture><source media="(prefers-color-scheme: light)" srcset="docs/readme/features-en-light.webp"><img src="docs/readme/features-en-dark.webp" alt="Features" width="100%"></picture></p>

## Fitur

- **📍 Lokasi dihapus, kualitas terjaga.** EXIF, XMP, dan IPTC dibersihkan tanpa mengompresi ulang piksel.
- **🔗 Tautan lengkap.** ID pendek rapi seperti `beach-sunset-x7Kp2Q`, direct link, Markdown, HTML, BBCode, dan kode QR.
- **🗂️ Album.** Unggah banyak gambar sekaligus dan bagikan satu tautan galeri terpadu.
- **⏳ Hapus otomatis.** Pengaturan self-destruct 10 menit, 1 jam, 1 hari, atau 1 bulan. Tamu mendapatkan token penghapusan privat.
- **🎨 Studio foto privat.** Kompres, pangkas, ubah ukuran, konversi ke WebP/AVIF, blur wajah, dan OCR langsung di peramban tanpa mengunggah.
- **👤 Akun mandiri atau login SSO HOP ID.** Masuk dengan akun HOP ID terpadu untuk semua aplikasi HOP atau gunakan mode tamu.
- **🔌 API & ShareX.** Konfigurasi ShareX siap pakai, token REST API personal, dan perintah CLI `pixelhop push`.
- **🏠 Mandiri dan open source.** Satu biner Go, Docker, atau systemd. Penyimpanan di disk lokal atau bucket S3/R2.

## Pemasangan

Unduh biner siap pakai atau jalankan melalui Docker:

```bash
# Binary (Linux amd64)
curl -L -o pixelhop https://github.com/navi-crwn/PixelHop/releases/latest/download/pixelhop-linux-amd64
chmod +x pixelhop
PIXELHOP_BASE_URL=https://img.example.com ./pixelhop

# Docker
docker run -d --name pixelhop -p 8080:8080 \
  -e PIXELHOP_BASE_URL=https://img.example.com \
  -v pixelhop:/data ghcr.io/navi-crwn/pixelhop:latest
```

## Keluarga HOP

PixelHop adalah bagian dari keluarga aplikasi **HOP** yang menghormati privasi, mandiri, dan open source.

<!-- hop-family:start -->
<!-- Generated from HEL.ink resources/hop-assets/apps.source.json; edit there, then run scripts/hop_assets.py build. -->
| App | Address | Repository | What it does |
| --- | --- | --- | --- |
| **HOP ID** | [hel.ink](https://hel.ink) · accounts at `hub.hel.ink` | [navi-crwn/HEL.ink](https://github.com/navi-crwn/HEL.ink) | One account for every HOP app. |
| **LinkHop** | [link.hel.ink](https://link.hel.ink) · `hel.ink/l/…` | [navi-crwn/LinkHop](https://github.com/navi-crwn/LinkHop) | Short links, QR codes and click analytics. |
| **BioHop** | [bio.hel.ink](https://bio.hel.ink) · `hel.ink/b/…` | [navi-crwn/BioHop](https://github.com/navi-crwn/BioHop) | One link-in-bio page for everything you do. |
| **PixelHop** | [pixel.hel.ink](https://pixel.hel.ink) · `p.hel.ink/…` | [navi-crwn/PixelHop](https://github.com/navi-crwn/PixelHop) | Image hosting with albums and direct links. |
| **NoteHop** | [note.hel.ink](https://note.hel.ink) · `n.hel.ink/…` | [navi-crwn/NoteHop](https://github.com/navi-crwn/NoteHop) | Encrypted notes you share with a link. |
| **Imperium** | private (owner only) | private | The HOP control panel. |
<!-- hop-family:end -->

## Kredit & Lisensi

- [MIT](LICENSE.md) © Ivan Novskies (navi-crwn).
- Dibuat dengan [Go](https://go.dev), SQLite, dan teknologi peramban modern.
