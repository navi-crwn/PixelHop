<p align="center">🌐 <a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.de.md">Deutsch</a> · <a href="README.id.md">Bahasa Indonesia</a> · <b>日本語</b> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.es.md">Español</a> · <a href="README.fr.md">Français</a> · <a href="README.ru.md">Русский</a></p>

# PixelHop

Go 製のプログラム 1 つで動くオープンソースの画像ホスティングです。写真から GPS の位置情報とカメラの情報を、誰かが見る前に取り除きます。フォトスタジオはブラウザー内で動くので、アップロードするまで写真は手元のデバイスから出ません。

公開サイト: [pixel.hel.ink](https://pixel.hel.ink)。短縮リンクは [p.hel.ink](https://p.hel.ink) を使います。

<p align="center">
  <a href="https://pixel.hel.ink"><b>pixel.hel.ink を試す</b></a> ·
  <a href="https://navi-crwn.github.io/PixelHop/">ウェブサイト</a> ·
  <a href="#install">インストール</a> ·
  <a href="#api">API</a> ·
  <a href="#upgrading">v2 からの移行</a> ·
  <a href="CHANGELOG.md">変更履歴</a> ·
  <a href="SECURITY.md">セキュリティ</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.27-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.27"/>
  <img src="https://img.shields.io/badge/SQLite-built_in-003B57?style=flat-square&logo=sqlite&logoColor=white" alt="SQLite"/>
  <img src="https://img.shields.io/badge/storage-disk_|_S3-a3e635?style=flat-square" alt="Disk or S3"/>
  <img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" alt="MIT License"/>
  <img src="https://img.shields.io/github/actions/workflow/status/navi-crwn/pixelhop/ci.yml?style=flat-square&label=tests" alt="Tests"/>
</p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/home-dark.webp"><img src="docs/readme/home-light.webp" alt="ゲストが写真をアップロードした後の PixelHop のアップロードページ。リンクが用意され、GPS の位置情報、カメラの情報、撮影日を取り除いたという表示が出ている" width="100%"></picture></p>

## できること

- 画像、動画、PDF をドラッグ＆ドロップ、貼り付け、ファイル選択、リンクで受け取り、`beach-sunset-x7Kp2Q` のような読みやすい短いリンクを付けます。
- 写真と動画から隠れたメタデータを取り除きます（ファイル形式ごとの正確な内容は[取り除かれるもの](#what-gets-removed)を参照）。
- 複数のファイルを 1 つのリンクのアルバムにまとめます。
- ブラウザー内で、圧縮、リサイズ、トリミング、変換、ぼかし、文字認識、背景の除去ができるフォトスタジオを備えています。
- SQLite 内蔵の静的な実行ファイル 1 つで動きます。PHP、Python、MySQL、cron は不要です。画像はディスクか、S3 互換のバケットに置けます。

チャットやフォーラム、GitHub でスクリーンショットや写真を共有する人、自分で管理できる [ShareX](https://getsharex.com) の送信先がほしい人、写真が何を漏らすかを気にする人に向いています。

## スクリーンショット

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/image-dark.webp"><img src="docs/readme/image-light.webp" alt="画像ページ。写真、直接リンク・Markdown・HTML・BBCode のコピーボタン、ファイルの詳細、QR コード" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/dashboard-dark.webp"><img src="docs/readme/dashboard-light.webp" alt="メンバー用ダッシュボード。HOP ファミリーを並べたサイドバー、ショートカット、タグで絞り込めるアップロード済みファイルの一覧" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/album-dark.webp"><img src="docs/readme/album-light.webp" alt="4 枚の写真があるアルバムページ。アルバムのリンク、埋め込みコード、1 つの形式の全リンクをコピーするボタン" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/studio-dark.webp"><img src="docs/readme/studio-light.webp" alt="写真に対して圧縮ツールを開いたフォトスタジオ。形式、品質、長辺、結果のファイルサイズ" width="100%"></picture></p>

これらのスクリーンショットは、生成した画像を使ったローカルのデモ環境で撮影したものです。[`scripts/readme-images.mjs`](scripts/readme-images.mjs) で作り直せます。

<a id="install"></a>

## インストール

インストーラーはコンピューターを確認し、いくつか質問をして、ソースから PixelHop をビルドします。Go 1.27 以降、または Docker が必要です。

```bash
curl -fsSL https://raw.githubusercontent.com/navi-crwn/PixelHop/main/install.sh | bash
```

クローンしたフォルダーでは `./install.sh` を実行します。あとで `./update.sh` を実行すると、新しいバージョンを取得し、データベースを `backups/` にコピーして、再ビルドします。`./uninstall.sh` はプログラムを削除しますが、`--purge` を付けない限りデータは残します。どのスクリプトも `--yes`（既定値をすべて受け入れる）、`--dry-run`（実行せず手順だけ表示）、`--help` に対応しています。インストーラーは PixelHop をフォアグラウンドで動かすほか、systemd ユニットや Docker Compose ファイルを書き出せ、希望すれば [HOP ID](https://github.com/navi-crwn/HEL.ink) のハブを隣にインストールできます。

手動でインストールすることもできます。

**バイナリ**（Linux、macOS、Windows、amd64 と arm64。[Releases](../../releases) ページにあります）:

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

イメージは root ではないユーザー 65532 で動くため、データ用ボリュームはそのユーザーが書き込める必要があります。リポジトリには [`docker-compose.yml`](docker-compose.yml) もあり、`.env` ファイル（`.env.example` をコピー）を読み、データを `./data` に置きます。

**ソースから**（Go 1.27 以降）:

```bash
git clone https://github.com/navi-crwn/pixelhop.git && cd pixelhop
make build && ./pixelhop
```

`http://localhost:8080` を開いてアカウントを作ります。最初のアカウントが管理者になります。コマンドラインでも作れます: `pixelhop user add you@example.com --admin`。

動画と PDF は、サーバーに `ffmpeg`、`ffprobe`、Ghostscript（`gs`）が入っている場合だけ処理されます。入っていなければ、アップロードされたままの状態で保存されます。Docker イメージにはどれも入っていないので、動画や PDF をホストするならホストにインストールするか、自分でイメージをビルドしてください。HEIC と AVIF の写真、リサイズ URL での WebP と AVIF の出力、サーバー側のスタジオツールには、任意の [`tools/ai-server.py`](tools/ai-server.py) が必要です。

<a id="what-gets-removed"></a>

## 取り除かれるもの

PixelHop はファイルの構造を操作し、圧縮された画像データはそのままコピーするので、画質は変わりません。

| ファイル | 処理内容 |
|---|---|
| JPEG | EXIF（GPS、カメラのメーカーと機種、シリアル番号、撮影日）、XMP、IPTC、コメントを取り除きます。カラープロファイルとその他のセグメントは残ります。EXIF に回転が記録されている写真は、向きを直し、品質 92 で JPEG として保存し直します。このときカラープロファイルは付きません。ピクセルを再エンコードするのはこの場合だけです。 |
| PNG | テキストチャンク、EXIF チャンク、更新時刻を取り除きます。カラープロファイルは残ります。 |
| WebP | 拡張形式のファイルから EXIF と XMP を取り除きます。単純な WebP ファイルにはメタデータがありません。 |
| GIF | アップロードされたまま保存します。何も取り除きません。 |
| BMP | PNG に変換します。 |
| HEIC, AVIF | AI サービスを設定してあれば、メタデータのない、向きを直した JPEG か PNG に変換します。設定していなければ、アップロードは拒否されます。 |
| 動画（MP4、MOV、3GP、WebM、MKV） | `ffmpeg` が入っていれば、位置情報、日付、カメラ、エンコーダーのタグを、再エンコードなしで取り除きます。残すのは最初の映像トラックと最初の音声トラックだけです。その後、バックグラウンドのジョブが H.264 と AAC に縮小します。長辺は最大 1280 px、フレームレートは最大 30 fps です。`ffmpeg` がなければ、ファイルはアップロードされたまま保存されます。 |
| PDF | Ghostscript が入っていれば、ファイルを小さくするために書き直し、10% 以上小さくなったときだけ小さい方を残します。タイトルや作成者などの文書プロパティは取り除かれません。Ghostscript がなければ、ファイルはアップロードされたまま保存されます。 |

アップロードのあと、ページにどの種類のデータを取り除いたかが表示されます。確認は種類ごとに行います（GPS の位置情報、カメラの情報、撮影日、編集ソフト、XMP、IPTC、コメント、埋め込みテキスト）。値そのものは読み出しません。

## 機能

共有:
- ファイルごとに直接リンク、ページのリンク、Markdown、HTML、BBCode、QR コードを用意します。Open Graph タグもあるので、Discord、X、WhatsApp でプレビューが正しく表示されます。
- 複数のファイルからアルバムを作れ、タイトル、説明、カバー、並び順はあとから編集できます。アルバムは iframe で別のサイトに埋め込めます（`/a/{id}/embed`）。
- アルバムや一括アップロードの全リンクを、1 つの形式でワンクリックでコピーできます。
- 10 分、1 時間、1 日、7 日、30 日、90 日後に有効期限が切れるよう設定できます。
- 各ファイルは、非公開リスト（既定）、自分だけ、パスワード付きのどれかにできます。ファイルにはコンテンツの警告（センシティブまたはアダルト）を付けられ、訪問者は確認してから表示します。所有者は、自分の画像を他のサイトに埋め込まれないようにもできます。
- タグ、検索、ダッシュボードでのタグによる絞り込み。削除したファイルはごみ箱に 7 日間残ります。
- 日ごとの閲覧数と、ファイルにリンクしているサイト。参照元はホスト名だけを保存します。
- 要求に応じたリサイズ版（例: `/i/photo.jpg?w=800`）。一度作ればキャッシュされます。
- `/u/username` の公開プロフィール、oEmbed、サイトマップ。

アカウント:
- ゲストもアップロードできます（既定ではファイルごとに 20 MB まで、アドレスごとに 1 日 25 ファイルまで、アルバムごとに 10 枚まで）。ファイルごとに非公開の削除リンクが発行され、ブラウザーも覚えています。
- メンバーには、検索、並べ替え、名前の変更、一括操作ができるギャラリー、保存容量の上限（既定は 128 MB）、最初のステップを示すカード、アップロードした全ファイルの ZIP エクスポートが付きます。
- 設定ではサインインしている場所を確認でき、それ以外のすべての場所からサインアウトでき、API キー、Webhook、プロフィール、アカウント削除を管理できます。
- メールとパスワード、Google、HEL.ink アカウント（HOP ID ハブ）でのサインインに対応します。いずれも任意です。`HELINK_URL` を設定しなければ、PixelHop は HEL.ink に接続しません。Cloudflare Turnstile でサインインと登録のフォームを保護できます。

スタジオ:
- ブラウザー内で、圧縮、リサイズ、トリミング、変換（JPG、PNG、WebP、ブラウザーがエンコードできる場合は AVIF）、調整と回転、ぼかしとモザイク、透かし、カラーパレット、QR コード、画像から文字を読み取る OCR（30 言語、最大 5 言語を同時に）、背景の除去ができます。「結果をアップロード」を押すまで何もアップロードされません。
- バッチ: 複数の画像をまとめてリサイズ、変換、透かし入れ、AI の処理にかけ、1 つの ZIP でダウンロードするか、アルバムとしてアップロードできます。
- サーバー側では、`tools/ai-server.py` を動かしている場合に、サインイン済みのユーザー向けに背景の除去、顔の自動ぼかし、消しゴム、2 倍の拡大ができます。処理は 1 枚ずつで、1 日の上限があり、何も保存されません。

開発者向け:
- Bearer キーを使う REST API。キーはアップロード専用に制限できます。
- 設定で作れる ShareX の設定ファイル。PixelHop 2 の ShareX 設定もそのまま動きます。
- `image.uploaded` と `image.deleted` の Webhook。`X-PixelHop-Signature` ヘッダーに HMAC-SHA256 の署名が付きます。
- 同じプログラムに入っているコマンドラインのアップローダー: `pixelhop push *.png --format markdown`。
- `/static/openapi.json` の [OpenAPI 定義](web/static/openapi.json)と、監視用の `/healthz`、`/metrics`、`/status`。
- スマートフォンにアプリとしてインストールでき、写真、動画、PDF の共有先になれます。

運用:
- 静的な実行ファイル 1 つ、Docker イメージ、または systemd ユニットで動きます。ファイルはディスクか S3 互換のストレージ（AWS S3、Cloudflare R2、MinIO、Backblaze B2、Wasabi、Contabo）に置けます。
- ディスクの空きが少なくなるとアップロードは自動で止まり、`PIXELHOP_READ_ONLY=true` でメンテナンス中も止められます。すでに公開されているファイルはそのまま使えます。[docs/UPLOAD-STORAGE.md](docs/UPLOAD-STORAGE.md) を参照してください。
- インターフェースは 9 言語です: English、简体中文、Deutsch、Bahasa Indonesia、日本語、Português (Brasil)、Español、Français、Русский。`?lang=de` のように指定して切り替えます。選択は Cookie に保存され、ブラウザーの言語設定は使いません。
- 共有リンクは、アプリとは別のホストに置けます（`PIXELHOP_LINK_URL`）。pixel.hel.ink ではアプリは pixel.hel.ink にあり、リンクは `p.hel.ink/...` のままです。

安全性:
- パスワードには Argon2id を使います。API キーとセッショントークンはハッシュで保存します。
- フォームはクロスサイトの送信から保護され、厳格な Content Security Policy を送ります。
- 「リンクからアップロード」は、接続ごとに名前解決したアドレスを確認するので、プライベートネットワークへ向けられません。非常に大きな画像は、デコードする前にヘッダーから判断して拒否します。アップロードにはレート制限があり、クライアントの IP は、あなたが指定したプロキシからのものだけを使います。
- モデレーターが削除したファイルは、SHA-256 を記憶しているため、再アップロードできません。
- アップロードがどう処理されるか、高負荷でもサーバーがどう安定を保つかは [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) に、バージョン 3 に至る経緯は [AUDIT.md](AUDIT.md) にあります。脆弱性の報告は [SECURITY.md](SECURITY.md) をお読みください。

モデレーション: 内蔵の管理ページは 3.7.0 で廃止しました。どのページにも報告ボタンがあります。報告キュー、コンテンツの非表示と削除、IP のブロック、アカウント操作、実行時の設定、お知らせバーは HOP コントロールセンターの担当で、署名付きの `/hop/*` API（HOP App Protocol）を通じて PixelHop と通信します。これは HEL.ink サインインと併用し、`HELINK_SSO_SECRET` で署名し、呼び出しを許可する相手は `HOP_ADMIN_CLIENTS`（既定は `imperium,helink`）に並べます。この方法で行った変更はすべて `hop_audit` テーブルに記録されます。コントロールセンター自体は HOP プロジェクトの非公開のものです。単独でインストールした環境には、報告ボタン、`pixelhop user` コマンド（`add`、`passwd`、`admin`、`merge`）、API 経由で使える管理者ロールの追加権限はありますが、報告キューを見る画面はありません。

## 設定

設定は環境変数で行い、すべて任意です。[`.env.example`](.env.example) によく使うものを既定値つきで挙げています。ほかの変数は [`internal/config/config.go`](internal/config/config.go) で読み込まれます。次の表は、変更する機会が多いものです。全一覧は英語の [README](README.md) にあります。

| 変数 | 既定値 | 内容 |
|---|---|---|
| `PIXELHOP_BASE_URL` | `http://localhost:8080` | アプリの公開アドレス（サインイン、アップロード、ダッシュボード）。本番では `https://` を使ってください。セキュアな Cookie も有効になります |
| `PIXELHOP_LINK_URL` | `PIXELHOP_BASE_URL` | 共有リンク（画像ページ、アルバムページ、直接のファイル）用の別ホスト。別のホストである必要があり、そのホストからも PixelHop に届く必要があります |
| `PIXELHOP_DATA` | `./data` | データベースと、ローカル保存の場合は画像 |
| `PIXELHOP_MAX_UPLOAD_MB` | `20` | 画像 1 枚あたり |
| `PIXELHOP_MAX_MEDIA_MB` | `100` | サインイン済みメンバーの動画または PDF 1 つあたり |
| `PIXELHOP_GUEST_MAX_MB` | `20` | ゲストの、種類を問わずファイル 1 つあたり |
| `PIXELHOP_GUEST_UPLOADS` | `true` | アカウントなしのアップロードを許可 |
| `PIXELHOP_GUEST_ALBUM_MAX` | `10` | ゲストが作ったアルバムに入れられる画像数（0 = 無制限）。コントロールセンターでも変更できます |
| `PIXELHOP_GUEST_DAILY_MAX` | `25` | ゲストのアドレス 1 つが 24 時間にアップロードできるファイル数（0 = 無制限）。コントロールセンターでも変更できます |
| `PIXELHOP_GUEST_TTL_DAYS` | `30` | 新しいゲスト投稿が公開され続ける日数（0 = 削除するまで）。既存のファイルは変わりません。コントロールセンターでも変更できます |
| `PIXELHOP_DEFAULT_QUOTA_MB` | `128` | 新しいアカウントごとの保存容量。既存のアカウントは今の容量のままです |
| `PIXELHOP_REGISTRATION` | `true` | 新規アカウントの作成を許可 |
| `PIXELHOP_READ_ONLY` | `false` | メンテナンスのためアップロードを一時停止 |
| `PIXELHOP_TRUSTED_PROXIES` | - | `cloudflare`、またはリバースプロキシの IP か CIDR |
| `PIXELHOP_STORAGE`, `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY` | `local` | `local` か `s3`。S3 ストレージには残りの 4 つが必要 |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | - | Google でサインイン |
| `HELINK_URL`, `HELINK_SSO_SECRET` | - | HEL.ink アカウントでサインイン。シークレットは 32 文字以上で、HEL.ink の `.env` の `PIXELHOP_SSO_SECRET` と同じ値 |
| `PIXELHOP_CONTROL_URL` | - | HOP コントロールセンターのアドレス。`/admin` を開いた管理者はそこへ送られる |
| `TURNSTILE_SITE_KEY`, `TURNSTILE_SECRET_KEY` | - | サインインと登録のフォームでの Cloudflare Turnstile。HEL.ink アカウントを有効にしている場合、キーがないとこれらのフォームはすべての試行を拒否する |
| `PIXELHOP_REMBG_URL` | - | `tools/ai-server.py` のアドレス。例: `http://127.0.0.1:7001` |

HEL.ink のシークレットは `HOP_SSO_SECRET_PIXELHOP` や `HOP_SSO_SECRET` という名前でも指定できます。ローテーション中は `HELINK_SSO_SECRET_PREV`（または `HOP_SSO_SECRET_PREV`）で以前のシークレットも受け付けます。

### リバースプロキシ、サービス、バックアップ

- [`deploy/nginx.conf`](deploy/nginx.conf) はリバースプロキシの例です。`PIXELHOP_TRUSTED_PROXIES=127.0.0.1` を設定して nginx の `X-Forwarded-For` を信頼させ、前段に Cloudflare がある場合は `cloudflare` も加えます。
- [`deploy/pixelhop.service`](deploy/pixelhop.service) は堅牢化した systemd ユニットです。[`deploy/pixelhop-ai.service`](deploy/pixelhop-ai.service) は任意の AI サービスを動かします。`rembg`、`opencv-python-headless`、`onnxruntime` が必要です（[`tools/ai-server.py`](tools/ai-server.py) の冒頭を参照）。
- バックアップは、データフォルダーをコピーするか、PixelHop の動作中に `sqlite3 data/pixelhop.db ".backup copy.db"` を実行します。`./update.sh` は更新のたびに、そのコピーを `backups/` に作ります。

## 使い方

1. サイトを開き、画像をドロップするか、貼り付ける（<kbd>Ctrl</kbd>+<kbd>V</kbd>）か、リンクを入力します。
2. 任意: 「削除までの期間」と、ファイル名をリンクに含めるかどうかを選びます。
3. リンクは自動でコピーされます。開くと、直接リンク、埋め込みコード、QR コードが見られます。

どのページでも `?` を押すと、キーボードショートカットが表示されます。

**ShareX。** 設定の API キーでキーを作り、「このキーで ShareX の設定をダウンロード」を押して、ファイルを開きます。

**コマンドライン。** 同じ `pixelhop` で、どの PixelHop サーバーにもアップロードできます。`pixelhop login` はアドレスとキーを `~/.config/pixelhop/client.json` に保存します。`PIXELHOP_SERVER` と `PIXELHOP_KEY` を設定すると、そちらが優先されます。

```bash
pixelhop login --server https://pixel.hel.ink --key ph_yourkey
pixelhop push screenshot.png photos/*.jpg --expire 7d --format markdown
```

**スマートフォン。** サイトを開き、「ホーム画面に追加」を選びます。以後、PixelHop が共有シートに表示されます。

<a id="api"></a>

## API

```bash
curl -H "Authorization: Bearer ph_yourkey" -F "file=@holiday.jpg" https://pixel.hel.ink/api/v1/upload
```

| エンドポイント | 内容 |
|---|---|
| `POST /api/v1/upload` | ファイル（`file`、複数可）またはリンク（`url`）をアップロード。オプション: `expire`（`10m`、`1h`、`1d`、`7d`、`30d`、`90d`）、`slug`（`0` でファイル名をリンクに含めない）、`title`、`alt`、`album_id`、`album`（`force` または `0`）、`listed`、`content_warning`（`sensitive` または `adult`）、`warning_reason` |
| `POST /api/v1/uploads`, `PUT /api/v1/uploads/{id}`, `POST /api/v1/uploads/{id}/finish` | 大きなファイルを 8 MB ずつに分けてアップロード |
| `GET /api/v1/images` | 自分の画像（`page`、`limit`、`q`、`sort`） |
| `GET`, `PATCH`, `DELETE /api/v1/images/{id}` | 1 件の取得、変更、削除。`PATCH` は `title`、`alt`、`listed`、`visibility`（`unlisted`、`private`、または `password` と `password`）、`tags`、`content_warning`、`warning_reason` を受け付けます。`DELETE` は自分のファイルをごみ箱に移します。`?permanent=1` を付けると完全に削除します |
| `POST /api/v1/images/{id}/restore` | ごみ箱からファイルを戻す |
| `GET /api/v1/images/{id}/stats` | 日ごとの閲覧数と参照元のサイト |
| `GET`, `POST /api/v1/albums` / `GET`, `PATCH`, `DELETE /api/v1/albums/{id}` | アルバムの一覧、作成、取得、編集、削除 |
| `GET /api/v1/users/{username}/images` | 公開: ユーザーのプロフィールにある画像 |
| `GET /api/v1/me` | 自分のアカウントと保存容量の使用状況 |
| `GET /api/v1/me/media` | 他のページに表示してよい自分の画像（`type=image`）または PDF（`type=pdf`）。非公開、パスワード付き、期限付き、コンテンツ警告付きのファイルは含まれません（`cursor`、`limit`）。`PIXELHOP_MEDIA_CLIENTS` に挙げた HOP アプリ（BioHop のメディア選択）からも、メンバー一人分としてサーバー間署名付きで呼び出せます |
| `GET /api/v1/stats` | 公開の合計数とサーバーのバージョン |
| `GET /api/v1/upload-status` | いまアップロードを受け付けているか |
| `POST /api/v1/tools/{remove-background,faces,erase,upscale}` | サーバーの AI ツール（サインイン済み。`file`、消去には `mask` も） |
| `GET /i/{id}.jpg?w=&h=&fit=&format=&q=` | リサイズ版。一度作ればキャッシュされます。`format=webp` と `avif` には AI サービスが必要です |
| `GET /oembed?url=...` | 画像ページの oEmbed |

どの応答も `{"success": true, "data": ...}` か `{"success": false, "error": "..."}` の形です。完全なリファレンスは、どの PixelHop サーバーでも [`/docs`](https://pixel.hel.ink/docs) にあります。

<a id="upgrading"></a>

## PixelHop 2 からの移行

PixelHop 2 は、MySQL、Python の venv、cron ジョブを使う PHP アプリでした。コードは [`v2.3.0-php`](../../tree/v2.3.0-php) タグに残っています。1 つのコマンドでデータを移せます:

```bash
pixelhop import --images /var/www/pichost/data/images.json \
  --mysql "user:pass@tcp(127.0.0.1:3306)/pixelhop" --copy
```

- ユーザーは ID、ロール、容量の上限、パスワードがそのまま残ります。PHP の argon2id ハッシュは次回のサインイン時に検証され、更新されます。Google アカウントの連携も維持されます。
- 画像は ID がそのまま残ります。`--copy` を付けると、PixelHop 3 のストレージにダウンロードされ、サムネイルも作り直されます。付けなければ、これまでのバケットから配信されます。
- 古いリンクも動き続けます: `/{id}`、`/i/2026/08/27/{id}_original.webp`、`/auth/google-callback`（`GOOGLE_REDIRECT_URI` を設定）、`/api/upload.php` に送る ShareX の設定。

作り直した理由は [AUDIT.md](AUDIT.md) にあります。

## 開発

```
cmd/pixelhop/     コマンド: serve、user、import、push、login
internal/app/     ウェブサーバー: ページ、API、アップロード、モデレーション、HOP App Protocol
internal/db/      SQLite のスキーマ、マイグレーション、クエリ
internal/imaging/ 種類の判別、メタデータの除去、サムネイル
internal/storage/ ローカルディスク、S3（独自の SigV4 署名）、旧 URL
internal/netsafe/ 「リンクからアップロード」用の SSRF 対策済みの取得
web/              テンプレート、スクリプト、9 言語のファイル（バイナリに埋め込み）
docs/             GitHub Pages のサイトと README の画像
scripts/          補助スクリプト（README の画像、変更履歴のデータ、HOP ファミリーのチェック）
tools/            任意の AI サービス
deploy/           nginx と systemd の例
```

`make test` は `go vet` とテストを実行し、`make build` は `./pixelhop` を作ります。`scripts/readme-images.mjs` は、動いているインスタンスから README のスクリーンショットを作り直します（ファイル冒頭のコメントを参照）。`python3 scripts/build-changelog.py --repo PixelHop` は、マージ済みのプルリクエストから `/changelog` ページの元データ `web/changelog.json` を作り直します。認証済みの `gh` が必要です。

## よくある質問

<details><summary>スタジオは本当にプライベートですか。</summary>

編集は Canvas と WebAssembly を使ってあなたのデバイス上で行われます。文字認識や背景除去を初めて使うときは、ブラウザーが jsDelivr から [tesseract.js](https://github.com/naptha/tesseract.js) と [@imgly/background-removal](https://github.com/imgly/background-removal-js) を、tessdata.projectnaptha.com と staticimgly.com から言語データとモデルをダウンロードします。「結果をアップロード」を押さない限り、画像はどこにも送られません。より厳しい Content Security Policy を設定したセルフホストのサーバーでは、これらのアドレスを許可する必要があります。
</details>

<details><summary>ゲストのアップロードや登録を無効にできますか。</summary>

できます。`PIXELHOP_GUEST_UPLOADS=false` または `PIXELHOP_REGISTRATION=false` を設定してください。HOP コントロールセンターを使えば、サーバーの稼働中に変更することもできます。
</details>

<details><summary>どれくらいのリソースが必要ですか。</summary>

小さなサーバーで足ります。macOS での試験では、起動直後の `pixelhop` プロセスは約 16 MB のメモリを使い、リリースのバイナリは 16〜17 MB です。大きな画像は一度に数枚ずつデコードするので、アップロードが集中してもメモリを使い切ることはありません。
</details>

## セルフホスティング

<!-- self-hosting:start -->
PixelHop に必要なものはすべて公開リポジトリにあります。非公開リポジトリ、トークン、hel.ink のアカウントは不要です。詳しくはファミリーガイド [docs/self-hosting.md](https://github.com/navi-crwn/HEL.ink/blob/main/docs/self-hosting.md)（英語）を参照してください。

| 構成要素 | PixelHop にとって | 役割 |
| --- | --- | --- |
| [HEL.ink](https://github.com/navi-crwn/HEL.ink) (ハブ、HOP ID) | 任意：「HOP ID でサインイン」を追加 | すべてのアプリのアカウントとサインイン |
| [Imperium](https://github.com/navi-crwn/Imperium) | 任意。ただし PixelHop の管理ページはそこにあります | ファミリー全体の管理パネル。ハブが必要 |
| ファミリーのフォントとタイル | インストール不要：`hel.ink/hop-assets` から読み込み | hel.ink がなければシステムフォント、浮遊タイルなし |

- **インストール順**：PixelHop は `./install.sh` だけで動きます。HOP ID でのサインインには先に[ハブ](https://github.com/navi-crwn/HEL.ink)を。`./install.sh --hub new` が隣にインストールします。
- **最小限の設定**：`PIXELHOP_BASE_URL`; ハブ利用時: `HELINK_URL`, `HELINK_SSO_SECRET`; Imperium 利用時: `PIXELHOP_CONTROL_URL`。
- **単体かファミリーか**：PixelHop は独自のアカウントで単体で動きます。ハブがあれば HOP ID でサインインでき、全アプリから一度にサインアウトできます。
- **Imperium なしの場合**：`/admin` には案内が表示されるだけです。`pixelhop user add` と署名付き `/hop/*` API（最初のアカウントが管理者）を使えます。
<!-- self-hosting:end -->

## HOP ファミリー

PixelHop は、プライバシーを大切にするセルフホスト可能なオープンソースアプリ、HOP ファミリーの一員です。

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

## クレジットとライセンス

- [MIT](LICENSE.md)、著作権は Ivan Novskies (navi-crwn)。サードパーティの部品は [CREDITS.md](CREDITS.md) に載せています。
- [Go](https://go.dev)、[modernc.org/sqlite](https://gitlab.com/cznic/sqlite)、[golang.org/x/image](https://pkg.go.dev/golang.org/x/image)、[rsc.io/qr](https://pkg.go.dev/rsc.io/qr) で作られています。
- スタジオは、必要になったときに [tesseract.js](https://github.com/naptha/tesseract.js)（Apache-2.0）と [@imgly/background-removal](https://github.com/imgly/background-removal-js)（AGPL-3.0、無改変、jsDelivr から）を読み込みます。

同じ作者の別のプロジェクト: [Amnesia](https://github.com/navi-crwn/amnesia)。ログアウトすると全部忘れる Mac です。
