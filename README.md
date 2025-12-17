# 🐰 PixelHop

<div align="center">

<img src="https://p.hel.ink/assets/img/logo.svg" alt="PixelHop" width="120">

**Free image hosting + powerful editing tools. No BS, just works.**

Built with PHP 8 • Self-hostable • Forever free

[Live Demo](https://p.hel.ink) • [API Docs](https://p.hel.ink/docs) • [Report Bug](https://github.com/navi-crwn/pixelhop/issues)

![PHP](https://img.shields.io/badge/PHP-8.1+-777BB4?style=flat-square&logo=php&logoColor=white)
![TailwindCSS](https://img.shields.io/badge/Tailwind-3.x-38B2AC?style=flat-square&logo=tailwind-css&logoColor=white)
![MySQL](https://img.shields.io/badge/MySQL-5.7+-4479A1?style=flat-square&logo=mysql&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)

![Stars](https://img.shields.io/github/stars/navi-crwn/pixelhop?style=flat-square)
![Forks](https://img.shields.io/github/forks/navi-crwn/pixelhop?style=flat-square)
![Issues](https://img.shields.io/github/issues/navi-crwn/pixelhop?style=flat-square)

</div>

---

## Apa sih PixelHop?

PixelHop itu image hosting + image tools dalam satu platform. Upload gambar, dapet link permanen. Mau compress? resize? crop? convert format? Bisa semua. Ada juga fitur AI buat extract text (OCR) sama hapus background otomatis.

Gak ribet, gak perlu install software macem-macem. Tinggal buka browser, drag drop, selesai. ✨

## Highlights

- **Image Hosting** — Upload, dapet link, share. Simpel.
- **Compress** — Kecilin file size tanpa rusak kualitas
- **Resize & Crop** — Ubah ukuran, potong sesuai ratio (1:1, 4:3, 16:9, custom)
- **Convert** — Pindah format: JPEG ↔ PNG ↔ WebP ↔ GIF ↔ BMP
- **OCR** — Extract text dari gambar pake Tesseract
- **Remove Background** — AI-powered, hasil bersih dalam hitungan detik
- **User Accounts** — Login pake Google, ada dashboard pribadi
- **Admin Panel** — Full control, monitoring, abuse prevention
- **Self-hostable** — Deploy di server sendiri, data lu tetep di lu

## User System

Sekarang ada sistem user buat tracking upload & akses fitur AI:

| Tier | Storage | OCR/day | RemBG/day |
|------|---------|---------|-----------|
| Guest | - | - | - |
| Free | 500MB | 5x | 3x |
| Premium | 🚧 Coming Soon | 🚧 | 🚧 |

> **Note:** Premium tier masih dalam pengembangan. Untuk sekarang, semua user bisa pake fitur Free tier.

## Tech Stack

```
Backend     : PHP 8.1+, MySQL/MariaDB
Frontend    : TailwindCSS, Vanilla JS, Lucide Icons
AI/Python   : Tesseract OCR, rembg (background removal)
Storage     : S3-compatible (Contabo, AWS, MinIO, dll)
Auth        : Google OAuth 2.0, Cloudflare Turnstile
```

## Quick Start

```bash
# Clone repo
git clone https://github.com/navi-crwn/pixelhop.git
cd pixelhop

# Copy config files
cp config/database.example.php config/database.php
cp config/oauth.example.php config/oauth.php
cp config/s3.example.php config/s3.php

# Import database
mysql -u your_user -p your_database < database/schema.sql

# Setup Python (untuk OCR & RemBG)
cd python && python3 -m venv venv
source venv/bin/activate
pip install -r ../requirements.txt

# Set permissions
chmod 755 temp/ data/
```

Abis itu tinggal point web server ke folder project, edit config sesuai credentials lu, done! 🎉

## API

Semua tools bisa diakses via API. Cocok buat integrasi ShareX, custom scripts, atau automasi lainnya.

| Endpoint | Method | Fungsi |
|----------|--------|--------|
| `/api/upload.php` | POST | Upload gambar |
| `/api/compress.php` | POST | Compress |
| `/api/resize.php` | POST | Resize |
| `/api/crop.php` | POST | Crop |
| `/api/convert.php` | POST | Convert format |
| `/api/ocr.php` | POST | Extract text |
| `/api/rembg.php` | POST | Hapus background |

## Project Structure

```
pixelhop/
├── admin/          # Admin panel
├── api/            # API endpoints
├── assets/         # CSS, JS, images
├── auth/           # Login, OAuth, middleware
├── config/         # Konfigurasi (database, S3, OAuth)
├── core/           # Core classes (Gatekeeper, AbuseGuard)
├── cron/           # Scheduled tasks
├── includes/       # Libraries (Database, ImageHandler, dll)
├── member/         # Member area
├── python/         # Python scripts (OCR, RemBG)
└── temp/           # Temporary files
```

## Cron Jobs

Tambah ke crontab buat auto maintenance:

```bash
# Tiap jam: cleanup temp files, abuse watchdog
0 * * * * php /path/to/pixelhop/cron/maintenance.php >> /var/log/pixelhop.log 2>&1
```

## Requirements

- PHP 8.1+ dengan extensions: pdo_mysql, gd/imagick, curl, json, mbstring
- MySQL 5.7+ atau MariaDB 10.3+
- Python 3.10+ (buat OCR & RemBG)
- S3-compatible storage
- Nginx/Apache

## License

MIT License - bebas dipake, dimodif, didistribusi. Lihat [LICENSE.md](LICENSE.md) buat detail lengkap.

## Credits & Thanks

Lihat [LICENSE.md](LICENSE.md) buat list lengkap library, tools, dan inspirasi yang dipake.

---

<div align="center">

Made with ☕ by [navi-crwn](https://github.com/navi-crwn)

Part of the [HEL.ink](https://hel.ink) family

</div>
