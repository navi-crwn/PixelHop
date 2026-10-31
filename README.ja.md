<p align="center">🌐 <a href="README.md">English</a> · <a href="README.zh.md">简体中文</a> · <a href="README.es.md">Español</a> · <a href="README.ar.md">العربية</a> · <a href="README.pt.md">Português</a> · <a href="README.fr.md">Français</a> · <b>日本語</b> · <a href="README.id.md">Bahasa Indonesia</a></p>

# PixelHop

<p align="center">
  <a href="LICENSE.md"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License MIT"></a>
  <a href="https://github.com/navi-crwn/PixelHop/actions"><img src="https://img.shields.io/github/actions/workflow/status/navi-crwn/PixelHop/ci.yml?branch=main&style=flat-square&label=tests" alt="Tests"></a>
</p>

<p align="center"><picture><source media="(prefers-color-scheme: light)" srcset="docs/readme/hero-en-light.webp"><img src="docs/readme/hero-en-dark.webp" alt="PixelHop: drop an image, get a link" width="100%"></picture></p>

<p align="center">
  <b>閲覧者に知られる前にGPS位置情報を自動除去するオープンソースの画像ホスティング。ブラウザ完結のフォトスタジオを内蔵。Go言語の軽量単一バイナリ。</b>
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

## PixelHop とは？

### 写真の共有
- 画像をドロップ、ペースト、またはURL指定するだけで短く分かりやすいリンクを発行。
- スマートフォンの写真に含まれるGPS座標やカメラ情報を、ピクセルを再圧縮することなく**自動的に消去**。
- 複数枚を一度にアップロードして**アルバム**を作成。

### 写真の編集
- 内蔵**フォトスタジオ**で圧縮、リサイズ、切り抜き、フォーマット変換が可能。
- 顔のぼかし処理、OCR文字認識、背景除去に対応。
- すべて**お使いのブラウザ内**で実行されるため、アップロードを押すまで画像は端末から送信されません。

### セルフホスト
- データベース内蔵、約16MBの単一実行ファイル。
- サーバーに配置して実行するだけで稼働。PHP、Python、MySQL、cronは不要。

<p align="center"><picture><source media="(prefers-color-scheme: light)" srcset="docs/readme/features-en-light.webp"><img src="docs/readme/features-en-dark.webp" alt="Features" width="100%"></picture></p>

## 主な機能

- **📍 位置情報を除去、画質はそのまま。** EXIF、XMP、IPTCをコンテナ書き換えで消去し、ピクセル再エンコードは行いません。
- **🔗 豊富な共有リンク。** `beach-sunset-x7Kp2Q` のような読みやすいID、直リンク、Markdown、HTML、BBCode、QRコード。
- **🗂️ アルバム機能。** 複数画像を一度にアップロードし、単一のギャラリーリンクとして共有。
- **⏳ 自動消滅。** 10分、1時間、1日、1ヶ月後の自動消滅タイマー。ゲスト用削除キーも発行。
- **🎨 ブラウザ完結フォトスタジオ。** アップロード前に端末内で圧縮、リサイズ、WebP/AVIF変換、モザイク、OCRを実行。
- **👤 単体運用またはHOP ID統合ログイン。** HOPファミリー共通のHOP ID SSOサインイン、またはゲスト利用。
- **🔌 API と ShareX 対応。** ShareX即時設定、REST APIトークン、`pixelhop push` コマンドラインツール。
- **🏠 自宅サーバー運用可能・オープンソース。** 単一Goバイナリ、Docker、systemd。ローカルディスクおよびS3/R2ストレージ。

## インストール

ビルド済みバイナリをダウンロードするか、Dockerで実行します：

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

## HOP ファミリー

PixelHop は、プライバシーを尊重しセルフホスト可能なオープンソースツール群「**HOP**」の一員です。

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

## クレジットとライセンス

- [MIT](LICENSE.md) © Ivan Novskies (navi-crwn).
- [Go](https://go.dev)、SQLite、最新Web技術で開発。
