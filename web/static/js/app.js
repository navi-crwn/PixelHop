// PixelHop — site-wide behaviour: theme, menus, copy buttons, the uploader,
// the image page and the dashboard. Plain JS, no build step.
(function () {
  'use strict';
  var $ = function (s, el) { return (el || document).querySelector(s); };
  var $$ = function (s, el) { return Array.prototype.slice.call((el || document).querySelectorAll(s)); };

  // Interface text: the English sentence is its own key; {name} marks a value.
  // The catalog (/i18n/<lang>.js) is loaded before this file.
  var I18N = window.PIXEL_I18N || {};
  function T(key, vars) {
    var s = Object.prototype.hasOwnProperty.call(I18N, key) ? I18N[key] : key;
    return vars ? s.replace(/\{(\w+)\}/g, function (m, k) { return Object.prototype.hasOwnProperty.call(vars, k) ? vars[k] : m; }) : s;
  }
  window.phT = T;

  // Questions and name prompts go through the HOP family's shared dialogs (hop-ui.js from
  // hel.ink, see base.html) and fall back to the browser's own when that script is not on the
  // page: offline, blocked, or not deployed yet. Both answer with a Promise.
  //   phConfirm(text | {title, message}, {danger, subject, ok, anchor})  -> true / false
  //   phPrompt({title, label, value, maxlength})                          -> the text, or null
  // A plain sentence is split after its first sentence: the question is the title, the rest the
  // message.
  function parts(text) {
    var m = /^(.*?(?:[?？。!！]|\.(?:\s|$)))\s*([\s\S]*)$/.exec(String(text));
    return m && m[2] ? { title: m[1], message: m[2] } : { title: String(text) };
  }
  function phConfirm(text, opts) {
    var o = Object.assign(typeof text === 'string' ? parts(text) : text, opts || {});
    if (window.HopUI && HopUI.confirm) return HopUI.confirm(o);
    return Promise.resolve(window.confirm([o.title, o.message].filter(Boolean).join('\n\n')));
  }
  function phPrompt(opts) {
    if (window.HopUI && HopUI.prompt) return HopUI.prompt(opts);
    return Promise.resolve(window.prompt(opts.title || opts.label || '', opts.value == null ? '' : opts.value));
  }
  window.phConfirm = phConfirm;
  window.phPrompt = phPrompt;

  // ---------- toasts ----------
  function toast(msg, err) {
    var box = $('#toasts'); if (!box) return;
    var t = document.createElement('div');
    t.className = 'toast' + (err ? ' err' : '');
    t.textContent = T(msg);
    box.appendChild(t);
    setTimeout(function () { t.remove(); }, err ? 5000 : 2600);
  }
  window.phToast = toast;

  function copy(text) {
    var done = function () { toast(T('Copied')); };
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
    if (window.hopTheme) window.hopTheme(root.dataset.theme); // the family cookie hop_theme (theme.js)
    try { localStorage.setItem('theme', root.dataset.theme); } catch (e) {}
  });
  // The phone navigation: the burger opens it, a tap outside, a chosen link or Escape closes it.
  var nav = $('#nav'), burger = $('#burger');
  function setNav(open) {
    if (!nav || !burger) return;
    nav.classList.toggle('open', open);
    burger.setAttribute('aria-expanded', String(open));
  }
  if (burger && nav) {
    burger.setAttribute('aria-expanded', 'false');
    burger.addEventListener('click', function (e) { e.stopPropagation(); setNav(!nav.classList.contains('open')); });
    nav.addEventListener('click', function (e) { if (e.target.closest('.nav-links a')) setNav(false); });
    document.addEventListener('click', function (e) { if (nav.classList.contains('open') && !e.target.closest('.nav-links, #burger')) setNav(false); });
  }
  // Drop-down menus (language, account): one open at a time; a tap outside or Escape closes them.
  var menus = $$('.menu');
  function closeMenus(except) {
    menus.forEach(function (m) {
      if (m === except) return;
      m.classList.remove('open');
      var b = m.querySelector(':scope > button'); if (b) b.setAttribute('aria-expanded', 'false');
    });
  }
  menus.forEach(function (m) {
    var b = m.querySelector(':scope > button'); if (!b) return;
    b.setAttribute('aria-expanded', 'false');
    b.addEventListener('click', function (e) {
      e.stopPropagation(); closeMenus(m);
      b.setAttribute('aria-expanded', String(m.classList.toggle('open')));
    });
  });
  document.addEventListener('click', function (e) { if (!e.target.closest('.menu')) closeMenus(); });
  document.addEventListener('keydown', function (e) {
    if (e.key !== 'Escape') return;
    closeMenus();
    if (nav && nav.classList.contains('open')) { setNav(false); burger.focus(); }
  });

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
    if (b.dataset.copyAll === 'report') {
      var root = b.closest('[data-copy-all-root]') || document.body, items = $$('[data-link]', scope);
      if (!items.length) return;
      var head = [], title = root.dataset.reportTitle || ($('#ticket') && $('#ticket').value.trim()), note = root.dataset.reportNote || ($('#note') && $('#note').value.trim());
      if (title) head.push(title);
      if (note) head.push(note);
      var link = root.dataset.reportLink || ($('#album-link') && !$('#album-banner').classList.contains('hide') && $('#album-link').value);
      if (link && items.length > 1) head.push(T('All files: {link}', { link: link }));
      var list = items.map(function (x, i) { return (i + 1) + '. ' + (x.dataset.title || T('file')) + ': ' + x.dataset.page; });
      copy((head.length ? head.join('\n') + '\n\n' : '') + list.join('\n'));
      return toast(T('{n} links copied, ready to paste in chat', { n: items.length }));
    }
    var fmt = b.dataset.copyAll, lines = $$('[data-link]', scope).map(function (x) {
      var d = x.dataset.direct, p = x.dataset.page, t = (x.dataset.title || 'image').replace(/[\[\]]/g, '');
      return { direct: d, page: p, markdown: '![' + t + '](' + d + ')', bbcode: '[url=' + p + '][img]' + d + '[/img][/url]',
        html: '<a href="' + p + '"><img src="' + d + '" alt="' + t.replace(/"/g, '&quot;') + '"></a>' }[fmt];
    });
    if (!lines.length) return;
    copy(lines.join('\n'));
    toast(T('{n} links copied', { n: lines.length }));
  });

  // Confirmations and prompts without inline scripts (the CSP forbids them).
  // A form may carry data-confirm, data-confirm-danger and data-confirm-subject (what goes), and
  // data-prompt with data-prompt-field. The question is a shared dialog; submit waits for it and
  // sends the form only when it is answered yes.
  var asked = false;
  document.addEventListener('submit', function (e) {
    var f = e.target, btn = e.submitter;
    var msg = (btn && btn.dataset.confirm) || f.dataset.confirm;
    var promptText = f.dataset.prompt;
    if (!msg && !promptText) return;
    if (asked) { asked = false; return; }
    e.preventDefault();
    var go = function () { asked = true; if (f.requestSubmit) { f.requestSubmit(btn); } else { f.submit(); } };
    if (promptText) {
      phPrompt({ title: promptText, value: '' }).then(function (v) {
        if (v === null) return;
        f.querySelector('[name="' + f.dataset.promptField + '"]').value = v;
        go();
      });
      return;
    }
    phConfirm(msg, {
      danger: f.dataset.confirmDanger != null,
      subject: f.dataset.confirmSubject || ''
    }).then(function (ok) { if (ok) go(); });
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
    // Folded by default: a few stacked thumbnails and a count. Inside, files
    // are grouped by day and shown a dozen at a time.
    $('#history-count').textContent = h.length;
    var peek = $('#history-peek'); peek.innerHTML = '';
    h.slice(0, 4).forEach(function (x) { var i = document.createElement('img'); i.src = x.thumb; i.alt = ''; i.loading = 'lazy'; i.onerror = function () { i.remove(); }; peek.appendChild(i); });
    var shown = +(box.dataset.shown || 12), day = '', grid = null;
    var label = function (t) {
      var d = new Date(t), today = new Date(); today.setHours(0, 0, 0, 0);
      var diff = Math.round((today - new Date(d.getFullYear(), d.getMonth(), d.getDate())) / 864e5);
      return diff === 0 ? T('Today') : diff === 1 ? T('Yesterday') : d.toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: d.getFullYear() === today.getFullYear() ? undefined : 'numeric' });
    };
    $('#history-more').hidden = h.length <= shown;
    h.slice(0, shown).forEach(function (x) {
      var l = label(x.t || Date.now());
      if (l !== day) {
        day = l; var hd = document.createElement('h3'); hd.className = 'history-day'; hd.textContent = l; box.appendChild(hd);
        grid = document.createElement('div'); grid.className = 'gallery history-grid'; box.appendChild(grid);
      }
      var tile = document.createElement('div'); tile.className = 'tile';
      var a = document.createElement('a'); a.href = x.page;
      var img = document.createElement('img'); img.src = x.thumb; img.alt = ''; img.loading = 'lazy';
      img.onerror = function () { tile.remove(); };
      a.appendChild(img); tile.appendChild(a);
      var cap = document.createElement('div'); cap.className = 'cap';
      var s = document.createElement('span'); s.textContent = x.title; cap.appendChild(s);
      if (x.del) { var d = document.createElement('a'); d.href = x.del; d.textContent = T('delete'); d.style.color = '#fff'; cap.appendChild(d); }
      tile.appendChild(cap); grid.appendChild(tile);
    });
  }
  var moreBtn = $('#history-more');
  if (moreBtn) moreBtn.addEventListener('click', function () {
    var box = $('#device-history'); box.dataset.shown = +(box.dataset.shown || 12) + 24; renderHistory();
  });
  if ($('#deleted-ok')) {
    var gone = location.pathname.split('/')[2];
    try { localStorage.setItem(HKEY, JSON.stringify(loadHistory().filter(function (x) { return x.id !== gone; }))); } catch (e) {}
  }
  var clearBtn = $('#clear-history');
  if (clearBtn) clearBtn.addEventListener('click', function () {
    phConfirm(T('Forget this list? The images stay online; you lose the delete links saved here.'), {
      message: T('The delete links saved in this browser are dropped. The images stay online.'),
      ok: T('Forget this list'), anchor: clearBtn
    }).then(function (ok) {
      if (!ok) return;
      try { localStorage.removeItem(HKEY); } catch (e) {}
      renderHistory();
    });
  });
  renderHistory();

  // ---------- claim guest uploads after signing in ----------
  // The delete link of every guest upload is kept in this browser (ph_history). Its last two parts are
  // the image id and the secret token. Claiming sends them in the body of one POST, never in a URL.
  (function claim() {
    var box = $('#claim-notice');
    if (!box) return;
    var DEL = /\/delete\/([A-Za-z0-9_-]{1,64})\/([A-Za-z0-9_-]{1,128})\/?$/;
    function pending() {
      var out = [];
      loadHistory().forEach(function (x) { var m = DEL.exec(x.del || ''); if (m) out.push({ id: m[1], token: m[2], thumb: x.thumb }); });
      return out;
    }
    function snoozed() { try { return sessionStorage.getItem('ph_claim_later') === '1'; } catch (e) { return false; } }
    function show() {
      var list = pending();
      if (!list.length || snoozed()) { box.hidden = true; return; }
      $('#claim-title').textContent = list.length === 1 ? T('claim.title_one') : T('claim.title', { n: list.length });
      var peek = $('#claim-peek'); peek.innerHTML = '';
      list.slice(0, 3).forEach(function (x) { var i = document.createElement('img'); i.src = x.thumb; i.alt = ''; i.loading = 'lazy'; i.onerror = function () { i.remove(); }; peek.appendChild(i); });
      box.hidden = false;
    }
    function say(text) { var r = $('#claim-result'); r.textContent = text; r.hidden = false; $('#claim-desc').hidden = true; }
    $('#claim-later').addEventListener('click', function () { try { sessionStorage.setItem('ph_claim_later', '1'); } catch (e) {} box.hidden = true; });
    $('#claim-go').addEventListener('click', function () {
      var list = pending().slice(0, 200);
      if (!list.length) return;
      box.setAttribute('aria-busy', 'true');
      fetch(box.dataset.claimUrl, {
        method: 'POST', credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json', 'Accept': 'application/json' },
        body: JSON.stringify({ items: list.map(function (x) { return { id: x.id, token: x.token }; }) })
      }).then(function (res) { return res.json().then(function (j) { return { ok: res.ok, j: j }; }); }).then(function (r) {
        box.removeAttribute('aria-busy');
        if (!r.ok || !r.j.success) { say(T('claim.error')); return; }
        var d = r.j.data, gone = {}, dead = {}, kept = 0;
        (d.claimed || []).concat(d.already || []).forEach(function (id) { gone[id] = 1; });
        (d.skipped || []).forEach(function (s) { if (s.reason === 'invalid') dead[s.id] = 1; else kept++; });
        // Forget the tokens that are spent or useless; keep the ones that may still work later.
        var h = loadHistory().filter(function (x) { return !gone[x.id]; }).map(function (x) { if (dead[x.id]) x.del = ''; return x; });
        try { localStorage.setItem(HKEY, JSON.stringify(h)); } catch (e) {}
        var n = (d.claimed || []).length;
        say(n ? T(kept ? 'claim.done_some' : 'claim.done', { n: n, k: kept }) : T('claim.none'));
        $('#claim-go').hidden = true; $('#claim-later').hidden = true;
        $('#claim-title').textContent = n ? T('claim.done_title') : T('claim.none_title');
        if (n) { var a = document.createElement('a'); a.className = 'btn sm'; a.href = '/dashboard'; a.textContent = T('claim.view'); box.querySelector('.claim-actions').appendChild(a); }
        renderHistory();
      }).catch(function () { box.removeAttribute('aria-busy'); say(T('claim.error')); });
    });
    show();
  })();

  // ---------- uploader ----------
  var drop = $('#drop');
  if (drop) {
    var maxMB = +drop.dataset.maxMb, maxBatch = +drop.dataset.maxBatch;
    var fileInput = $('#file'), list = $('#uploads');
    var okTypes = /^image\/(jpeg|png|gif|webp|bmp|heic|heif|avif)$/, okExt = /\.(jpe?g|png|gif|webp|bmp|heic|heif|avif)$/i;

    $('#pick').addEventListener('click', function (e) { e.stopPropagation(); fileInput.click(); });
    $$('[data-pick-files]').forEach(function (b) { b.addEventListener('click', function () { fileInput.click(); }); });
    $('#drop-in').addEventListener('click', function (e) {
      if (e.target.closest('#opts,#url-form,#paste-box,#chips,#ticket-box,button,select,input,textarea,label,details')) return;
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
      if (/^https?:\/\/\S+$/i.test(text.trim())) { e.preventDefault(); send([], text.trim()); return; }
      if (e.target.id === 'paste-box') { e.preventDefault(); toast(T('There is no image in the clipboard. Copy an image first.'), true); }
    });

    // Phones have no Ctrl+V: the Paste button reads the clipboard directly
    // (Chrome asks once for permission). Where that isn't allowed, a box
    // appears that takes a long-press → Paste instead.
    var pasteBox = $('#paste-box');
    function pasteName(type) {
      var d = new Date(), p = function (n) { return ('0' + n).slice(-2); };
      return 'pasted-' + d.getFullYear() + p(d.getMonth() + 1) + p(d.getDate()) + '-' + p(d.getHours()) + p(d.getMinutes()) + p(d.getSeconds()) + '.' + (type.split('/')[1] || 'png').replace('jpeg', 'jpg');
    }
    function showPasteBox() { pasteBox.classList.remove('hide'); pasteBox.focus(); }
    $('#paste-btn').addEventListener('click', function (e) {
      e.stopPropagation();
      if (!navigator.clipboard || !navigator.clipboard.read) return showPasteBox();
      navigator.clipboard.read().then(function (items) {
        var jobs = [];
        items.forEach(function (it) {
          var t = it.types.filter(function (x) { return /^image\//.test(x); })[0];
          if (t) jobs.push(it.getType(t).then(function (b) { return new File([b], pasteName(t), { type: t }); }));
        });
        if (jobs.length) return Promise.all(jobs).then(function (files) { send(files); });
        return navigator.clipboard.readText().then(function (txt) {
          txt = (txt || '').trim();
          if (/^https?:\/\/\S+$/i.test(txt)) return send([], txt);
          toast(T('There is no image in the clipboard. Copy an image first.'), true);
        });
      }).catch(function () { showPasteBox(); });
    });
    pasteBox.addEventListener('input', function () { pasteBox.innerHTML = ''; });
    pasteBox.addEventListener('blur', function () { if (!pasteBox.textContent) pasteBox.classList.add('hide'); });

    function hasFiles(e) { return e.dataTransfer && Array.prototype.indexOf.call(e.dataTransfer.types, 'Files') >= 0; }

    // Options and the ticket stay folded away until asked for.
    function fold(btnSel, boxSel) {
      var b = $(btnSel), box = $(boxSel); if (!b || !box) return;
      b.addEventListener('click', function (e) {
        e.stopPropagation();
        var open = box.classList.toggle('hide') === false;
        b.setAttribute('aria-expanded', String(open));
        if (open) {
          var first = box.querySelector('select, input, textarea, button');
          if (first) first.focus();
        }
      });
      box.addEventListener('keydown', function (e) {
        if (e.key === 'Escape') {
          e.stopPropagation();
          box.classList.add('hide');
          b.setAttribute('aria-expanded', 'false');
          b.focus();
        }
      });
    }
    fold('#opts-toggle', '#opts'); fold('#ticket-toggle', '#ticket-box');
    var ticketInput = $('#ticket');
    if (ticketInput) ticketInput.addEventListener('input', function () {
      $('#ticket-toggle').lastChild.textContent = ticketInput.value.trim() ? 'Ticket: ' + ticketInput.value.trim().slice(0, 24) : 'Ticket & note';
    });

    // Results appear beside the upload box (below it on phones).
    function showResults() {
      var first = $('#results').classList.contains('hide');
      $('#results').classList.remove('hide'); $('#hero-grid').classList.add('has-results'); if ($('.hero')) $('.hero').classList.add('compact');
      // on phones the results sit below the upload box: bring them into view
      if (first && window.matchMedia && matchMedia('(max-width: 960px)').matches) setTimeout(function () { $('#results').scrollIntoView({ behavior: 'smooth', block: 'start' }); }, 60);
    }
    $('#results-clear').addEventListener('click', function () {
      list.innerHTML = ''; $('#results').classList.add('hide'); $('#hero-grid').classList.remove('has-results'); if ($('.hero')) $('.hero').classList.remove('compact');
      $('#album-banner').classList.add('hide'); $('#copy-all-bar').classList.add('hide');
    });

    function row(name, preview) {
      showResults();
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

    // A few results stay here; a bigger batch gets a page of its own. On the dashboard the page
    // never moves: the files are already under "My files", so the results page is linked instead.
    function settle(r, id) {
      var b = r.batch; if (!b) return;
      if (id) b.ids.push(id);
      if (--b.left > 0 || b.ids.length <= 3) return;
      if ($('#my-gallery')) {
        var done = $('#up-done');
        if (done) {
          done.innerHTML = '';
          var a = document.createElement('a');
          a.href = '/uploaded?id=' + b.ids.join(',');
          a.textContent = T('upload.done_see', { n: b.ids.length });
          done.appendChild(a);
          done.classList.remove('hide');
        }
        return;
      }
      setTimeout(function () { location.href = '/uploaded?id=' + b.ids.join(','); }, 500);
    }

    function done(r, im) {
      if (!r.img.src) r.img.src = im.thumb_url; // keep the local preview when there is one
      r.name.textContent = im.title || im.id;
      r.bar.parentNode.remove();
      var link = document.createElement('div'); link.className = 'link';
      var inp = document.createElement('input'); inp.type = 'text'; inp.readOnly = true; inp.value = im.view_url;
      inp.addEventListener('focus', function () { inp.select(); });
      link.appendChild(inp); r.meta.appendChild(link);
      if (im.removed && im.removed.length) { var rm = document.createElement('div'); rm.className = 'small muted'; rm.textContent = T('Removed hidden data: {list}', { list: im.removed.join(', ') }); r.meta.appendChild(rm); }
      if (im.duplicate) { var s = document.createElement('div'); s.className = 'small muted'; s.textContent = T('You uploaded this before, here is the same link.'); r.meta.appendChild(s); }
      r.act.innerHTML = '<button class="btn sm primary" type="button"></button><button class="btn sm" type="button"></button><a class="btn sm" href="' + im.view_url + '"></a>';
      r.act.children[0].textContent = T('Copy'); r.act.children[1].textContent = T('Direct'); r.act.children[2].textContent = T('Open');
      r.act.children[0].addEventListener('click', function () { copy(im.view_url); });
      r.act.children[1].addEventListener('click', function () { copy(im.url); });
      // remembered for "Copy all links"
      r.el.setAttribute('data-link', ''); r.el.dataset.direct = im.url; r.el.dataset.page = im.view_url; r.el.dataset.title = im.title || im.id;
      var bar = $('#copy-all-bar');
      if (bar && $$('#uploads [data-link]').length > 1) bar.classList.remove('hide');
      remember(im);
      addToGallery(im);
      refreshNumbers();
      settle(r, im.id);
    }

    // After an upload the dashboard's numbers (files by kind, storage, albums, the filter chips and the
    // "My files" count) come fresh from the server instead of waiting for a reload.
    var numbersTimer;
    function refreshNumbers() {
      if (!$('[data-stat="files"]')) return;
      clearTimeout(numbersTimer);
      numbersTimer = setTimeout(function () {
        api('GET', '/api/v1/me').then(function (me) {
          var k = me.kinds || {}, set = function (sel, v) { var el = $(sel); if (el && v !== undefined) el.textContent = v; };
          set('[data-stat="files"]', me.files);
          set('[data-stat="files_sub"]', T('dash.files_sub', { images: k.image, videos: k.video, pdfs: k.pdf }));
          set('[data-stat="albums"]', me.albums);
          set('[data-stat="storage"]', fmtBytes(me.storage_used));
          set('[data-count="all"]', me.files); set('[data-count="image"]', k.image); set('[data-count="video"]', k.video); set('[data-count="pdf"]', k.pdf);
          var tot = $('[data-stat="total"]');
          if (tot && !tot.dataset.filtered) {
            var kind = tot.dataset.kind;
            tot.textContent = kind === 'image' ? T('album.n_images', { n: k.image }) : kind === 'video' ? T('dash.n_videos', { n: k.video })
              : kind === 'pdf' ? T('dash.n_pdfs', { n: k.pdf }) : T('dash.n_files', { n: me.files });
          }
        }, function () {});
      }, 400);
    }
    function fmtBytes(b) {
      if (!(b >= 1024)) return (b || 0) + ' B';
      var u = 'KMGTPE', e = -1;
      do { b /= 1024; e++; } while (b >= 1024 && e < u.length - 1);
      return (b >= 100 ? b.toFixed(0) : b.toFixed(1)) + ' ' + u[e] + 'B';
    }

    // On the dashboard a new upload lands at the top of "My files" straight away.
    function addToGallery(im) {
      var g = $('#my-gallery'); if (!g || im.duplicate || g.querySelector('[data-id="' + im.id + '"]')) return;
      g.hidden = false; if ($('#dash-empty')) $('#dash-empty').remove();
      var tile = document.createElement('div'); tile.className = 'tile fresh'; tile.dataset.id = im.id;
      tile.dataset.direct = im.url; tile.dataset.page = im.view_url;
      var a = document.createElement('a'); a.href = im.view_url;
      var img = document.createElement('img'); img.src = im.thumb_url; img.alt = im.title || ''; img.loading = 'lazy';
      a.appendChild(img); tile.appendChild(a);
      var tick = document.createElement('input'); tick.type = 'checkbox'; tick.className = 'tick'; tick.value = im.id; tick.setAttribute('aria-label', T('Select {title}', { title: im.title || im.id }));
      tile.appendChild(tick);
      var cap = document.createElement('div'); cap.className = 'cap';
      var t = document.createElement('span'); t.textContent = im.title || im.id; cap.appendChild(t);
      var nb = document.createElement('span'); nb.className = 'pill acc'; nb.textContent = T('new'); cap.appendChild(nb);
      tile.appendChild(cap);
      g.prepend(tile);
      var c = $('[data-stat="files"]'); if (c) c.textContent = +c.textContent + 1;
    }

    function fail(r, msg) {
      r.el.classList.add('err');
      if (r.bar.parentNode) r.bar.parentNode.remove();
      var e = document.createElement('div'); e.className = 'small'; e.style.color = 'var(--red)'; e.textContent = msg;
      r.meta.appendChild(e);
      settle(r);
    }

    var maxMediaMB = +drop.dataset.maxMediaMb || maxMB, memberMediaMB = +drop.dataset.memberMediaMb || maxMediaMB, signedIn = !!drop.dataset.signedIn;
    var mediaRe = /^(video\/|application\/pdf$)/, mediaExt = /\.(mp4|mov|m4v|webm|3gp|mkv|pdf)$/i;
    function isMedia(f) { return mediaRe.test(f.type) || (!f.type && mediaExt.test(f.name)); }

    // One global progress line for the whole batch: bytes sent over the running jobs (one multipart
    // request for the images, one job per big file), so the file count for images only lands when
    // their request does.
    var upJobs = [];
    function upTick() {
      var box = $('#up-global'); if (!box) return;
      var total = 0, done = 0, live = 0, files = 0, doneFiles = 0, active = 0;
      upJobs.forEach(function (j) {
        total += j.bytes; files += j.files;
        if (j.done) { done += j.bytes; doneFiles += j.files; } else { live += j.live; active++; }
      });
      if (!active) { box.classList.add('hide'); return; }
      var pct = total ? Math.min(100, Math.round((done + live) / total * 100)) : 0;
      box.classList.remove('hide');
      $('#up-global-bar').style.width = pct + '%';
      $('#up-global-text').textContent = T('upload.progress', { done: doneFiles, total: files, pct: pct });
    }
    function upJob(bytes, files) {
      var j = { bytes: bytes || 1, files: files || 1, live: 0, done: false };
      upJobs.push(j); upTick();
      return j;
    }

    function send(files, url) {
      files = files.filter(function (f) {
        if (isMedia(f)) {
          if (f.size > maxMediaMB * 1048576) { toast(T('{name} is larger than {mb} MB.', { name: f.name, mb: maxMediaMB }) + (!signedIn && memberMediaMB > maxMediaMB ? ' ' + T('Sign in to upload videos and PDFs up to {mb} MB.', { mb: memberMediaMB }) : ''), true); return false; }
          return true;
        }
        if (!okTypes.test(f.type) && !(f.type === '' && okExt.test(f.name))) { toast(T('{name}: not a supported file', { name: f.name }), true); return false; }
        if (f.size > maxMB * 1048576) { toast(T('{name} is larger than {mb} MB', { name: f.name, mb: maxMB }), true); return false; }
        return true;
      });
      if (!files.length && !url) return;
      if (files.length > maxBatch) { toast(T('Up to {n} files at once; uploading the first {n}', { n: maxBatch }), true); files = files.slice(0, maxBatch); }

      var media = files.filter(isMedia), images = files.filter(function (f) { return !isMedia(f); });
      var ticket = $('#ticket') ? $('#ticket').value.trim() : '', note = $('#note') ? $('#note').value.trim() : '';
      var count = files.length + (url ? 1 : 0);
      // A ticket, or a mix with videos/PDFs, becomes one album made up front.
      var wantAlbum = signedIn && (ticket || (media.length && count > 1 && $('#album').checked));
      var batch = { left: count, ids: [] };
      var mediaRows = media.map(function (f) { var r = row(f.name, ''); r.batch = batch; return r; });
      var albumReady = wantAlbum
        ? api('POST', '/api/v1/albums', { title: ticket, admin_note: note, content_warning: $('#upload-warning').value, warning_reason: $('#upload-warning-reason').value }).then(function (al) {
            $('#album-banner').classList.remove('hide');
            $('#album-link').value = al.url; $('#album-open').href = al.url;
            return al.id;
          }, function (e) { toast(T('Could not make the album: {error}', { error: e.message }), true); return ''; })
        : Promise.resolve('');

      albumReady.then(function (albumId) {
        if (images.length || url) sendImages(images, url, albumId, ticket, note, batch);
        // Big files go one at a time, in pieces, so a weak phone signal only repeats one piece.
        media.reduce(function (p, f, i) { return p.then(function () { return sendMedia(f, mediaRows[i], albumId); }); }, Promise.resolve())
          .then(renderHistory);
      });
    }

    function sendImages(files, url, albumId, ticket, note, batch) {
      var fd = new FormData();
      files.forEach(function (f) { fd.append('file', f, f.name); });
      if (url) fd.append('url', url);
      fd.append('expire', $('#expire').value);
      fd.append('content_warning', $('#upload-warning').value);
      fd.append('warning_reason', $('#upload-warning-reason').value);
      fd.append('slug', $('#slug').checked ? '1' : '0');
      fd.append('album', $('#album').checked ? '1' : '0');
      if (albumId) fd.append('album_id', albumId);
      else if (ticket) { fd.append('album', 'force'); fd.append('title', ticket); fd.append('note', note); }
      if ($('#listed') && $('#listed').checked) fd.append('listed', '1');

      var rows = files.map(function (f) { return row(f.name, /heic|heif|avif/i.test(f.type + f.name) ? '' : URL.createObjectURL(f)); });
      if (url) rows.push(row(url, ''));
      rows.forEach(function (r) { r.batch = batch; });

      var job = upJob(files.reduce(function (s, f) { return s + (f.size || 0); }, url ? 300000 : 0), files.length + (url ? 1 : 0));
      var xhr = new XMLHttpRequest();
      xhr.open('POST', '/api/v1/upload');
      xhr.setRequestHeader('Accept', 'application/json');
      xhr.upload.onprogress = function (e) {
        if (!e.lengthComputable) return;
        job.live = e.loaded; upTick();
        var p = Math.min(96, e.loaded / e.total * 100) + '%';
        rows.forEach(function (r) { r.bar.style.width = p; });
      };
      xhr.onload = function () {
        var res; try { res = JSON.parse(xhr.responseText); } catch (e) { res = { success: false, error: T('Unexpected answer from the server ({status})', { status: xhr.status }) }; }
        if (!res.success) { rows.forEach(function (r) { fail(r, res.error || T('Upload failed')); }); return; }
        var imgs = res.data.images || [res.data], errs = res.data.errors || [];
        var used = 0;
        rows.forEach(function (r, i) {
          var err = errs.filter(function (x) { return x.file === (files[i] ? files[i].name : url); })[0];
          if (err) { fail(r, err.error); return; }
          if (imgs[used]) done(r, imgs[used++]);
        });
        if (res.data.album_url && !albumId && (imgs.length > 1 || ticket)) {
          $('#album-banner').classList.remove('hide');
          $('#album-link').value = res.data.album_url;
          $('#album-open').href = res.data.album_url;
        }
        if (used === 1 && rows.length === 1 && !ticket) copy(imgs[0].view_url);
        else if (res.data.album_url && imgs.length > 1 && !ticket) copy(res.data.album_url); // several files: the album link is the one to paste
        renderHistory();
        job.done = true; upTick();
      };
      xhr.onerror = function () { rows.forEach(function (r) { fail(r, T('Network error. Check your connection and try again.')); }); job.done = true; upTick(); };
      xhr.send(fd);
    }

    function putChunk(id, off, blob, onProgress) {
      return new Promise(function (res, rej) {
        var x = new XMLHttpRequest();
        x.open('PUT', '/api/v1/uploads/' + id + '?offset=' + off);
        x.setRequestHeader('Accept', 'application/json');
        x.upload.onprogress = function (e) { if (e.lengthComputable) onProgress(e.loaded); };
        x.onload = function () {
          var j; try { j = JSON.parse(x.responseText); } catch (e) { j = {}; }
          if (x.status === 200 && j.success) return res(j.data.received);
          if (x.status === 409 && typeof j.received === 'number') return res(j.received); // carry on from where the server is
          var err = new Error(j.error || T('Upload failed ({status})', { status: x.status })); err.fatal = x.status === 404 || x.status === 400; rej(err);
        };
        x.onerror = function () { rej(new Error(T('Network error'))); };
        x.send(blob);
      });
    }

    function sendMedia(f, r, albumId) {
      var size = f.size, tries = 0, job = upJob(f.size, 1);
      return api('POST', '/api/v1/uploads', { name: f.name, size: size }).then(function (start) {
        var cs = start.chunk_size;
        function from(off) {
          if (off >= size) return Promise.resolve();
          return putChunk(start.id, off, f.slice(off, off + cs), function (n) { job.live = Math.min(job.bytes, off + n); upTick(); r.bar.style.width = Math.min(97, (off + n) / size * 100) + '%'; })
            .then(function (got) { tries = 0; return from(got); }, function (e) {
              if (e.fatal || ++tries > 6) throw e;
              r.name.textContent = f.name + ' · ' + T('connection lost, retrying…');
              return new Promise(function (ok) { setTimeout(ok, 1500 * tries); }).then(function () { r.name.textContent = f.name; return from(off); });
            });
        }
        return from(0).then(function () {
          r.name.textContent = f.name + ' · ' + (/pdf$/i.test(f.type + f.name) ? T('compressing…') : T('preparing video…'));
          var fd = new FormData();
          fd.append('expire', $('#expire').value);
          fd.append('content_warning', $('#upload-warning').value);
          fd.append('warning_reason', $('#upload-warning-reason').value);
          fd.append('slug', $('#slug').checked ? '1' : '0');
          if (albumId) fd.append('album_id', albumId);
          return fetch('/api/v1/uploads/' + start.id + '/finish', { method: 'POST', body: fd, headers: { Accept: 'application/json' } }).then(function (x) { return x.json(); });
        });
      }).then(function (j) {
        if (!j.success) throw new Error(j.error || T('Upload failed'));
        done(r, j.data);
        job.done = true; upTick();
      }).catch(function (e) { fail(r, e.message || T('Upload failed')); job.done = true; upTick(); });
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
        .then(function (j) { toast(j.success ? T('Thanks, the moderators will take a look.') : (j.error || T('Could not send the report')), !j.success); })
        .catch(function () { toast(T('Could not send the report'), true); });
      f.reset();
    });
  }

  function api(method, path, body) {
    return fetch(path, { method: method, headers: { 'Content-Type': 'application/json', Accept: 'application/json' }, body: body ? JSON.stringify(body) : undefined })
      .then(function (r) {
        // nginx or Cloudflare can answer with an HTML page (file too large, timeout): say what happened.
        return r.json().catch(function () { throw new Error(r.status === 413 ? T('That is too large for the server') : T('The server answered {status}. Try again in a moment.', { status: r.status })); });
      })
      .then(function (j) { if (!j.success) throw new Error(j.error || T('Failed')); return j.data; });
  }
  var ren = $('#rename');
  if (ren) ren.addEventListener('click', function () {
    phPrompt({ title: T('New name for this image:'), value: $('#title').textContent.trim(), maxlength: 200, anchor: ren })
      .then(function (t) {
        if (t === null) return;
        api('PATCH', '/api/v1/images/' + ren.dataset.id, { title: t }).then(function (d) { $('#title').textContent = d.title || d.id; toast(T('Renamed')); }, function (e) { toast(e.message, true); });
      });
  });
  // dialogs opened by buttons
  document.addEventListener('click', function (e) {
    var b = e.target.closest('[data-open-dialog]'); if (b) $(b.dataset.openDialog).showModal();
  });
  var ef = $('#edit-form');
  if (ef) {
    var passRow = $('#e-pass-row'), vis = function () { var c = ef.querySelector('[name=vis]:checked'); return c ? c.value : 'unlisted'; };
    var sync = function () { passRow.style.display = vis() === 'password' ? '' : 'none'; };
    ef.addEventListener('change', sync); sync();
    $('#edit-image').addEventListener('close', function () {
      if ($('#edit-image').returnValue !== 'save') return;
      var body = { title: $('#e-title').value, alt: $('#e-alt').value, tags: $('#e-tags').value.split(','), visibility: vis() };
      if (vis() === 'password' && $('#e-pass').value) body.password = $('#e-pass').value;
      if ($('#e-listed')) body.listed = $('#e-listed').checked;
      var cwEl = ef.querySelector('[name=content_warning]:checked');
      if (cwEl) body.content_warning = cwEl.value;
      var wrEl = $('#e-warning-reason');
      if (wrEl) body.warning_reason = wrEl.value;
      api('PATCH', '/api/v1/images/' + ef.dataset.id, body).then(function () { location.reload(); }, function (e) { toast(e.message, true); });
    });
  }
  var st = $('#stats');
  if (st) fetch('/api/v1/images/' + st.dataset.id + '/stats', { headers: { Accept: 'application/json' } }).then(function (r) { return r.json(); }).then(function (j) {
    var d = j.data, byDay = {}, max = 1, total = 0;
    (d.days || []).forEach(function (x) { byDay[x.day] = x.views; max = Math.max(max, x.views); total += x.views; });
    var bars = $('#stats-bars');
    for (var i = 29; i >= 0; i--) {
      var day = new Date(Date.now() - i * 864e5).toISOString().slice(0, 10), v = byDay[day] || 0;
      var el = document.createElement('i'); el.style.height = Math.max(2, v / max * 100) + '%'; el.title = T('{day}: {n} views', { day: day, n: v }); bars.appendChild(el);
    }
    $('#stats-total').textContent = '· ' + T('{n} in 30 days, {all} all time', { n: total, all: d.views });
    var ref = $('#stats-ref'); ref.textContent = '';
    if (!(d.referrers || []).length) { ref.textContent = T('No other sites yet: views came from direct links, chats or apps.'); return; }
    d.referrers.forEach(function (x) { var row = document.createElement('div'); row.className = 'ref-row'; row.innerHTML = '<span></span><b></b>'; row.firstChild.textContent = x.host; row.lastChild.textContent = x.views; ref.appendChild(row); });
  });
  var lt = $('#list-toggle');
  if (lt) lt.addEventListener('click', function () {
    var on = lt.dataset.listed !== '1';
    api('PATCH', '/api/v1/images/' + lt.dataset.id, { listed: on }).then(function () {
      lt.dataset.listed = on ? '1' : ''; lt.querySelector('span').textContent = on ? T('On my profile') : T('Show on my profile');
      toast(on ? T('Now shown on your public profile') : T('Removed from your profile'));
    }, function (e) { toast(e.message, true); });
  });
  var del = $('#delete');
  if (del) del.addEventListener('click', function () {
    phConfirm(T('Move this image to the trash? The link stops working now; you can restore it for 7 days.'), {
      danger: true, anchor: del
    }).then(function (ok) {
      if (!ok) return;
      api('DELETE', '/api/v1/images/' + del.dataset.id).then(function () { location.href = '/dashboard'; }, function (e) { toast(e.message, true); });
    });
  });

  // ---------- dashboard: select, copy, download & delete many ----------
  var selBtn = $('#select-mode'), gal = $('#my-gallery');
  if (selBtn && gal) {
    var picked = function () { return $$('.tick:checked', gal); };
    var count = function () { $('#sel-count').textContent = picked().length; };
    selBtn.addEventListener('click', function () {
      var on = gal.classList.toggle('selecting');
      $('#sel-actions').classList.toggle('hide', !on);
      selBtn.textContent = on ? T('Done') : T('Select');
    });
    gal.addEventListener('change', count);
    $('#select-all').addEventListener('click', function () {
      var all = $$('.tick', gal), on = picked().length < all.length;
      all.forEach(function (c) { c.checked = on; }); count();
    });
    $('#copy-selected').addEventListener('click', function () {
      var links = picked().map(function (c) { return c.closest('.tile').dataset.direct; });
      if (!links.length) return toast(T('Select some images first'), true);
      copy(links.join('\n')); toast(T('{n} direct links copied', { n: links.length }));
    });
    $('#zip-selected').addEventListener('click', function () {
      var f = $('#zip-form'); f.innerHTML = '';
      picked().forEach(function (c) { var i = document.createElement('input'); i.type = 'hidden'; i.name = 'id'; i.value = c.value; f.appendChild(i); });
      if (!f.children.length) return toast(T('Select some images first'), true);
      f.submit();
    });
    [['#list-selected', true], ['#unlist-selected', false]].forEach(function (p) {
      var b = $(p[0]); if (!b) return;
      b.addEventListener('click', function () {
        var ids = picked().map(function (c) { return c.value; });
        if (!ids.length) return toast(T('Select some images first'), true);
        Promise.all(ids.map(function (id) {
          return api('PATCH', '/api/v1/images/' + id, { listed: p[1] }).then(function (d) { return d.listed === p[1] ? 'ok' : 'locked'; }, function () { return 'failed'; });
        })).then(function (res) { reloadAfter(res, p[1] ? 'shown' : 'hidden'); });
      });
    });
    // Many files at once: say how many worked and which didn't, then show the new state.
    var reloadAfter = function (res, what) {
      var n = function (k) { return res.filter(function (x) { return x === k; }).length; };
      var ok = n('ok'), locked = n('locked'), failed = n('failed'), msg = [];
      if (ok) msg.push(T({ shown: '{n} shown on your profile', hidden: '{n} hidden from your profile', deleted: '{n} moved to the trash' }[what], { n: ok }));
      if (locked) msg.push(T('{n} skipped: private or password-protected files never appear on profiles', { n: locked }));
      if (failed) msg.push(T('{n} failed; try those again', { n: failed }));
      var bad = locked || failed;
      toast(msg.join(' · '), bad);
      setTimeout(function () { location.reload(); }, bad ? 3500 : 700);
    };
    var mk = $('#album-selected');
    if (mk) mk.addEventListener('click', function () {
      var ids = picked().map(function (c) { return c.value; });
      if (!ids.length) return toast(T('Select some images first'), true);
      phPrompt({ title: T('Name for the new album ({n} images)', { n: ids.length }), value: '', maxlength: 120, anchor: mk })
        .then(function (t) {
          if (t === null) return;
          api('POST', '/api/v1/albums', { title: t, ids: ids }).then(function (d) { location.href = '/a/' + d.id + '/edit'; }, function (e) { toast(e.message, true); });
        });
    });
    if (/[?&]select=1/.test(location.search)) selBtn.click();
    var delSelected = $('#delete-selected');
    delSelected.addEventListener('click', function () {
      var ids = picked().map(function (c) { return c.value; });
      if (!ids.length) return toast(T('Select some images first'), true);
      phConfirm(T('Move {n} images to the trash? You can restore them for 7 days.', { n: ids.length }), {
        danger: true, anchor: delSelected
      }).then(function (ok) {
        if (!ok) return;
        Promise.all(ids.map(function (id) { return api('DELETE', '/api/v1/images/' + id).then(function () { return 'ok'; }, function () { return 'failed'; }); }))
          .then(function (res) { reloadAfter(res, 'deleted'); });
      });
    });
  }

  // The selection menu closes once an action is chosen.
  var selMenu = $('#sel-menu');
  if (selMenu) selMenu.addEventListener('click', function (e) {
    if (e.target.closest('.menu-list button')) selMenu.classList.remove('open');
  });

  // ---------- settings subtabs ----------
  // One card group at a time, chosen by the address (#api, #profile, #webhooks ...), so every old
  // link and server redirect still lands on the right card. Without the script all groups stay.
  (function () {
    var tabs = $$('#set-tabs [data-settab]');
    if (!tabs.length) return;
    function show() {
      var id = location.hash.slice(1);
      var target = id && document.getElementById(id);
      var panel = target && target.closest('[data-setpanel]');
      if (!panel) panel = $('[data-setpanel]');
      $$('[data-setpanel]').forEach(function (p) { p.hidden = p !== panel; });
      tabs.forEach(function (t) {
        if (t.dataset.settab === panel.dataset.setpanel) t.setAttribute('aria-current', 'true');
        else t.removeAttribute('aria-current');
      });
    }
    window.addEventListener('hashchange', show);
    show();
  })();

  // ---------- album editor ----------
  var ae = $('#album-edit');
  if (ae) {
    var grid = $('#ae-order'), aid = ae.dataset.id, removed = [], cover = null, dragged = null;
    var sv = function (body) { return api('PATCH', '/api/v1/albums/' + aid, body); };
    grid.addEventListener('click', function (e) {
      var b = e.target.closest('[data-act]'); if (!b) return;
      var it = b.closest('.order-item');
      if (b.dataset.act === 'left' && it.previousElementSibling) grid.insertBefore(it, it.previousElementSibling);
      if (b.dataset.act === 'right' && it.nextElementSibling) grid.insertBefore(it.nextElementSibling, it);
      if (b.dataset.act === 'cover') { $$('.order-item', grid).forEach(function (x) { x.classList.remove('is-cover'); }); it.classList.add('is-cover'); cover = it.dataset.id; }
      if (b.dataset.act === 'remove') { removed.push(it.dataset.id); it.remove(); }
    });
    grid.addEventListener('dragstart', function (e) { dragged = e.target.closest('.order-item'); if (dragged) dragged.classList.add('dragging'); });
    grid.addEventListener('dragend', function () { if (dragged) dragged.classList.remove('dragging'); dragged = null; });
    grid.addEventListener('dragover', function (e) {
      e.preventDefault(); var over = e.target.closest('.order-item'); if (!dragged || !over || over === dragged) return;
      var r = over.getBoundingClientRect(), after = (e.clientX - r.left) > r.width / 2;
      grid.insertBefore(dragged, after ? over.nextSibling : over);
    });
    $('#ae-save').addEventListener('click', function () {
      var body = { content_warning: $('#ae-warning').value, warning_reason: $('#ae-warning-reason').value, title: $('#ae-title').value, description: $('#ae-desc').value, admin_note: $('#ae-note').value, remove: removed,
        order: $$('.order-item', grid).map(function (x) { return x.dataset.id; }) };
      if (cover) body.cover_id = cover;
      sv(body).then(function () { toast(T('Album saved')); removed = []; }, function (e) { toast(e.message, true); });
    });
    var add = $('#ae-add');
    if (add) add.addEventListener('click', function () {
      var ids = $$('#ae-others input:checked').map(function (c) { return c.value; });
      if (!ids.length) return toast(T('Pick some images first'), true);
      sv({ add: ids }).then(function () { location.reload(); }, function (e) { toast(e.message, true); });
    });
    $('#ae-delete').addEventListener('click', function () {
      var aeDel = $('#ae-delete');
      phConfirm(T('Delete this album? The images themselves stay.'), {
        danger: true, subject: aeDel.dataset.album || '', anchor: aeDel
      }).then(function (ok) {
        if (!ok) return;
        api('DELETE', '/api/v1/albums/' + aid).then(function () { location.href = '/dashboard/albums'; }, function (e) { toast(e.message, true); });
      });
    });
  }

  // results page: guests' delete links live in this browser's history
  if ($('.result-page')) {
    var hist = {}; loadHistory().forEach(function (x) { hist[x.id] = x.del; });
    $$('.result-item[data-id]').forEach(function (it) {
      var a = $('.del-link', it);
      if (a && !a.getAttribute('href') && hist[it.dataset.id]) { a.href = hist[it.dataset.id]; a.classList.remove('hide'); }
    });
  }

  // "Install app" in the top bar: Android/desktop Chrome offer it; iPhone needs Share → Add to Home Screen.
  var ib = $('#install-app'), deferred = null;
  var standalone = (window.matchMedia && matchMedia('(display-mode: standalone)').matches) || navigator.standalone;
  if (ib && !standalone) {
    window.addEventListener('beforeinstallprompt', function (e) { e.preventDefault(); deferred = e; ib.classList.remove('hide'); });
    window.addEventListener('appinstalled', function () { ib.classList.add('hide'); });
    ib.addEventListener('click', function () {
      if (deferred) { deferred.prompt(); deferred.userChoice.then(function () { deferred = null; ib.classList.add('hide'); }); return; }
      toast(T('On iPhone: tap Share, then “Add to Home Screen”.'));
    });
    if (/iPhone|iPad/.test(navigator.userAgent) && /Safari/.test(navigator.userAgent) && !navigator.standalone) ib.classList.remove('hide');
  }

  // ---------- keyboard shortcuts ("?" lists them) ----------
  (function () {
    var typing = function (t) { return t && (/^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName) || t.isContentEditable); };
    var keys = [
      ['?', 'Show this list'], ['u', 'Upload files'], ['s', 'Open the studio'], ['d', 'Open the dashboard'],
      ['/', 'Search my files'], ['c', 'Copy this link']
    ];
    function help() {
      var dlg = $('#shortcuts');
      if (!dlg) {
        dlg = document.createElement('dialog'); dlg.id = 'shortcuts';
        var h = document.createElement('h2'); h.textContent = T('Keyboard shortcuts'); dlg.appendChild(h);
        var dl = document.createElement('dl'); dl.className = 'facts';
        keys.forEach(function (k) {
          var dt = document.createElement('dt'), dd = document.createElement('dd'), kb = document.createElement('span');
          kb.className = 'kbd'; kb.textContent = k[0]; dt.appendChild(kb); dd.textContent = T(k[1]); dl.appendChild(dt); dl.appendChild(dd);
        });
        dlg.appendChild(dl);
        var f = document.createElement('form'); f.method = 'dialog'; f.className = 'btn-row'; f.style.justifyContent = 'flex-end';
        var b = document.createElement('button'); b.className = 'btn'; b.textContent = T('Close'); f.appendChild(b); dlg.appendChild(f);
        document.body.appendChild(dlg);
      }
      if (dlg.showModal && !dlg.open) dlg.showModal();
    }
    document.addEventListener('keydown', function (e) {
      if (e.ctrlKey || e.metaKey || e.altKey || typing(e.target) || document.querySelector('dialog[open]') && e.key !== '?') return;
      switch (e.key) {
        case '?': e.preventDefault(); help(); break;
        case 'u': var pick = $('[data-pick-files]') || $('#drop-in'); if (pick) { e.preventDefault(); if (pick.click) pick.click(); } else location.href = '/'; break;
        case 's': location.href = '/tools'; break;
        case 'd': location.href = '/dashboard'; break;
        case '/': var q = $('input[type=search]'); if (q) { e.preventDefault(); q.focus(); } break;
        case 'c': var cb = $('[data-copy-text].primary') || $('[data-copy-text]'); if (cb) { e.preventDefault(); copy(cb.dataset.copyText); } break;
      }
    });
  })();

  var warn = $('dialog[data-open]');
  if (warn && warn.showModal) { warn.showModal(); warn.addEventListener('cancel', function (e) { e.preventDefault(); }); }

  if ('serviceWorker' in navigator && location.protocol === 'https:') navigator.serviceWorker.register('/sw.js').catch(function () {});

  // relative times refresh themselves
  $$('time[datetime]').forEach(function (t) { t.title = new Date(t.getAttribute('datetime')).toLocaleString(); });
})();
