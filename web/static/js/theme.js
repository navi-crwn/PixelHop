// Applies the saved theme before the page paints. The HOP family keeps it in the cookie hop_theme
// (light or dark; anything else follows the system), shared by every app on *.hel.ink. Without the
// cookie, PixelHop's old localStorage key is used once and copied into it. hopTheme(t) writes the
// cookie; the theme button in app.js calls it and still writes the old key too.
window.hopTheme = function (t) {
  document.cookie = 'hop_theme=' + t + '; Path=/; Max-Age=31536000; SameSite=Lax' +
    (/(^|\.)hel\.ink$/.test(location.hostname) ? '; domain=.hel.ink; Secure' : '');
};
(function () {
  var m = /(?:^|;\s*)hop_theme=([^;]*)/.exec(document.cookie), t = m ? m[1] : null;
  if (!m) {
    try { t = localStorage.getItem('theme'); } catch (e) {}
    if (t === 'light' || t === 'dark') window.hopTheme(t);
  }
  if (t === 'light' || t === 'dark') document.documentElement.dataset.theme = t;
})();

// The colour palette (classic | mist | sand) is one family setting in the cookie hop_palette, written like
// hop_theme. It is applied as <html data-palette> before the page paints, and only on the dashboard shell
// (the page marks itself data-shell); anything else, or no cookie, is classic (no attribute).
window.hopPref = function (name, value) {
  try {
    document.cookie = name + '=' + encodeURIComponent(value) + '; Path=/; Max-Age=31536000; SameSite=Lax' +
      (/(^|\.)hel\.ink$/.test(location.hostname) ? '; domain=.hel.ink; Secure' : '');
  } catch (e) {}
};
(function () {
  try {
    var m = /(?:^|;\s*)hop_palette=([^;]*)/.exec(document.cookie), p = m ? decodeURIComponent(m[1]) : '';
    if ((p === 'mist' || p === 'sand') && document.documentElement.hasAttribute('data-shell')) document.documentElement.dataset.palette = p;
  } catch (e) {}
})();
