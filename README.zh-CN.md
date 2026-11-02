<p align="center">🌐 <a href="README.md">English</a> · <b>简体中文</b> · <a href="README.de.md">Deutsch</a> · <a href="README.id.md">Bahasa Indonesia</a> · <a href="README.ja.md">日本語</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.es.md">Español</a> · <a href="README.fr.md">Français</a> · <a href="README.ru.md">Русский</a></p>

# PixelHop

开源图片托管，一个 Go 程序即可运行。它会在别人看到照片之前，去掉照片里的 GPS 位置和相机信息。内置的照片工坊在浏览器里运行，所以照片在你上传之前一直留在你的设备上。

线上地址：[pixel.hel.ink](https://pixel.hel.ink)。短链接使用 [p.hel.ink](https://p.hel.ink)。

<p align="center">
  <a href="https://pixel.hel.ink"><b>试用 pixel.hel.ink</b></a> ·
  <a href="https://navi-crwn.github.io/PixelHop/">网站</a> ·
  <a href="#install">安装</a> ·
  <a href="#api">API</a> ·
  <a href="#upgrading">从 v2 升级</a> ·
  <a href="CHANGELOG.md">更新日志</a> ·
  <a href="SECURITY.md">安全</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.27-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.27"/>
  <img src="https://img.shields.io/badge/SQLite-built_in-003B57?style=flat-square&logo=sqlite&logoColor=white" alt="SQLite"/>
  <img src="https://img.shields.io/badge/storage-disk_|_S3-a3e635?style=flat-square" alt="Disk or S3"/>
  <img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" alt="MIT License"/>
  <img src="https://img.shields.io/github/actions/workflow/status/navi-crwn/pixelhop/ci.yml?style=flat-square&label=tests" alt="Tests"/>
</p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/home-dark.webp"><img src="docs/readme/home-light.webp" alt="PixelHop 上传页：访客上传了一张照片，链接已生成，并提示已去掉 GPS 位置、相机信息和拍摄日期" width="100%"></picture></p>

## 它能做什么

- 通过拖放、粘贴、选择文件或链接接收图片、视频和 PDF，并为每个文件生成易读的短链接，例如 `beach-sunset-x7Kp2Q`。
- 去掉照片和视频里的隐藏元数据（各文件类型的具体处理见[删除了什么](#what-gets-removed)）。
- 把多个文件合成一个相册，只需一个链接。
- 内置照片工坊，可在浏览器里压缩、缩放、裁剪、转换格式、打码、识别文字和去除背景。
- 单个静态程序，内置 SQLite，不需要 PHP、Python、MySQL 或 cron。图片存放在本地磁盘或任何兼容 S3 的存储里。

适合在聊天、论坛或 GitHub 上分享截图和照片的人，想要一个自己掌控的 [ShareX](https://getsharex.com) 上传目标的人，以及在意照片会泄露什么信息的人。

## 截图

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/image-dark.webp"><img src="docs/readme/image-light.webp" alt="图片页：照片、直链、Markdown、HTML 和 BBCode 的复制按钮、文件信息和二维码" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/dashboard-dark.webp"><img src="docs/readme/dashboard-light.webp" alt="会员仪表盘：带 HOP 家族的侧边栏、快捷入口，以及带标签筛选的文件网格" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/album-dark.webp"><img src="docs/readme/album-light.webp" alt="相册页：四张照片，以及复制相册链接、嵌入代码和某一种格式的全部链接的按钮" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/studio-dark.webp"><img src="docs/readme/studio-light.webp" alt="照片工坊中打开的压缩工具：格式、质量、最长边和压缩后的文件大小" width="100%"></picture></p>

这些截图来自本地演示实例，图片均为生成的画面。[`scripts/readme-images.mjs`](scripts/readme-images.mjs) 可以重新生成它们。

<a id="install"></a>

## 安装

安装脚本会检查你的电脑，问几个问题，然后从源码构建 PixelHop。需要 Go 1.27 或更高版本，或者 Docker。

```bash
curl -fsSL https://raw.githubusercontent.com/navi-crwn/PixelHop/main/install.sh | bash
```

已克隆仓库时运行 `./install.sh`。之后用 `./update.sh` 拉取新版本、把数据库复制到 `backups/` 并重新构建；`./uninstall.sh` 会删除程序，但保留你的数据，除非加上 `--purge`。每个脚本都支持 `--yes`（接受默认值）、`--dry-run`（只显示步骤，不执行）和 `--help`。安装脚本可以让 PixelHop 在前台运行，也可以生成 systemd 单元或 Docker Compose 文件；如果你愿意，还可以在旁边装一个 [HOP ID](https://github.com/navi-crwn/HEL.ink) 中心。

也可以手动安装。

**二进制文件**（Linux、macOS 和 Windows，amd64 和 arm64，见 [Releases](../../releases) 页面）：

```bash
curl -L -o pixelhop https://github.com/navi-crwn/pixelhop/releases/latest/download/pixelhop-linux-amd64
chmod +x pixelhop
PIXELHOP_BASE_URL=https://img.example.com ./pixelhop
```

**Docker：**

```bash
docker run -d --name pixelhop -p 8080:8080 \
  -e PIXELHOP_BASE_URL=https://img.example.com \
  -v pixelhop:/data ghcr.io/navi-crwn/pixelhop:latest
```

镜像以非 root 用户 65532 运行，所以数据卷必须能被该用户写入。仓库里还有 [`docker-compose.yml`](docker-compose.yml)，它读取 `.env` 文件（复制 `.env.example` 即可），数据保存在 `./data`。

**从源码构建**（Go 1.27 或更高版本）：

```bash
git clone https://github.com/navi-crwn/pixelhop.git && cd pixelhop
make build && ./pixelhop
```

打开 `http://localhost:8080` 并创建账户。第一个账户会成为管理员。也可以在命令行创建：`pixelhop user add you@example.com --admin`。

服务器上装有 `ffmpeg`、`ffprobe` 和 Ghostscript（`gs`）时，才会处理视频和 PDF；没有它们，这些文件按上传时的原样保存。Docker 镜像里一个都没有，所以如果要托管视频或 PDF，请在宿主机上安装，或者自己构建镜像。HEIC 和 AVIF 照片、缩放链接输出 WebP 和 AVIF，以及服务器端的工坊工具，都需要可选的 [`tools/ai-server.py`](tools/ai-server.py)。

<a id="what-gets-removed"></a>

## 删除了什么

PixelHop 只处理文件结构，压缩后的图像数据原样复制，所以画质不变。

| 文件 | 处理方式 |
|---|---|
| JPEG | 删除 EXIF（GPS、相机品牌和型号、序列号、拍摄日期）、XMP、IPTC 和注释；色彩配置文件和其他段保留。如果 EXIF 标明照片被旋转过，照片会被转正，并以 JPEG 质量 92 重新保存，且不带色彩配置文件。这是唯一会重新编码像素的情况。 |
| PNG | 删除文本块、EXIF 块和修改时间；色彩配置文件保留。 |
| WebP | 从扩展格式文件中删除 EXIF 和 XMP。简单的 WebP 文件本身不含元数据。 |
| GIF | 按上传时的原样保存，不删除任何内容。 |
| BMP | 转换为 PNG。 |
| HEIC, AVIF | 设置好 AI 服务时，转换为转正后的、不含元数据的 JPEG 或 PNG；否则上传会被拒绝。 |
| 视频（MP4、MOV、3GP、WebM、MKV） | 装有 `ffmpeg` 时，不重新编码就删除位置、日期、相机和编码器标签，只保留第一条视频轨和第一条音轨。随后后台任务会把视频压成 H.264 和 AAC，长边最多 1280 像素，最高 30 帧每秒。没有 `ffmpeg` 时，文件按上传时的原样保存。 |
| PDF | 装有 Ghostscript 时，文件会被重写以减小体积，如果至少小 10%，就保留较小的版本。标题、作者等文档属性不会被删除。没有 Ghostscript 时，文件按上传时的原样保存。 |

上传完成后，页面会说明删除了哪几类数据。检查按类型进行（GPS 位置、相机信息、拍摄日期、编辑软件、XMP、IPTC、注释、内嵌文本），不会读出具体的值。

## 功能

分享：
- 每个文件都有直链、页面链接、Markdown、HTML、BBCode 和二维码，并带有 Open Graph 标签，所以在 Discord、X 和 WhatsApp 里能正常预览。
- 多个文件可合成相册，标题、描述、封面和顺序之后都能修改。相册可以用 iframe 嵌入其他网站（`/a/{id}/embed`）。
- 一键复制相册或批量上传的全部链接，可选一种格式。
- 可设置在 10 分钟、1 小时、1 天、7 天、30 天或 90 天后过期。
- 每个文件可以是不公开列出（默认）、仅自己可见或设密码保护。文件可以带内容警告（敏感或成人），访客需先确认才能查看。所有者可以禁止其他网站嵌入自己的图片。
- 标签、搜索，以及仪表盘上的标签筛选。删除的文件会在回收站里保留 7 天。
- 每日浏览量和链到该文件的网站。来源只保存主机名。
- 即时缩放，例如 `/i/photo.jpg?w=800`，生成一次后缓存。
- 位于 `/u/username` 的公开主页、公开展示文件的“探索”页、oEmbed 和站点地图。

账户：
- 访客可以上传（默认每个文件最大 20 MB），每个文件都有一个私密删除链接，浏览器也会记住它。
- 会员有图库（搜索、排序、重命名和批量操作）、存储配额（默认 1 GB）、新手引导卡片，以及把自己上传的所有文件导出为 ZIP 的功能。
- 设置页会列出你登录过的设备，可以退出其他所有设备，还能管理 API 密钥、webhook、个人资料和账户删除。
- 可用邮箱和密码、Google 或 HEL.ink 账户（HOP ID 中心）登录，均为可选。没设置 `HELINK_URL` 时，PixelHop 不会联系 HEL.ink。登录和注册表单可以用 Cloudflare Turnstile 保护。

工坊：
- 在浏览器里：压缩、缩放、裁剪、转换（JPG、PNG、WebP，浏览器能编码时还有 AVIF）、调整与旋转、模糊与马赛克、水印、调色板、二维码、图片转文字（OCR，30 种语言，最多同时 5 种）和去除背景。按下“上传结果”之前不会上传任何内容。
- 批处理：对一组图片统一缩放、转换、加水印或运行 AI 步骤，然后下载一个 ZIP，或上传成一个相册。
- 在服务器上，面向已登录用户，需要运行 `tools/ai-server.py`：去除背景、自动人脸模糊、魔术橡皮擦和 2 倍放大。一次处理一张图，有每日上限，不保存任何内容。

开发者工具：
- 使用 Bearer 密钥的 REST API。密钥可以限制为只能上传。
- 在设置里生成 ShareX 配置文件；PixelHop 2 的 ShareX 配置仍然可用。
- `image.uploaded` 和 `image.deleted` 的 webhook，用 HMAC-SHA256 签名，放在 `X-PixelHop-Signature` 头里。
- 同一个程序里自带命令行上传工具：`pixelhop push *.png --format markdown`。
- [OpenAPI 描述](web/static/openapi.json)（位于 `/static/openapi.json`），以及用于监控的 `/healthz`、`/metrics` 和 `/status`。
- 可以作为应用安装到手机上，并支持接收照片、视频和 PDF 的分享。

运行：
- 单个静态程序、Docker 镜像或 systemd 单元。文件可存放在磁盘或兼容 S3 的存储里（AWS S3、Cloudflare R2、MinIO、Backblaze B2、Wasabi、Contabo）。
- 磁盘剩余空间不足时，上传会自动暂停；`PIXELHOP_READ_ONLY=true` 可在维护期间暂停上传。已经上线的文件继续可用。详见 [docs/UPLOAD-STORAGE.md](docs/UPLOAD-STORAGE.md)。
- 界面有九种语言：English、简体中文、Deutsch、Bahasa Indonesia、日本語、Português (Brasil)、Español、Français 和 Русский。用 `?lang=de` 切换；选择会保存在 cookie 里，不会使用浏览器的语言设置。
- 分享链接可以放在与应用不同的域名上（`PIXELHOP_LINK_URL`）。在 pixel.hel.ink 上，应用位于 pixel.hel.ink，链接仍是 `p.hel.ink/...`。

安全：
- 密码使用 Argon2id。API 密钥和会话令牌以哈希形式保存。
- 表单有跨站提交保护，并发送严格的内容安全策略（CSP）。
- “从链接上传”会检查每个连接解析出的地址，所以无法被指向内网。过大的图片根据文件头直接拒绝，不会先解码。上传有速率限制，客户端 IP 只会取自你列出的代理。
- 被管理员下架的文件无法再次上传，因为系统记住了它们的 SHA-256。
- 上传如何处理，以及服务器如何在高负载下保持稳定：[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)。第 3 版背后的历史：[AUDIT.md](AUDIT.md)。报告漏洞请阅读 [SECURITY.md](SECURITY.md)。

审核：内置的管理页面已在 3.7.0 中移除。每个页面都有“举报”按钮。举报队列、隐藏和删除内容、IP 封禁、账户操作、运行时设置和公告栏都由 HOP 控制中心负责，它通过带签名的 `/hop/*` API（HOP 应用协议）与 PixelHop 通信。它需配合 HEL.ink 登录使用，用 `HELINK_SSO_SECRET` 签名，允许的调用方列在 `HOP_ADMIN_CLIENTS` 中（默认 `imperium,helink`）。以这种方式做的每项改动都会写入 `hop_audit` 表。控制中心本身是 HOP 项目的私有组件。独立安装有“举报”按钮、`pixelhop user` 命令（`add`、`passwd`、`admin`、`merge`）和通过 API 使用的管理员额外权限，但没有查看举报队列的界面。

## 设置

设置通过环境变量完成，全部可选。[`.env.example`](.env.example) 列出了常用的变量及其默认值，其余变量见 [`internal/config/config.go`](internal/config/config.go)。下表是最常改动的一部分；完整列表见英文 [README](README.md)。

| 变量 | 默认值 | 作用 |
|---|---|---|
| `PIXELHOP_BASE_URL` | `http://localhost:8080` | 应用的公开地址（登录、上传、仪表盘）。生产环境请用 `https://`，这也会启用安全 cookie |
| `PIXELHOP_LINK_URL` | `PIXELHOP_BASE_URL` | 分享链接使用的另一个域名（图片页、相册页、直链文件）。必须是不同的域名，并且也要能访问到 PixelHop |
| `PIXELHOP_DATA` | `./data` | 数据库，以及本地存储时的图片 |
| `PIXELHOP_MAX_UPLOAD_MB` | `20` | 每张图片的上限 |
| `PIXELHOP_MAX_MEDIA_MB` | `100` | 登录会员每个视频或 PDF 的上限 |
| `PIXELHOP_GUEST_MAX_MB` | `20` | 访客每个文件（任何类型）的上限 |
| `PIXELHOP_GUEST_UPLOADS` | `true` | 允许不登录就上传 |
| `PIXELHOP_REGISTRATION` | `true` | 允许注册新账户 |
| `PIXELHOP_READ_ONLY` | `false` | 维护期间暂停上传 |
| `PIXELHOP_TRUSTED_PROXIES` | - | `cloudflare`，或你的反向代理的 IP 或 CIDR |
| `PIXELHOP_STORAGE`, `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY` | `local` | `local` 或 `s3`；使用 S3 存储时需要其余四项 |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | - | 使用 Google 登录 |
| `HELINK_URL`, `HELINK_SSO_SECRET` | - | 使用 HEL.ink 账户登录。密钥至少 32 个字符，且与 HEL.ink 的 `.env` 里的 `PIXELHOP_SSO_SECRET` 相同 |
| `PIXELHOP_CONTROL_URL` | - | HOP 控制中心的地址；打开 `/admin` 的管理员会被转到那里 |
| `TURNSTILE_SITE_KEY`, `TURNSTILE_SECRET_KEY` | - | 在登录和注册表单上启用 Cloudflare Turnstile。启用 HEL.ink 账户后，没有这两个密钥，这些表单会拒绝所有请求 |
| `PIXELHOP_REMBG_URL` | - | `tools/ai-server.py` 的地址，例如 `http://127.0.0.1:7001` |

HEL.ink 密钥也可以叫 `HOP_SSO_SECRET_PIXELHOP` 或 `HOP_SSO_SECRET`；轮换密钥时，旧密钥用 `HELINK_SSO_SECRET_PREV`（或 `HOP_SSO_SECRET_PREV`）。

### 反向代理、服务与备份

- [`deploy/nginx.conf`](deploy/nginx.conf) 是反向代理示例。设置 `PIXELHOP_TRUSTED_PROXIES=127.0.0.1`，让 PixelHop 信任 nginx 的 `X-Forwarded-For`；如果前面还有 Cloudflare，再加上 `cloudflare`。
- [`deploy/pixelhop.service`](deploy/pixelhop.service) 是加固过的 systemd 单元。[`deploy/pixelhop-ai.service`](deploy/pixelhop-ai.service) 运行可选的 AI 服务，它需要 `rembg`、`opencv-python-headless` 和 `onnxruntime`（见 [`tools/ai-server.py`](tools/ai-server.py) 开头的说明）。
- 备份时复制数据文件夹，或在 PixelHop 运行时执行 `sqlite3 data/pixelhop.db ".backup copy.db"`。`./update.sh` 在每次更新前都会在 `backups/` 里做这样一份副本。

## 使用方法

1. 打开网站，然后拖放、粘贴（<kbd>Ctrl</kbd>+<kbd>V</kbd>）或输入图片链接。
2. 可选：选择“多久后删除”，以及文件名是否放进链接里。
3. 链接会自动复制。打开它可以看到直链、嵌入代码和二维码。

在任何页面按 `?` 可以查看键盘快捷键。

**ShareX。** 进入设置，API 密钥，创建一个密钥，然后点击“下载带此密钥的 ShareX 配置”，打开下载的文件。

**命令行。** 同一个 `pixelhop` 程序可以向任何 PixelHop 服务器上传。`pixelhop login` 会把地址和密钥保存到 `~/.config/pixelhop/client.json`；环境变量 `PIXELHOP_SERVER` 和 `PIXELHOP_KEY` 优先。

```bash
pixelhop login --server https://pixel.hel.ink --key ph_yourkey
pixelhop push screenshot.png photos/*.jpg --expire 7d --format markdown
```

**手机。** 打开网站并选择“添加到主屏幕”，之后 PixelHop 会出现在分享菜单里。

<a id="api"></a>

## API

```bash
curl -H "Authorization: Bearer ph_yourkey" -F "file=@holiday.jpg" https://pixel.hel.ink/api/v1/upload
```

| 接口 | 作用 |
|---|---|
| `POST /api/v1/upload` | 上传文件（`file`，可重复）或链接（`url`）。选项：`expire`（`10m`、`1h`、`1d`、`7d`、`30d`、`90d`）、`slug`（`0` 表示链接里不带文件名）、`title`、`alt`、`album_id`、`album`（`force` 或 `0`）、`listed`、`content_warning`（`sensitive` 或 `adult`）、`warning_reason` |
| `POST /api/v1/uploads`, `PUT /api/v1/uploads/{id}`, `POST /api/v1/uploads/{id}/finish` | 把大文件分成 8 MB 的块上传 |
| `GET /api/v1/images` | 你的图片（`page`、`limit`、`q`、`sort`） |
| `GET`, `PATCH`, `DELETE /api/v1/images/{id}` | 读取、修改或删除单张图片。`PATCH` 接受 `title`、`alt`、`listed`、`visibility`（`unlisted`、`private`，或 `password` 加 `password`）、`tags`、`content_warning` 和 `warning_reason`。`DELETE` 会把你自己的文件移入回收站；加上 `?permanent=1` 则永久删除 |
| `POST /api/v1/images/{id}/restore` | 把文件从回收站恢复 |
| `GET /api/v1/images/{id}/stats` | 每日浏览量和来源网站 |
| `GET`, `POST /api/v1/albums` / `GET`, `PATCH`, `DELETE /api/v1/albums/{id}` | 列出、创建、读取、编辑和删除相册 |
| `GET /api/v1/users/{username}/images` | 公开接口：某人个人主页上的图片 |
| `GET /api/v1/me` | 你的账户和存储用量 |
| `GET /api/v1/stats` | 公开的统计总数和服务器版本 |
| `GET /api/v1/upload-status` | 当前是否接受上传 |
| `POST /api/v1/tools/{remove-background,faces,erase,upscale}` | 服务器 AI 工具（需登录；`file`，擦除还需要 `mask`） |
| `GET /i/{id}.jpg?w=&h=&fit=&format=&q=` | 缩放后的副本，生成一次后缓存。`format=webp` 和 `avif` 需要 AI 服务 |
| `GET /oembed?url=...` | 图片页的 oEmbed |
| `GET /tools/qr.png`, `/tools/qr.svg` | `text` 的二维码（`size`、`level`、`fg`、`bg`） |

每个响应的形式都是 `{"success": true, "data": ...}` 或 `{"success": false, "error": "..."}`。完整参考见任何 PixelHop 服务器上的 [`/docs`](https://pixel.hel.ink/docs)。

<a id="upgrading"></a>

## 从 PixelHop 2 升级

PixelHop 2 是使用 MySQL、Python 虚拟环境和 cron 任务的 PHP 应用，代码保留在 [`v2.3.0-php`](../../tree/v2.3.0-php) 标签里。一条命令就能迁移数据：

```bash
pixelhop import --images /var/www/pichost/data/images.json \
  --mysql "user:pass@tcp(127.0.0.1:3306)/pixelhop" --copy
```

- 用户保留原来的 id、角色、配额和密码。PHP 的 argon2id 哈希会在下次登录时验证并升级。Google 账户保持关联。
- 图片保留原来的 id。加上 `--copy` 时，它们会被下载到 PixelHop 3 的存储里并重新生成缩略图；不加则继续从旧存储桶提供。
- 旧链接继续有效：`/{id}`、`/i/2026/08/27/{id}_original.webp`、`/auth/google-callback`（设置 `GOOGLE_REDIRECT_URI`），以及向 `/api/upload.php` 提交的 ShareX 配置。

重写的原因见 [AUDIT.md](AUDIT.md)。

## 开发

```
cmd/pixelhop/     命令入口：serve、user、import、push、login
internal/app/     Web 服务：页面、API、上传、审核、HOP 应用协议
internal/db/      SQLite 表结构、迁移和查询
internal/imaging/ 类型识别、元数据删除、缩略图
internal/storage/ 本地磁盘、S3（自带 SigV4 签名）、旧版 URL
internal/netsafe/ “从链接上传”的防 SSRF 抓取
web/              模板、脚本和九种语言文件，嵌入程序中
docs/             GitHub Pages 网站和 README 图片
scripts/          辅助脚本（README 图片、更新日志数据、HOP 家族检查）
tools/            可选的 AI 服务
deploy/           nginx 和 systemd 示例
```

`make test` 运行 `go vet` 和测试，`make build` 生成 `./pixelhop`。`scripts/readme-images.mjs` 根据运行中的实例重新生成 README 截图（见文件开头的说明）。`python3 scripts/build-changelog.py --repo PixelHop` 根据已合并的 pull request 重新生成 `web/changelog.json`，也就是 `/changelog` 页面的数据；它需要已登录的 `gh`。

## 常见问题

<details><summary>工坊真的是私密的吗？</summary>

编辑在你的设备上用 Canvas 和 WebAssembly 完成。第一次使用文字识别或去除背景时，浏览器会从 jsDelivr 下载 [tesseract.js](https://github.com/naptha/tesseract.js) 和 [@imgly/background-removal](https://github.com/imgly/background-removal-js)，并从 tessdata.projectnaptha.com 和 staticimgly.com 下载语言数据和模型。除非你按下“上传结果”，图片不会发送到任何地方。如果自托管的服务器设置了更严格的内容安全策略，需要放行这些地址。
</details>

<details><summary>可以关闭访客上传或注册吗？</summary>

可以。设置 `PIXELHOP_GUEST_UPLOADS=false` 或 `PIXELHOP_REGISTRATION=false`。使用 HOP 控制中心时，也可以在服务器运行期间修改这两项。
</details>

<details><summary>需要多少资源？</summary>

一台小服务器就够了。在 macOS 上的一次测试中，刚启动的 `pixelhop` 进程约占用 16 MB 内存，发布的二进制文件为 16 到 17 MB。大图一次只解码几张，所以突发的上传不会耗尽内存。
</details>

## HOP 家族

PixelHop 属于 HOP 家族：注重隐私、可自托管的开源应用。

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

## 致谢与许可证

- [MIT](LICENSE.md)，版权归 Ivan Novskies (navi-crwn) 所有。第三方组件列在 [CREDITS.md](CREDITS.md) 中。
- 基于 [Go](https://go.dev)、[modernc.org/sqlite](https://gitlab.com/cznic/sqlite)、[golang.org/x/image](https://pkg.go.dev/golang.org/x/image) 和 [rsc.io/qr](https://pkg.go.dev/rsc.io/qr) 构建。
- 工坊按需加载 [tesseract.js](https://github.com/naptha/tesseract.js)（Apache-2.0）和 [@imgly/background-removal](https://github.com/imgly/background-removal-js)（AGPL-3.0，未修改，来自 jsDelivr）。

同一作者的其他项目：[Amnesia](https://github.com/navi-crwn/amnesia)，一台在注销时会遗忘一切的 Mac。
