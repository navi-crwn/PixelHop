// An SVG file opens in the studio and the batch page by being drawn once as a picture and kept as pixels.
// It is loaded through an <img> (which never runs script, never loads other files) and goes on as a PNG; the
// server still does not take SVG files, so what is uploaded is always the PNG made here.
(function () {
  function isSvg(b, name) { return !!b && (b.type === 'image/svg+xml' || /\.svg$/i.test(name || b.name || '')); }
  function rasterize(blob) {
    return new Promise(function (resolve, reject) {
      var svg = blob.type === 'image/svg+xml' ? blob : new Blob([blob], { type: 'image/svg+xml' });
      var url = URL.createObjectURL(svg), img = new Image();
      img.onload = function () {
        var w = img.naturalWidth || 1024, h = img.naturalHeight || 1024, big = Math.max(w, h);
        var s = big > 2048 ? 2048 / big : Math.min(4, Math.max(1, 512 / big)); // never huge, never a speck
        var c = document.createElement('canvas'); c.width = Math.max(1, Math.round(w * s)); c.height = Math.max(1, Math.round(h * s));
        try { c.getContext('2d').drawImage(img, 0, 0, c.width, c.height); } catch (e) { URL.revokeObjectURL(url); return reject(e); }
        URL.revokeObjectURL(url);
        createImageBitmap(c).then(resolve, reject);
      };
      img.onerror = function () { URL.revokeObjectURL(url); reject(new Error('svg')); };
      img.src = url;
    });
  }
  window.phIsSvg = isSvg;
  window.phBitmap = function (blob, opts, name) { return isSvg(blob, name) ? rasterize(blob) : createImageBitmap(blob, opts); };
})();
