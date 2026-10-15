// Renders the README images (docs/readme/*.webp), the website's social card
// (docs/social.jpg) and the app's link-preview card (web/static/img/social.png)
// from the GitHub Pages site in docs/, so the README and the website always match.
//
//   npm install && npx playwright install chromium
//   node scripts/readme-images.mjs
import { createServer } from 'node:http';
import { readFile, writeFile, mkdir } from 'node:fs/promises';
import { extname, join, normalize } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

const root = fileURLToPath(new URL('../', import.meta.url));
const docs = join(root, 'docs');
const types = { '.html': 'text/html', '.css': 'text/css', '.js': 'text/javascript', '.svg': 'image/svg+xml', '.webp': 'image/webp', '.jpg': 'image/jpeg', '.png': 'image/png' };

const server = createServer(async (req, res) => {
  const path = normalize(decodeURIComponent(new URL(req.url, 'http://x').pathname)).replace(/^([/\\])+/, '');
  try {
    const body = await readFile(join(docs, path === '' || path.endsWith('/') ? path + 'index.html' : path));
    res.writeHead(200, { 'Content-Type': types[extname(path) || '.html'] || 'application/octet-stream' }).end(body);
  } catch { res.writeHead(404).end(); }
}).listen(0);
const base = `http://127.0.0.1:${server.address().port}/`;

// section id on the site -> image name in docs/readme
const shots = { top: 'hero', features: 'features', studio: 'studio' };

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1200, height: 900 }, deviceScaleFactor: 2 });

// Chromium encodes WebP/JPEG through a canvas, so no image library is needed.
async function encode(png, type, quality) {
  const data = await page.evaluate(async ([b64, type, quality]) => {
    const img = new Image(); img.src = 'data:image/png;base64,' + b64; await img.decode();
    const c = document.createElement('canvas'); c.width = img.width; c.height = img.height;
    c.getContext('2d').drawImage(img, 0, 0);
    return c.toDataURL(type, quality).split(',')[1];
  }, [png.toString('base64'), type, quality]);
  return Buffer.from(data, 'base64');
}

await mkdir(join(docs, 'readme'), { recursive: true });
for (const lang of ['en', 'id']) {
  for (const theme of ['dark', 'light']) {
    await page.goto(`${base}?shot&lang=${lang}&theme=${theme}`, { waitUntil: 'networkidle' });
    await page.evaluate(() => document.fonts.ready);
    for (const [id, name] of Object.entries(shots)) {
      const png = await page.locator('#' + id).screenshot({ animations: 'disabled' });
      await writeFile(join(docs, 'readme', `${name}-${lang}-${theme}.webp`), await encode(png, 'image/webp', 0.86));
      console.log(`${name}-${lang}-${theme}.webp`);
    }
  }
}

// 1200×630 social cards: the hero, cropped.
await page.setViewportSize({ width: 1200, height: 630 });
await page.goto(`${base}?shot&lang=en&theme=dark`, { waitUntil: 'networkidle' });
await page.evaluate(() => document.fonts.ready);
const card = await page.screenshot({ clip: { x: 0, y: 0, width: 1200, height: 630 } });
await writeFile(join(docs, 'social.jpg'), await encode(card, 'image/jpeg', 0.88));
await writeFile(join(root, 'web/static/img/social.jpg'), await encode(card, 'image/jpeg', 0.88));
console.log('social cards');

await browser.close();
server.close();
