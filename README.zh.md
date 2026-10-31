<p align="center">🌐 <a href="README.md">English</a> · <b>简体中文</b> · <a href="README.es.md">Español</a> · <a href="README.ar.md">العربية</a> · <a href="README.pt.md">Português</a> · <a href="README.fr.md">Français</a> · <a href="README.ja.md">日本語</a> · <a href="README.id.md">Bahasa Indonesia</a></p>

# PixelHop

<p align="center">
  <a href="LICENSE.md"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License MIT"></a>
  <a href="https://github.com/navi-crwn/PixelHop/actions"><img src="https://img.shields.io/github/actions/workflow/status/navi-crwn/PixelHop/ci.yml?branch=main&style=flat-square&label=tests" alt="Tests"></a>
</p>

<p align="center"><picture><source media="(prefers-color-scheme: light)" srcset="docs/readme/hero-en-light.webp"><img src="docs/readme/hero-en-dark.webp" alt="PixelHop: drop an image, get a link" width="100%"></picture></p>

<p align="center">
  <b>开源图像托管服务，在他人查阅前抹除 GPS 位置与相机信息，内置完全在浏览器运行的图片工坊。单一轻量 Go 二进制程序。</b>
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

## 什么是 PixelHop？

### 图片分享
- 拖入、粘贴或输入链接即可获得短小易读的分享链接。
- 手机照片往往包含精确的 GPS 坐标与相机信息。PixelHop **自动剥离隐藏元数据**，且绝不对像素二次压缩。
- 一次上传多张图片自动归入**相册**。

### 图片编辑
- 内置**工坊**支持压缩、缩放、裁剪和格式转换。
- 支持人脸打码、OCR 文字识别以及背景消除。
- 全部在**本地浏览器**内计算，图片在您主动确认前绝不上传。

### 自行搭建
- PixelHop 是仅约 16 MB 的单一独立可执行文件，内嵌数据库。
- 复制到服务器即可运行，无需 PHP、Python、MySQL 或 cron 定时任务。

<p align="center"><picture><source media="(prefers-color-scheme: light)" srcset="docs/readme/features-en-light.webp"><img src="docs/readme/features-en-dark.webp" alt="Features" width="100%"></picture></p>

## 功能特性

- **📍 抹除位置，保留画质。** 剥离 EXIF、XMP 和 IPTC，绝不重新编码图像像素。
- **🔗 格式齐全的链接。** 友好 ID 如 `beach-sunset-x7Kp2Q`、直链、Markdown、HTML、BBCode 与二维码。
- **🗂️ 图集相册。** 一次性上传多张照片并生成单一图集链接。
- **⏳ 阅后即焚。** 支持设定 10 分钟、1 小时、1 天或 1 个月后自动销毁，游客拥有私密删除凭据。
- **🎨 浏览器私密工坊。** 本地运行压缩、缩放、裁剪、转为 WebP/AVIF、人脸打码及 OCR 文字识别。
- **👤 独立账户或 HOP ID 统一登录。** 支持 HOP 全家族一键登录或游客免登录。
- **🔌 API 与 ShareX。** 提供开箱即用的 ShareX 配置、REST API 令牌与 `pixelhop push` 命令行工具。
- **🏠 易于自建与开源。** 单一 Go 二进制、Docker 或 systemd。支持本地磁盘与 S3/R2 存储。

## 安装部署

下载编译好的二进制文件或使用 Docker 运行：

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

## HOP 家族

PixelHop 是 **HOP** 隐私优先、可私有化部署开源工具家族的一员。

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

## 鸣谢与开源许可

- [MIT](LICENSE.md) © Ivan Novskies (navi-crwn).
- 基于 [Go](https://go.dev)、SQLite 及现代 Web 技术构建。
