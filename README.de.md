<p align="center">🌐 <a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a> · <b>Deutsch</b> · <a href="README.id.md">Bahasa Indonesia</a> · <a href="README.ja.md">日本語</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.es.md">Español</a> · <a href="README.fr.md">Français</a> · <a href="README.ru.md">Русский</a></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/readme/hero-en-dark.webp"><img src="docs/readme/hero-en-light.webp" alt="PixelHop: drop an image, get a link" width="100%"></picture></p>

<p align="center"><b>Open-Source-Bilderhosting, das deinen GPS-Standort entfernt, bevor jemand das Bild sieht, mit einem Fotostudio, das dein Foto nie hochlädt. Ein kleines Go-Programm.</b></p>

---

PixelHop hostet Bilder mit Alben, direkten Links und Selbstzerstörung. Es entfernt Standortdaten aus den Fotos, und das Fotostudio arbeitet vollständig im Browser. Ein Go-Programm und eine SQLite-Datei genügen.

## Installation

Mit einem Befehl installieren (es prüft zuerst deinen Rechner, stellt ein paar Fragen und kann auch den HOP-ID-Hub für dich einrichten):

```bash
curl -fsSL https://raw.githubusercontent.com/navi-crwn/PixelHop/main/install.sh | bash
```

Aus einem Klon: `./install.sh` · aktualisieren: `./update.sh` · entfernen: `./uninstall.sh`. Mit `--yes` gelten alle Standardwerte; `--help` zeigt die Optionen.

## Mehr erfahren

Die vollständige Dokumentation (Funktionen, Einstellungen, Sicherheit) steht auf Englisch in der [README](README.md).

Lizenz: MIT.
