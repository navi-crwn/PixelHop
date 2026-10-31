<p align="center">🌐 <a href="README.md">English</a> · <a href="README.zh.md">简体中文</a> · <a href="README.es.md">Español</a> · <a href="README.ar.md">العربية</a> · <b>Português</b> · <a href="README.fr.md">Français</a> · <a href="README.ja.md">日本語</a> · <a href="README.id.md">Bahasa Indonesia</a></p>

# PixelHop

<p align="center">
  <a href="LICENSE.md"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License MIT"></a>
  <a href="https://github.com/navi-crwn/PixelHop/actions"><img src="https://img.shields.io/github/actions/workflow/status/navi-crwn/PixelHop/ci.yml?branch=main&style=flat-square&label=tests" alt="Tests"></a>
</p>

<p align="center"><picture><source media="(prefers-color-scheme: light)" srcset="docs/readme/hero-en-light.webp"><img src="docs/readme/hero-en-dark.webp" alt="PixelHop: drop an image, get a link" width="100%"></picture></p>

<p align="center">
  <b>Hospedagem de imagens de código aberto que remove sua localização GPS antes de qualquer um ver, com estúdio fotográfico no navegador. Um binário compacto em Go.</b>
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

## O que é o PixelHop?

### Compartilhar fotos
- Solte, cole ou vincule uma imagem para obter um link curto e legível.
- Fotos de celular carregam localização GPS e dados da câmera. O PixelHop **remove esses dados ocultos** sem recomprimir os pixels.
- Envie vários arquivos de uma vez para criar um **álbum**.

### Editar fotos
- O **estúdio** integrado comprime, redimensiona, corta e converte formatos.
- Desfoca rostos, reconhece texto (OCR) e remove fundos.
- Tudo é executado **no seu navegador**, mantendo a foto no seu dispositivo.

### Auto-hospedagem
- O PixelHop é um único executável de ~16 MB com banco de dados integrado.
- Copie para o servidor, inicie e pronto. Sem PHP, Python, MySQL ou tarefas cron.

<p align="center"><picture><source media="(prefers-color-scheme: light)" srcset="docs/readme/features-en-light.webp"><img src="docs/readme/features-en-dark.webp" alt="Features" width="100%"></picture></p>

## Recursos

- **📍 Localização removida, qualidade mantida.** Limpa EXIF, XMP e IPTC sem recodificar os pixels originais.
- **🔗 Todos os links necessários.** IDs amigáveis como `beach-sunset-x7Kp2Q`, links diretos, Markdown, HTML, BBCode e código QR.
- **🗂️ Álbuns.** Envie várias imagens e compartilhe um único link de galeria.
- **⏳ Autodestruição.** Exclua após 10 minutos, 1 hora, 1 dia ou 1 mês. Visitantes recebem tokens de exclusão.
- **🎨 Estúdio fotográfico no navegador.** Comprima, corte, converta para WebP/AVIF, desfoque rostos e extraia texto via OCR sem fazer upload.
- **👤 Contas próprias ou login unificado HOP ID.** Acesse com a conta única HOP ID em todos os apps HOP ou use como visitante.
- **🔌 API e ShareX.** Configurações prontas para ShareX, API REST com tokens e uploads CLI via `pixelhop push`.
- **🏠 Auto-hospedável e open source.** Binário único em Go, Docker ou systemd. Suporte a disco local ou S3/R2.

## Instalação

Baixe o binário compilado ou execute via Docker:

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

## A família HOP

O PixelHop faz parte do **HOP**, uma família de aplicativos de código aberto focados em privacidade e auto-hospedagem.

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

## Créditos e licença

- [MIT](LICENSE.md) © Ivan Novskies (navi-crwn).
- Desenvolvido com [Go](https://go.dev), SQLite e tecnologias web modernas.
