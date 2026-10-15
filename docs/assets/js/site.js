// PixelHop website: language, theme, tabs, copy buttons, nav shadow, reveal on scroll.
// The language and theme are picked before paint by the inline script in <head>.
(function () {
  var root = document.documentElement;
  function save(k, v) { try { localStorage.setItem(k, v); } catch (e) {} }

  var en = document.getElementById('lang-en'), id = document.getElementById('lang-id');
  function setLang(l, keep) {
    root.lang = l;
    en.setAttribute('aria-pressed', l === 'en');
    id.setAttribute('aria-pressed', l === 'id');
    document.title = l === 'id' ? 'PixelHop: hosting gambar open source dengan studio foto pribadi' : 'PixelHop: open-source image hosting with a private photo studio';
    if (keep) save('lang', l);
  }
  en.addEventListener('click', function () { setLang('en', true); });
  id.addEventListener('click', function () { setLang('id', true); });
  setLang(root.lang === 'id' ? 'id' : 'en', false);

  document.getElementById('theme').addEventListener('click', function () {
    var cur = root.getAttribute('data-theme') || (matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark');
    var next = cur === 'light' ? 'dark' : 'light';
    root.setAttribute('data-theme', next); save('theme', next);
  });

  document.querySelectorAll('.tabs [data-tab]').forEach(function (t) {
    t.addEventListener('click', function () {
      var box = t.closest('.tabs').parentElement;
      box.querySelectorAll('[data-tab]').forEach(function (x) { x.setAttribute('aria-selected', x === t); });
      box.querySelectorAll('[data-panel]').forEach(function (p) { p.hidden = p.dataset.panel !== t.dataset.tab; });
    });
  });

  document.querySelectorAll('.copy').forEach(function (b) {
    b.addEventListener('click', function () {
      var text = b.parentElement.querySelector('pre').innerText.split('\n').filter(function (l) { return l.charAt(0) !== '#'; }).join('\n').trim();
      var done = function () { var t = b.textContent; b.textContent = root.lang === 'id' ? 'Tersalin' : 'Copied'; setTimeout(function () { b.textContent = t; }, 1400); };
      if (navigator.clipboard) navigator.clipboard.writeText(text).then(done, function () {}); else done();
    });
  });

  var nav = document.querySelector('.nav');
  function onScroll() { nav.classList.toggle('solid', scrollY > 24); }
  addEventListener('scroll', onScroll, { passive: true }); onScroll();

  var items = document.querySelectorAll('.rv');
  if ('IntersectionObserver' in window && !root.classList.contains('shot-mode')) {
    var io = new IntersectionObserver(function (es) {
      es.forEach(function (e) { if (e.isIntersecting) { e.target.classList.add('in'); io.unobserve(e.target); } });
    }, { rootMargin: '0px 0px -8% 0px' });
    items.forEach(function (el) { io.observe(el); });
  } else items.forEach(function (el) { el.classList.add('in'); });

  document.querySelectorAll('.feats>div').forEach(function (c) {
    c.addEventListener('pointermove', function (e) {
      var r = c.getBoundingClientRect();
      c.style.setProperty('--mx', (e.clientX - r.left) + 'px');
      c.style.setProperty('--my', (e.clientY - r.top) + 'px');
    });
  });
})();
