# Credits

## PixelHop 3

| Component | License | Link |
|---|---|---|
| Go standard library | BSD-3-Clause | [go.dev](https://go.dev) |
| modernc.org/sqlite (pure-Go SQLite) | BSD-3-Clause | [gitlab.com/cznic/sqlite](https://gitlab.com/cznic/sqlite) |
| golang.org/x/image, x/crypto | BSD-3-Clause | [pkg.go.dev/golang.org/x](https://pkg.go.dev/golang.org/x/image) |
| rsc.io/qr | BSD-3-Clause | [pkg.go.dev/rsc.io/qr](https://pkg.go.dev/rsc.io/qr) |
| go-sql-driver/mysql (only for importing PixelHop 2) | MPL-2.0 | [github.com/go-sql-driver/mysql](https://github.com/go-sql-driver/mysql) |

### Bundled with the program (web/static/vendor/tesseract)

| Library | License | Used for |
|---|---|---|
| [tesseract.js](https://github.com/naptha/tesseract.js) 5.1.1 | Apache-2.0 | Image to text (OCR), unmodified |
| [tesseract.js-core](https://github.com/naptha/tesseract.js-core) 5.1.0 | Apache-2.0 | The WebAssembly OCR engine, unmodified |
| English model (tessdata 4.0.0_best_int, via @tesseract.js-data/eng 1.0.0) | Apache-2.0 | OCR for English |

The licence texts sit next to the files.

Nothing is loaded from other websites any more. Background removal runs on the server (rembg, MIT); the AGPL-licensed @imgly/background-removal is no longer used.

Icons and the pixel-bunny logo were drawn for PixelHop. The GitHub mark is from Octicons (MIT).

PixelHop 2 (PHP, tag `v2.3.0-php`) also used PHP, MySQL, TailwindCSS, Lucide, Tesseract, rembg, Pillow and Cloudflare Turnstile.

## Contributing

Pull requests are welcome. Please open an issue first to discuss bigger changes, and run `make test` before sending.

- Found a bug? [Open an issue](https://github.com/navi-crwn/pixelhop/issues)

---

<div align="center">

Built by [navi-crwn](https://github.com/navi-crwn)

</div>

## Brand name fonts

The app names in the header and footer each use their own typeface, as name-only subsets, all under the SIL Open Font License 1.1: Syne (HOP ID), Pixelify Sans (LinkHop), Space Grotesk (BioHop), Silkscreen (PixelHop), Caveat (NoteHop), Sora (FileHop) and Unbounded (PayHop). The licence texts sit next to the files in `web/static/hop-brand/fonts/brand/`; The Unbounded subset (`payhop-name.woff2`) and its licence ship next to the Silkscreen one.
