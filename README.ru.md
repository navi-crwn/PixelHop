<p align="center">🌐 <a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.de.md">Deutsch</a> · <a href="README.id.md">Bahasa Indonesia</a> · <a href="README.ja.md">日本語</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.es.md">Español</a> · <a href="README.fr.md">Français</a> · <b>Русский</b></p>

# PixelHop

Хостинг изображений с открытым кодом в одной программе на Go. Он убирает из фотографий GPS-координаты и сведения о камере до того, как кто-нибудь их увидит. Фотостудия работает в браузере, поэтому фото остаётся на вашем устройстве, пока вы его не загрузите.

Работает на [pixel.hel.ink](https://pixel.hel.ink). Короткие ссылки используют [p.hel.ink](https://p.hel.ink).

<p align="center">
  <a href="https://pixel.hel.ink"><b>Попробовать pixel.hel.ink</b></a> ·
  <a href="https://navi-crwn.github.io/PixelHop/">Сайт</a> ·
  <a href="#install">Установка</a> ·
  <a href="#api">API</a> ·
  <a href="#upgrading">Переход с v2</a> ·
  <a href="CHANGELOG.md">Журнал изменений</a> ·
  <a href="SECURITY.md">Безопасность</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.27-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.27"/>
  <img src="https://img.shields.io/badge/SQLite-built_in-003B57?style=flat-square&logo=sqlite&logoColor=white" alt="SQLite"/>
  <img src="https://img.shields.io/badge/storage-disk_|_S3-a3e635?style=flat-square" alt="Disk or S3"/>
  <img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" alt="MIT License"/>
  <img src="https://img.shields.io/github/actions/workflow/status/navi-crwn/pixelhop/ci.yml?style=flat-square&label=tests" alt="Tests"/>
</p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/home-dark.webp"><img src="docs/readme/home-light.webp" alt="Страница загрузки PixelHop после того, как гость загрузил фото: ссылка готова, а пометка сообщает, что GPS-координаты, сведения о камере и дата съёмки удалены" width="100%"></picture></p>

## Что он делает

- Принимает изображения, видео и PDF перетаскиванием, вставкой, выбором файла или по ссылке и даёт каждому файлу короткую читаемую ссылку, например `beach-sunset-x7Kp2Q`.
- Удаляет скрытые метаданные из фото и видео (что именно происходит с каждым типом файла, описано в разделе [Что удаляется](#what-gets-removed)).
- Собирает несколько файлов в альбом с одной ссылкой.
- Включает фотостудию, которая в браузере сжимает, меняет размер, обрезает, конвертирует, размывает, распознаёт текст и убирает фон.
- Работает как один статический файл со встроенной SQLite. PHP, Python, MySQL и cron не нужны. Изображения хранятся на диске или в любом S3-совместимом хранилище.

Подойдёт тем, кто делится скриншотами и фото в чатах, на форумах или на GitHub, кому нужен собственный получатель для [ShareX](https://getsharex.com) и кто следит за тем, что выдаёт фотография.

## Скриншоты

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/image-dark.webp"><img src="docs/readme/image-light.webp" alt="Страница изображения: фото, кнопки копирования прямой ссылки, Markdown, HTML и BBCode, сведения о файле и QR-код" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/dashboard-dark.webp"><img src="docs/readme/dashboard-light.webp" alt="Панель участника: боковое меню с семейством HOP, быстрые ссылки и сетка загруженных файлов с фильтрами по тегам" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/album-dark.webp"><img src="docs/readme/album-light.webp" alt="Страница альбома с четырьмя фото и кнопками копирования ссылки на альбом, кода для вставки и всех ссылок в одном формате" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/studio-dark.webp"><img src="docs/readme/studio-light.webp" alt="Фотостудия с открытым инструментом сжатия на фото: формат, качество, длинная сторона и получившийся размер файла" width="100%"></picture></p>

Скриншоты сделаны на локальном демо-экземпляре со сгенерированными картинками. Заново их создаёт [`scripts/readme-images.mjs`](scripts/readme-images.mjs).

<a id="install"></a>

## Установка

Установщик проверяет компьютер, задаёт несколько вопросов и собирает PixelHop из исходников. Нужен Go 1.27 или новее либо Docker.

```bash
curl -fsSL https://raw.githubusercontent.com/navi-crwn/PixelHop/main/install.sh | bash
```

Из клона запустите `./install.sh`. Позже `./update.sh` скачает новую версию, скопирует базу данных в `backups/` и пересоберёт программу, а `./uninstall.sh` удалит программу, но оставит ваши данные, если не добавить `--purge`. Каждый скрипт понимает `--yes` (принять все значения по умолчанию), `--dry-run` (показать шаги, не выполняя их) и `--help`. Установщик может запустить PixelHop на переднем плане, записать юнит systemd или файл Docker Compose и, если хотите, поставить рядом хаб [HOP ID](https://github.com/navi-crwn/HEL.ink).

Можно установить и вручную.

**Бинарный файл** (Linux, macOS и Windows, amd64 и arm64, на странице [Releases](../../releases)):

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

Образ запускается от непривилегированного пользователя 65532, поэтому том с данными должен быть доступен этому пользователю на запись. В репозитории есть и [`docker-compose.yml`](docker-compose.yml): он читает файл `.env` (скопируйте `.env.example`) и хранит данные в `./data`.

**Из исходников** (Go 1.27 или новее):

```bash
git clone https://github.com/navi-crwn/pixelhop.git && cd pixelhop
make build && ./pixelhop
```

Откройте `http://localhost:8080` и создайте учётную запись. Первая учётная запись становится администратором. Её можно создать и из командной строки: `pixelhop user add you@example.com --admin`.

Видео и PDF обрабатываются, только если на сервере установлены `ffmpeg`, `ffprobe` и Ghostscript (`gs`). Без них такие файлы сохраняются в том виде, в каком были загружены. В образе Docker нет ни одного из них, поэтому установите их на хосте или соберите свой образ, если вы размещаете видео или PDF. Для фото HEIC и AVIF, вывода WebP и AVIF по ссылке с изменением размера и серверных инструментов студии нужен необязательный [`tools/ai-server.py`](tools/ai-server.py).

<a id="what-gets-removed"></a>

## Что удаляется

PixelHop работает со структурой файла и копирует сжатые данные изображения без изменений, поэтому качество не меняется.

| Файл | Что происходит |
|---|---|
| JPEG | Удаляются EXIF (GPS, марка и модель камеры, серийные номера, дата съёмки), XMP, IPTC и комментарии. Цветовой профиль и другие сегменты остаются. Фото, у которого в EXIF указан поворот, выравнивается и сохраняется заново как JPEG с качеством 92, без цветового профиля. Это единственный случай, когда пиксели кодируются заново. |
| PNG | Удаляются текстовые блоки, блок EXIF и время изменения. Цветовой профиль остаётся. |
| WebP | EXIF и XMP удаляются из расширенных файлов. В простых файлах WebP метаданных нет. |
| GIF | Сохраняется как загружен. Ничего не удаляется. |
| BMP | Преобразуется в PNG. |
| HEIC, AVIF | Если настроен ИИ-сервис, преобразуется в выровненный JPEG или PNG без метаданных. Иначе загрузка отклоняется. |
| Видео (MP4, MOV, 3GP, WebM, MKV) | При установленном `ffmpeg` местоположение, даты, сведения о камере и теги кодировщика удаляются без перекодирования. Остаются только первая видеодорожка и первая звуковая. Затем фоновая задача уменьшает видео до H.264 и AAC: не более 1280 px по длинной стороне и 30 кадров в секунду. Без `ffmpeg` файл сохраняется как загружен. |
| PDF | При установленном Ghostscript файл переписывается, чтобы стать меньше, и меньшая копия остаётся, если экономит не менее 10 %. Свойства документа, например название и автор, не удаляются. Без Ghostscript файл сохраняется как загружен. |

После загрузки страница сообщает, какие виды данных удалены. Проверка идёт по видам (GPS-координаты, сведения о камере, дата съёмки, программа редактирования, XMP, IPTC, комментарий, встроенный текст). Сами значения не считываются.

## Возможности

Обмен файлами:
- У каждого файла есть прямая ссылка, ссылка на страницу, Markdown, HTML, BBCode и QR-код, а также теги Open Graph, чтобы предпросмотр работал в Discord, X и WhatsApp.
- Альбомы из нескольких файлов, у которых можно потом изменить название, описание, обложку и порядок. Альбом можно встроить на другой сайт через iframe (`/a/{id}/embed`).
- Все ссылки альбома или пакетной загрузки копируются в одном формате одним нажатием.
- Срок жизни: 10 минут, 1 час, 1 день, 7 дней, 30 дней или 90 дней.
- Каждый файл может быть вне списков (по умолчанию), виден только вам или защищён паролем. У файла может быть предупреждение о содержимом (чувствительное или для взрослых): посетитель подтверждает его, прежде чем увидеть файл. Владельцы могут запретить встраивание своих изображений на других сайтах.
- Теги, поиск и фильтр по тегам на панели. Удалённые файлы 7 дней лежат в корзине.
- Просмотры по дням и сайты, которые ссылаются на файл. Источники переходов хранятся только как имя хоста.
- Копии нужного размера по запросу, например `/i/photo.jpg?w=800`: создаются один раз и кэшируются.
- Публичные профили на `/u/username`, oEmbed и карта сайта.

Учётные записи:
- Гости могут загружать файлы (по умолчанию до 20 МБ каждый, 25 файлов в день с одного адреса и 10 изображений в альбоме) и получают для каждого файла личную ссылку для удаления, которую запоминает и браузер.
- У участников есть галерея с поиском, сортировкой, переименованием и массовыми действиями, квота хранилища (по умолчанию 128 МБ), карточка с первыми шагами и экспорт всего загруженного в ZIP.
- Настройки показывают, где вы вошли, позволяют выйти везде, кроме текущего устройства, и управляют API-ключами, вебхуками, профилем и удалением учётной записи.
- Вход по почте и паролю, через Google или учётную запись HEL.ink (хаб HOP ID), всё необязательно. Если `HELINK_URL` не задан, PixelHop никогда не обращается к HEL.ink. Cloudflare Turnstile может защищать формы входа и регистрации.

Студия:
- В браузере: сжатие, изменение размера, обрезка, конвертация (JPG, PNG, WebP и AVIF, если браузер умеет его кодировать), коррекция и поворот, размытие и пикселизация, водяной знак, цветовая палитра, QR-код, текст с изображений (OCR, 30 языков, до пяти одновременно) и удаление фона. Пока вы не нажмёте «Загрузить результат», ничего не загружается.
- Пакетная обработка: изменение размера, конвертация, водяные знаки или ИИ-шаги для набора изображений, затем скачивание одного ZIP или загрузка альбома.
- На сервере, для вошедших пользователей, если вы запускаете `tools/ai-server.py`: удаление фона, автоматическое размытие лиц, волшебный ластик и увеличение в 2 раза. По одному изображению за раз, с суточным лимитом, и ничего не сохраняется.

Для разработчиков:
- REST API с ключами Bearer. Ключ можно ограничить только загрузкой.
- Файл настроек ShareX, который создаётся в настройках; конфигурации ShareX от PixelHop 2 продолжают работать.
- Вебхуки для `image.uploaded` и `image.deleted`, подписанные HMAC-SHA256 в заголовке `X-PixelHop-Signature`.
- Загрузчик для командной строки в той же программе: `pixelhop push *.png --format markdown`.
- [Описание OpenAPI](web/static/openapi.json) по адресу `/static/openapi.json`, а также `/healthz`, `/metrics` и `/status` для мониторинга.
- Устанавливается на телефон как приложение, с пунктом «Поделиться» для фото, видео и PDF.

Эксплуатация:
- Один статический файл, образ Docker или юнит systemd. Файлы хранятся на диске или в S3-совместимом хранилище (AWS S3, Cloudflare R2, MinIO, Backblaze B2, Wasabi, Contabo).
- Загрузки сами приостанавливаются, когда свободного места на диске остаётся мало, а `PIXELHOP_READ_ONLY=true` приостанавливает их на время обслуживания. Файлы, которые уже опубликованы, продолжают работать. См. [docs/UPLOAD-STORAGE.md](docs/UPLOAD-STORAGE.md).
- Интерфейс есть на девяти языках: English, 简体中文, Deutsch, Bahasa Indonesia, 日本語, Português (Brasil), Español, Français и Русский. Язык переключается параметром `?lang=de`; выбор сохраняется в cookie, а языковая настройка браузера не используется.
- Общие ссылки могут находиться на другом хосте, чем приложение (`PIXELHOP_LINK_URL`). На pixel.hel.ink приложение живёт на pixel.hel.ink, а ссылки остаются вида `p.hel.ink/...`.

Безопасность:
- Пароли хранятся с Argon2id. API-ключи и токены сессий хранятся в виде хешей.
- Формы защищены от межсайтовых запросов, и отправляется строгая политика Content Security Policy.
- «Загрузка по ссылке» проверяет разрешённый адрес каждого соединения, поэтому её нельзя направить в частные сети. Очень большие изображения отклоняются по заголовку, до декодирования. У загрузок есть ограничение частоты, а IP клиента берётся только от указанных вами прокси.
- Файлы, удалённые модераторами, нельзя загрузить снова, потому что их SHA-256 запоминается.
- Как обрабатывается загрузка и как сервер остаётся устойчивым под нагрузкой: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). Предыстория третьей версии: [AUDIT.md](AUDIT.md). О найденной уязвимости сообщайте по [SECURITY.md](SECURITY.md).

Модерация: встроенные страницы администратора убраны в 3.7.0. На каждой странице есть кнопка «Пожаловаться». Очередь жалоб, скрытие и удаление содержимого, блокировка IP, действия с учётными записями, настройки во время работы и строка объявлений находятся в центре управления HOP, который общается с PixelHop через подписанный API `/hop/*` (HOP App Protocol). Он используется вместе со входом через HEL.ink, подписывается `HELINK_SSO_SECRET`, а допустимые вызывающие стороны перечислены в `HOP_ADMIN_CLIENTS` (по умолчанию `imperium,helink`). Каждое изменение, сделанное так, записывается в таблицу `hop_audit`. Сам центр управления закрыт и принадлежит проекту HOP. У отдельной установки есть кнопка «Пожаловаться», команды `pixelhop user` (`add`, `passwd`, `admin`, `merge`) и дополнительные права роли администратора через API, но нет экрана для очереди жалоб.

## Настройки

Настройки задаются переменными окружения, все они необязательны. В [`.env.example`](.env.example) перечислены обычные переменные с их значениями по умолчанию, остальные читаются в [`internal/config/config.go`](internal/config/config.go). В таблице те, что вы, скорее всего, захотите изменить. Полный список есть в английском [README](README.md).

| Переменная | По умолчанию | Что делает |
|---|---|---|
| `PIXELHOP_BASE_URL` | `http://localhost:8080` | Публичный адрес приложения (вход, загрузка, панель). В рабочей среде используйте `https://`, это же включает защищённые cookie |
| `PIXELHOP_LINK_URL` | `PIXELHOP_BASE_URL` | Другой хост для общих ссылок (страницы изображений и альбомов, прямые файлы). Это должен быть другой хост, и он тоже должен доходить до PixelHop |
| `PIXELHOP_DATA` | `./data` | База данных и, при локальном хранении, изображения |
| `PIXELHOP_MAX_UPLOAD_MB` | `20` | На одно изображение |
| `PIXELHOP_MAX_MEDIA_MB` | `100` | На одно видео или PDF, для вошедших участников |
| `PIXELHOP_GUEST_MAX_MB` | `20` | На один файл любого типа, для гостей |
| `PIXELHOP_GUEST_UPLOADS` | `true` | Разрешить загрузку без учётной записи |
| `PIXELHOP_GUEST_ALBUM_MAX` | `10` | Изображений в альбоме, созданном гостем (0 = без ограничения); также настраивается в центре управления |
| `PIXELHOP_GUEST_DAILY_MAX` | `25` | Файлов, которые один гостевой адрес может загрузить за 24 часа (0 = без ограничения); также настраивается в центре управления |
| `PIXELHOP_GUEST_TTL_DAYS` | `30` | Сколько дней новая гостевая загрузка остаётся онлайн (0 = до удаления); прежние файлы не меняются; также настраивается в центре управления |
| `PIXELHOP_SHARE_STORAGE` | `true` | Один сохранённый файл, когда разные люди загружают одно и то же изображение; у каждого остаются своя ссылка, владелец и срок хранения (false = каждая загрузка хранит свою копию); уже сохранённые файлы не меняются; также настраивается в центре управления |
| `PIXELHOP_DEFAULT_QUOTA_MB` | `128` | Хранилище для каждой новой учётной записи; существующие сохраняют свою квоту |
| `PIXELHOP_REGISTRATION` | `true` | Разрешить новые учётные записи |
| `PIXELHOP_READ_ONLY` | `false` | Приостановить загрузки на время обслуживания |
| `PIXELHOP_TRUSTED_PROXIES` | - | `cloudflare` или IP-адреса либо CIDR вашего обратного прокси |
| `PIXELHOP_STORAGE`, `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY` | `local` | `local` или `s3`; для S3 нужны остальные четыре |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | - | Вход через Google |
| `HELINK_URL`, `HELINK_SSO_SECRET` | - | Вход через учётную запись HEL.ink. Секрет не короче 32 символов и совпадает с `PIXELHOP_SSO_SECRET` в `.env` HEL.ink |
| `PIXELHOP_CONTROL_URL` | - | Адрес центра управления HOP; администраторов, открывающих `/admin`, отправляют туда |
| `TURNSTILE_SITE_KEY`, `TURNSTILE_SECRET_KEY` | - | Cloudflare Turnstile на формах входа и регистрации. При включённых учётных записях HEL.ink без ключей эти формы отклоняют любую попытку |
| `PIXELHOP_REMBG_URL` | - | Адрес `tools/ai-server.py`, например `http://127.0.0.1:7001` |

Секрет HEL.ink можно назвать и `HOP_SSO_SECRET_PIXELHOP`, и `HOP_SSO_SECRET`. При смене секрета прежний принимается через `HELINK_SSO_SECRET_PREV` (или `HOP_SSO_SECRET_PREV`).

### Обратный прокси, служба и резервные копии

- [`deploy/nginx.conf`](deploy/nginx.conf): пример обратного прокси. Задайте `PIXELHOP_TRUSTED_PROXIES=127.0.0.1`, чтобы PixelHop доверял заголовку `X-Forwarded-For` от nginx, и добавьте `cloudflare`, если перед ним стоит Cloudflare.
- [`deploy/pixelhop.service`](deploy/pixelhop.service): усиленный юнит systemd. [`deploy/pixelhop-ai.service`](deploy/pixelhop-ai.service) запускает необязательный ИИ-сервис, которому нужны `rembg`, `opencv-python-headless` и `onnxruntime` (см. начало [`tools/ai-server.py`](tools/ai-server.py)).
- Для резервной копии скопируйте папку с данными или выполните `sqlite3 data/pixelhop.db ".backup copy.db"`, пока PixelHop работает. `./update.sh` делает такую копию в `backups/` перед каждым обновлением.

## Как пользоваться

1. Откройте сайт и перетащите изображение, вставьте его (<kbd>Ctrl</kbd>+<kbd>V</kbd>) или укажите ссылку.
2. По желанию выберите «Удалить через» и решите, попадёт ли имя файла в ссылку.
3. Ссылка копируется сама. Откройте её, чтобы получить прямую ссылку, коды для вставки и QR-код.

Нажмите `?` на любой странице, чтобы увидеть сочетания клавиш.

**ShareX.** Откройте «Настройки», «API-ключи», создайте ключ, нажмите «Скачать конфигурацию ShareX с этим ключом» и откройте файл.

**Командная строка.** Та же программа `pixelhop` загружает файлы на любой сервер PixelHop. `pixelhop login` сохраняет адрес и ключ в `~/.config/pixelhop/client.json`; `PIXELHOP_SERVER` и `PIXELHOP_KEY` имеют приоритет.

```bash
pixelhop login --server https://pixel.hel.ink --key ph_yourkey
pixelhop push screenshot.png photos/*.jpg --expire 7d --format markdown
```

**Телефон.** Откройте сайт и выберите «Добавить на главный экран». После этого PixelHop появится в меню «Поделиться».

<a id="api"></a>

## API

```bash
curl -H "Authorization: Bearer ph_yourkey" -F "file=@holiday.jpg" https://pixel.hel.ink/api/v1/upload
```

| Точка доступа | Что делает |
|---|---|
| `POST /api/v1/upload` | Загружает файлы (`file`, можно несколько) или ссылки (`url`). Параметры: `expire` (`10m`, `1h`, `1d`, `7d`, `30d`, `90d`), `slug` (`0` не добавляет имя файла в ссылку), `title`, `alt`, `album_id`, `album` (`force` или `0`), `listed`, `content_warning` (`sensitive` или `adult`), `warning_reason` |
| `POST /api/v1/uploads`, `PUT /api/v1/uploads/{id}`, `POST /api/v1/uploads/{id}/finish` | Загружает большой файл частями по 8 МБ |
| `GET /api/v1/images` | Ваши изображения (`page`, `limit`, `q`, `sort`) |
| `GET`, `PATCH`, `DELETE /api/v1/images/{id}` | Читает, меняет или удаляет одно изображение. `PATCH` принимает `title`, `alt`, `listed`, `visibility` (`unlisted`, `private` или `password` вместе с `password`), `tags`, `content_warning` и `warning_reason`. `DELETE` переносит ваш файл в корзину; добавьте `?permanent=1`, чтобы удалить его насовсем |
| `POST /api/v1/images/{id}/restore` | Возвращает файл из корзины |
| `GET /api/v1/images/{id}/stats` | Просмотры по дням и сайты-источники |
| `GET`, `POST /api/v1/albums` / `GET`, `PATCH`, `DELETE /api/v1/albums/{id}` | Список, создание, чтение, изменение и удаление альбомов |
| `GET /api/v1/users/{username}/images` | Публично: изображения из профиля пользователя |
| `GET /api/v1/me` | Ваша учётная запись и занятое место |
| `GET /api/v1/me/media` | Ваши собственные изображения (`type=image`) или PDF (`type=pdf`), которые может показать другая страница: без приватных, защищённых паролем, истекающих файлов и файлов с предупреждением о содержимом (`cursor`, `limit`). Доступно и приложениям HOP из `PIXELHOP_MEDIA_CLIENTS` (выбор медиа в BioHop) — подписанный вызов сервер-сервер от имени одного участника |
| `GET /api/v1/stats` | Общие итоги и версия сервера |
| `GET /api/v1/upload-status` | Принимаются ли сейчас загрузки |
| `POST /api/v1/tools/{remove-background,faces,erase,upscale}` | Серверные ИИ-инструменты (нужен вход; `file`, а для erase ещё `mask`) |
| `GET /i/{id}.jpg?w=&h=&fit=&format=&q=` | Копии нужного размера: создаются один раз и кэшируются. Для `format=webp` и `avif` нужен ИИ-сервис |
| `GET /oembed?url=...` | oEmbed для страниц изображений |

Любой ответ выглядит как `{"success": true, "data": ...}` или `{"success": false, "error": "..."}`. Полный справочник есть на странице [`/docs`](https://pixel.hel.ink/docs) любого сервера PixelHop.

<a id="upgrading"></a>

## Переход с PixelHop 2

PixelHop 2 был приложением на PHP с MySQL, виртуальным окружением Python и заданиями cron. Его код остаётся в теге [`v2.3.0-php`](../../tree/v2.3.0-php). Одна команда переносит данные:

```bash
pixelhop import --images /var/www/pichost/data/images.json \
  --mysql "user:pass@tcp(127.0.0.1:3306)/pixelhop" --copy
```

- У пользователей сохраняются идентификаторы, роли, квоты и пароли. Хеши argon2id из PHP проверяются и обновляются при следующем входе. Учётные записи Google остаются привязанными.
- У изображений сохраняются идентификаторы. С `--copy` они скачиваются в хранилище PixelHop 3 и получают новые миниатюры; без него они продолжают отдаваться из прежних бакетов.
- Старые ссылки продолжают работать: `/{id}`, `/i/2026/08/27/{id}_original.webp`, `/auth/google-callback` (задайте `GOOGLE_REDIRECT_URI`) и конфигурации ShareX, которые отправляют запросы на `/api/upload.php`.

Почему всё переписали, объясняет [AUDIT.md](AUDIT.md).

## Разработка

```
cmd/pixelhop/     команда: serve, user, import, push, login
internal/app/     веб-сервер: страницы, API, загрузки, модерация, HOP App Protocol
internal/db/      схема SQLite, миграции и запросы
internal/imaging/ определение типа, удаление метаданных, миниатюры
internal/storage/ локальный диск, S3 (собственная подпись SigV4), старые URL
internal/netsafe/ получение файлов с защитой от SSRF для «Загрузки по ссылке»
web/              шаблоны, скрипты и файлы девяти языков, встроенные в бинарный файл
docs/             сайт на GitHub Pages и картинки для README
scripts/          вспомогательные скрипты (картинки README, данные журнала изменений, проверка семейства HOP)
tools/            необязательный ИИ-сервис
deploy/           примеры для nginx и systemd
```

`make test` запускает `go vet` и тесты, а `make build` собирает `./pixelhop`. `scripts/readme-images.mjs` заново делает скриншоты для README на работающем экземпляре (см. комментарий в начале файла). `python3 scripts/build-changelog.py --repo PixelHop` пересоздаёт `web/changelog.json`, данные страницы `/changelog`, из `CHANGELOG.md` (понятным языком, одна запись на версию); ему не нужны ни сеть, ни `gh`, а `make check-changelog` завершается ошибкой, если файл непонятен или устарел.

## Вопросы и ответы

<details><summary>Студия действительно приватна?</summary>

Редактирование идёт на вашем устройстве с помощью Canvas и WebAssembly. Распознавание текста ([tesseract.js](https://github.com/naptha/tesseract.js), входит в PixelHop) тоже работает на вашем устройстве и читает английский сразу; для других языков положите файлы моделей в `PIXELHOP_DATA/tessdata` командой `scripts/ocr-language.sh deu fra jpn` (она скачивает их из официального источника tessdata; студия тогда предлагает ровно установленные языки). Удаление фона, размытие лиц, ластик и увеличение работают в ИИ-сервисе вашего сервера (`PIXELHOP_REMBG_URL`) и требуют входа в аккаунт, поэтому эти изображения отправляются на ваш сервер. В остальном изображение никуда не отправляется, пока вы не нажмёте «Загрузить результат», и студия ничего не загружает с других сайтов.
</details>

<details><summary>Можно ли отключить загрузки от гостей или регистрацию?</summary>

Да. Задайте `PIXELHOP_GUEST_UPLOADS=false` или `PIXELHOP_REGISTRATION=false`. С центром управления HOP оба параметра можно менять и во время работы сервера.
</details>

<details><summary>Сколько ресурсов нужно?</summary>

Хватит небольшого сервера. Только что запущенный процесс `pixelhop` занял около 16 МБ памяти в тесте на macOS, а бинарные файлы релизов весят от 16 до 17 МБ. Большие изображения декодируются по несколько штук за раз, поэтому поток загрузок не может исчерпать всю память.
</details>

## Собственный хостинг

<!-- self-hosting:start -->
Всё, что нужно PixelHop, лежит в публичных репозиториях: не нужны ни приватный репозиторий, ни токен, ни учётная запись на hel.ink. Подробности — в руководстве по семейству: [docs/self-hosting.md](https://github.com/navi-crwn/HEL.ink/blob/main/docs/self-hosting.md) (на английском).

| Компонент | Для PixelHop | Зачем |
| --- | --- | --- |
| [HEL.ink](https://github.com/navi-crwn/HEL.ink) (хаб, HOP ID) | Необязательно: добавляет «Войти через HOP ID» | Учётные записи и вход для всех приложений |
| [Imperium](https://github.com/navi-crwn/Imperium) | Необязательно, но страницы администрирования PixelHop находятся там | Панель администратора для всего семейства; нужен хаб |
| Шрифты и плитки семейства | Ничего устанавливать не нужно: загружаются с `hel.ink/hop-assets` | Без hel.ink: системные шрифты, без плавающих плиток |

- **Порядок установки**: PixelHop отдельно через `./install.sh`. Для входа через HOP ID сначала [хаб](https://github.com/navi-crwn/HEL.ink): `./install.sh --hub new` установит его рядом.
- **Минимальные настройки**: `PIXELHOP_BASE_URL`; с хабом: `HELINK_URL`, `HELINK_SSO_SECRET`; с Imperium: `PIXELHOP_CONTROL_URL`.
- **Отдельно или в семействе**: PixelHop работает отдельно со своими учётными записями. С хабом можно также входить через HOP ID и выходить сразу из всех приложений.
- **Без Imperium**: `/admin` показывает уведомление. Используйте `pixelhop user add` и подписанный API `/hop/*` (первая учётная запись — администратор).
<!-- self-hosting:end -->

## Семейство HOP

PixelHop входит в семейство HOP: открытых приложений, которые уважают приватность и которые можно разместить у себя.

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

## Благодарности и лицензия

- [MIT](LICENSE.md), авторские права принадлежат Ivan Novskies (navi-crwn). Сторонние компоненты перечислены в [CREDITS.md](CREDITS.md).
- Построен на [Go](https://go.dev), [modernc.org/sqlite](https://gitlab.com/cznic/sqlite), [golang.org/x/image](https://pkg.go.dev/golang.org/x/image) и [rsc.io/qr](https://pkg.go.dev/rsc.io/qr).
- Студия включает [tesseract.js](https://github.com/naptha/tesseract.js) (Apache-2.0). Удаление фона работает на сервере (rembg, MIT); код под AGPL не используется и не загружается.

Ещё от того же автора: [Amnesia](https://github.com/navi-crwn/amnesia), Mac, который забывает всё при выходе.
