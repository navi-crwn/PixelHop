# MIT License

Copyright (c) 2025-2026 Ivan Novskies (navi-crwn)

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

---

# Credits & Acknowledgments

## PixelHop 3

| Component | License | Link |
|---|---|---|
| Go standard library | BSD-3-Clause | [go.dev](https://go.dev) |
| modernc.org/sqlite (pure-Go SQLite) | BSD-3-Clause | [gitlab.com/cznic/sqlite](https://gitlab.com/cznic/sqlite) |
| golang.org/x/image, x/crypto | BSD-3-Clause | [pkg.go.dev/golang.org/x](https://pkg.go.dev/golang.org/x/image) |
| rsc.io/qr | BSD-3-Clause | [pkg.go.dev/rsc.io/qr](https://pkg.go.dev/rsc.io/qr) |
| go-sql-driver/mysql (only for importing PixelHop 2) | MPL-2.0 | [github.com/go-sql-driver/mysql](https://github.com/go-sql-driver/mysql) |

### Loaded in the browser on demand (not bundled)

| Library | License | Used for |
|---|---|---|
| [tesseract.js](https://github.com/naptha/tesseract.js) | Apache-2.0 | Image to text (OCR) |
| [@imgly/background-removal](https://github.com/imgly/background-removal-js) | AGPL-3.0 | Background removal; loaded unmodified from jsDelivr |

Icons and the pixel-bunny logo were drawn for PixelHop. The GitHub mark is from Octicons (MIT).

PixelHop 2 (PHP, in `legacy/`) also used PHP, MySQL, TailwindCSS, Lucide, Tesseract, rembg, Pillow and Cloudflare Turnstile.

## Contributing

Pull requests are welcome. Please open an issue first to discuss bigger changes, and run `make test` before sending.

- Found a bug? [Open an issue](https://github.com/navi-crwn/pixelhop/issues)

---

<div align="center">

Built by [navi-crwn](https://github.com/navi-crwn)

</div>
