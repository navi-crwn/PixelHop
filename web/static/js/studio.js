// PixelHop Studio — image tools that run entirely in the browser.
// Canvas does the pixel work; OCR (tesseract.js, Apache-2.0) and background
// removal (@imgly/background-removal, AGPL-3.0, loaded unmodified from a CDN)
// are fetched only when someone opens those tools.
(function () {
  'use strict';
  var $ = function (s, el) { return (el || document).querySelector(s); };
  var $$ = function (s, el) { return Array.prototype.slice.call((el || document).querySelectorAll(s)); };
  var toast = function (m, e) { window.phToast && window.phToast(m, e); };

  var TESSERACT = 'https://cdn.jsdelivr.net/npm/tesseract.js@5.1.1/dist/tesseract.min.js';
  var REMBG = 'https://cdn.jsdelivr.net/npm/@imgly/background-removal@1.7.0/+esm';

  var box = $('#st-box'), view = $('#st-canvas'), empty = $('#st-empty'), toolBox = $('#st-tool');
  var cropEl = $('#st-crop'), busyEl = $('#st-busy');

  var cur = null;          // working canvas (full resolution)
  var undo = [];           // undo stack of canvases
  var original = null;     // first loaded canvas, for "start over"
  var srcName = 'image', srcType = 'image/png', srcSize = 0;
  var out = { type: null, quality: 0.82 }; // export settings (null type = same as source)
  var tool = null, overlay = null;

  // ---------- loading ----------
  function canvasFrom(src) {
    var c = document.createElement('canvas');
    c.width = src.width; c.height = src.height;
    c.getContext('2d').drawImage(src, 0, 0);
    return c;
  }
  function clone(c) { return canvasFrom(c); }

  function load(blob, name) {
    if (!blob || !/^image\//.test(blob.type)) { toast('That is not an image', true); return; }
    busy('Opening…');
    createImageBitmap(blob, { imageOrientation: 'from-image' }).then(function (bmp) {
      cur = canvasFrom(bmp); original = clone(cur); undo = [];
      srcName = (name || 'image').replace(/\.[a-z0-9]+$/i, ''); srcType = blob.type; srcSize = blob.size;
      out.type = null;
      show(); busy(false);
      $('#st-export').classList.remove('hide');
      $('#st-uploaded').classList.add('hide');
      if (!tool) pick(location.hash.slice(1) || 'compress'); else pick(tool.id);
    }).catch(function () { busy(false); toast('This image could not be opened', true); });
  }

  $('#st-open').addEventListener('click', function () { $('#st-file').click(); });
  empty.addEventListener('click', function () { $('#st-file').click(); });
  empty.addEventListener('keydown', function (e) { if (e.key === 'Enter') $('#st-file').click(); });
  $('#st-file').addEventListener('change', function () { var f = this.files[0]; if (f) load(f, f.name); this.value = ''; });
  document.addEventListener('dragover', function (e) { e.preventDefault(); });
  document.addEventListener('drop', function (e) {
    e.preventDefault();
    var f = e.dataTransfer.files[0]; if (f) load(f, f.name);
  });
  document.addEventListener('paste', function (e) {
    if (e.target.closest && e.target.closest('input,textarea')) return;
    var it = Array.prototype.filter.call(e.clipboardData.items || [], function (i) { return i.kind === 'file'; })[0];
    if (it) { e.preventDefault(); load(it.getAsFile(), 'pasted'); }
  });
  var src = new URLSearchParams(location.search).get('src');
  if (src && /^[A-Za-z0-9_-]+$/.test(src)) {
    fetch('/i/' + src + '.img').then(function (r) { if (!r.ok) throw 0; return r.blob(); })
      .then(function (b) { load(b, src); }).catch(function () { toast('Could not open that image', true); });
  }

  // ---------- display ----------
  function show() {
    empty.classList.add('hide'); view.classList.remove('hide');
    view.width = cur.width; view.height = cur.height;
    view.getContext('2d').drawImage(cur, 0, 0);
    view.style.filter = '';
    $('#st-undo').disabled = undo.length === 0;
    $('#st-reset').disabled = !original;
    if ($('#st-compare')) $('#st-compare').disabled = !original;
    info();
    if (overlay && overlay.refresh) overlay.refresh();
  }
  function commit(next) {
    undo.push(cur); if (undo.length > 12) undo.shift();
    cur = next; show();
  }
  $('#st-undo').addEventListener('click', function () { if (undo.length) { cur = undo.pop(); show(); if (tool) pick(tool.id); } });
  $('#st-reset').addEventListener('click', function () { if (original) { undo.push(cur); cur = clone(original); show(); if (tool) pick(tool.id); } });

  function busy(text) {
    busyEl.classList.toggle('hide', !text);
    if (text) $('#st-busy-text').textContent = text;
  }

  // ---------- export ----------
  var EXT = { 'image/jpeg': 'jpg', 'image/png': 'png', 'image/webp': 'webp', 'image/avif': 'avif' };
  function outType() {
    var t = out.type || srcType;
    return EXT[t] && canEncode(t) ? t : 'image/png';
  }
  var encodeCache = {};
  function canEncode(t) {
    if (t in encodeCache) return encodeCache[t];
    var c = document.createElement('canvas'); c.width = c.height = 1;
    return (encodeCache[t] = c.toDataURL(t).indexOf('data:' + t) === 0);
  }
  function exportBlob() {
    return new Promise(function (res, rej) {
      var t = outType(), c = cur;
      if (t === 'image/jpeg') { // JPEG has no transparency: put it on white
        c = document.createElement('canvas'); c.width = cur.width; c.height = cur.height;
        var x = c.getContext('2d'); x.fillStyle = '#fff'; x.fillRect(0, 0, c.width, c.height); x.drawImage(cur, 0, 0);
      }
      c.toBlob(function (b) { b ? res(b) : rej(new Error('encode')); }, t, t === 'image/png' ? undefined : out.quality);
    });
  }
  var infoTimer;
  function info() {
    clearTimeout(infoTimer);
    var el = $('#st-info');
    el.innerHTML = '<span>' + cur.width + ' × ' + cur.height + '</span><span>measuring…</span>';
    infoTimer = setTimeout(function () {
      exportBlob().then(function (b) {
        var diff = srcSize ? Math.round((1 - b.size / srcSize) * 100) : 0;
        var cls = diff > 0 ? 'down' : '';
        el.innerHTML = '<span>' + cur.width + ' × ' + cur.height + ' · ' + EXT[outType()].toUpperCase() + '</span><span><b>' + size(b.size) + '</b>' +
          (srcSize ? ' <span class="' + cls + '">(' + (diff > 0 ? '−' + diff : '+' + (-diff)) + '%)</span>' : '') + '</span>';
      });
    }, 250);
  }
  function size(n) { return n < 1024 ? n + ' B' : n < 1048576 ? (n / 1024).toFixed(0) + ' KB' : (n / 1048576).toFixed(2) + ' MB'; }

  $('#st-download').addEventListener('click', function () {
    exportBlob().then(function (b) { window.phSave(b, srcName + '-pixelhop.' + EXT[outType()]); });
  });
  $('#st-upload').addEventListener('click', function () {
    var btn = this; btn.disabled = true;
    exportBlob().then(function (b) {
      if (outType() === 'image/avif') throw new Error('AVIF can be downloaded but not hosted yet: pick JPG, PNG or WebP to upload.');
      var fd = new FormData(); fd.append('file', b, srcName + '.' + EXT[outType()]);
      return fetch('/api/v1/upload', { method: 'POST', body: fd, headers: { Accept: 'application/json' } }).then(function (r) { return r.json(); });
    }).then(function (j) {
      if (!j.success) throw new Error(j.error);
      $('#st-uploaded').classList.remove('hide');
      $('#st-link').value = j.data.view_url;
      window.phCopy(j.data.view_url);
      try {
        var h = JSON.parse(localStorage.getItem('ph_history') || '[]');
        h.unshift({ id: j.data.id, page: j.data.view_url, thumb: j.data.thumb_url, del: j.data.delete_url || '', t: Date.now(), title: j.data.title });
        localStorage.setItem('ph_history', JSON.stringify(h.slice(0, 200)));
      } catch (e) {}
    }).catch(function (e) { toast(e.message || 'Upload failed', true); })
      .then(function () { btn.disabled = false; });
  });

  // ---------- tool switching ----------
  var tools = {};
  function pick(id) {
    if (!tools[id]) id = 'compress';
    if (overlay && overlay.off) overlay.off();
    overlay = null;
    view.style.filter = '';
    if (cur) view.getContext('2d').drawImage(cur, 0, 0);
    tool = tools[id]; tool.id = id;
    $$('.tool-list [data-tool]').forEach(function (b) { b.setAttribute('aria-selected', b.dataset.tool === id); });
    if (location.hash.slice(1) !== id) window.history.replaceState(null, '', '#' + id);
    toolBox.innerHTML = '';
    var h = document.createElement('h2'); h.textContent = tool.title; toolBox.appendChild(h);
    var p = document.createElement('p'); p.className = 'desc'; p.textContent = tool.desc; toolBox.appendChild(p);
    if (!cur && !tool.noImage) { var e = document.createElement('p'); e.className = 'small muted'; e.textContent = 'Open an image to start.'; toolBox.appendChild(e); return; }
    tool.ui(toolBox);
  }
  $$('.tool-list [data-tool]').forEach(function (b) { b.addEventListener('click', function () { pick(b.dataset.tool); }); });
  window.addEventListener('hashchange', function () { if (tools[location.hash.slice(1)]) pick(location.hash.slice(1)); });

  // tiny UI helpers
  function el(tag, attrs, kids) {
    var e = document.createElement(tag);
    for (var k in attrs || {}) {
      if (k === 'text') e.textContent = attrs[k];
      else if (k.indexOf('on') === 0) e.addEventListener(k.slice(2), attrs[k]);
      else e.setAttribute(k, attrs[k]);
    }
    (kids || []).forEach(function (c) { if (c) e.appendChild(typeof c === 'string' ? document.createTextNode(c) : c); });
    return e;
  }
  function field(label, input, hint) {
    return el('div', { class: 'field' }, [el('label', {}, [label, hint ? el('span', { class: 'hint', text: ' ' + hint }) : null]), input]);
  }
  function range(min, max, val, step, onInput, fmt) {
    var r = el('input', { type: 'range', min: min, max: max, value: val, step: step || 1 });
    var o = el('output', { text: fmt ? fmt(val) : val });
    r.addEventListener('input', function () { o.textContent = fmt ? fmt(+r.value) : r.value; onInput(+r.value); });
    return el('div', { class: 'range-row' }, [r, o]);
  }
  function seg(options, current, onPick) {
    var s = el('div', { class: 'seg' });
    options.forEach(function (o) {
      var b = el('button', { type: 'button', 'aria-pressed': String(o[0] === current), text: o[1] });
      b.addEventListener('click', function () { $$('button', s).forEach(function (x) { x.setAttribute('aria-pressed', String(x === b)); }); onPick(o[0]); });
      s.appendChild(b);
    });
    return s;
  }
  function btn(text, onClick, cls) { return el('button', { type: 'button', class: 'btn ' + (cls || ''), text: text, onclick: onClick }); }
  function formats(withSame) {
    var f = [];
    if (withSame) f.push([null, 'Same']);
    f.push(['image/jpeg', 'JPG'], ['image/png', 'PNG']);
    if (canEncode('image/webp')) f.push(['image/webp', 'WebP']);
    if (canEncode('image/avif')) f.push(['image/avif', 'AVIF']);
    return f;
  }

  // high-quality downscale: halve repeatedly, then the last step
  function scaled(src, w, h) {
    var c = src;
    while (c.width / 2 >= w && c.height / 2 >= h) {
      var half = document.createElement('canvas'); half.width = Math.round(c.width / 2); half.height = Math.round(c.height / 2);
      var hx = half.getContext('2d'); hx.imageSmoothingQuality = 'high'; hx.drawImage(c, 0, 0, half.width, half.height); c = half;
    }
    var o = document.createElement('canvas'); o.width = w; o.height = h;
    var x = o.getContext('2d'); x.imageSmoothingQuality = 'high'; x.drawImage(c, 0, 0, w, h);
    return o;
  }

  // ---------- compress ----------
  tools.compress = {
    title: 'Compress', desc: 'Make the file smaller. Lower quality means a smaller file; around 75–85 looks the same to most eyes.',
    ui: function (p) {
      if (!out.type && srcType === 'image/png') out.type = canEncode('image/webp') ? 'image/webp' : 'image/jpeg';
      p.appendChild(field('Format', seg(formats(true), out.type, function (t) { out.type = t; info(); })));
      p.appendChild(field('Quality', range(10, 100, Math.round(out.quality * 100), 1, function (v) { out.quality = v / 100; info(); }), 'for JPG, WebP and AVIF'));
      var maxW = el('input', { type: 'number', min: 16, placeholder: String(cur.width) });
      p.appendChild(field('Longest side', maxW, 'optional, in pixels'));
      p.appendChild(btn('Apply size', function () {
        var m = +maxW.value; if (!m || m >= Math.max(cur.width, cur.height)) return;
        var k = m / Math.max(cur.width, cur.height);
        commit(scaled(cur, Math.round(cur.width * k), Math.round(cur.height * k)));
      }));
      info();
    }
  };

  // ---------- resize ----------
  tools.resize = {
    title: 'Resize', desc: 'Change the size in pixels. Keep the lock on to avoid stretching.',
    ui: function (p) {
      var ratio = cur.width / cur.height, lock = true;
      var w = el('input', { type: 'number', min: 1, max: 16384, value: cur.width });
      var h = el('input', { type: 'number', min: 1, max: 16384, value: cur.height });
      w.addEventListener('input', function () { if (lock) h.value = Math.max(1, Math.round(w.value / ratio)); });
      h.addEventListener('input', function () { if (lock) w.value = Math.max(1, Math.round(h.value * ratio)); });
      var lk = el('label', { class: 'check', style: 'margin-bottom:14px' }, [el('input', { type: 'checkbox', checked: '' }), ' Keep proportions']);
      lk.firstChild.addEventListener('change', function () { lock = this.checked; });
      p.appendChild(el('div', { class: 'grid-2', style: 'gap:10px' }, [field('Width', w), field('Height', h)]));
      p.appendChild(lk);
      p.appendChild(field('Quick sizes', seg([[25, '25%'], [50, '50%'], [75, '75%'], [1080, '1080'], [1920, '1920'], [3840, '4K']], null, function (v) {
        var nw = v <= 100 ? Math.round(cur.width * v / 100) : v;
        w.value = nw; h.value = Math.max(1, Math.round(nw / ratio)); if (!lock) h.value = Math.round(cur.height * nw / cur.width);
      })));
      p.appendChild(btn('Resize', function () {
        var nw = Math.min(16384, Math.max(1, +w.value)), nh = Math.min(16384, Math.max(1, +h.value));
        if (nw === cur.width && nh === cur.height) return;
        commit(nw < cur.width && nh < cur.height ? scaled(cur, nw, nh) : (function () {
          var o = document.createElement('canvas'); o.width = nw; o.height = nh;
          var x = o.getContext('2d'); x.imageSmoothingQuality = 'high'; x.drawImage(cur, 0, 0, nw, nh); return o;
        })());
        pick('resize');
      }, 'primary'));
    }
  };

  // ---------- crop ----------
  tools.crop = {
    title: 'Crop', desc: 'Drag the box or its corners, or pick a shape.',
    ui: function (p) {
      var aspect = 0, r = { x: .1, y: .1, w: .8, h: .8 }; // fractions of the image
      function place() {
        var vr = view.getBoundingClientRect(), br = box.getBoundingClientRect();
        cropEl.style.left = (vr.left - br.left + r.x * vr.width) + 'px';
        cropEl.style.top = (vr.top - br.top + r.y * vr.height) + 'px';
        cropEl.style.width = r.w * vr.width + 'px';
        cropEl.style.height = r.h * vr.height + 'px';
        dims.textContent = Math.round(r.w * cur.width) + ' × ' + Math.round(r.h * cur.height) + ' px';
      }
      function fit(a) {
        aspect = a;
        if (!a) return place();
        var imgA = cur.width / cur.height, w = .9, h = w * imgA / a;
        if (h > .9) { h = .9; w = h * a / imgA; }
        r = { x: (1 - w) / 2, y: (1 - h) / 2, w: w, h: h }; place();
      }
      var dims = el('p', { class: 'small muted' });
      p.appendChild(field('Shape', seg([[0, 'Free'], [1, '1:1'], [4 / 3, '4:3'], [3 / 2, '3:2'], [16 / 9, '16:9'], [9 / 16, '9:16'], [4 / 5, '4:5']], 0, fit)));
      p.appendChild(dims);
      p.appendChild(btn('Crop', function () {
        var x = Math.round(r.x * cur.width), y = Math.round(r.y * cur.height), w = Math.round(r.w * cur.width), h = Math.round(r.h * cur.height);
        var o = document.createElement('canvas'); o.width = Math.max(1, w); o.height = Math.max(1, h);
        o.getContext('2d').drawImage(cur, x, y, w, h, 0, 0, w, h);
        commit(o); pick('crop');
      }, 'primary'));

      cropEl.classList.remove('hide');
      var drag = null;
      function down(e) {
        e.preventDefault();
        var vr = view.getBoundingClientRect();
        drag = { h: e.target.dataset.h || 'move', x: e.clientX, y: e.clientY, r: Object.assign({}, r), vw: vr.width, vh: vr.height };
        cropEl.setPointerCapture(e.pointerId);
      }
      function move(e) {
        if (!drag) return;
        var dx = (e.clientX - drag.x) / drag.vw, dy = (e.clientY - drag.y) / drag.vh, o = drag.r, n = Object.assign({}, o);
        var clamp = function (v, a, b) { return Math.max(a, Math.min(b, v)); };
        if (drag.h === 'move') { n.x = clamp(o.x + dx, 0, 1 - o.w); n.y = clamp(o.y + dy, 0, 1 - o.h); }
        else {
          if (drag.h.indexOf('w') >= 0) { n.x = clamp(o.x + dx, 0, o.x + o.w - .02); n.w = o.x + o.w - n.x; }
          if (drag.h.indexOf('e') >= 0) { n.w = clamp(o.w + dx, .02, 1 - o.x); }
          if (drag.h.indexOf('n') >= 0) { n.y = clamp(o.y + dy, 0, o.y + o.h - .02); n.h = o.y + o.h - n.y; }
          if (drag.h.indexOf('s') >= 0) { n.h = clamp(o.h + dy, .02, 1 - o.y); }
          if (aspect) {
            var imgA = cur.width / cur.height;
            n.h = n.w * imgA / aspect;
            if (drag.h.indexOf('n') >= 0) n.y = o.y + o.h - n.h;
            if (n.y < 0 || n.y + n.h > 1) return;
          }
        }
        r = n; place();
      }
      function up() { drag = null; }
      cropEl.addEventListener('pointerdown', down);
      cropEl.addEventListener('pointermove', move);
      cropEl.addEventListener('pointerup', up);
      window.addEventListener('resize', place);
      overlay = {
        refresh: place,
        off: function () {
          cropEl.classList.add('hide');
          cropEl.removeEventListener('pointerdown', down); cropEl.removeEventListener('pointermove', move); cropEl.removeEventListener('pointerup', up);
          window.removeEventListener('resize', place);
        }
      };
      requestAnimationFrame(place);
    }
  };

  // ---------- convert ----------
  tools.convert = {
    title: 'Convert', desc: 'Save in another format. WebP and AVIF are much smaller than JPG and PNG and work in all modern browsers.',
    ui: function (p) {
      if (!out.type) out.type = srcType;
      p.appendChild(field('Save as', seg(formats(false), outType(), function (t) { out.type = t; info(); qrow.classList.toggle('hide', t === 'image/png'); })));
      var qrow = field('Quality', range(10, 100, Math.round(out.quality * 100), 1, function (v) { out.quality = v / 100; info(); }));
      if (outType() === 'image/png') qrow.classList.add('hide');
      p.appendChild(qrow);
      p.appendChild(el('p', { class: 'small muted', text: 'Transparent areas turn white in JPG.' }));
      info();
    }
  };

  // ---------- adjust ----------
  tools.adjust = {
    title: 'Adjust & rotate', desc: 'Light, contrast and color, plus turning and flipping.',
    ui: function (p) {
      var v = { b: 100, c: 100, s: 100, g: 0, sp: 0, inv: 0 };
      function preview() {
        view.style.filter = 'brightness(' + v.b + '%) contrast(' + v.c + '%) saturate(' + v.s + '%) grayscale(' + v.g + '%) sepia(' + v.sp + '%) invert(' + v.inv + '%)';
      }
      p.appendChild(el('div', { class: 'btn-row', style: 'margin-bottom:16px' }, [
        btn('⟲ Left', function () { commit(rotate(cur, -1)); }, 'sm'), btn('⟳ Right', function () { commit(rotate(cur, 1)); }, 'sm'),
        btn('⇋ Flip', function () { commit(flip(cur, true)); }, 'sm'), btn('⇵ Flip', function () { commit(flip(cur, false)); }, 'sm')
      ]));
      p.appendChild(field('Brightness', range(0, 200, 100, 1, function (x) { v.b = x; preview(); }, pc)));
      p.appendChild(field('Contrast', range(0, 200, 100, 1, function (x) { v.c = x; preview(); }, pc)));
      p.appendChild(field('Saturation', range(0, 300, 100, 1, function (x) { v.s = x; preview(); }, pc)));
      p.appendChild(field('Black & white', range(0, 100, 0, 1, function (x) { v.g = x; preview(); }, pc)));
      p.appendChild(field('Sepia', range(0, 100, 0, 1, function (x) { v.sp = x; preview(); }, pc)));
      p.appendChild(field('Invert', range(0, 100, 0, 1, function (x) { v.inv = x; preview(); }, pc)));
      p.appendChild(btn('Apply', function () {
        busy('Applying…');
        setTimeout(function () { commit(applyFilters(cur, v)); busy(false); pick('adjust'); }, 20);
      }, 'primary'));
    }
  };
  function pc(x) { return x + '%'; }
  function rotate(src, dir) {
    var o = document.createElement('canvas'); o.width = src.height; o.height = src.width;
    var x = o.getContext('2d'); x.translate(o.width / 2, o.height / 2); x.rotate(dir * Math.PI / 2); x.drawImage(src, -src.width / 2, -src.height / 2);
    return o;
  }
  function flip(src, horiz) {
    var o = document.createElement('canvas'); o.width = src.width; o.height = src.height;
    var x = o.getContext('2d');
    if (horiz) { x.translate(o.width, 0); x.scale(-1, 1); } else { x.translate(0, o.height); x.scale(1, -1); }
    x.drawImage(src, 0, 0); return o;
  }
  // The same math as the CSS filter functions, done on pixels so it works in
  // every browser (canvas ctx.filter isn't everywhere yet).
  function applyFilters(src, v) {
    var o = clone(src), x = o.getContext('2d'), d = x.getImageData(0, 0, o.width, o.height), a = d.data;
    var b = v.b / 100, c = v.c / 100, s = v.s / 100, g = v.g / 100, sp = v.sp / 100, inv = v.inv / 100;
    for (var i = 0; i < a.length; i += 4) {
      var r = a[i] * b, gg = a[i + 1] * b, bb = a[i + 2] * b;
      r = (r - 128) * c + 128; gg = (gg - 128) * c + 128; bb = (bb - 128) * c + 128;
      var l = .2126 * r + .7152 * gg + .0722 * bb;
      r = l + (r - l) * s; gg = l + (gg - l) * s; bb = l + (bb - l) * s;
      if (g) { l = .2126 * r + .7152 * gg + .0722 * bb; r += (l - r) * g; gg += (l - gg) * g; bb += (l - bb) * g; }
      if (sp) {
        var sr = .393 * r + .769 * gg + .189 * bb, sg = .349 * r + .686 * gg + .168 * bb, sb = .272 * r + .534 * gg + .131 * bb;
        r += (sr - r) * sp; gg += (sg - gg) * sp; bb += (sb - bb) * sp;
      }
      if (inv) { r += (255 - 2 * r) * inv; gg += (255 - 2 * gg) * inv; bb += (255 - 2 * bb) * inv; }
      a[i] = r; a[i + 1] = gg; a[i + 2] = bb; // Uint8Clamped clamps for us
    }
    x.putImageData(d, 0, 0);
    return o;
  }

  // obscure hides a rectangle of src, drawn into dst: pixelate, blur or a black box.
  function obscure(dst, src, x, y, w, h, mode, strength) {
    var cx = dst.getContext('2d');
    if (mode === 'box') { cx.fillStyle = '#000'; cx.fillRect(x, y, w, h); return; }
    var k = mode === 'pixelate' ? Math.max(2, Math.round(Math.max(w, h) / (60 - strength))) : Math.max(2, strength / 2);
    var small = document.createElement('canvas'); small.width = Math.max(1, Math.round(w / k)); small.height = Math.max(1, Math.round(h / k));
    var sx = small.getContext('2d'); sx.imageSmoothingEnabled = mode === 'blur';
    sx.drawImage(src, x, y, w, h, 0, 0, small.width, small.height);
    if (mode === 'blur') { // a second pass smooths the blocky edges
      var tiny = document.createElement('canvas'); tiny.width = Math.max(1, small.width >> 1); tiny.height = Math.max(1, small.height >> 1);
      tiny.getContext('2d').drawImage(small, 0, 0, tiny.width, tiny.height); small = tiny;
    }
    cx.imageSmoothingEnabled = mode === 'blur'; cx.imageSmoothingQuality = 'high';
    cx.drawImage(small, 0, 0, small.width, small.height, x, y, w, h);
  }
  window.phObscure = obscure;

  // ---------- redact ----------
  tools.redact = {
    title: 'Blur & pixelate', desc: 'Hide faces, plates, names or chat bubbles. Drag a box over each part you want to hide.',
    ui: function (p) {
      var mode = 'pixelate', strength = 14;
      p.appendChild(field('Style', seg([['pixelate', 'Pixelate'], ['blur', 'Blur'], ['box', 'Black box']], mode, function (m) { mode = m; })));
      p.appendChild(field('Strength', range(4, 40, strength, 1, function (x) { strength = x; })));
      p.appendChild(el('p', { class: 'small muted', text: 'Tip: pixelate or black box can’t be reversed; strong blur is safe too.' }));
      var start = null, sel = el('div', { class: 'crop-box', style: 'box-shadow:none;border-style:dashed;pointer-events:none' });
      box.appendChild(sel); sel.classList.add('hide');
      function pt(e) {
        var vr = view.getBoundingClientRect();
        return { x: Math.max(0, Math.min(1, (e.clientX - vr.left) / vr.width)), y: Math.max(0, Math.min(1, (e.clientY - vr.top) / vr.height)) };
      }
      function down(e) { e.preventDefault(); start = pt(e); view.setPointerCapture(e.pointerId); }
      function move(e) {
        if (!start) return;
        var q = pt(e), vr = view.getBoundingClientRect(), br = box.getBoundingClientRect();
        sel.classList.remove('hide');
        sel.style.left = (vr.left - br.left + Math.min(start.x, q.x) * vr.width) + 'px';
        sel.style.top = (vr.top - br.top + Math.min(start.y, q.y) * vr.height) + 'px';
        sel.style.width = Math.abs(q.x - start.x) * vr.width + 'px';
        sel.style.height = Math.abs(q.y - start.y) * vr.height + 'px';
      }
      function up(e) {
        if (!start) return;
        var q = pt(e), s = start; start = null; sel.classList.add('hide');
        var x = Math.round(Math.min(s.x, q.x) * cur.width), y = Math.round(Math.min(s.y, q.y) * cur.height);
        var w = Math.round(Math.abs(q.x - s.x) * cur.width), h = Math.round(Math.abs(q.y - s.y) * cur.height);
        if (w < 3 || h < 3) return;
        var o = clone(cur);
        obscure(o, cur, x, y, w, h, mode, strength);
        commit(o);
      }
      view.style.cursor = 'crosshair';
      view.addEventListener('pointerdown', down); view.addEventListener('pointermove', move); view.addEventListener('pointerup', up);
      overlay = { off: function () {
        view.style.cursor = ''; sel.remove();
        view.removeEventListener('pointerdown', down); view.removeEventListener('pointermove', move); view.removeEventListener('pointerup', up);
      } };
    }
  };

  // ---------- palette ----------
  // ---------- watermark ----------
  tools.watermark = {
    title: 'Watermark', desc: 'Put your name, handle or logo on the image so it stays credited when shared.',
    ui: function (p) {
      var o = { text: '© ' + location.hostname, size: 5, opacity: 60, pos: 'br', color: '#ffffff', logo: null };
      var txt = el('input', { type: 'text', value: o.text, maxlength: '80' });
      txt.addEventListener('input', function () { o.text = txt.value; draw(); });
      var logoIn = el('input', { type: 'file', accept: 'image/*', class: 'hide' });
      var logoBtn = btn('Use a logo instead', function () { logoIn.click(); }, 'sm');
      logoIn.addEventListener('change', function () {
        var f = logoIn.files[0]; if (!f) return;
        createImageBitmap(f).then(function (b) { o.logo = b; logoBtn.textContent = 'Logo: ' + f.name; draw(); }, function () { toast('That logo could not be opened', true); });
      });
      function render(c) {
        var x = c.getContext('2d'), W = c.width, H = c.height, m = Math.round(Math.min(W, H) * 0.03);
        x.save(); x.globalAlpha = o.opacity / 100;
        var w, h, fs = Math.max(10, Math.round(Math.min(W, H) * o.size / 100));
        if (o.logo) { h = fs * 2; w = Math.round(o.logo.width * h / o.logo.height); }
        else { x.font = '700 ' + fs + 'px system-ui, sans-serif'; x.textBaseline = 'top'; w = x.measureText(o.text).width; h = fs; }
        function put(px, py) {
          if (o.logo) { x.drawImage(o.logo, px, py, w, h); return; }
          x.shadowColor = o.color === '#ffffff' ? 'rgba(0,0,0,.45)' : 'rgba(255,255,255,.45)'; x.shadowBlur = fs / 6;
          x.fillStyle = o.color; x.fillText(o.text, px, py);
        }
        if (o.pos === 'tile') {
          x.translate(W / 2, H / 2); x.rotate(-Math.PI / 8); x.translate(-W / 2, -H / 2);
          for (var ty = -H; ty < H * 2; ty += h * 4) for (var tx = -W; tx < W * 2; tx += w + fs * 3) put(tx + ((ty / (h * 4)) % 2 ? (w + fs * 3) / 2 : 0), ty);
        } else {
          var px = o.pos[1] === 'l' ? m : o.pos[1] === 'r' ? W - w - m : (W - w) / 2;
          var py = o.pos[0] === 't' ? m : o.pos[0] === 'b' ? H - h - m : (H - h) / 2;
          put(px, py);
        }
        x.restore();
      }
      function draw() { var c = clone(cur); render(c); view.getContext('2d').drawImage(c, 0, 0); }
      overlay = { off: function () { view.getContext('2d').drawImage(cur, 0, 0); }, refresh: draw };
      p.appendChild(field('Text', txt));
      p.appendChild(el('div', { style: 'margin:-6px 0 12px' }, [logoBtn, logoIn]));
      p.appendChild(field('Position', seg([['tl', '↖'], ['tc', '↑'], ['tr', '↗'], ['cc', '•'], ['bl', '↙'], ['bc', '↓'], ['br', '↘'], ['tile', 'Tile']], o.pos, function (v) { o.pos = v; draw(); })));
      p.appendChild(field('Size', range(1, 20, o.size, 1, function (v) { o.size = v; draw(); }, function (v) { return v + '%'; })));
      p.appendChild(field('Opacity', range(10, 100, o.opacity, 5, function (v) { o.opacity = v; draw(); }, function (v) { return v + '%'; })));
      p.appendChild(field('Color', seg([['#ffffff', 'White'], ['#000000', 'Black']], o.color, function (v) { o.color = v; draw(); })));
      p.appendChild(btn('Apply watermark', function () { var c = clone(cur); render(c); overlay = null; commit(c); pick('watermark'); }, 'primary'));
      draw();
    }
  };

  // hold to see the original (before / after)
  var cmp = $('#st-compare');
  if (cmp) {
    var before = function (e) {
      if (!original || !cur) return; e.preventDefault();
      var x = view.getContext('2d'); x.clearRect(0, 0, view.width, view.height); view.style.filter = 'none';
      x.drawImage(original, 0, 0, view.width, view.height); cmp.setAttribute('aria-pressed', 'true');
    };
    var after = function () { if (cmp.getAttribute('aria-pressed') !== 'true') return; cmp.setAttribute('aria-pressed', 'false'); show(); };
    cmp.addEventListener('pointerdown', before);
    ['pointerup', 'pointerleave', 'pointercancel'].forEach(function (ev) { cmp.addEventListener(ev, after); });
    cmp.addEventListener('keydown', function (e) { if (e.key === ' ' || e.key === 'Enter') before(e); });
    cmp.addEventListener('keyup', after);
  }

  tools.palette = {
    title: 'Color palette', desc: 'The main colors in your image. Click a color to copy its code.',
    ui: function (p) {
      var n = 6, grid = el('div', { class: 'palette' });
      function run() {
        var cols = kmeans(sample(cur, 120), n);
        grid.innerHTML = '';
        cols.forEach(function (c) {
          var hex = '#' + c.map(function (v) { return ('0' + Math.round(v).toString(16)).slice(-2); }).join('');
          var b = el('button', { type: 'button', title: 'Copy ' + hex }, [el('span', { style: 'background:' + hex }), el('code', { text: hex })]);
          b.addEventListener('click', function () { window.phCopy(hex); });
          grid.appendChild(b);
        });
        css.onclick = function () { window.phCopy(cols.map(function (c, i) { return '--color-' + (i + 1) + ': ' + $$('code', grid)[i].textContent + ';'; }).join('\n')); };
      }
      var css = btn('Copy as CSS variables', null, 'sm');
      p.appendChild(field('How many', seg([[4, '4'], [6, '6'], [8, '8'], [10, '10']], n, function (x) { n = x; run(); })));
      p.appendChild(grid);
      p.appendChild(el('div', { style: 'margin-top:14px' }, [css]));
      run();
    }
  };
  function sample(src, maxSide) {
    var k = Math.min(1, maxSide / Math.max(src.width, src.height));
    var c = document.createElement('canvas'); c.width = Math.max(1, src.width * k | 0); c.height = Math.max(1, src.height * k | 0);
    var x = c.getContext('2d'); x.drawImage(src, 0, 0, c.width, c.height);
    var d = x.getImageData(0, 0, c.width, c.height).data, px = [];
    for (var i = 0; i < d.length; i += 4) if (d[i + 3] > 125) px.push([d[i], d[i + 1], d[i + 2]]);
    return px;
  }
  function kmeans(px, k) {
    if (!px.length) return [];
    var cs = [], i, j;
    for (i = 0; i < k; i++) cs.push(px[Math.floor((i + .5) * px.length / k)].slice()); // spread starts
    var counts;
    for (var it = 0; it < 12; it++) {
      var sums = cs.map(function () { return [0, 0, 0]; }); counts = cs.map(function () { return 0; });
      for (i = 0; i < px.length; i++) {
        var best = 0, bd = 1e9;
        for (j = 0; j < k; j++) {
          var dr = px[i][0] - cs[j][0], dg = px[i][1] - cs[j][1], db = px[i][2] - cs[j][2], dd = dr * dr + dg * dg + db * db;
          if (dd < bd) { bd = dd; best = j; }
        }
        sums[best][0] += px[i][0]; sums[best][1] += px[i][1]; sums[best][2] += px[i][2]; counts[best]++;
      }
      for (j = 0; j < k; j++) if (counts[j]) cs[j] = [sums[j][0] / counts[j], sums[j][1] / counts[j], sums[j][2] / counts[j]];
    }
    return cs.map(function (c, i) { return { c: c, n: counts[i] }; }).filter(function (x) { return x.n; })
      .sort(function (a, b) { return b.n - a.n; }).map(function (x) { return x.c; });
  }

  // ---------- OCR ----------
  function script(url) {
    return new Promise(function (res, rej) {
      if (document.querySelector('script[src="' + url + '"]')) return res();
      var s = document.createElement('script'); s.src = url; s.onload = res; s.onerror = function () { rej(new Error('Could not load ' + url)); };
      document.head.appendChild(s);
    });
  }
  tools.ocr = {
    title: 'Image to text', desc: 'Read the text in a photo or screenshot. The reading model (about 10 MB per language) downloads once, then runs on your device.',
    ui: function (p) {
      var lang = 'eng+ind';
      p.appendChild(field('Language', seg([['eng+ind', 'English + Indonesian'], ['eng', 'English'], ['jpn', '日本語'], ['chi_sim', '中文'], ['kor', '한국어'], ['ara', 'العربية']], lang, function (l) { lang = l; })));
      var outBox = el('textarea', { class: 'ocr-out', placeholder: 'The text will appear here.' });
      p.appendChild(btn('Read text', function () {
        busy('Loading the reader…');
        script(TESSERACT).then(function () {
          return window.Tesseract.recognize(cur, lang, { logger: function (m) {
            if (m.status) busy(m.status.replace(/^\w/, function (c) { return c.toUpperCase(); }) + (m.progress ? ' ' + Math.round(m.progress * 100) + '%' : ''));
          } });
        }).then(function (r) {
          outBox.value = r.data.text.trim() || '(no text found)'; busy(false);
        }).catch(function (e) { busy(false); toast(e.message || 'Reading failed', true); });
      }, 'primary'));
      p.appendChild(el('div', { style: 'margin-top:14px' }, [outBox]));
      p.appendChild(el('div', { class: 'btn-row', style: 'margin-top:8px' }, [
        btn('Copy text', function () { window.phCopy(outBox.value); }, 'sm'),
        btn('Save .txt', function () { window.phSave(new Blob([outBox.value], { type: 'text/plain' }), srcName + '.txt'); }, 'sm')
      ]));
    }
  };

  // ---------- remove background ----------
  var rembgLib = null;
  var studioEl = $('#studio');
  var serverRembg = studioEl && studioEl.dataset.rembgServer === '1';
  var signedIn = studioEl && studioEl.dataset.signedIn === '1';
  var rembgWhere = serverRembg && signedIn ? 'server' : 'device';

  function rembgOnServer() {
    busy('Removing the background on the server…');
    return new Promise(function (res) { cur.toBlob(res, 'image/png'); }).then(function (blob) {
      var fd = new FormData(); fd.append('file', blob, srcName + '.png');
      return fetch('/api/v1/tools/remove-background', { method: 'POST', body: fd, headers: { Accept: 'image/png, application/json' } });
    }).then(function (r) {
      if (r.ok) return r.blob();
      return r.json().then(function (j) { throw new Error(j.error || 'Server removal failed'); }, function () { throw new Error('Server removal failed (' + r.status + ')'); });
    });
  }

  tools.rembg = {
    title: 'Remove background', desc: 'Cut out the subject and make the background transparent.',
    ui: function (p) {
      if (serverRembg) {
        p.appendChild(field('Where', seg([['server', 'On our server · fast'], ['device', 'On this device · private']], rembgWhere, function (w) {
          rembgWhere = w; note.textContent = whereNote();
        })));
      }
      var note = el('p', { class: 'small muted', style: 'margin:-6px 0 14px' });
      function whereNote() {
        if (rembgWhere === 'server') return signedIn
          ? 'About a second. Your image is sent to this server, processed and discarded; it is not stored.'
          : 'Sign in to use the server. “On this device” works without an account.';
        return 'Nothing leaves your device. The AI model (about 40 MB) downloads once, then it is cached by your browser.';
      }
      note.textContent = whereNote();
      p.appendChild(note);
      p.appendChild(btn('Remove background', function () {
        if (rembgWhere === 'server') {
          if (!signedIn) { location.href = '/login?next=' + encodeURIComponent('/tools#rembg'); return; }
          rembgOnServer()
            .then(function (b) { return createImageBitmap(b); })
            .then(function (bmp) { out.type = 'image/png'; commit(canvasFrom(bmp)); busy(false); pick('rembg'); })
            .catch(function (e) { busy(false); toast(e.message, true); });
          return;
        }
        busy('Loading the AI model…');
        (rembgLib ? Promise.resolve(rembgLib) : import(REMBG).then(function (m) { return (rembgLib = m); }))
          .then(function (m) {
            return new Promise(function (res) { cur.toBlob(res, 'image/png'); }).then(function (blob) {
              return m.removeBackground(blob, { progress: function (key, done, total) {
                busy((/fetch/.test(key) ? 'Downloading model ' : 'Working ') + (total ? Math.round(done / total * 100) + '%' : '…'));
              } });
            });
          })
          .then(function (b) { return createImageBitmap(b); })
          .then(function (bmp) { out.type = 'image/png'; commit(canvasFrom(bmp)); busy(false); pick('rembg'); })
          .catch(function (e) { busy(false); toast((e && e.message) || 'Background removal failed', true); });
      }, 'primary'));
      p.appendChild(el('p', { class: 'small muted', style: 'margin-top:12px', text: 'Saved as PNG to keep the transparency. Works best on people, products and animals.' }));
      p.appendChild(el('p', { class: 'small faint', text: 'On this device: @imgly/background-removal (AGPL-3.0), loaded unmodified from jsDelivr. On the server: rembg (MIT).' }));
    }
  };

  // ---------- server AI tools (sign-in required) ----------
  function needServer(p) {
    if (!serverRembg) { p.appendChild(el('p', { class: 'small muted', text: 'This tool runs on the server, which is not set up here.' })); return false; }
    if (!signedIn) {
      p.appendChild(el('p', { class: 'small muted', text: 'This tool runs on our server. Sign in (free) to use it.' }));
      p.appendChild(el('a', { class: 'btn primary', href: '/login?next=' + encodeURIComponent('/tools' + location.hash), text: 'Sign in' }));
      return false;
    }
    return true;
  }
  function canvasBlob(c, type) { return new Promise(function (res) { c.toBlob(res, type || 'image/png', 0.95); }); }
  // serverTool posts the current image (and optional extra files) and returns the Response.
  function serverTool(name, extra) {
    return canvasBlob(cur, cur.width * cur.height > 4e6 ? 'image/jpeg' : 'image/png').then(function (blob) {
      var fd = new FormData(); fd.append('file', blob, srcName + (blob.type === 'image/jpeg' ? '.jpg' : '.png'));
      for (var k in (extra || {})) fd.append(k, extra[k], k + '.png');
      return fetch('/api/v1/tools/' + name, { method: 'POST', body: fd, headers: { Accept: 'image/png, application/json' } });
    }).then(function (r) {
      if (r.ok) return r;
      return r.json().then(function (j) { throw new Error(j.error || 'The server could not do that'); }, function () { throw new Error('The server could not do that (' + r.status + ')'); });
    });
  }
  function applyResult(r, tool) {
    return r.blob().then(function (b) { return createImageBitmap(b); })
      .then(function (bmp) { out.type = 'image/png'; commit(canvasFrom(bmp)); busy(false); pick(tool); });
  }

  // ---------- face blur ----------
  tools.faces = {
    title: 'Blur faces', desc: 'Finds faces and hides them in one go. Missed one? Use “Blur & pixelate” to cover it by hand.',
    ui: function (p) {
      if (!needServer(p)) return;
      var mode = 'blur', strength = 22;
      p.appendChild(field('Style', seg([['blur', 'Blur'], ['pixelate', 'Pixelate'], ['box', 'Black box']], mode, function (m) { mode = m; })));
      p.appendChild(field('Strength', range(6, 40, strength, 1, function (x) { strength = x; })));
      var found = el('p', { class: 'small muted' });
      p.appendChild(btn('Find & blur faces', function () {
        busy('Looking for faces…');
        serverTool('faces').then(function (r) { return r.json(); }).then(function (j) {
          busy(false);
          var list = j.faces || [];
          if (!list.length) { found.textContent = 'No faces found. Use “Blur & pixelate” to cover them by hand.'; return; }
          var o = clone(cur), kx = cur.width / j.width, ky = cur.height / j.height;
          list.forEach(function (f) {
            // grow each box a little so hair and chin are covered too
            var w = f[2] * kx * 1.3, h = f[3] * ky * 1.45, x = f[0] * kx - (w - f[2] * kx) / 2, y = f[1] * ky - (h - f[3] * ky) / 2.6;
            x = Math.max(0, x); y = Math.max(0, y); w = Math.min(cur.width - x, w); h = Math.min(cur.height - y, h);
            obscure(o, cur, Math.round(x), Math.round(y), Math.round(w), Math.round(h), mode, strength);
          });
          commit(o);
          found.textContent = list.length === 1 ? '1 face hidden.' : list.length + ' faces hidden.';
        }).catch(function (e) { busy(false); toast(e.message, true); });
      }, 'primary'));
      p.appendChild(el('div', { style: 'margin-top:12px' }, [found]));
    }
  };

  // ---------- magic eraser ----------
  tools.erase = {
    title: 'Magic eraser', desc: 'Paint over people, objects or text you want gone; AI fills the space with what should be behind them.',
    ui: function (p) {
      if (!needServer(p)) return;
      var size = 40;
      var mask = document.createElement('canvas'); mask.width = cur.width; mask.height = cur.height;
      var mx = mask.getContext('2d'); mx.lineCap = mx.lineJoin = 'round'; mx.strokeStyle = '#fff';
      var paint = document.createElement('canvas'); paint.className = 'paint';
      box.appendChild(paint);
      function fit() {
        var vr = view.getBoundingClientRect(), br = box.getBoundingClientRect();
        paint.style.left = (vr.left - br.left) + 'px'; paint.style.top = (vr.top - br.top) + 'px';
        paint.style.width = vr.width + 'px'; paint.style.height = vr.height + 'px';
        paint.width = mask.width; paint.height = mask.height; redraw();
      }
      function redraw() {
        var c = paint.getContext('2d'); c.clearRect(0, 0, paint.width, paint.height);
        c.globalAlpha = .5; c.drawImage(mask, 0, 0); c.globalCompositeOperation = 'source-in';
        c.fillStyle = '#f43f5e'; c.fillRect(0, 0, paint.width, paint.height); c.globalCompositeOperation = 'source-over'; c.globalAlpha = 1;
      }
      var last = null, any = false;
      function pos(e) { var r = paint.getBoundingClientRect(); return { x: (e.clientX - r.left) * mask.width / r.width, y: (e.clientY - r.top) * mask.height / r.height }; }
      function stroke(a, b) {
        mx.lineWidth = size * mask.width / paint.getBoundingClientRect().width;
        mx.beginPath(); mx.moveTo(a.x, a.y); mx.lineTo(b.x, b.y); mx.stroke(); any = true; redraw();
      }
      function down(e) { e.preventDefault(); paint.setPointerCapture(e.pointerId); last = pos(e); stroke(last, last); }
      function move(e) { if (!last) return; var q = pos(e); stroke(last, q); last = q; }
      function up() { last = null; }
      paint.addEventListener('pointerdown', down); paint.addEventListener('pointermove', move); paint.addEventListener('pointerup', up);
      window.addEventListener('resize', fit);
      overlay = { refresh: fit, off: function () { paint.remove(); window.removeEventListener('resize', fit); } };
      requestAnimationFrame(fit);

      p.appendChild(field('Brush size', range(10, 120, size, 1, function (x) { size = x; }, function (x) { return x + ' px'; })));
      p.appendChild(el('div', { class: 'btn-row' }, [
        btn('Erase', function () {
          if (!any) { toast('Paint over what to erase first', true); return; }
          busy('Erasing… this takes about 10 seconds');
          canvasBlob(mask).then(function (m) { return serverTool('erase', { mask: m }); })
            .then(function (r) { return applyResult(r, 'erase'); })
            .catch(function (e) { busy(false); toast(e.message, true); });
        }, 'primary'),
        btn('Clear', function () { mx.clearRect(0, 0, mask.width, mask.height); any = false; redraw(); })
      ]));
      p.appendChild(el('p', { class: 'small muted', style: 'margin-top:12px', text: 'Works best on small things against a simple background. Do several small erases rather than one big one.' }));
    }
  };

  // ---------- HD upscaler ----------
  tools.upscale = {
    title: 'HD upscale', desc: 'Doubles the width and height with an AI model (Real-ESRGAN) that sharpens detail instead of blurring it.',
    ui: function (p) {
      if (!needServer(p)) return;
      var mp = cur.width * cur.height / 1e6;
      p.appendChild(el('p', { class: 'small', text: 'Now ' + cur.width + ' × ' + cur.height + ' → ' + cur.width * 2 + ' × ' + cur.height * 2 }));
      if (mp > 1) p.appendChild(el('p', { class: 'small muted', text: 'Larger than 1 megapixel: it will be shrunk to 1 MP first, then doubled.' }));
      p.appendChild(btn('Upscale 2×', function () {
        if (mp > 1) {
          var k = Math.sqrt(1 / mp);
          commit(scaled(cur, Math.floor(cur.width * k), Math.floor(cur.height * k)));
        }
        busy('Upscaling… up to a minute for big images');
        serverTool('upscale').then(function (r) { return applyResult(r, 'upscale'); })
          .catch(function (e) { busy(false); toast(e.message, true); });
      }, 'primary'));
      p.appendChild(el('p', { class: 'small muted', style: 'margin-top:12px', text: 'Best for small photos, screenshots and artwork. Limit: 10 per day.' }));
    }
  };

  // ---------- QR code ----------
  tools.qr = {
    title: 'QR code', desc: 'Make a QR code for any link or text. Download it, or open it here to edit further.',
    noImage: true,
    ui: function (p) {
      var text = el('textarea', { placeholder: 'https://hel.ink/b/you', rows: 3, maxlength: 1500 });
      var fg = el('input', { type: 'color', value: '#000000' }), bg = el('input', { type: 'color', value: '#ffffff' });
      var trans = el('label', { class: 'check' }, [el('input', { type: 'checkbox' }), ' Transparent background']);
      var level = 'M', img = el('img', { alt: 'QR code preview', class: 'qr', style: 'width:200px;height:200px;margin:14px auto;display:none' });
      function url(fmt) {
        var q = new URLSearchParams({ text: text.value.trim(), size: 1024, level: level, fg: fg.value.slice(1), bg: trans.firstChild.checked ? 'transparent' : bg.value.slice(1) });
        return '/tools/qr.' + fmt + '?' + q;
      }
      function update() { var ok = text.value.trim() !== ''; img.style.display = ok ? 'block' : 'none'; if (ok) img.src = url('png'); }
      var t; text.addEventListener('input', function () { clearTimeout(t); t = setTimeout(update, 300); });
      [fg, bg, trans.firstChild].forEach(function (x) { x.addEventListener('change', update); });
      p.appendChild(field('Link or text', text));
      p.appendChild(el('div', { class: 'grid-2', style: 'gap:10px' }, [field('Color', fg), field('Background', bg)]));
      p.appendChild(trans);
      p.appendChild(field('Error correction', seg([['L', 'Low'], ['M', 'Medium'], ['Q', 'High'], ['H', 'Max']], level, function (l) { level = l; update(); }), 'higher survives logos & damage'));
      p.appendChild(img);
      p.appendChild(el('div', { class: 'btn-row' }, [
        btn('PNG', function () { if (text.value.trim()) fetch(url('png')).then(function (r) { return r.blob(); }).then(function (b) { window.phSave(b, 'qr-code.png'); }); }, 'primary'),
        btn('SVG', function () { if (text.value.trim()) fetch(url('svg')).then(function (r) { return r.blob(); }).then(function (b) { window.phSave(b, 'qr-code.svg'); }); }),
        btn('Open in studio', function () { if (text.value.trim()) fetch(url('png')).then(function (r) { return r.blob(); }).then(function (b) { load(b, 'qr-code'); }); })
      ]));
    }
  };

  var legacyOpen = { faceblur: 'faces' }[new URLSearchParams(location.search).get('open')] || new URLSearchParams(location.search).get('open');
  pick(location.hash.slice(1) || (tools[legacyOpen] ? legacyOpen : 'compress'));
})();
