/* Sample-only feature tour; actual uploads continue through the uploader scripts. */
(function () {
  'use strict';
  document.querySelectorAll('[data-share-demo]').forEach(function (root) {
    root.querySelector('select').addEventListener('change', function (event) {
      var url = 'https://p.hel.ink/demo';
      root.querySelector('output').textContent = {link:url, direct:url + '.png', markdown:'![](' + url + '.png)', html:'<img src="' + url + '.png" alt="">', bbcode:'[img]' + url + '.png[/img]'}[event.target.value];
    });
  });
  document.querySelectorAll('[data-resize-demo]').forEach(function (root) {
    root.querySelector('input').addEventListener('input', function (event) {
      var width = Number(event.target.value), height = Math.round(width * 2 / 3), image = root.querySelector('img');
      image.width = width; image.height = height;
      root.querySelector('output').textContent = width + ' × ' + height + ' px';
    });
  });
  document.querySelectorAll('[data-privacy-demo]').forEach(function (root) {
    root.querySelector('button').addEventListener('click', function (event) {
      var button = event.currentTarget, active = button.getAttribute('aria-pressed') !== 'true';
      button.setAttribute('aria-pressed', String(active));
      root.querySelector('output').textContent = active ? button.dataset.after : button.dataset.before;
    });
  });
  // EXIF: one press strips the location, camera and time; the picture only gets smaller.
  document.querySelectorAll('[data-exif-demo]').forEach(function (root) {
    var button = root.querySelector('button'), output = root.querySelector('output');
    button.addEventListener('click', function () {
      var clean = button.getAttribute('aria-pressed') !== 'true';
      button.setAttribute('aria-pressed', String(clean));
      root.querySelectorAll('td').forEach(function (td) { td.textContent = clean ? td.dataset.after : td.dataset.before; td.classList.toggle('gone', clean && td.dataset.after === '—'); });
      output.textContent = clean ? output.dataset.after || 'EXIF ✕' : output.dataset.before || output.textContent;
    });
    output.dataset.before = output.textContent;
    output.dataset.after = output.getAttribute('data-after-text') || (output.textContent + ' → ✓');
  });
  // Expiry: a countdown that follows the choice.
  document.querySelectorAll('[data-expire-demo]').forEach(function (root) {
    var select = root.querySelector('select'), out = root.querySelector('[data-expire-out]'), timer = 0, left = 0;
    function show() {
      if (!left) { out.textContent = '∞'; return; }
      var d = Math.floor(left / 86400), h = Math.floor(left % 86400 / 3600), m = Math.floor(left % 3600 / 60), s = left % 60;
      out.textContent = d ? d + 'd ' + h + 'h' : h ? h + 'h ' + ('0' + m).slice(-2) + 'm' : ('0' + m).slice(-2) + ':' + ('0' + s).slice(-2);
    }
    function start() {
      clearInterval(timer); left = Number(select.value) || 0; show();
      if (left) timer = setInterval(function () { left = Math.max(0, left - 1); show(); if (!left) clearInterval(timer); }, 1000);
    }
    select.addEventListener('change', start); start();
  });
  // Albums: three pictures become one link.
  document.querySelectorAll('[data-album-demo]').forEach(function (root) {
    root.querySelector('button').addEventListener('click', function () {
      root.classList.add('is-album');
      root.querySelector('[data-album-out]').textContent = 'p.hel.ink/a/' + Math.random().toString(36).slice(2, 7);
    });
  });
})();
