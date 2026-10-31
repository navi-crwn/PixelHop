<p align="center">🌐 <a href="README.md">English</a> · <a href="README.zh.md">简体中文</a> · <a href="README.es.md">Español</a> · <b>العربية</b> · <a href="README.pt.md">Português</a> · <a href="README.fr.md">Français</a> · <a href="README.ja.md">日本語</a> · <a href="README.id.md">Bahasa Indonesia</a></p>

# PixelHop

<p align="center">
  <a href="LICENSE.md"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License MIT"></a>
  <a href="https://github.com/navi-crwn/PixelHop/actions"><img src="https://img.shields.io/github/actions/workflow/status/navi-crwn/PixelHop/ci.yml?branch=main&style=flat-square&label=tests" alt="Tests"></a>
</p>

<p align="center"><picture><source media="(prefers-color-scheme: light)" srcset="docs/readme/hero-en-light.webp"><img src="docs/readme/hero-en-dark.webp" alt="PixelHop: drop an image, get a link" width="100%"></picture></p>

<p align="center">
  <b>استضافة صور مفتوحة المصدر تجرد إحداثيات GPS وبيانات الكاميرا قبل أن يراها أحد، مع استوديو صور خاص داخل المتصفح. ملف تنفيذي واحد وخفيف بلغة Go.</b>
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

## ما هو PixelHop؟

### مشاركة الصور
- أفلت صورتك أو الصقها للحصول على رابط قصير ونظيف ومقروء.
- تحمل صور الهواتف إحداثيات GPS وبيانات الكاميرا، وPixelHop **يزيل هذه البيانات المخفية** دون إعادة ضغط البكسلات.
- رفع عدة صور دفعة واحدة ينشئ **ألبوماً** متكاملاً.

### تعديل الصور
- **الاستوديو** المدمج يتيح ضغط الصور وتغيير حجمها وقصها وتحويل صيغها.
- يشوش على الوجوه، ويتعرف على النصوص (OCR)، ويزيل الخلفيات.
- يعمل كل ذلك **داخل متصفحك** مباشرة لتبقى الصورة على جهازك.

### الاستضافة الذاتية
- برنامج مستقل بحجم ~16 ميغابايت مع قاعدة بيانات مدمجة.
- انسخه إلى الخادم وشغله مباشرة. دون الحاجة إلى PHP أو Python أو MySQL أو cron.

<p align="center"><picture><source media="(prefers-color-scheme: light)" srcset="docs/readme/features-en-light.webp"><img src="docs/readme/features-en-dark.webp" alt="Features" width="100%"></picture></p>

## المميزات

- **📍 إزالة الموقع والحفاظ على الجودة.** تجريد EXIF وXMP وIPTC دون إعادة تشفير البكسلات الأصلية.
- **🔗 روابط متعددة وشاملة.** معرفات نظيفة مثل `beach-sunset-x7Kp2Q`، وروابط مباشرة، وكود Markdown، وHTML، وBBCode، ورمز QR.
- **🗂️ ألبومات متجاوبة.** ارفع عدة صور وشاركها في معرض صور واحد.
- **⏳ تدمير ذاتي.** حذف تلقائي بعد 10 دقائق أو ساعة أو يوم أو شهر مع رمز حذف خاص للزوار.
- **🎨 استوديو متصفح خاص.** ضغط، وتحويل لصيغ WebP/AVIF، وتشويش الوجوه، واستخراج النصوص دون رفع مسبق.
- **👤 حسابات مستقلة أو دخول موحد عبر HOP ID.** تسجيل دخول موحد لكافة تطبيقات HOP أو وضع زائر.
- **🔌 دعم API و ShareX.** ملفات إعداد جاهزة لـ ShareX، ومفاتيح REST API، وأداة `pixelhop push` لسطر الأوامر.
- **🏠 استضافة ذاتية ومصدر مفتوح.** ملف تنفيذي Go أو Docker أو systemd. تخزين محلي أو على S3/R2.

## التثبيت

حمل الملف التنفيذي المترجم أو شغّله عبر Docker:

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

## عائلة HOP

PixelHop جزء من عائلة **HOP** للتطبيقات مفتوحة المصدر ذات الاستضافة الذاتية وحماية الخصوصية.

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

## الحقوق والترخيص

- [MIT](LICENSE.md) © Ivan Novskies (navi-crwn).
- مبني باستخدام [Go](https://go.dev) و SQLite وتقنيات الويب الحديثة.
