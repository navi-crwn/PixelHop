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
