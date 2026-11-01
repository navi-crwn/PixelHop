(function () {
  'use strict';
  var inputs = Array.from(document.querySelectorAll('input[type="file"]'));
  if (!inputs.length) return;
  var warning = document.createElement('p');
  warning.setAttribute('role', 'status');
  warning.className = 'notice';
  warning.hidden = true;
  inputs[0].parentElement.insertAdjacentElement('beforebegin', warning);
  function check() {
    fetch('/api/v1/upload-status', { cache: 'no-store' }).then(function (r) {
      if (!r.ok) throw new Error();
      return r.json();
    }).then(function (r) {
      var status = r.data || r;
      warning.hidden = status.available;
      warning.textContent = status.message || '';
      inputs.forEach(function (input) {
        if (!status.available && !input.disabled) { input.dataset.storagePaused = '1'; input.disabled = true; }
        else if (status.available && input.dataset.storagePaused) { delete input.dataset.storagePaused; input.disabled = false; }
      });
    }).catch(function () {});
  }
  check();
  setInterval(function () { if (!document.hidden) check(); }, 30000);
})();
