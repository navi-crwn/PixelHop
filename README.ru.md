<p align="center">🌐 <a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.de.md">Deutsch</a> · <a href="README.id.md">Bahasa Indonesia</a> · <a href="README.ja.md">日本語</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.es.md">Español</a> · <a href="README.fr.md">Français</a> · <b>Русский</b></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/hero-en-dark.webp"><img src="docs/readme/hero-en-light.webp" alt="PixelHop: drop an image, get a link" width="100%"></picture></p>

<p align="center"><b>Хостинг изображений с открытым кодом: убирает GPS-координаты до того, как кто-то увидит снимок, а фотостудия никогда не загружает ваше фото. Одна небольшая программа на Go.</b></p>

---

PixelHop размещает изображения с альбомами, прямыми ссылками и самоудалением. Он убирает геоданные из фотографий, а фотостудия работает целиком в браузере. Достаточно одной программы на Go и одного файла SQLite.

## Установка

Установка одной командой (сначала проверяет ваш компьютер, задаёт несколько вопросов и может сам установить хаб HOP ID):

```bash
curl -fsSL https://raw.githubusercontent.com/navi-crwn/PixelHop/main/install.sh | bash
```

Из клона: `./install.sh` · обновление: `./update.sh` · удаление: `./uninstall.sh`. С `--yes` берутся все значения по умолчанию; `--help` показывает параметры.

## Подробнее

Полная документация (возможности, настройки, безопасность) — на английском в [README](README.md).

Лицензия: MIT.
