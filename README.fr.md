<p align="center">🌐 <a href="README.md">English</a> · <a href="README.zh.md">简体中文</a> · <a href="README.es.md">Español</a> · <a href="README.ar.md">العربية</a> · <a href="README.pt.md">Português</a> · <b>Français</b> · <a href="README.ja.md">日本語</a> · <a href="README.id.md">Bahasa Indonesia</a></p>

# PixelHop

<p align="center">
  <a href="LICENSE.md"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License MIT"></a>
  <a href="https://github.com/navi-crwn/PixelHop/actions"><img src="https://img.shields.io/github/actions/workflow/status/navi-crwn/PixelHop/ci.yml?branch=main&style=flat-square&label=tests" alt="Tests"></a>
</p>

<p align="center"><picture><source media="(prefers-color-scheme: light)" srcset="docs/readme/hero-en-light.webp"><img src="docs/readme/hero-en-dark.webp" alt="PixelHop: drop an image, get a link" width="100%"></picture></p>

<p align="center">
  <b>Hébergement d’images open source qui supprime votre localisation GPS avant que quiconque ne la voie, avec studio photo privé dans le navigateur. Un binaire Go unique et léger.</b>
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

## Qu’est-ce que PixelHop ?

### Partager une photo
- Glissez, collez ou liez une image pour obtenir un lien court et lisible.
- Les photos de smartphone contiennent coordonnées GPS et données appareil. PixelHop **retire ces données masquées** sans recomprimer les pixels.
- Téléversez plusieurs fichiers d’un coup pour former un **album**.

### Éditer une photo
- Le **studio** intégré compresse, redimensionne, recadre et convertit les formats.
- Floutez les visages, extrayez du texte (OCR) et supprimez les arrière-plans.
- Tout s’exécute **dans votre navigateur**, l’image restant sur votre appareil.

### Auto-hébergement
- PixelHop est un binaire autonome de ~16 Mo avec base de données intégrée.
- Copiez-le sur votre serveur et lancez-le. Sans PHP, Python, MySQL ni cron.

<p align="center"><picture><source media="(prefers-color-scheme: light)" srcset="docs/readme/features-en-light.webp"><img src="docs/readme/features-en-dark.webp" alt="Features" width="100%"></picture></p>

## Fonctionnalités

- **📍 Localisation retirée, qualité préservée.** Nettoie EXIF, XMP et IPTC sans réencoder les pixels.
- **🔗 Tous les liens utiles.** Identifiants lisibles comme `beach-sunset-x7Kp2Q`, liens directs, Markdown, HTML, BBCode et QR code.
- **🗂️ Albums.** Envoyez plusieurs photos et partagez un lien de galerie unique.
- **⏳ Autodestruction.** Suppression après 10 minutes, 1 heure, 1 jour ou 1 mois. Jeton de suppression privé pour les invités.
- **🎨 Studio photo privé.** Compressez, recadrez, convertissez en WebP/AVIF, floutez les visages et lancez l’OCR localement sans téléversement préalable.
- **👤 Comptes autonomes ou SSO HOP ID.** Connexion unifiée HOP ID pour toutes les applications HOP ou mode invité.
- **🔌 API et ShareX.** Profils ShareX prêts à l’emploi, jetons d’API REST et téléversement en ligne de commande via `pixelhop push`.
- **🏠 Auto-hébergeable et open source.** Binaire Go unique, Docker ou systemd. Stockage disque local ou compartiments S3/R2.

## Installation

Téléchargez le binaire précompilé ou lancez avec Docker :

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

## La famille HOP

PixelHop fait partie de **HOP**, une suite d’applications open source auto-hébergeables respectueuses de la vie privée.

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

## Crédits et licence

- [MIT](LICENSE.md) © Ivan Novskies (navi-crwn).
- Développé avec [Go](https://go.dev), SQLite et les technologies web modernes.
