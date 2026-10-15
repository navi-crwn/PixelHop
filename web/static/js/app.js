// PixelHop — site-wide behaviour: theme, menus, copy buttons, the uploader,
// the image page and the dashboard. Plain JS, no build step.
(function () {
  'use strict';
  var $ = function (s, el) { return (el || document).querySelector(s); };
  var $$ = function (s, el) { return Array.prototype.slice.call((el || document).querySelectorAll(s)); };

  // ---------- toasts ----------
  function toast(msg, err) {
    var box = $('#toasts'); if (!box) return;
    var t = document.createElement('div');
    t.className = 'toast' + (err ? ' err' : '');
    t.textContent = msg;
    box.appendChild(t);
    setTimeout(function () { t.remove(); }, 2600);
  }
  window.phToast = toast;

  function copy(text) {
    var done = function () { toast('Copied'); };
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(text).then(done, function () { fallback(text); done(); });
    } else { fallback(text); done(); }
  }
  function fallback(text) {
    var ta = document.createElement('textarea');
    ta.value = text; ta.style.position = 'fixed'; ta.style.opacity = '0';
    document.body.appendChild(ta); ta.select();
    try { document.execCommand('copy'); } catch (e) {}
    ta.remove();
  }
  window.phCopy = copy;

  // ---------- theme & nav ----------
  var themeBtn = $('#theme');
  if (themeBtn) themeBtn.addEventListener('click', function () {
    var root = document.documentElement;
    var dark = root.dataset.theme ? root.dataset.theme === 'dark' : !matchMedia('(prefers-color-scheme: light)').matches;
    root.dataset.theme = dark ? 'light' : 'dark';
    try { localStorage.setItem('theme', root.dataset.theme); } catch (e) {}
  });
  var burger = $('#burger');
  if (burger) burger.addEventListener('click', function () { $('#nav').classList.toggle('open'); });
  var menu = $('#menu');
  if (menu) {
    menu.querySelector('button').addEventListener('click', function (e) { e.stopPropagation(); menu.classList.toggle('open'); });
    document.addEventListener('click', function (e) { if (!menu.contains(e.target)) menu.classList.remove('open'); });
  }

  // ---------- generic helpers wired by data attributes ----------
  document.addEventListener('click', function (e) {
    var b = e.target.closest('[data-copy],[data-copy-text],[data-copy-prev]');
    if (b) {
      if (b.dataset.copyText) copy(b.dataset.copyText);
      else if (b.dataset.copy) copy($(b.dataset.copy).value);
      else copy(b.previousElementSibling.value);
      return;
    }
    var sx = e.target.closest('[data-sharex]');
    if (sx) {
      var cfg = {
        Version: '15.0.0', Name: sx.dataset.sharexName, DestinationType: 'ImageUploader', RequestMethod: 'POST',
        RequestURL: sx.dataset.sharexUrl, Headers: { Authorization: 'Bearer ' + sx.dataset.sharex },
        Body: 'MultipartFormData', FileFormName: 'file', URL: '{json:data.url}', ThumbnailURL: '{json:data.thumb_url}',
        ErrorMessage: '{json:error}'
      };
      save(new Blob([JSON.stringify(cfg, null, 2)], { type: 'application/json' }), sx.dataset.sharexName.toLowerCase() + '.sxcu');
    }
  });
  function save(blob, name) {
    var a = document.createElement('a');
    a.href = URL.createObjectURL(blob); a.download = name;
    document.body.appendChild(a); a.click(); a.remove();
    setTimeout(function () { URL.revokeObjectURL(a.href); }, 4000);
  }
  window.phSave = save;

  // "Copy all links" in one format, for every image in the surrounding list.
  document.addEventListener('click', function (e) {
    var b = e.target.closest('[data-copy-all]');
    if (!b) return;
    var scope = b.closest('[data-copy-all-root]') || document;
    var fmt = b.dataset.copyAll, lines = $$('[data-link]', scope).map(function (x) {
      var d = x.dataset.direct, p = x.dataset.page, t = (x.dataset.title || 'image').replace(/[\[\]]/g, '');
      return { direct: d, page: p, markdown: '![' + t + '](' + d + ')', bbcode: '[url=' + p + '][img]' + d + '[/img][/url]',
        html: '<a href="' + p + '"><img src="' + d + '" alt="' + t.replace(/"/g, '&quot;') + '"></a>' }[fmt];
    });
    if (!lines.length) return;
    copy(lines.join('\n'));
    toast(lines.length + ' ' + b.textContent.trim() + ' links copied');
  });

  // Confirmations and prompts without inline scripts (the CSP forbids them).
  document.addEventListener('submit', function (e) {
    var f = e.target, btn = e.submitter;
    var msg = (btn && btn.dataset.confirm) || f.dataset.confirm;
    if (msg && !confirm(msg)) { e.preventDefault(); return; }
    if (f.dataset.prompt) {
      var v = prompt(f.dataset.prompt);
      if (v === null) { e.preventDefault(); return; }
      f.querySelector('[name="' + f.dataset.promptField + '"]').value = v;
    }
  }, true);
  $$('[data-autosubmit]').forEach(function (s) { s.addEventListener('change', function () { s.form.submit(); }); });
  $$('input[readonly]').forEach(function (i) { i.addEventListener('focus', function () { i.select(); }); });

  // ---------- device history (guests' uploads + delete links) ----------
  var HKEY = 'ph_history';
  function loadHistory() { try { return JSON.parse(localStorage.getItem(HKEY) || '[]'); } catch (e) { return []; } }
  function remember(img) {
    var h = loadHistory().filter(function (x) { return x.id !== img.id; });
    h.unshift({ id: img.id, page: img.view_url, thumb: img.thumb_url, del: img.delete_url || '', t: Date.now(), title: img.title || img.id });
    try { localStorage.setItem(HKEY, JSON.stringify(h.slice(0, 200))); } catch (e) {}
  }
  function renderHistory() {
    var wrap = $('#device-history-wrap'), box = $('#device-history');
    if (!wrap || !box) return;
    var h = loadHistory();
    wrap.hidden = h.length === 0;
    box.innerHTML = '';
    h.slice(0, 24).forEach(function (x) {
      var tile = document.createElement('div'); tile.className = 'tile';
      var a = document.createElement('a'); a.href = x.page;
      var img = document.createElement('img'); img.src = x.thumb; img.alt = ''; img.loading = 'lazy';
      img.onerror = function () { tile.remove(); };
      a.appendChild(img); tile.appendChild(a);
      var cap = document.createElement('div'); cap.className = 'cap';
      var s = document.createElement('span'); s.textContent = x.title; cap.appendChild(s);
      if (x.del) { var d = document.createElement('a'); d.href = x.del; d.textContent = 'delete'; d.style.color = '#fff'; cap.appendChild(d); }
      tile.appendChild(cap); box.appendChild(tile);
    });
  }
  if ($('#deleted-ok')) {
    var gone = location.pathname.split('/')[2];
    try { localStorage.setItem(HKEY, JSON.stringify(loadHistory().filter(function (x) { return x.id !== gone; }))); } catch (e) {}
  }
  var clearBtn = $('#clear-history');
  if (clearBtn) clearBtn.addEventListener('click', function () {
    if (!confirm('Forget this list? The images stay online; you lose the delete links saved here.')) return;
    try { localStorage.removeItem(HKEY); } catch (e) {}
    renderHistory();
  });
  renderHistory();

  // ---------- uploader ----------
  var drop = $('#drop');
  if (drop) {
    var maxMB = +drop.dataset.maxMb, maxBatch = +drop.dataset.maxBatch;
    var fileInput = $('#file'), list = $('#uploads');
    var okTypes = /^image\/(jpeg|png|gif|webp|bmp)$/;

    $('#pick').addEventListener('click', function (e) { e.stopPropagation(); fileInput.click(); });
    $('#drop-in').addEventListener('click', function (e) {
      if (e.target.closest('#opts,#url-form,button,select,input,label')) return;
      fileInput.click();
    });
    $('#drop-in').addEventListener('keydown', function (e) { if ((e.key === 'Enter' || e.key === ' ') && e.target.id === 'drop-in') { e.preventDefault(); fileInput.click(); } });
    $('#show-url').addEventListener('click', function (e) { e.stopPropagation(); $('#url-form').classList.toggle('hide'); $('#url').focus(); });
    fileInput.addEventListener('change', function () { send(Array.prototype.slice.call(fileInput.files)); fileInput.value = ''; });
    $('#url-form').addEventListener('submit', function (e) {
      e.preventDefault();
      var u = $('#url').value.trim();
      if (u) { send([], u); $('#url').value = ''; }
    });

    ['dragenter', 'dragover'].forEach(function (ev) {
      document.addEventListener(ev, function (e) { if (hasFiles(e)) { e.preventDefault(); drop.classList.add('over'); } });
    });
    ['dragleave', 'drop'].forEach(function (ev) {
      document.addEventListener(ev, function (e) { if (ev === 'drop' || !e.relatedTarget) drop.classList.remove('over'); });
    });
    document.addEventListener('drop', function (e) {
      if (!hasFiles(e)) return;
      e.preventDefault();
      send(Array.prototype.slice.call(e.dataTransfer.files));
    });
    document.addEventListener('paste', function (e) {
      if (e.target.closest && e.target.closest('input,textarea')) return;
      var files = [];
      Array.prototype.forEach.call(e.clipboardData.items || [], function (it) {
        if (it.kind === 'file') { var f = it.getAsFile(); if (f) files.push(f); }
      });
      if (files.length) { e.preventDefault(); send(files); return; }
      var text = e.clipboardData.getData('text');
      if (/^https?:\/\/\S+$/i.test(text.trim())) { e.preventDefault(); send([], text.trim()); }
    });

    function hasFiles(e) { return e.dataTransfer && Array.prototype.indexOf.call(e.dataTransfer.types, 'Files') >= 0; }

    function row(name, preview) {
      var el = document.createElement('div'); el.className = 'up';
      var img = document.createElement('img'); img.className = 'thumb'; img.alt = '';
      if (preview) img.src = preview;
      var meta = document.createElement('div'); meta.className = 'meta';
      var n = document.createElement('div'); n.className = 'name'; n.textContent = name;
      var bar = document.createElement('div'); bar.className = 'bar'; bar.appendChild(document.createElement('i'));
      meta.appendChild(n); meta.appendChild(bar);
      var act = document.createElement('div'); act.className = 'actions';
      el.appendChild(img); el.appendChild(meta); el.appendChild(act);
      list.prepend(el);
      return { el: el, img: img, meta: meta, bar: bar.firstChild, act: act, name: n };
    }

    function done(r, im) {
      if (!r.img.src) r.img.src = im.thumb_url; // keep the local preview when there is one
      r.name.textContent = im.title || im.id;
      r.bar.parentNode.remove();
      var link = document.createElement('div'); link.className = 'link';
      var inp = document.createElement('input'); inp.type = 'text'; inp.readOnly = true; inp.value = im.view_url;
      inp.addEventListener('focus', function () { inp.select(); });
      link.appendChild(inp); r.meta.appendChild(link);
      if (im.duplicate) { var s = document.createElement('div'); s.className = 'small muted'; s.textContent = 'You uploaded this before, here is the same link.'; r.meta.appendChild(s); }
      r.act.innerHTML = '<button class="btn sm primary" type="button">Copy</button><button class="btn sm" type="button">Direct</button><a class="btn sm" href="' + im.view_url + '">Open</a>';
      r.act.children[0].addEventListener('click', function () { copy(im.view_url); });
      r.act.children[1].addEventListener('click', function () { copy(im.url); });
      // remembered for "Copy all links"
      r.el.setAttribute('data-link', ''); r.el.dataset.direct = im.url; r.el.dataset.page = im.view_url; r.el.dataset.title = im.title || im.id;
      var bar = $('#copy-all-bar');
      if (bar && $$('#uploads [data-link]').length > 1) bar.classList.remove('hide');
      remember(im);
    }

    function fail(r, msg) {
      r.el.classList.add('err');
      if (r.bar.parentNode) r.bar.parentNode.remove();
      var e = document.createElement('div'); e.className = 'small'; e.style.color = 'var(--red)'; e.textContent = msg;
      r.meta.appendChild(e);
    }

    function send(files, url) {
      files = files.filter(function (f) {
        if (!okTypes.test(f.type)) { toast(f.name + ': not a supported image', true); return false; }
        if (f.size > maxMB * 1048576) { toast(f.name + ' is larger than ' + maxMB + ' MB', true); return false; }
        return true;
      });
      if (!files.length && !url) return;
      if (files.length > maxBatch) { toast('Up to ' + maxBatch + ' images at once; uploading the first ' + maxBatch, true); files = files.slice(0, maxBatch); }

      var fd = new FormData();
      files.forEach(function (f) { fd.append('file', f, f.name); });
      if (url) fd.append('url', url);
      fd.append('expire', $('#expire').value);
      fd.append('slug', $('#slug').checked ? '1' : '0');
      fd.append('album', $('#album').checked ? '1' : '0');

      var rows = files.map(function (f) { return row(f.name, URL.createObjectURL(f)); });
      if (url) rows.push(row(url, ''));

      var xhr = new XMLHttpRequest();
      xhr.open('POST', '/api/v1/upload');
      xhr.setRequestHeader('Accept', 'application/json');
      xhr.upload.onprogress = function (e) {
        if (!e.lengthComputable) return;
        var p = Math.min(96, e.loaded / e.total * 100) + '%';
        rows.forEach(function (r) { r.bar.style.width = p; });
      };
      xhr.onload = function () {
        var res; try { res = JSON.parse(xhr.responseText); } catch (e) { res = { success: false, error: 'Unexpected answer from the server (' + xhr.status + ')' }; }
        if (!res.success) { rows.forEach(function (r) { fail(r, res.error || 'Upload failed'); }); return; }
        var imgs = res.data.images || [res.data], errs = res.data.errors || [];
        var used = 0;
        rows.forEach(function (r, i) {
          var err = errs.filter(function (x) { return x.file === (files[i] ? files[i].name : url); })[0];
          if (err) { fail(r, err.error); return; }
          if (imgs[used]) done(r, imgs[used++]);
        });
        if (res.data.album_url && imgs.length > 1) {
          $('#album-banner').classList.remove('hide');
          $('#album-link').value = res.data.album_url;
          $('#album-open').href = res.data.album_url;
        }
        if (used === 1 && rows.length === 1) copy(imgs[0].view_url);
        renderHistory();
      };
      xhr.onerror = function () { rows.forEach(function (r) { fail(r, 'Network error. Check your connection and try again.'); }); };
      xhr.send(fd);
    }
  }

  // ---------- image page ----------
  $$('.tabs [data-tab]').forEach(function (t) {
    t.addEventListener('click', function () {
      var card = t.closest('.card');
      $$('[data-tab]', card).forEach(function (x) { x.setAttribute('aria-selected', x === t); });
      $$('[data-panel]', card).forEach(function (p) { p.classList.toggle('hide', p.dataset.panel !== t.dataset.tab); });
    });
  });
  var photo = $('#photo');
  if (photo) photo.addEventListener('click', function () { $('#stage').classList.toggle('zoom'); });

  var dlg = $('#report');
  if (dlg) {
    $('#report-open').addEventListener('click', function () { dlg.showModal(); });
    dlg.addEventListener('close', function () {
      if (dlg.returnValue !== 'send') return;
      var f = $('#report-form'), fd = new FormData(f);
      if (!fd.get('reason')) return;
      fetch('/report/' + f.dataset.id, { method: 'POST', body: fd, headers: { Accept: 'application/json' } })
        .then(function (r) { return r.json(); })
        .then(function (j) { toast(j.success ? 'Thanks, the moderators will take a look.' : (j.error || 'Could not send the report'), !j.success); })
        .catch(function () { toast('Could not send the report', true); });
      f.reset();
    });
  }

  function api(method, path, body) {
    return fetch(path, { method: method, headers: { 'Content-Type': 'application/json', Accept: 'application/json' }, body: body ? JSON.stringify(body) : undefined })
      .then(function (r) { return r.json(); })
      .then(function (j) { if (!j.success) throw new Error(j.error || 'Failed'); return j.data; });
  }
  var ren = $('#rename');
  if (ren) ren.addEventListener('click', function () {
    var t = prompt('New name for this image:', $('#title').textContent.trim());
    if (t === null) return;
    api('PATCH', '/api/v1/images/' + ren.dataset.id, { title: t }).then(function (d) { $('#title').textContent = d.title || d.id; toast('Renamed'); }, function (e) { toast(e.message, true); });
  });
  var del = $('#delete');
  if (del) del.addEventListener('click', function () {
    if (!confirm('Delete this image for good? The link stops working right away.')) return;
    api('DELETE', '/api/v1/images/' + del.dataset.id).then(function () { location.href = '/dashboard'; }, function (e) { toast(e.message, true); });
  });

  // ---------- dashboard: select, copy, download & delete many ----------
  var selBtn = $('#select-mode'), gal = $('#my-gallery');
  if (selBtn && gal) {
    var picked = function () { return $$('.tick:checked', gal); };
    var count = function () { $('#sel-count').textContent = picked().length; };
    selBtn.addEventListener('click', function () {
      var on = gal.classList.toggle('selecting');
      $('#sel-actions').classList.toggle('hide', !on);
      selBtn.textContent = on ? 'Done' : 'Select';
    });
    gal.addEventListener('change', count);
    $('#select-all').addEventListener('click', function () {
      var all = $$('.tick', gal), on = picked().length < all.length;
      all.forEach(function (c) { c.checked = on; }); count();
    });
    $('#copy-selected').addEventListener('click', function () {
      var links = picked().map(function (c) { return c.closest('.tile').dataset.direct; });
      if (!links.length) return toast('Select some images first', true);
      copy(links.join('\n')); toast(links.length + ' direct links copied');
    });
    $('#zip-selected').addEventListener('click', function () {
      var f = $('#zip-form'); f.innerHTML = '';
      picked().forEach(function (c) { var i = document.createElement('input'); i.type = 'hidden'; i.name = 'id'; i.value = c.value; f.appendChild(i); });
      if (!f.children.length) return toast('Select some images first', true);
      f.submit();
    });
    $('#delete-selected').addEventListener('click', function () {
      var ids = picked().map(function (c) { return c.value; });
      if (!ids.length) return toast('Select some images first', true);
      if (!confirm('Delete ' + ids.length + ' images for good?')) return;
      Promise.all(ids.map(function (id) { return api('DELETE', '/api/v1/images/' + id).catch(function () {}); }))
        .then(function () { location.reload(); });
    });
  }

  var warn = $('dialog[data-open]');
  if (warn && warn.showModal) { warn.showModal(); warn.addEventListener('cancel', function (e) { e.preventDefault(); }); }

  // relative times refresh themselves
  $$('time[datetime]').forEach(function (t) { t.title = new Date(t.getAttribute('datetime')).toLocaleString(); });
})();
