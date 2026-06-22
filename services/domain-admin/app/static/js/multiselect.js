// multiselect.js — multi-select tipo "select2" liviano (vanilla, sin libs).
// Estructura esperada:
//   <div data-multiselect data-ms-placeholder="Todos los roles">
//     <button data-ms-trigger><span data-ms-label></span></button>
//     <div data-ms-panel hidden>
//       <input data-ms-search>
//       <label data-ms-option="valor"><input type=checkbox ...><span>...</span></label>
//     </div>
//   </div>
// Los checkboxes internos pueden ser [data-filter] (maintainer.js los lee igual).
// Este componente solo maneja abrir/cerrar, buscar y el label de seleccionados.
(function () {
  function closeAll(except) {
    document.querySelectorAll('[data-ms-panel]').forEach(function (p) {
      if (p !== except) p.hidden = true;
    });
  }

  function updateLabel(ms) {
    var label = ms.querySelector('[data-ms-label]');
    if (!label) return;
    var checked = ms.querySelectorAll('input[type="checkbox"]:checked');
    var placeholder = ms.dataset.msPlaceholder || 'Todos';
    if (!checked.length) {
      label.textContent = placeholder;
      label.classList.add('ms-muted');
      return;
    }
    var names = Array.prototype.map.call(checked, function (c) {
      var opt = c.closest('[data-ms-option]');
      return opt ? opt.getAttribute('data-ms-option') : c.value;
    });
    label.textContent = names.length > 2
      ? names.length + ' seleccionados'
      : names.join(', ');
    label.classList.remove('ms-muted');
  }

  document.addEventListener('click', function (e) {
    var trigger = e.target.closest('[data-ms-trigger]');
    if (trigger) {
      e.preventDefault();
      var ms = trigger.closest('[data-multiselect]');
      var panel = ms.querySelector('[data-ms-panel]');
      var willOpen = panel.hidden;
      closeAll(willOpen ? panel : null);
      panel.hidden = !willOpen;
      if (willOpen) {
        var s = ms.querySelector('[data-ms-search]');
        if (s) { s.value = ''; s.dispatchEvent(new Event('input')); s.focus(); }
      }
      return;
    }
    // Click fuera de cualquier multiselect → cerrar paneles.
    if (!e.target.closest('[data-multiselect]')) closeAll(null);
  });

  document.addEventListener('input', function (e) {
    var search = e.target.closest('[data-ms-search]');
    if (!search) return;
    var q = search.value.toLowerCase();
    search.closest('[data-ms-panel]').querySelectorAll('[data-ms-option]').forEach(function (o) {
      var txt = (o.getAttribute('data-ms-option') || '').toLowerCase();
      o.style.display = txt.indexOf(q) >= 0 ? '' : 'none';
    });
  });

  document.addEventListener('change', function (e) {
    if (e.target.type !== 'checkbox') return;
    var ms = e.target.closest('[data-multiselect]');
    if (ms) updateLabel(ms);
  });

  function initAll() {
    document.querySelectorAll('[data-multiselect]').forEach(updateLabel);
  }
  if (document.readyState !== 'loading') initAll();
  else document.addEventListener('DOMContentLoaded', initAll);
})();
