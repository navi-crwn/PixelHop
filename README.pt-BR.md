<p align="center">🌐 <a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.de.md">Deutsch</a> · <a href="README.id.md">Bahasa Indonesia</a> · <a href="README.ja.md">日本語</a> · <b>Português (Brasil)</b> · <a href="README.es.md">Español</a> · <a href="README.fr.md">Français</a> · <a href="README.ru.md">Русский</a></p>

# PixelHop

Hospedagem de imagens de código aberto em um único programa Go. Ele remove a localização GPS e os dados da câmera das fotos antes que alguém as veja, e tem um estúdio de fotos que roda no navegador, então a foto fica no seu dispositivo até você enviá-la.

No ar em [pixel.hel.ink](https://pixel.hel.ink). Os links curtos usam [p.hel.ink](https://p.hel.ink).

<p align="center">
  <a href="https://pixel.hel.ink"><b>Experimente pixel.hel.ink</b></a> ·
  <a href="https://navi-crwn.github.io/PixelHop/">Site</a> ·
  <a href="#install">Instalação</a> ·
  <a href="#api">API</a> ·
  <a href="#upgrading">Atualizar da v2</a> ·
  <a href="CHANGELOG.md">Changelog</a> ·
  <a href="SECURITY.md">Segurança</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.27-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.27"/>
  <img src="https://img.shields.io/badge/SQLite-built_in-003B57?style=flat-square&logo=sqlite&logoColor=white" alt="SQLite"/>
  <img src="https://img.shields.io/badge/storage-disk_|_S3-a3e635?style=flat-square" alt="Disk or S3"/>
  <img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" alt="MIT License"/>
  <img src="https://img.shields.io/github/actions/workflow/status/navi-crwn/pixelhop/ci.yml?style=flat-square&label=tests" alt="Tests"/>
</p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/home-dark.webp"><img src="docs/readme/home-light.webp" alt="A página de envio do PixelHop depois que um visitante enviou uma foto: o link está pronto e um aviso diz que localização GPS, dados da câmera e data da foto foram removidos" width="100%"></picture></p>

## O que ele faz

- Recebe imagens, vídeos e PDFs por arrastar e soltar, colar, seleção de arquivo ou link, e dá a cada arquivo um link curto e legível, como `beach-sunset-x7Kp2Q`.
- Remove metadados ocultos de fotos e vídeos (o que acontece exatamente em cada tipo de arquivo está em [O que é removido](#what-gets-removed)).
- Reúne vários arquivos em um álbum com um único link.
- Inclui um estúdio de fotos que comprime, redimensiona, corta, converte, desfoca, lê texto e remove fundos no navegador.
- Roda como um único arquivo estático, com SQLite embutido. Não precisa de PHP, Python, MySQL nem cron. As imagens ficam em disco ou em qualquer bucket compatível com S3.

Serve para quem compartilha capturas de tela e fotos em chats, fóruns ou no GitHub, para quem quer um destino do [ShareX](https://getsharex.com) sob seu controle e para quem se preocupa com o que uma foto revela.

## Capturas de tela

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/image-dark.webp"><img src="docs/readme/image-light.webp" alt="Uma página de imagem: a foto, botões para copiar os links direto, Markdown, HTML e BBCode, detalhes do arquivo e um QR code" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/dashboard-dark.webp"><img src="docs/readme/dashboard-light.webp" alt="O painel de membros: barra lateral com a família HOP, atalhos e uma grade de arquivos enviados com filtros por tag" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/album-dark.webp"><img src="docs/readme/album-light.webp" alt="Uma página de álbum com quatro fotos e botões para copiar o link do álbum, o código de incorporação e todos os links em um formato" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/studio-dark.webp"><img src="docs/readme/studio-light.webp" alt="O estúdio de fotos com a ferramenta de compressão aberta em uma foto: formato, qualidade, maior lado e o tamanho de arquivo resultante" width="100%"></picture></p>

Estas capturas vêm de uma instância de demonstração local, com imagens geradas. O [`scripts/readme-images.mjs`](scripts/readme-images.mjs) as recria.

<a id="install"></a>

## Instalação

O instalador verifica seu computador, faz algumas perguntas e compila o PixelHop a partir do código-fonte. Ele precisa do Go 1.27 ou mais novo, ou do Docker.

```bash
curl -fsSL https://raw.githubusercontent.com/navi-crwn/PixelHop/main/install.sh | bash
```

A partir de um clone, rode `./install.sh`. Depois, `./update.sh` baixa a nova versão, copia o banco de dados para `backups/` e recompila, e `./uninstall.sh` remove o programa, mas mantém seus dados, a menos que você use `--purge`. Todos os scripts aceitam `--yes` (aceitar todos os padrões), `--dry-run` (mostrar os passos sem executá-los) e `--help`. O instalador pode rodar o PixelHop em primeiro plano, escrever uma unidade do systemd ou um arquivo do Docker Compose e, se você quiser, instalar ao lado um hub [HOP ID](https://github.com/navi-crwn/HEL.ink).

Você também pode instalar à mão.

**Binário** (Linux, macOS e Windows, amd64 e arm64, na página de [Releases](../../releases)):

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

A imagem roda como o usuário não root 65532, então o volume de dados precisa ser gravável por esse usuário. O repositório também tem um [`docker-compose.yml`](docker-compose.yml) que lê um arquivo `.env` (copie o `.env.example`) e guarda os dados em `./data`.

**A partir do código-fonte** (Go 1.27 ou mais novo):

```bash
git clone https://github.com/navi-crwn/pixelhop.git && cd pixelhop
make build && ./pixelhop
```

Abra `http://localhost:8080` e crie uma conta. A primeira conta vira administradora. Também dá para criar uma pela linha de comando: `pixelhop user add voce@example.com --admin`.

Vídeos e PDFs só são processados quando `ffmpeg`, `ffprobe` e o Ghostscript (`gs`) estão instalados no servidor. Sem eles, esses arquivos são guardados como foram enviados. A imagem Docker não traz nenhum deles, então instale-os no host ou monte uma imagem própria se você hospeda vídeos ou PDFs. Fotos HEIC e AVIF, saída WebP e AVIF pela URL de redimensionamento e as ferramentas do estúdio no servidor precisam do [`tools/ai-server.py`](tools/ai-server.py), que é opcional.

<a id="what-gets-removed"></a>

## O que é removido

O PixelHop trabalha na estrutura do arquivo e copia os dados comprimidos da imagem sem alterá-los, então a qualidade continua a mesma.

| Arquivo | O que acontece |
|---|---|
| JPEG | EXIF (GPS, marca e modelo da câmera, números de série, data da foto), XMP, IPTC e comentários são removidos. O perfil de cor e outros segmentos ficam. Uma foto cujo EXIF indica rotação é endireitada e salva de novo como JPEG com qualidade 92, sem perfil de cor. Esse é o único caso em que os pixels são recodificados. |
| PNG | Blocos de texto, o bloco EXIF e a hora de modificação são removidos. O perfil de cor fica. |
| WebP | EXIF e XMP são removidos de arquivos estendidos. Arquivos WebP simples não carregam metadados. |
| GIF | Guardado como foi enviado. Nada é removido. |
| BMP | Convertido para PNG. |
| HEIC, AVIF | Com o serviço de IA configurado, é convertido para JPEG ou PNG na vertical e sem metadados. Caso contrário, o envio é recusado. |
| Vídeo (MP4, MOV, 3GP, WebM, MKV) | Com o `ffmpeg` instalado, localização, datas, câmera e tags do codificador são removidos sem recodificar. Só a primeira faixa de vídeo e a primeira de áudio ficam. Depois, uma tarefa em segundo plano reduz o vídeo para H.264 e AAC, com no máximo 1280 px no lado maior e 30 quadros por segundo. Sem o `ffmpeg`, o arquivo é guardado como foi enviado. |
| PDF | Com o Ghostscript instalado, o arquivo é reescrito para ficar menor, e a cópia menor é mantida se economizar pelo menos 10%. Propriedades do documento, como título e autor, não são removidas. Sem o Ghostscript, o arquivo é guardado como foi enviado. |

Depois de um envio, a página diz que tipos de dados foram removidos. A verificação é por tipo (localização GPS, dados da câmera, data da foto, software de edição, XMP, IPTC, comentário, texto incorporado). Os valores em si nunca são lidos.

## Recursos

Compartilhamento:
- Cada arquivo tem link direto, link da página, Markdown, HTML, BBCode e QR code, além de tags Open Graph para que as prévias funcionem no Discord, no X e no WhatsApp.
- Álbuns a partir de vários arquivos, com título, descrição, capa e ordem que você pode editar depois. Um álbum pode ser incorporado em outro site com um iframe (`/a/{id}/embed`).
- Copie todos os links de um álbum ou de um envio em lote, em um formato, com um clique.
- Expiração após 10 minutos, 1 hora, 1 dia, 7 dias, 30 dias ou 90 dias.
- Cada arquivo pode ficar fora das listas (padrão), visível só para você ou protegido por senha. Um arquivo pode ter um aviso de conteúdo (sensível ou adulto); os visitantes confirmam antes de vê-lo. Os donos podem recusar que suas imagens sejam incorporadas em outros sites.
- Tags, busca e filtro por tag no painel. Os arquivos apagados ficam 7 dias na lixeira.
- Visualizações por dia e os sites que apontam para um arquivo. Os referenciadores são guardados só pelo nome do host.
- Cópias redimensionadas sob demanda, por exemplo `/i/photo.jpg?w=800`, geradas uma vez e guardadas em cache.
- Perfis públicos em `/u/username`, oEmbed e um sitemap.

Contas:
- Visitantes podem enviar arquivos (por padrão, até 20 MB cada, 25 arquivos por dia por endereço e 10 imagens por álbum) e recebem um link de exclusão privado para cada um, que o navegador também lembra.
- Membros ganham uma galeria com busca, ordenação, renomeação e ações em lote, uma cota de armazenamento (128 MB por padrão), um cartão de primeiros passos e uma exportação em ZIP de tudo o que enviaram.
- As configurações mostram onde você está conectado, encerram a sessão em todos os outros lugares e gerenciam chaves de API, webhooks, seu perfil e a exclusão da conta.
- Login com e-mail e senha, Google ou uma conta HEL.ink (o hub HOP ID), tudo opcional. Sem `HELINK_URL`, o PixelHop nunca contata o HEL.ink. O Cloudflare Turnstile pode proteger os formulários de login e de cadastro.

Estúdio:
- No navegador: comprimir, redimensionar, cortar, converter (JPG, PNG, WebP e AVIF, onde o navegador consegue codificar), ajustar e girar, desfocar e pixelizar, marca d'água, paleta de cores, QR code, texto a partir de imagens (OCR, 30 idiomas, até cinco ao mesmo tempo) e remoção de fundo. Nada é enviado até você apertar Enviar resultado.
- Lote: redimensionar, converter, aplicar marca d'água ou rodar os passos de IA em um conjunto de imagens e depois baixar um ZIP ou enviar um álbum.
- No servidor, para usuários conectados, se você rodar o `tools/ai-server.py`: remoção de fundo, desfoque automático de rostos, borracha mágica e ampliação 2x. Uma imagem por vez, com limite diário, e nada é guardado.

Para desenvolvedores:
- Uma API REST com chaves bearer. Uma chave pode ser limitada a apenas enviar.
- Um arquivo de configuração do ShareX, gerado nas configurações; as configurações do ShareX do PixelHop 2 continuam funcionando.
- Webhooks para `image.uploaded` e `image.deleted`, assinados com HMAC-SHA256 no cabeçalho `X-PixelHop-Signature`.
- Um enviador de linha de comando no mesmo programa: `pixelhop push *.png --format markdown`.
- Uma [descrição OpenAPI](web/static/openapi.json), servida em `/static/openapi.json`, e `/healthz`, `/metrics` e `/status` para monitoramento.
- Instalável como app no celular, com destino de compartilhamento para fotos, vídeos e PDFs.

Operação:
- Um único binário estático, uma imagem Docker ou uma unidade do systemd. Guarde os arquivos em disco ou em armazenamento compatível com S3 (AWS S3, Cloudflare R2, MinIO, Backblaze B2, Wasabi, Contabo).
- Os envios pausam sozinhos quando o espaço livre em disco fica baixo, e `PIXELHOP_READ_ONLY=true` os pausa para manutenção. Os arquivos que já estão no ar continuam funcionando. Veja [docs/UPLOAD-STORAGE.md](docs/UPLOAD-STORAGE.md).
- A interface existe em nove idiomas: English, 简体中文, Deutsch, Bahasa Indonesia, 日本語, Português (Brasil), Español, Français e Русский. Use `?lang=de` para trocar; a escolha fica guardada em um cookie, e a configuração de idioma do navegador não é usada.
- Os links compartilhados podem ficar em outro host que não o do app (`PIXELHOP_LINK_URL`). No pixel.hel.ink, o app fica em pixel.hel.ink e os links continuam `p.hel.ink/...`.

Segurança:
- As senhas usam Argon2id. Chaves de API e tokens de sessão são guardados como hashes.
- Os formulários são protegidos contra envios entre sites, e uma Content Security Policy estrita é enviada.
- "Enviar por link" confere o endereço resolvido de cada conexão, então não dá para apontá-lo para redes privadas. Imagens muito grandes são recusadas pelo cabeçalho, antes de serem decodificadas. Os envios têm limite de taxa, e o IP do cliente só é aceito de proxies que você listar.
- Arquivos removidos pelos moderadores não podem ser enviados de novo, porque o SHA-256 deles fica registrado.
- Como um envio é processado e como o servidor se mantém estável sob carga: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). A história por trás da versão 3: [AUDIT.md](AUDIT.md). Para relatar uma vulnerabilidade, leia o [SECURITY.md](SECURITY.md).

Moderação: as páginas de administração embutidas foram aposentadas na 3.7.0. Toda página tem um botão Denunciar. A fila de denúncias, ocultar e apagar conteúdo, bloqueios de IP, ações em contas, configurações em tempo de execução e a barra de avisos ficam com a central de controle HOP, que conversa com o PixelHop pela API assinada `/hop/*` (o HOP App Protocol). Ela é usada com o login do HEL.ink, assinada com `HELINK_SSO_SECRET`, e os chamadores permitidos ficam em `HOP_ADMIN_CLIENTS` (padrão `imperium,helink`). Toda alteração feita desse modo é gravada na tabela `hop_audit`. A central de controle em si é privada do projeto HOP. Uma instalação independente tem o botão Denunciar, os comandos `pixelhop user` (`add`, `passwd`, `admin`, `merge`) e os direitos extras do papel de administrador pela API, mas não tem uma tela para a fila de denúncias.

## Configurações

As configurações são variáveis de ambiente, todas opcionais. O [`.env.example`](.env.example) lista as mais comuns com seus padrões; o resto é lido em [`internal/config/config.go`](internal/config/config.go). A tabela mostra as que você mais provavelmente vai mudar. A lista completa está no [README](README.md) em inglês.

| Variável | Padrão | O que faz |
|---|---|---|
| `PIXELHOP_BASE_URL` | `http://localhost:8080` | Endereço público do app (login, envio, painel). Use `https://` em produção, o que também liga os cookies seguros |
| `PIXELHOP_LINK_URL` | `PIXELHOP_BASE_URL` | Outro host para os links compartilhados (páginas de imagem e de álbum, arquivos diretos). Precisa ser um host diferente e também alcançar o PixelHop |
| `PIXELHOP_DATA` | `./data` | Banco de dados e, com armazenamento local, as imagens |
| `PIXELHOP_MAX_UPLOAD_MB` | `20` | Por imagem |
| `PIXELHOP_MAX_MEDIA_MB` | `100` | Por vídeo ou PDF, para membros conectados |
| `PIXELHOP_GUEST_MAX_MB` | `20` | Por arquivo de qualquer tipo, para visitantes |
| `PIXELHOP_GUEST_UPLOADS` | `true` | Permitir envios sem conta |
| `PIXELHOP_GUEST_ALBUM_MAX` | `10` | Imagens em um álbum criado por um visitante (0 = sem limite); também ajustável na central de controle |
| `PIXELHOP_GUEST_DAILY_MAX` | `25` | Arquivos que um endereço de visitante pode enviar em 24 horas (0 = sem limite); também ajustável na central de controle |
| `PIXELHOP_GUEST_TTL_DAYS` | `30` | Dias em que um novo envio de visitante fica no ar (0 = até apagar); arquivos antigos não mudam; também ajustável na central de controle |
| `PIXELHOP_DEFAULT_QUOTA_MB` | `128` | Armazenamento de cada conta nova; as contas existentes mantêm o seu |
| `PIXELHOP_REGISTRATION` | `true` | Permitir novas contas |
| `PIXELHOP_READ_ONLY` | `false` | Pausar os envios para manutenção |
| `PIXELHOP_TRUSTED_PROXIES` | - | `cloudflare`, ou os IPs ou CIDRs do seu proxy reverso |
| `PIXELHOP_STORAGE`, `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY` | `local` | `local` ou `s3`; o armazenamento S3 precisa dos outros quatro |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | - | Entrar com o Google |
| `HELINK_URL`, `HELINK_SSO_SECRET` | - | Entrar com uma conta HEL.ink. O segredo tem pelo menos 32 caracteres e é igual ao `PIXELHOP_SSO_SECRET` no `.env` do HEL.ink |
| `PIXELHOP_CONTROL_URL` | - | Endereço da central de controle HOP; admins que abrem `/admin` são enviados para lá |
| `TURNSTILE_SITE_KEY`, `TURNSTILE_SECRET_KEY` | - | Cloudflare Turnstile no login e no cadastro. Com contas HEL.ink ativas, esses formulários recusam toda tentativa sem as chaves |
| `PIXELHOP_REMBG_URL` | - | Endereço do `tools/ai-server.py`, por exemplo `http://127.0.0.1:7001` |

O segredo do HEL.ink também pode se chamar `HOP_SSO_SECRET_PIXELHOP` ou `HOP_SSO_SECRET`. Ao rotacioná-lo, o segredo anterior continua sendo aceito por `HELINK_SSO_SECRET_PREV` (ou `HOP_SSO_SECRET_PREV`).

### Proxy reverso, serviço e backups

- [`deploy/nginx.conf`](deploy/nginx.conf) é um exemplo de proxy reverso. Defina `PIXELHOP_TRUSTED_PROXIES=127.0.0.1` para o PixelHop confiar no `X-Forwarded-For` do nginx e acrescente `cloudflare` se houver o Cloudflare na frente.
- [`deploy/pixelhop.service`](deploy/pixelhop.service) é uma unidade do systemd reforçada. [`deploy/pixelhop-ai.service`](deploy/pixelhop-ai.service) roda o serviço de IA opcional, que precisa de `rembg`, `opencv-python-headless` e `onnxruntime` (veja o início do [`tools/ai-server.py`](tools/ai-server.py)).
- Para fazer backup, copie a pasta de dados ou rode `sqlite3 data/pixelhop.db ".backup copy.db"` com o PixelHop em execução. O `./update.sh` faz uma cópia assim em `backups/` antes de cada atualização.

## Como usar

1. Abra o site e arraste, cole (<kbd>Ctrl</kbd>+<kbd>V</kbd>) ou informe o link de uma imagem.
2. Opcional: escolha Apagar depois de e se o nome do arquivo entra no link.
3. O link é copiado para você. Abra-o para ver o link direto, os códigos de incorporação e o QR code.

Aperte `?` em qualquer página para ver os atalhos de teclado.

**ShareX.** Vá em Configurações, Chaves de API, crie uma chave, aperte Baixar configuração do ShareX com esta chave e abra o arquivo.

**Linha de comando.** O mesmo programa `pixelhop` envia para qualquer servidor PixelHop. O `pixelhop login` guarda o endereço e a chave em `~/.config/pixelhop/client.json`; `PIXELHOP_SERVER` e `PIXELHOP_KEY` têm prioridade.

```bash
pixelhop login --server https://pixel.hel.ink --key ph_yourkey
pixelhop push screenshot.png photos/*.jpg --expire 7d --format markdown
```

**Celular.** Abra o site e escolha Adicionar à tela inicial. O PixelHop passa a aparecer na folha de compartilhamento.

<a id="api"></a>

## API

```bash
curl -H "Authorization: Bearer ph_yourkey" -F "file=@holiday.jpg" https://pixel.hel.ink/api/v1/upload
```

| Endpoint | O que faz |
|---|---|
| `POST /api/v1/upload` | Envia arquivos (`file`, repetível) ou links (`url`). Opções: `expire` (`10m`, `1h`, `1d`, `7d`, `30d`, `90d`), `slug` (`0` deixa o nome do arquivo fora do link), `title`, `alt`, `album_id`, `album` (`force` ou `0`), `listed`, `content_warning` (`sensitive` ou `adult`), `warning_reason` |
| `POST /api/v1/uploads`, `PUT /api/v1/uploads/{id}`, `POST /api/v1/uploads/{id}/finish` | Envia um arquivo grande em partes de 8 MB |
| `GET /api/v1/images` | Suas imagens (`page`, `limit`, `q`, `sort`) |
| `GET`, `PATCH`, `DELETE /api/v1/images/{id}` | Lê, altera ou apaga uma imagem. O `PATCH` aceita `title`, `alt`, `listed`, `visibility` (`unlisted`, `private` ou `password` com `password`), `tags`, `content_warning` e `warning_reason`. O `DELETE` move seu próprio arquivo para a lixeira; acrescente `?permanent=1` para removê-lo de vez |
| `POST /api/v1/images/{id}/restore` | Traz um arquivo de volta da lixeira |
| `GET /api/v1/images/{id}/stats` | Visualizações por dia e sites de origem |
| `GET`, `POST /api/v1/albums` / `GET`, `PATCH`, `DELETE /api/v1/albums/{id}` | Lista, cria, lê, edita e apaga álbuns |
| `GET /api/v1/users/{username}/images` | Público: as imagens do perfil de uma pessoa |
| `GET /api/v1/me` | Sua conta e o uso de armazenamento |
| `GET /api/v1/me/media` | Suas próprias imagens (`type=image`) ou PDFs (`type=pdf`) que outra página pode mostrar: sem arquivos privados, com senha, com validade ou com aviso de conteúdo (`cursor`, `limit`). Também aberto aos apps HOP listados em `PIXELHOP_MEDIA_CLIENTS` (o seletor de mídia do BioHop), assinado de servidor para servidor em nome de um membro |
| `GET /api/v1/stats` | Totais públicos e a versão do servidor |
| `GET /api/v1/upload-status` | Se o servidor está aceitando envios agora |
| `POST /api/v1/tools/{remove-background,faces,erase,upscale}` | Ferramentas de IA no servidor (conectado; `file`, mais `mask` para apagar) |
| `GET /i/{id}.jpg?w=&h=&fit=&format=&q=` | Cópias redimensionadas, geradas uma vez e guardadas em cache. `format=webp` e `avif` precisam do serviço de IA |
| `GET /oembed?url=...` | oEmbed para páginas de imagem |

Toda resposta tem o formato `{"success": true, "data": ...}` ou `{"success": false, "error": "..."}`. A referência completa está em [`/docs`](https://pixel.hel.ink/docs) em qualquer servidor PixelHop.

<a id="upgrading"></a>

## Atualizar do PixelHop 2

O PixelHop 2 era um app PHP com MySQL, um venv do Python e tarefas do cron. O código dele continua na tag [`v2.3.0-php`](../../tree/v2.3.0-php). Um comando traz os dados:

```bash
pixelhop import --images /var/www/pichost/data/images.json \
  --mysql "user:pass@tcp(127.0.0.1:3306)/pixelhop" --copy
```

- Os usuários mantêm seus ids, papéis, cotas e senhas. Os hashes argon2id do PHP são verificados e atualizados no próximo login. As contas do Google continuam vinculadas.
- As imagens mantêm seus ids. Com `--copy`, elas são baixadas para o armazenamento do PixelHop 3 com miniaturas novas; sem ele, continuam sendo servidas pelos buckets antigos.
- Os links antigos continuam funcionando: `/{id}`, `/i/2026/08/27/{id}_original.webp`, `/auth/google-callback` (defina `GOOGLE_REDIRECT_URI`) e configurações do ShareX que enviam para `/api/upload.php`.

O [AUDIT.md](AUDIT.md) explica por que ele foi reescrito.

## Desenvolvimento

```
cmd/pixelhop/     o comando: serve, user, import, push, login
internal/app/     servidor web: páginas, API, envios, moderação, HOP App Protocol
internal/db/      esquema do SQLite, migrações e consultas
internal/imaging/ detecção de tipo, remoção de metadados, miniaturas
internal/storage/ disco local, S3 (assinador SigV4 próprio), URLs antigas
internal/netsafe/ busca à prova de SSRF para "Enviar por link"
web/              modelos, scripts e os arquivos dos nove idiomas, embutidos no binário
docs/             o site do GitHub Pages e as imagens do README
scripts/          scripts auxiliares (imagens do README, dados do changelog, verificação da família HOP)
tools/            o serviço de IA opcional
deploy/           exemplos de nginx e systemd
```

`make test` roda `go vet` e os testes, e `make build` gera `./pixelhop`. O `scripts/readme-images.mjs` refaz as capturas do README a partir de uma instância em execução (veja o comentário no início do arquivo). O `python3 scripts/build-changelog.py --repo PixelHop` refaz o `web/changelog.json`, os dados por trás da página `/changelog`, a partir dos pull requests já mesclados; ele precisa de um `gh` autenticado.

## Perguntas frequentes

<details><summary>O estúdio é mesmo privado?</summary>

A edição acontece no seu dispositivo, com Canvas e WebAssembly. Na primeira vez que você usa o reconhecimento de texto ou a remoção de fundo, o navegador baixa o [tesseract.js](https://github.com/naptha/tesseract.js) e o [@imgly/background-removal](https://github.com/imgly/background-removal-js) do jsDelivr, e os dados de idioma e os modelos de tessdata.projectnaptha.com e staticimgly.com. Sua imagem não é enviada a lugar nenhum, a menos que você aperte Enviar resultado. Um servidor próprio que defina uma Content Security Policy mais estrita precisa permitir esses endereços.
</details>

<details><summary>Posso desligar os envios de visitantes ou os cadastros?</summary>

Pode. Defina `PIXELHOP_GUEST_UPLOADS=false` ou `PIXELHOP_REGISTRATION=false`. Com a central de controle HOP, também dá para mudar os dois com o servidor em execução.
</details>

<details><summary>O que ele exige de recursos?</summary>

Um servidor pequeno basta. Um processo `pixelhop` recém-iniciado usou cerca de 16 MB de memória em um teste no macOS, e os binários das versões têm de 16 a 17 MB. Imagens grandes são decodificadas poucas por vez, então uma rajada de envios não consegue usar toda a memória.
</details>

## A família HOP

O PixelHop faz parte da família HOP de apps de código aberto que respeitam a privacidade e podem ser hospedados por você mesmo.

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

## Hospedagem própria

<!-- self-hosting:start -->
Tudo o que o PixelHop precisa está em repositórios públicos: nenhum repositório privado, nenhum token e nenhuma conta no hel.ink. Os detalhes estão no guia da família: [docs/self-hosting.md](https://github.com/navi-crwn/HEL.ink/blob/main/docs/self-hosting.md) (em inglês).

| Peça | Para o PixelHop | O que faz |
| --- | --- | --- |
| [HEL.ink](https://github.com/navi-crwn/HEL.ink) (hub, HOP ID) | Opcional: adiciona “Entrar com HOP ID” | Contas e login para todos os apps |
| [Imperium](https://github.com/navi-crwn/Imperium) | Opcional, mas as páginas de administração do PixelHop ficam lá | Painel de administração de toda a família; precisa do hub |
| Fontes e blocos da família | Nada a instalar: carregados de `hel.ink/hop-assets` | Sem hel.ink: fontes do sistema, sem blocos flutuantes |

- **Ordem de instalação**: o PixelHop sozinho com `./install.sh`. Para login com HOP ID, primeiro o [hub](https://github.com/navi-crwn/HEL.ink): `./install.sh --hub new` instala um ao lado.
- **Configuração mínima**: `PIXELHOP_BASE_URL`; com o hub: `HELINK_URL`, `HELINK_SSO_SECRET`; com o Imperium: `PIXELHOP_CONTROL_URL`.
- **Sozinho ou na família**: o PixelHop funciona sozinho, com contas próprias. Com um hub, as pessoas também podem entrar com HOP ID e sair de todos os apps de uma vez.
- **Sem o Imperium**: `/admin` mostra um aviso. Use `pixelhop user add` e a API assinada `/hop/*` (a primeira conta é a de administrador).
<!-- self-hosting:end -->

## Créditos e licença

- [MIT](LICENSE.md), copyright Ivan Novskies (navi-crwn). As partes de terceiros estão listadas em [CREDITS.md](CREDITS.md).
- Feito com [Go](https://go.dev), [modernc.org/sqlite](https://gitlab.com/cznic/sqlite), [golang.org/x/image](https://pkg.go.dev/golang.org/x/image) e [rsc.io/qr](https://pkg.go.dev/rsc.io/qr).
- O estúdio carrega, quando preciso, o [tesseract.js](https://github.com/naptha/tesseract.js) (Apache-2.0) e o [@imgly/background-removal](https://github.com/imgly/background-removal-js) (AGPL-3.0, sem modificações, do jsDelivr).

Mais do mesmo autor: [Amnesia](https://github.com/navi-crwn/amnesia), um Mac que esquece tudo ao sair.
