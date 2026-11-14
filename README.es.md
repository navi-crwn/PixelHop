<p align="center">🌐 <a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.de.md">Deutsch</a> · <a href="README.id.md">Bahasa Indonesia</a> · <a href="README.ja.md">日本語</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <b>Español</b> · <a href="README.fr.md">Français</a> · <a href="README.ru.md">Русский</a></p>

# PixelHop

Alojamiento de imágenes de código abierto en un solo programa de Go. Quita la ubicación GPS y los datos de la cámara de las fotos antes de que nadie las vea, y tiene un estudio de fotos que funciona en el navegador, así que la foto se queda en tu dispositivo hasta que la subes.

En línea en [pixel.hel.ink](https://pixel.hel.ink). Los enlaces cortos usan [p.hel.ink](https://p.hel.ink).

<p align="center">
  <a href="https://pixel.hel.ink"><b>Prueba pixel.hel.ink</b></a> ·
  <a href="https://navi-crwn.github.io/PixelHop/">Sitio web</a> ·
  <a href="#install">Instalación</a> ·
  <a href="#api">API</a> ·
  <a href="#upgrading">Actualizar desde v2</a> ·
  <a href="CHANGELOG.md">Registro de cambios</a> ·
  <a href="SECURITY.md">Seguridad</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.27-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.27"/>
  <img src="https://img.shields.io/badge/SQLite-built_in-003B57?style=flat-square&logo=sqlite&logoColor=white" alt="SQLite"/>
  <img src="https://img.shields.io/badge/storage-disk_|_S3-a3e635?style=flat-square" alt="Disk or S3"/>
  <img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" alt="MIT License"/>
  <img src="https://img.shields.io/github/actions/workflow/status/navi-crwn/pixelhop/ci.yml?style=flat-square&label=tests" alt="Tests"/>
</p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/home-dark.webp"><img src="docs/readme/home-light.webp" alt="La página de subida de PixelHop después de que un invitado subió una foto: el enlace está listo y un aviso dice que se quitaron la ubicación GPS, los datos de la cámara y la fecha de la toma" width="100%"></picture></p>

## Qué hace

- Recibe imágenes, vídeos y PDF arrastrándolos, pegándolos, eligiendo un archivo o con un enlace, y da a cada archivo un enlace corto y legible, como `beach-sunset-x7Kp2Q`.
- Quita los metadatos ocultos de fotos y vídeos (qué ocurre exactamente con cada tipo de archivo está en [Qué se elimina](#what-gets-removed)).
- Agrupa varios archivos en un álbum con un solo enlace.
- Incluye un estudio de fotos que comprime, cambia el tamaño, recorta, convierte, desenfoca, lee texto y quita fondos en el navegador.
- Funciona como un único archivo estático con SQLite integrado. No hacen falta PHP, Python, MySQL ni cron. Las imágenes se guardan en disco o en cualquier bucket compatible con S3.

Sirve a quienes comparten capturas de pantalla y fotos en chats, foros o GitHub, a quienes quieren un destino de [ShareX](https://getsharex.com) bajo su control y a quienes se preocupan por lo que revela una foto.

## Capturas de pantalla

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/image-dark.webp"><img src="docs/readme/image-light.webp" alt="Una página de imagen: la foto, botones para copiar los enlaces directo, Markdown, HTML y BBCode, detalles del archivo y un código QR" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/dashboard-dark.webp"><img src="docs/readme/dashboard-light.webp" alt="El panel de miembros: barra lateral con la familia HOP, accesos rápidos y una cuadrícula de archivos subidos con filtros por etiqueta" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/album-dark.webp"><img src="docs/readme/album-light.webp" alt="Una página de álbum con cuatro fotos y botones para copiar el enlace del álbum, el código para incrustarlo y todos los enlaces en un formato" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/studio-dark.webp"><img src="docs/readme/studio-light.webp" alt="El estudio de fotos con la herramienta de compresión abierta sobre una foto: formato, calidad, lado más largo y el tamaño de archivo resultante" width="100%"></picture></p>

Estas capturas salen de una instancia de demostración local con imágenes generadas. [`scripts/readme-images.mjs`](scripts/readme-images.mjs) las vuelve a crear.

<a id="install"></a>

## Instalación

El instalador revisa tu equipo, hace unas preguntas y compila PixelHop desde el código fuente. Necesita Go 1.27 o más reciente, o Docker.

```bash
curl -fsSL https://raw.githubusercontent.com/navi-crwn/PixelHop/main/install.sh | bash
```

Desde un clon, ejecuta `./install.sh`. Más adelante, `./update.sh` descarga la versión nueva, copia la base de datos a `backups/` y recompila, y `./uninstall.sh` quita el programa pero conserva tus datos, salvo que añadas `--purge`. Todos los scripts aceptan `--yes` (aceptar todos los valores por defecto), `--dry-run` (mostrar los pasos sin ejecutarlos) y `--help`. El instalador puede ejecutar PixelHop en primer plano, escribir una unidad de systemd o un archivo de Docker Compose y, si quieres, instalar a su lado un hub de [HOP ID](https://github.com/navi-crwn/HEL.ink).

También puedes instalarlo a mano.

**Binario** (Linux, macOS y Windows, amd64 y arm64, en la página de [Releases](../../releases)):

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

La imagen se ejecuta como el usuario no root 65532, así que el volumen de datos tiene que poder escribirlo ese usuario. El repositorio también trae un [`docker-compose.yml`](docker-compose.yml) que lee un archivo `.env` (copia `.env.example`) y guarda los datos en `./data`.

**Desde el código fuente** (Go 1.27 o más reciente):

```bash
git clone https://github.com/navi-crwn/pixelhop.git && cd pixelhop
make build && ./pixelhop
```

Abre `http://localhost:8080` y crea una cuenta. La primera cuenta pasa a ser la de administrador. También puedes crear una desde la línea de comandos: `pixelhop user add tu@example.com --admin`.

Los vídeos y los PDF solo se procesan si `ffmpeg`, `ffprobe` y Ghostscript (`gs`) están instalados en el servidor. Sin ellos, esos archivos se guardan tal como se subieron. La imagen de Docker no incluye ninguno, así que instálalos en el host o crea tu propia imagen si alojas vídeos o PDF. Las fotos HEIC y AVIF, la salida en WebP y AVIF desde la URL de cambio de tamaño y las herramientas del estudio en el servidor necesitan el [`tools/ai-server.py`](tools/ai-server.py) opcional.

<a id="what-gets-removed"></a>

## Qué se elimina

PixelHop trabaja sobre la estructura del archivo y copia sin cambios los datos comprimidos de la imagen, así que la calidad se mantiene.

| Archivo | Qué ocurre |
|---|---|
| JPEG | Se eliminan EXIF (GPS, marca y modelo de la cámara, números de serie, fecha de la toma), XMP, IPTC y comentarios. El perfil de color y otros segmentos se conservan. Una foto cuyo EXIF indica rotación se endereza y se vuelve a guardar como JPEG con calidad 92, sin perfil de color. Es el único caso en que se recodifican los píxeles. |
| PNG | Se eliminan los bloques de texto, el bloque EXIF y la hora de modificación. El perfil de color se conserva. |
| WebP | Se eliminan EXIF y XMP de los archivos extendidos. Los archivos WebP simples no llevan metadatos. |
| GIF | Se guarda tal como se subió. No se elimina nada. |
| BMP | Se convierte a PNG. |
| HEIC, AVIF | Si el servicio de IA está configurado, se convierte a JPEG o PNG derecho y sin metadatos. Si no, la subida se rechaza. |
| Vídeo (MP4, MOV, 3GP, WebM, MKV) | Con `ffmpeg` instalado, se quitan la ubicación, las fechas, la cámara y las etiquetas del codificador sin recodificar. Solo se conservan la primera pista de vídeo y la primera de audio. Después, una tarea en segundo plano reduce el vídeo a H.264 y AAC, con 1280 px como máximo en el lado largo y 30 fotogramas por segundo. Sin `ffmpeg`, el archivo se guarda tal como se subió. |
| PDF | Con Ghostscript instalado, el archivo se reescribe para que pese menos, y se conserva la copia más pequeña si ahorra al menos un 10 %. Las propiedades del documento, como el título y el autor, no se eliminan. Sin Ghostscript, el archivo se guarda tal como se subió. |

Después de una subida, la página indica qué tipos de datos se eliminaron. La comprobación es por tipo (ubicación GPS, datos de la cámara, fecha de la toma, software de edición, XMP, IPTC, comentario, texto incrustado). Los valores en sí nunca se leen.

## Funciones

Compartir:
- Cada archivo tiene enlace directo, enlace de página, Markdown, HTML, BBCode y código QR, además de etiquetas Open Graph para que las vistas previas funcionen en Discord, X y WhatsApp.
- Álbumes a partir de varios archivos, con título, descripción, portada y orden que puedes editar después. Un álbum se puede incrustar en otro sitio con un iframe (`/a/{id}/embed`).
- Copia con un clic todos los enlaces de un álbum o de una subida múltiple en un formato.
- Caducidad a los 10 minutos, 1 hora, 1 día, 7 días, 30 días o 90 días.
- Cada archivo puede ser no listado (por defecto), solo para ti o estar protegido con contraseña. Un archivo puede llevar un aviso de contenido (sensible o para adultos); los visitantes lo confirman antes de verlo. Los propietarios pueden impedir que sus imágenes se incrusten en otros sitios.
- Etiquetas, búsqueda y un filtro por etiqueta en el panel. Los archivos borrados esperan 7 días en la papelera.
- Visitas por día y los sitios que enlazan a un archivo. Los referentes se guardan solo por nombre de host.
- Copias redimensionadas al momento, por ejemplo `/i/photo.jpg?w=800`, que se crean una vez y se guardan en caché.
- Perfiles públicos en `/u/username`, oEmbed y un sitemap.

Cuentas:
- Los invitados pueden subir archivos (por defecto hasta 20 MB cada uno, 25 archivos al día por dirección y 10 imágenes por álbum) y reciben un enlace privado para borrar cada archivo, que el navegador también recuerda.
- Los miembros tienen una galería con búsqueda, orden, cambio de nombre y acciones en bloque, una cuota de almacenamiento (128 MB por defecto), una tarjeta de primeros pasos y una exportación en ZIP de todo lo que han subido.
- Los ajustes muestran dónde has iniciado sesión, cierran la sesión en todos los demás sitios y gestionan claves de API, webhooks, tu perfil y el borrado de la cuenta.
- Inicio de sesión con correo y contraseña, con Google o con una cuenta de HEL.ink (el hub de HOP ID), todo opcional. Sin `HELINK_URL`, PixelHop nunca contacta con HEL.ink. Cloudflare Turnstile puede proteger los formularios de inicio de sesión y de registro.

Estudio:
- En el navegador: comprimir, cambiar tamaño, recortar, convertir (JPG, PNG, WebP y AVIF, donde el navegador pueda codificarlo), ajustar y girar, desenfocar y pixelar, marca de agua, paleta de colores, código QR, texto a partir de imágenes (OCR, 30 idiomas, hasta cinco a la vez) y eliminar el fondo. No se sube nada hasta que pulsas Subir resultado.
- Lotes: cambiar el tamaño, convertir, poner marca de agua o aplicar los pasos de IA a un conjunto de imágenes, y luego descargar un ZIP o subir un álbum.
- En el servidor, para usuarios con sesión iniciada, si ejecutas `tools/ai-server.py`: eliminar el fondo, desenfoque automático de rostros, borrador mágico y ampliación 2x. Una imagen cada vez, con límite diario, y no se guarda nada.

Para desarrolladores:
- Una API REST con claves bearer. Una clave se puede limitar a solo subir.
- Un archivo de configuración de ShareX que se genera en los ajustes; las configuraciones de ShareX de PixelHop 2 siguen funcionando.
- Webhooks para `image.uploaded` y `image.deleted`, firmados con HMAC-SHA256 en la cabecera `X-PixelHop-Signature`.
- Un cargador de línea de comandos en el mismo programa: `pixelhop push *.png --format markdown`.
- Una [descripción OpenAPI](web/static/openapi.json), servida en `/static/openapi.json`, y `/healthz`, `/metrics` y `/status` para la supervisión.
- Se puede instalar como app en el móvil, con destino para compartir fotos, vídeos y PDF.

Puesta en marcha:
- Un único binario estático, una imagen de Docker o una unidad de systemd. Guarda los archivos en disco o en almacenamiento compatible con S3 (AWS S3, Cloudflare R2, MinIO, Backblaze B2, Wasabi, Contabo).
- Las subidas se pausan solas cuando queda poco espacio libre en disco, y `PIXELHOP_READ_ONLY=true` las pausa por mantenimiento. Los archivos que ya están en línea siguen funcionando. Consulta [docs/UPLOAD-STORAGE.md](docs/UPLOAD-STORAGE.md).
- La interfaz está en nueve idiomas: English, 简体中文, Deutsch, Bahasa Indonesia, 日本語, Português (Brasil), Español, Français y Русский. Usa `?lang=de` para cambiar; la elección se guarda en una cookie y no se usa el idioma configurado en el navegador.
- Los enlaces compartidos pueden estar en otro host distinto del de la app (`PIXELHOP_LINK_URL`). En pixel.hel.ink la app está en pixel.hel.ink y los enlaces siguen siendo `p.hel.ink/...`.

Seguridad:
- Las contraseñas usan Argon2id. Las claves de API y los tokens de sesión se guardan como hashes.
- Los formularios están protegidos contra envíos entre sitios, y se envía una Content Security Policy estricta.
- "Subir desde un enlace" comprueba la dirección resuelta de cada conexión, así que no se puede apuntar a redes privadas. Las imágenes muy grandes se rechazan según su cabecera, antes de decodificarlas. Las subidas tienen límite de frecuencia, y la IP del cliente solo se toma de los proxies que indiques.
- Los archivos retirados por los moderadores no se pueden volver a subir, porque se recuerda su SHA-256.
- Cómo se procesa una subida y cómo se mantiene estable el servidor bajo carga: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). La historia detrás de la versión 3: [AUDIT.md](AUDIT.md). Para informar de una vulnerabilidad, lee [SECURITY.md](SECURITY.md).

Moderación: las páginas de administración integradas se retiraron en la 3.7.0. Todas las páginas tienen un botón Denunciar. La cola de denuncias, ocultar y borrar contenido, los bloqueos de IP, las acciones sobre cuentas, los ajustes en ejecución y la barra de anuncios corresponden al centro de control de HOP, que habla con PixelHop mediante la API firmada `/hop/*` (el HOP App Protocol). Se usa con el inicio de sesión de HEL.ink, se firma con `HELINK_SSO_SECRET`, y los llamadores permitidos están en `HOP_ADMIN_CLIENTS` (por defecto `imperium,helink`). Cada cambio hecho así queda registrado en la tabla `hop_audit`. El centro de control es privado del proyecto HOP. Una instalación independiente tiene el botón Denunciar, los comandos `pixelhop user` (`add`, `passwd`, `admin`, `merge`) y los derechos adicionales del rol de administrador a través de la API, pero no tiene una pantalla para la cola de denuncias.

## Ajustes

Los ajustes son variables de entorno, todas opcionales. [`.env.example`](.env.example) enumera las habituales con sus valores por defecto; el resto se lee en [`internal/config/config.go`](internal/config/config.go). La tabla recoge las que es más probable que cambies. La lista completa está en el [README](README.md) en inglés.

| Variable | Por defecto | Qué hace |
|---|---|---|
| `PIXELHOP_BASE_URL` | `http://localhost:8080` | Dirección pública de la app (inicio de sesión, subida, panel). Usa `https://` en producción, lo que además activa las cookies seguras |
| `PIXELHOP_LINK_URL` | `PIXELHOP_BASE_URL` | Otro host para los enlaces compartidos (páginas de imagen y de álbum, archivos directos). Tiene que ser un host distinto y también debe llegar a PixelHop |
| `PIXELHOP_DATA` | `./data` | Base de datos y, con almacenamiento local, las imágenes |
| `PIXELHOP_MAX_UPLOAD_MB` | `20` | Por imagen |
| `PIXELHOP_MAX_MEDIA_MB` | `100` | Por vídeo o PDF, para miembros con sesión iniciada |
| `PIXELHOP_GUEST_MAX_MB` | `20` | Por archivo de cualquier tipo, para invitados |
| `PIXELHOP_GUEST_UPLOADS` | `true` | Permitir subidas sin cuenta |
| `PIXELHOP_GUEST_ALBUM_MAX` | `10` | Imágenes en un álbum creado por un invitado (0 = sin límite); también se ajusta en el centro de control |
| `PIXELHOP_GUEST_DAILY_MAX` | `25` | Archivos que una dirección de invitado puede subir en 24 horas (0 = sin límite); también se ajusta en el centro de control |
| `PIXELHOP_GUEST_TTL_DAYS` | `30` | Días que una subida nueva de invitado permanece en línea (0 = hasta que se elimine); los archivos anteriores no cambian; también se ajusta en el centro de control |
| `PIXELHOP_SHARE_STORAGE` | `true` | Un solo archivo guardado cuando distintas personas suben la misma imagen; cada una conserva su enlace, propietario y caducidad (false = cada subida guarda su propia copia); los archivos ya guardados no cambian; también se ajusta en el centro de control |
| `PIXELHOP_DEFAULT_QUOTA_MB` | `128` | Almacenamiento de cada cuenta nueva; las cuentas existentes conservan el suyo |
| `PIXELHOP_REGISTRATION` | `true` | Permitir cuentas nuevas |
| `PIXELHOP_READ_ONLY` | `false` | Pausar las subidas por mantenimiento |
| `PIXELHOP_TRUSTED_PROXIES` | - | `cloudflare`, o las IP o CIDR de tu proxy inverso |
| `PIXELHOP_STORAGE`, `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY` | `local` | `local` o `s3`; el almacenamiento S3 necesita las otras cuatro |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | - | Iniciar sesión con Google |
| `HELINK_URL`, `HELINK_SSO_SECRET` | - | Iniciar sesión con una cuenta de HEL.ink. El secreto tiene al menos 32 caracteres y es igual a `PIXELHOP_SSO_SECRET` en el `.env` de HEL.ink |
| `PIXELHOP_CONTROL_URL` | - | Dirección del centro de control de HOP; los administradores que abren `/admin` son enviados allí |
| `TURNSTILE_SITE_KEY`, `TURNSTILE_SECRET_KEY` | - | Cloudflare Turnstile en el inicio de sesión y el registro. Con cuentas de HEL.ink activas, estos formularios rechazan todo intento si faltan las claves |
| `PIXELHOP_REMBG_URL` | - | Dirección de `tools/ai-server.py`, por ejemplo `http://127.0.0.1:7001` |

El secreto de HEL.ink también puede llamarse `HOP_SSO_SECRET_PIXELHOP` o `HOP_SSO_SECRET`. Al rotarlo, el secreto anterior se sigue aceptando mediante `HELINK_SSO_SECRET_PREV` (o `HOP_SSO_SECRET_PREV`).

### Proxy inverso, servicio y copias de seguridad

- [`deploy/nginx.conf`](deploy/nginx.conf) es un ejemplo de proxy inverso. Define `PIXELHOP_TRUSTED_PROXIES=127.0.0.1` para que PixelHop confíe en el `X-Forwarded-For` de nginx, y añade `cloudflare` si Cloudflare está delante.
- [`deploy/pixelhop.service`](deploy/pixelhop.service) es una unidad de systemd reforzada. [`deploy/pixelhop-ai.service`](deploy/pixelhop-ai.service) ejecuta el servicio de IA opcional, que necesita `rembg`, `opencv-python-headless` y `onnxruntime` (consulta el encabezado de [`tools/ai-server.py`](tools/ai-server.py)).
- Para hacer una copia de seguridad, copia la carpeta de datos o ejecuta `sqlite3 data/pixelhop.db ".backup copy.db"` con PixelHop en marcha. `./update.sh` hace una copia así en `backups/` antes de cada actualización.

## Cómo se usa

1. Abre el sitio y arrastra una imagen, pégala (<kbd>Ctrl</kbd>+<kbd>V</kbd>) o indica su enlace.
2. Opcional: elige Borrar después de, y si el nombre del archivo entra en el enlace.
3. El enlace se copia por ti. Ábrelo para ver el enlace directo, los códigos para incrustar y el código QR.

Pulsa `?` en cualquier página para ver los atajos de teclado.

**ShareX.** Ve a Ajustes, Claves de API, crea una clave, pulsa Descargar configuración de ShareX con esta clave y abre el archivo.

**Línea de comandos.** El mismo programa `pixelhop` sube a cualquier servidor PixelHop. `pixelhop login` guarda la dirección y la clave en `~/.config/pixelhop/client.json`; `PIXELHOP_SERVER` y `PIXELHOP_KEY` tienen prioridad.

```bash
pixelhop login --server https://pixel.hel.ink --key ph_yourkey
pixelhop push screenshot.png photos/*.jpg --expire 7d --format markdown
```

**Móvil.** Abre el sitio y elige Añadir a la pantalla de inicio. PixelHop aparecerá entonces en el menú de compartir.

<a id="api"></a>

## API

```bash
curl -H "Authorization: Bearer ph_yourkey" -F "file=@holiday.jpg" https://pixel.hel.ink/api/v1/upload
```

| Endpoint | Qué hace |
|---|---|
| `POST /api/v1/upload` | Sube archivos (`file`, repetible) o enlaces (`url`). Opciones: `expire` (`10m`, `1h`, `1d`, `7d`, `30d`, `90d`), `slug` (`0` deja el nombre del archivo fuera del enlace), `title`, `alt`, `album_id`, `album` (`force` o `0`), `listed`, `content_warning` (`sensitive` o `adult`), `warning_reason` |
| `POST /api/v1/uploads`, `PUT /api/v1/uploads/{id}`, `POST /api/v1/uploads/{id}/finish` | Sube un archivo grande en trozos de 8 MB |
| `GET /api/v1/images` | Tus imágenes (`page`, `limit`, `q`, `sort`) |
| `GET`, `PATCH`, `DELETE /api/v1/images/{id}` | Lee, cambia o borra una imagen. `PATCH` acepta `title`, `alt`, `listed`, `visibility` (`unlisted`, `private`, o `password` con `password`), `tags`, `content_warning` y `warning_reason`. `DELETE` mueve tu propio archivo a la papelera; añade `?permanent=1` para eliminarlo definitivamente |
| `POST /api/v1/images/{id}/restore` | Recupera un archivo de la papelera |
| `GET /api/v1/images/{id}/stats` | Visitas por día y sitios de origen |
| `GET`, `POST /api/v1/albums` / `GET`, `PATCH`, `DELETE /api/v1/albums/{id}` | Lista, crea, lee, edita y borra álbumes |
| `GET /api/v1/users/{username}/images` | Público: las imágenes del perfil de una persona |
| `GET /api/v1/me` | Tu cuenta y el uso de almacenamiento |
| `GET /api/v1/me/media` | Tus propias imágenes (`type=image`) o PDF (`type=pdf`) que otra página puede mostrar: sin archivos privados, con contraseña, con caducidad ni con aviso de contenido (`cursor`, `limit`). También para las apps HOP nombradas en `PIXELHOP_MEDIA_CLIENTS` (el selector de medios de BioHop), firmado de servidor a servidor para un miembro |
| `GET /api/v1/stats` | Totales públicos y la versión del servidor |
| `GET /api/v1/upload-status` | Si ahora mismo se aceptan subidas |
| `POST /api/v1/tools/{remove-background,faces,erase,upscale}` | Herramientas de IA del servidor (con sesión iniciada; `file`, más `mask` para borrar) |
| `GET /i/{id}.jpg?w=&h=&fit=&format=&q=` | Copias redimensionadas, creadas una vez y guardadas en caché. `format=webp` y `avif` necesitan el servicio de IA |
| `GET /oembed?url=...` | oEmbed para páginas de imagen |

Cada respuesta tiene la forma `{"success": true, "data": ...}` o `{"success": false, "error": "..."}`. La referencia completa está en [`/docs`](https://pixel.hel.ink/docs) en cualquier servidor PixelHop.

<a id="upgrading"></a>

## Actualizar desde PixelHop 2

PixelHop 2 era una app en PHP con MySQL, un venv de Python y tareas de cron. Su código sigue en la etiqueta [`v2.3.0-php`](../../tree/v2.3.0-php). Un comando trae los datos:

```bash
pixelhop import --images /var/www/pichost/data/images.json \
  --mysql "user:pass@tcp(127.0.0.1:3306)/pixelhop" --copy
```

- Los usuarios conservan sus ids, roles, cuotas y contraseñas. Los hashes argon2id de PHP se verifican y se actualizan en el siguiente inicio de sesión. Las cuentas de Google siguen vinculadas.
- Las imágenes conservan sus ids. Con `--copy` se descargan al almacenamiento de PixelHop 3 con miniaturas nuevas; sin él se siguen sirviendo desde los buckets antiguos.
- Los enlaces antiguos siguen funcionando: `/{id}`, `/i/2026/08/27/{id}_original.webp`, `/auth/google-callback` (define `GOOGLE_REDIRECT_URI`) y las configuraciones de ShareX que envían a `/api/upload.php`.

[AUDIT.md](AUDIT.md) explica por qué se reescribió.

## Desarrollo

```
cmd/pixelhop/     el comando: serve, user, import, push, login
internal/app/     servidor web: páginas, API, subidas, moderación, HOP App Protocol
internal/db/      esquema de SQLite, migraciones y consultas
internal/imaging/ detección de tipo, eliminación de metadatos, miniaturas
internal/storage/ disco local, S3 (firmador SigV4 propio), URL antiguas
internal/netsafe/ descarga a prueba de SSRF para "Subir desde un enlace"
web/              plantillas, scripts y los archivos de los nueve idiomas, incluidos en el binario
docs/             el sitio de GitHub Pages y las imágenes del README
scripts/          scripts de ayuda (imágenes del README, datos del registro de cambios, comprobación de la familia HOP)
tools/            el servicio de IA opcional
deploy/           ejemplos de nginx y systemd
```

`make test` ejecuta `go vet` y las pruebas, y `make build` genera `./pixelhop`. `scripts/readme-images.mjs` vuelve a generar las capturas del README a partir de una instancia en marcha (consulta el comentario al principio del archivo). `python3 scripts/build-changelog.py --repo PixelHop` regenera `web/changelog.json`, los datos de la página `/changelog`, a partir de `CHANGELOG.md` (lenguaje claro, una entrada por versión); no necesita red ni `gh`, y `make check-changelog` falla si el archivo no es claro o no está al día.

## Preguntas frecuentes

<details><summary>¿El estudio es de verdad privado?</summary>

La edición ocurre en tu dispositivo con Canvas y WebAssembly. El reconocimiento de texto ([tesseract.js](https://github.com/naptha/tesseract.js), incluido en PixelHop) también funciona en tu dispositivo y lee inglés de serie; para leer otros idiomas, pon sus archivos de modelo en `PIXELHOP_DATA/tessdata` con `scripts/ocr-language.sh deu fra jpn` (los descarga de la fuente oficial de tessdata; el estudio ofrece entonces exactamente los idiomas instalados). La eliminación de fondo, el desenfoque de caras, el borrador y el escalado funcionan en el servicio de IA de tu servidor (`PIXELHOP_REMBG_URL`) y requieren una cuenta con sesión iniciada, así que esas imágenes se envían a tu servidor. Por lo demás, tu imagen no se envía a ningún sitio salvo que pulses Subir resultado, y el estudio no carga nada de otros sitios web.
</details>

<details><summary>¿Puedo desactivar las subidas de invitados o los registros?</summary>

Sí. Define `PIXELHOP_GUEST_UPLOADS=false` o `PIXELHOP_REGISTRATION=false`. Con el centro de control de HOP también puedes cambiar ambos mientras el servidor funciona.
</details>

<details><summary>¿Cuántos recursos necesita?</summary>

Basta un servidor pequeño. Un proceso `pixelhop` recién iniciado usó unos 16 MB de memoria en una prueba en macOS, y los binarios de las versiones pesan entre 16 y 17 MB. Las imágenes grandes se decodifican de pocas en pocas, así que una ráfaga de subidas no puede agotar la memoria.
</details>

## Autoalojamiento

<!-- self-hosting:start -->
Todo lo que PixelHop necesita está en repositorios públicos: ningún repositorio privado, ningún token y ninguna cuenta en hel.ink. Los detalles están en la guía de la familia: [docs/self-hosting.md](https://github.com/navi-crwn/HEL.ink/blob/main/docs/self-hosting.md) (en inglés).

| Pieza | Para PixelHop | Qué hace |
| --- | --- | --- |
| [HEL.ink](https://github.com/navi-crwn/HEL.ink) (hub, HOP ID) | Opcional: añade «Iniciar sesión con HOP ID» | Cuentas e inicio de sesión para todas las apps |
| [Imperium](https://github.com/navi-crwn/Imperium) | Opcional, pero las páginas de administración de PixelHop están allí | Panel de administración de toda la familia; necesita el hub |
| Fuentes y mosaicos de la familia | Nada que instalar: se cargan desde `hel.ink/hop-assets` | Sin hel.ink: fuentes del sistema, sin mosaicos flotantes |

- **Orden de instalación**: PixelHop solo con `./install.sh`. Para iniciar sesión con HOP ID, primero el [hub](https://github.com/navi-crwn/HEL.ink): `./install.sh --hub new` instala uno al lado.
- **Ajustes mínimos**: `PIXELHOP_BASE_URL`; con el hub: `HELINK_URL`, `HELINK_SSO_SECRET`; con Imperium: `PIXELHOP_CONTROL_URL`.
- **Solo o en la familia**: PixelHop funciona solo, con sus propias cuentas. Con un hub, también se puede iniciar sesión con HOP ID y cerrar sesión en todas las apps a la vez.
- **Sin Imperium**: `/admin` muestra un aviso. Usa `pixelhop user add` y la API firmada `/hop/*` (la primera cuenta es la de administración).
<!-- self-hosting:end -->

## La familia HOP

PixelHop forma parte de la familia HOP de apps de código abierto que respetan la privacidad y se pueden alojar uno mismo.

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

## Créditos y licencia

- [MIT](LICENSE.md), copyright de Ivan Novskies (navi-crwn). Los componentes de terceros se enumeran en [CREDITS.md](CREDITS.md).
- Hecho con [Go](https://go.dev), [modernc.org/sqlite](https://gitlab.com/cznic/sqlite), [golang.org/x/image](https://pkg.go.dev/golang.org/x/image) y [rsc.io/qr](https://pkg.go.dev/rsc.io/qr).
- El estudio incluye [tesseract.js](https://github.com/naptha/tesseract.js) (Apache-2.0). La eliminación de fondo funciona en el servidor (rembg, MIT); no se usa ni se carga código AGPL.

Más del mismo autor: [Amnesia](https://github.com/navi-crwn/amnesia), un Mac que lo olvida todo al cerrar sesión.
