/* Sample-only feature tour; actual uploads continue through upload.js. */
(function () {
  'use strict';
  document.querySelectorAll('[data-share-demo]').forEach(function (root) {
    root.querySelector('select').addEventListener('change', function (event) {
      var url = 'https://p.hel.ink/demo';
      root.querySelector('output').textContent = {link:url, markdown:'![](' + url + ')', html:'<img src="' + url + '" alt="">', bbcode:'[img]' + url + '[/img]'}[event.target.value];
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
})();
