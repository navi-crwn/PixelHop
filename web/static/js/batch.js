// PixelHop batch: the same steps for many images, then one ZIP or one album.
// Resizing and converting happen in the browser; AI steps call the server one
// image at a time (signed-in only). The ZIP is written here without a library:
// images are already compressed, so files are simply stored.
(function () {
  'use strict';
  var $ = function (s) { return document.querySelector(s); };
  var toast = function (m, e) { window.phToast && window.phToast(m, e); };
  var root = $('#batch');
  var signedIn = root.dataset.signedIn === '1', server = root.dataset.server === '1';
  var maxBatch = +root.dataset.maxBatch || 20;
  var items = []; // {file, row, out: Blob, name}
  var EXT = { 'image/jpeg': 'jpg', 'image/png': 'png', 'image/webp': 'webp', 'image/avif': 'avif', 'image/gif': 'gif' };

  // ---- picking files ----
  var input = $('#b-file');
  $('#b-pick').addEventListener('click', function (e) { e.stopPropagation(); input.click(); });
  $('#b-drop').addEventListener('click', function (e) { if (!e.target.closest('button')) input.click(); });
  input.addEventListener('change', function () { add(input.files); input.value = ''; });
  document.addEventListener('dragover', function (e) { e.preventDefault(); });
  document.addEventListener('drop', function (e) { e.preventDefault(); add(e.dataTransfer.files); });
  document.addEventListener('paste', function (e) {
    var fs = Array.prototype.filter.call(e.clipboardData.items || [], function (i) { return i.kind === 'file'; }).map(function (i) { return i.getAsFile(); });
    if (fs.length) add(fs);
  });

  function add(files) {
    Array.prototype.forEach.call(files, function (f) {
      if (!/^image\//.test(f.type) || items.length >= 200) return;
      var row = document.createElement('div'); row.className = 'up';
      var img = document.createElement('img'); img.className = 'thumb'; img.alt = ''; img.src = URL.createObjectURL(f);
      var meta = document.createElement('div'); meta.className = 'meta';
      meta.innerHTML = '<div class="name"></div><div class="small muted state">Ready</div>';
      meta.firstChild.textContent = f.name;
      var rm = document.createElement('button'); rm.className = 'btn sm ghost'; rm.type = 'button'; rm.textContent = 'Remove';
      row.appendChild(img); row.appendChild(meta); row.appendChild(rm);
      var it = { file: f, row: row };
      rm.addEventListener('click', function () { items.splice(items.indexOf(it), 1); row.remove(); refresh(); });
      items.push(it); $('#b-list').appendChild(row);
    });
    refresh();
  }

  // Any change to the steps means results must be made again.
  ['#b-size', '#b-format', '#b-quality', '#b-rembg', '#b-faces', '#b-upscale'].forEach(function (s) {
    $(s).addEventListener('change', function () { items.forEach(function (it) { it.out = null; state(it, 'Ready'); }); refresh(); });
  });
  $('#b-quality').addEventListener('input', function () { $('#b-qout').textContent = this.value; });

  function refresh() {
    var n = items.length;
    $('#b-zip').disabled = $('#b-upload').disabled = n === 0;
    var ai = ['#b-rembg', '#b-faces', '#b-upscale'];
    ai.forEach(function (s) { $(s).disabled = !server || !signedIn; });
    $('#b-ai-note').textContent = !server ? 'AI steps are not available on this server.'
      : !signedIn ? 'Sign in (free) to use the AI steps.' : 'AI steps run on our server, one image at a time.';
    if (!signedIn && n > maxBatch) $('#b-ai-note').textContent += ' Albums from guests hold up to ' + maxBatch + ' images; sign in for bigger ones.';
  }
  refresh();

  function state(it, text, err) { var s = it.row.querySelector('.state'); s.textContent = text; s.style.color = err ? 'var(--red)' : ''; }
  function status(text) { var s = $('#b-status'); s.style.display = text ? 'flex' : 'none'; s.textContent = text; }

  // ---- image work ----
  function toCanvas(src, w, h) {
    var c = document.createElement('canvas'); c.width = w || src.width; c.height = h || src.height;
    var x = c.getContext('2d'); x.imageSmoothingQuality = 'high'; x.drawImage(src, 0, 0, c.width, c.height); return c;
  }
  function shrink(c, side) {
    var k = side / Math.max(c.width, c.height);
    if (k >= 1) return c;
    var cur = c; // halve first for a clean downscale
    while (cur.width * k < cur.width / 2) cur = toCanvas(cur, Math.round(cur.width / 2), Math.round(cur.height / 2));
    return toCanvas(cur, Math.round(c.width * k), Math.round(c.height * k));
  }
  function blob(c, type, q) {
    return new Promise(function (res) {
      if (type === 'image/jpeg') { var w = toCanvas(c); var x = w.getContext('2d'); x.globalCompositeOperation = 'destination-over'; x.fillStyle = '#fff'; x.fillRect(0, 0, w.width, w.height); c = w; }
      c.toBlob(res, type, q);
    });
  }
  function canEncode(t) { var c = document.createElement('canvas'); c.width = c.height = 1; return c.toDataURL(t).indexOf('data:' + t) === 0; }

  function ai(name, c) {
    return blob(c, 'image/png').then(function (b) {
      var fd = new FormData(); fd.append('file', b, 'image.png');
      return fetch('/api/v1/tools/' + name, { method: 'POST', body: fd, headers: { Accept: 'image/png, application/json' } });
    }).then(function (r) {
      if (!r.ok) return r.json().then(function (j) { throw new Error(j.error || name + ' failed'); }, function () { throw new Error(name + ' failed (' + r.status + ')'); });
      return name === 'faces' ? r.json() : r.blob().then(createImageBitmap).then(function (b) { return toCanvas(b); });
    });
  }
  function blurFaces(c) {
    return ai('faces', c).then(function (j) {
      var x = c.getContext('2d'), kx = c.width / j.width, ky = c.height / j.height;
      (j.faces || []).forEach(function (f) {
        var w = f[2] * kx * 1.3, h = f[3] * ky * 1.45, fx = Math.max(0, f[0] * kx - (w - f[2] * kx) / 2), fy = Math.max(0, f[1] * ky - (h - f[3] * ky) / 2.6);
        w = Math.min(c.width - fx, w); h = Math.min(c.height - fy, h);
        var s = document.createElement('canvas'); s.width = Math.max(1, w / 12 | 0); s.height = Math.max(1, h / 12 | 0);
        s.getContext('2d').drawImage(c, fx, fy, w, h, 0, 0, s.width, s.height);
        x.imageSmoothingEnabled = true; x.drawImage(s, 0, 0, s.width, s.height, fx, fy, w, h);
      });
      return c;
    });
  }

  function process(it) {
    if (it.out) return Promise.resolve(it);
    var size = +$('#b-size').value, fmt = $('#b-format').value, q = +$('#b-quality').value / 100;
    state(it, 'Working…');
    return createImageBitmap(it.file, { imageOrientation: 'from-image' }).then(function (bmp) {
      var c = toCanvas(bmp), step = Promise.resolve(c);
      if ($('#b-upscale').checked) step = step.then(function (c) { var mp = c.width * c.height / 1e6; state(it, 'Upscaling…'); return ai('upscale', mp > 1 ? shrink(c, Math.floor(Math.max(c.width, c.height) / Math.sqrt(mp))) : c); });
      if ($('#b-rembg').checked) step = step.then(function (c) { state(it, 'Removing background…'); return ai('remove-background', c); });
      if ($('#b-faces').checked) step = step.then(function (c) { state(it, 'Blurring faces…'); return blurFaces(c); });
      if (size) step = step.then(function (c) { return shrink(c, size); });
      return step.then(function (c) {
        var type = fmt || it.file.type;
        if ($('#b-rembg').checked && (type === 'image/jpeg' || !fmt)) type = 'image/png'; // keep the transparency
        if (!EXT[type] || !canEncode(type) || type === 'image/gif') type = 'image/png';
        return blob(c, type, q).then(function (b) {
          it.out = b; it.name = it.file.name.replace(/\.[^.]+$/, '') + '.' + EXT[type];
          state(it, 'Done · ' + kb(it.file.size) + ' → ' + kb(b.size));
          return it;
        });
      });
    }).catch(function (e) { state(it, e.message || 'Failed', true); it.out = null; return it; });
  }
  function kb(n) { return n < 1048576 ? Math.round(n / 1024) + ' KB' : (n / 1048576).toFixed(1) + ' MB'; }

  // One at a time keeps memory low and the AI queue fair.
  function processAll() {
    var i = 0;
    return (function next() {
      if (i >= items.length) return Promise.resolve();
      status('Working on ' + (i + 1) + ' of ' + items.length + '…');
      return process(items[i++]).then(next);
    })().then(function () { status(''); return items.filter(function (it) { return it.out; }); });
  }

  // ---- ZIP (stored, UTF-8 names) ----
  var crcTable = (function () { var t = [], c; for (var n = 0; n < 256; n++) { c = n; for (var k = 0; k < 8; k++) c = c & 1 ? 0xEDB88320 ^ (c >>> 1) : c >>> 1; t[n] = c >>> 0; } return t; })();
  function crc32(u8) { var c = 0xFFFFFFFF; for (var i = 0; i < u8.length; i++) c = crcTable[(c ^ u8[i]) & 0xFF] ^ (c >>> 8); return (c ^ 0xFFFFFFFF) >>> 0; }
  function zip(files) { // files: [{name, data: Uint8Array}]
    var parts = [], central = [], offset = 0, enc = new TextEncoder(), used = {};
    var d = new Date(), dosTime = (d.getHours() << 11) | (d.getMinutes() << 5) | (d.getSeconds() >> 1),
      dosDate = ((d.getFullYear() - 1980) << 9) | ((d.getMonth() + 1) << 5) | d.getDate();
    files.forEach(function (f) {
      var name = f.name, n = 1;
      while (used[name]) name = f.name.replace(/(\.[^.]+)?$/, '-' + (++n) + '$1');
      used[name] = 1;
      var nm = enc.encode(name), crc = crc32(f.data), sz = f.data.length;
      var h = new DataView(new ArrayBuffer(30));
      h.setUint32(0, 0x04034b50, true); h.setUint16(4, 20, true); h.setUint16(6, 0x0800, true);
      h.setUint16(10, dosTime, true); h.setUint16(12, dosDate, true);
      h.setUint32(14, crc, true); h.setUint32(18, sz, true); h.setUint32(22, sz, true); h.setUint16(26, nm.length, true);
      parts.push(new Uint8Array(h.buffer), nm, f.data);
      var c = new DataView(new ArrayBuffer(46));
      c.setUint32(0, 0x02014b50, true); c.setUint16(4, 20, true); c.setUint16(6, 20, true); c.setUint16(8, 0x0800, true);
      c.setUint16(12, dosTime, true); c.setUint16(14, dosDate, true);
      c.setUint32(16, crc, true); c.setUint32(20, sz, true); c.setUint32(24, sz, true); c.setUint16(28, nm.length, true); c.setUint32(42, offset, true);
      central.push(new Uint8Array(c.buffer), nm);
      offset += 30 + nm.length + sz;
    });
    var cdSize = central.reduce(function (s, p) { return s + p.length; }, 0);
    var e = new DataView(new ArrayBuffer(22));
    e.setUint32(0, 0x06054b50, true); e.setUint16(8, files.length, true); e.setUint16(10, files.length, true);
    e.setUint32(12, cdSize, true); e.setUint32(16, offset, true);
    return new Blob(parts.concat(central, [new Uint8Array(e.buffer)]), { type: 'application/zip' });
  }

  $('#b-zip').addEventListener('click', function () {
    var b = this; b.disabled = true;
    processAll().then(function (done) {
      if (!done.length) throw new Error('Nothing to download');
      return Promise.all(done.map(function (it) { return it.out.arrayBuffer().then(function (a) { return { name: it.name, data: new Uint8Array(a) }; }); }));
    }).then(function (files) { window.phSave(zip(files), 'pixelhop-batch.zip'); toast(files.length + ' images in the ZIP'); })
      .catch(function (e) { toast(e.message, true); }).then(function () { b.disabled = false; });
  });

  // ---- upload as one album, in chunks ----
  $('#b-upload').addEventListener('click', function () {
    var b = this; b.disabled = true;
    processAll().then(function (done) {
      if (!done.length) throw new Error('Nothing to upload');
      if (!signedIn && done.length > maxBatch) { done = done.slice(0, maxBatch); toast('Guests can upload ' + maxBatch + ' at once; uploading the first ' + maxBatch, true); }
      var albumID = '', albumURL = '', sent = 0;
      var chunks = []; for (var i = 0; i < done.length; i += maxBatch) chunks.push(done.slice(i, i + maxBatch));
      return chunks.reduce(function (p, chunk) {
        return p.then(function () {
          status('Uploading ' + (sent + 1) + '–' + (sent + chunk.length) + ' of ' + done.length + '…');
          var fd = new FormData();
          chunk.forEach(function (it) { fd.append('file', it.out, it.name); });
          fd.append('title', 'Batch of ' + done.length);
          if (albumID) fd.append('album_id', albumID); else fd.append('album', 'force');
          return fetch('/api/v1/upload', { method: 'POST', body: fd, headers: { Accept: 'application/json' } }).then(function (r) { return r.json(); }).then(function (j) {
            if (!j.success) throw new Error(j.error || 'Upload failed');
            var url = j.data.album_url || (j.data.images && j.data.images[0] && j.data.images[0].album_url);
            if (url && !albumURL) { albumURL = url; albumID = url.split('/a/')[1]; }
            sent += chunk.length;
            chunk.forEach(function (it) { state(it, 'Uploaded'); });
          });
        });
      }, Promise.resolve()).then(function () {
        status('');
        if (albumURL) { $('#b-result').classList.remove('hide'); $('#b-link').value = albumURL; window.phCopy(albumURL); }
      });
    }).catch(function (e) { status(''); toast(e.message, true); }).then(function () { b.disabled = false; });
  });
})();
