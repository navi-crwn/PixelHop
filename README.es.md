<p align="center">🌐 <a href="README.md">English</a> · <a href="README.zh.md">简体中文</a> · <b>Español</b> · <a href="README.ar.md">العربية</a> · <a href="README.pt.md">Português</a> · <a href="README.fr.md">Français</a> · <a href="README.ja.md">日本語</a> · <a href="README.id.md">Bahasa Indonesia</a></p>

# PixelHop

<p align="center">
  <a href="LICENSE.md"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License MIT"></a>
  <a href="https://github.com/navi-crwn/PixelHop/actions"><img src="https://img.shields.io/github/actions/workflow/status/navi-crwn/PixelHop/ci.yml?branch=main&style=flat-square&label=tests" alt="Tests"></a>
</p>

<p align="center"><picture><source media="(prefers-color-scheme: light)" srcset="docs/readme/hero-en-light.webp"><img src="docs/readme/hero-en-dark.webp" alt="PixelHop: drop an image, get a link" width="100%"></picture></p>

<p align="center">
  <b>Alojamiento de imágenes de código abierto que elimina tu ubicación GPS antes de que nadie la vea, con un taller fotográfico en el navegador. Un binario único en Go.</b>
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

## ¿Qué es PixelHop?

### Compartir fotos
- Suelta, pega o enlaza una imagen para obtener un enlace corto y legible.
- Las fotos del móvil contienen coordenadas GPS y datos de cámara. PixelHop **elimina esos metadatos** sin recomprimir los píxeles.
- Sube varios archivos a la vez para crear un **álbum**.

### Editar fotos
- El **taller** integrado comprime, redimensiona, recorta y convierte formatos.
- Desenfoca rostros, reconoce texto (OCR) y elimina fondos.
- Todo se ejecuta **en tu navegador**, manteniendo la imagen en tu dispositivo.

### Autoalojamiento
- PixelHop es un único programa de ~16 MB con base de datos integrada.
- Cópialo a un servidor, ejecútalo y listo. Sin PHP, Python, MySQL ni tareas cron.

<p align="center"><picture><source media="(prefers-color-scheme: light)" srcset="docs/readme/features-en-light.webp"><img src="docs/readme/features-en-dark.webp" alt="Features" width="100%"></picture></p>

## Características

- **📍 Ubicación eliminada, calidad intacta.** Limpia EXIF, XMP e IPTC sin recodificar los píxeles originales.
- **🔗 Todos los enlaces que necesitas.** Enlaces cortos legibles como `beach-sunset-x7Kp2Q`, enlaces directos, Markdown, HTML, BBCode y código QR.
- **🗂️ Álbumes.** Sube varias fotos a la vez y comparte un solo enlace de galería.
- **⏳ Autodestrucción.** Elimina tras 10 minutos, 1 hora, 1 día o 1 mes. Los invitados obtienen un enlace de borrado privado.
- **🎨 Taller fotográfico en el navegador.** Comprime, recorta, convierte a WebP/AVIF, desenfoca rostros y aplica OCR en local sin subir la imagen.
- **👤 Cuentas propias o inicio de sesión con HOP ID.** Inicio de sesión único con HOP ID o modo invitado.
- **🔌 API y ShareX.** Archivos de configuración para ShareX, API REST con tokens y subidas por CLI con `pixelhop push`.
- **🏠 Autoalojable y código abierto.** Binario único en Go, Docker o systemd. Compatible con disco local o S3/R2.

## Instalación

Descarga el binario o ejecuta mediante Docker:

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

## La familia HOP

PixelHop forma parte de **HOP**, una familia de aplicaciones de código abierto, autoalojables y respetuosas con la privacidad.

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

## Créditos y licencia

- [MIT](LICENSE.md) © Ivan Novskies (navi-crwn).
- Desarrollado con [Go](https://go.dev), SQLite y estándares web modernos.
