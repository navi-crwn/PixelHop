// Renders the README pictures from a running PixelHop with demo data.
//
//   1. Start PixelHop on an empty data folder (the demo account becomes the admin):
//        PIXELHOP_DATA=/tmp/ph-demo PIXELHOP_ADDR=127.0.0.1:8317 \
//        PIXELHOP_BASE_URL=http://localhost:8317 ./pixelhop
//   2. From the repository root:
//        npm install && PH_URL=http://localhost:8317 npm run readme-images
//
// The script creates a demo member, uploads six generated landscapes (see
// scripts/demo-images) and writes docs/readme/{home,dashboard,image,album,studio}-{dark,light}.webp.
// Only demo data is used. Set PH_CHANNEL=chrome to use an installed Chrome instead of
// the bundled Chromium. Use a server that has no real accounts.
import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

const root = fileURLToPath(new URL('../', import.meta.url));
const base = (process.env.PH_URL || 'http://localhost:8080').replace(/\/$/, '');
const out = process.env.PH_OUT || join(root, 'docs', 'readme');
const work = mkdtempSync(join(tmpdir(), 'pixelhop-readme-'));
const imgDir = process.env.PH_IMAGES || join(work, 'images');
const statePath = join(work, 'state.json');
const demo = { name: 'Maya Demo', email: 'demo@example.test', password: 'demo-pass-2026-local' };

if (!existsSync(join(imgDir, 'harbour-at-dusk.jpg'))) {
  execFileSync('go', ['run', './scripts/demo-images', imgDir], { cwd: root, stdio: 'inherit' });
}
mkdirSync(out, { recursive: true });
const file = (n) => join(imgDir, n);

const browser = await chromium.launch(process.env.PH_CHANNEL ? { channel: process.env.PH_CHANNEL } : {});
const newContext = (theme) => browser.newContext({
  locale: 'en-US', viewport: { width: 1280, height: 800 }, deviceScaleFactor: 1.5,
  colorScheme: theme, reducedMotion: 'reduce',
});

async function themed(ctx, theme) {
  await ctx.addInitScript((t) => { try { localStorage.setItem('theme', t); } catch (e) {} }, theme);
}

// Turns a screenshot into WebP with the browser's own encoder (no extra tools).
const encoder = await (await newContext('light')).newPage();
async function encode(png) {
  const b64 = await encoder.evaluate(async (b64) => {
    const img = new Image();
    img.src = 'data:image/png;base64,' + b64;
    await img.decode();
    const c = document.createElement('canvas');
    c.width = img.width; c.height = img.height;
    c.getContext('2d').drawImage(img, 0, 0);
    return c.toDataURL('image/webp', 0.8).split(',')[1];
  }, png.toString('base64'));
  return Buffer.from(b64, 'base64');
}
async function save(page, name, theme) {
  await page.waitForTimeout(700);
  const png = await page.screenshot({ animations: 'disabled' });
  const webp = await encode(png);
  writeFileSync(join(out, `${name}-${theme}.webp`), webp);
  console.log(`${name}-${theme}.webp`, Math.round(webp.length / 1024), 'KB');
}

// --- Seed: a member with two single photos (with made-up EXIF) and an album of four.
const seedCtx = await newContext('dark');
const seed = await seedCtx.newPage();
await seed.goto(`${base}/register?lang=en`);
await seed.fill('input[name=name]', demo.name);
await seed.fill('input[name=email]', demo.email);
await seed.fill('input[name=password]', demo.password);
await Promise.all([seed.waitForNavigation(), seed.click('form button[type=submit]')]);
await seed.goto(`${base}/dashboard?lang=en`);

const upload = (names, fields) => seed.evaluate(async ({ items, fields }) => {
  const fd = new FormData();
  for (const [k, v] of Object.entries(fields)) fd.append(k, v);
  for (const it of items) {
    const bin = Uint8Array.from(atob(it.b64), (c) => c.charCodeAt(0));
    fd.append('file', new Blob([bin], { type: 'image/jpeg' }), it.name);
  }
  const r = await fetch('/api/v1/upload', { method: 'POST', body: fd });
  return r.json();
}, { items: names.map((n) => ({ name: n, b64: readFileSync(file(n)).toString('base64') })), fields });

const single = {};
for (const n of ['harbour-at-dusk.jpg', 'mountain-ridge.jpg']) single[n] = (await upload([n], {})).data;
const trip = (await upload(['desert-dunes.jpg', 'forest-lake.jpg', 'night-city.jpg', 'autumn-hills.jpg'], { title: 'Weekend trip' })).data;
const patch = (id, body) => seed.evaluate(({ id, body }) => fetch('/api/v1/images/' + id, {
  method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body),
}).then((r) => r.status), { id, body });
await patch(single['harbour-at-dusk.jpg'].id, { title: 'Harbour at dusk', alt: 'Sun setting behind purple hills', tags: ['sunset', 'travel'] });
await patch(single['mountain-ridge.jpg'].id, { title: 'Mountain ridge', tags: ['travel'] });
for (const im of trip.images) await patch(im.id, { tags: ['trip'] });
const pageUrl = single['harbour-at-dusk.jpg'].links?.page || `${base}/${single['harbour-at-dusk.jpg'].id}`;
const albumUrl = trip.album_url;
await seedCtx.storageState({ path: statePath });
await seedCtx.close();

for (const theme of ['dark', 'light']) {
  // Home: a guest upload that shows what was removed.
  const guest = await newContext(theme);
  await themed(guest, theme);
  let page = await guest.newPage();
  await page.goto(`${base}/?lang=en`);
  await page.waitForLoadState('networkidle');
  await page.setInputFiles('input[type=file]', file('mountain-ridge.jpg'));
  await page.waitForSelector('text=Removed hidden data');
  await page.waitForTimeout(3500); // let the "Copied" toast go
  await save(page, 'home', theme);
  await guest.close();

  // Signed-in screens.
  const ctx = await browser.newContext({
    locale: 'en-US', viewport: { width: 1280, height: 800 }, deviceScaleFactor: 1.5,
    colorScheme: theme, reducedMotion: 'reduce', storageState: statePath,
  });
  await themed(ctx, theme);
  page = await ctx.newPage();

  await page.goto(`${base}/dashboard?lang=en`);
  await page.waitForLoadState('networkidle');
  await page.evaluate(() => { const h = Array.from(document.querySelectorAll('h2,h3')).find((e) => /my files/i.test(e.textContent)); if (h) window.scrollTo(0, h.getBoundingClientRect().top + scrollY - 200); });
  await save(page, 'dashboard', theme);

  await page.goto(pageUrl);
  await page.waitForLoadState('networkidle');
  await save(page, 'image', theme);

  await page.goto(albumUrl);
  await page.waitForLoadState('networkidle');
  await save(page, 'album', theme);

  await page.goto(`${base}/tools#compress`);
  await page.waitForLoadState('networkidle');
  await page.setInputFiles('#st-file', file('harbour-at-dusk.jpg'));
  await page.waitForSelector('#st-export:not(.hide)');
  await save(page, 'studio', theme);
  await ctx.close();
}

await browser.close();
rmSync(work, { recursive: true, force: true });
console.log('done');
