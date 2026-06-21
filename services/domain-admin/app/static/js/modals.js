// ============================================================
// modals.js — handler global de modales + CRUD genérico via
// data-action. Entity-agnostic: la base URL del recurso se
// resuelve del contenedor más cercano con [data-base-url]
// (ej. data-base-url="/usuarios/"), o del data-url del botón.
//
// Depende de: csrf.js (getCSRFToken). Cargar csrf.js ANTES.
//
// API global:
//   openDynamicModal(html)  — inyecta HTML y abre el modal dinámico
//   closeDynamicModal()     — cierra el modal dinámico
//
// Markup esperado en base.html:
//   #modal-dynamic > #modal-dynamic-content
//   #modal-confirm-delete > #confirm-delete-form, #confirm-delete-name
// ============================================================
(function () {
  // ----------------------------------------------------------
  // Modal genérico (open/close + content dinámico)
  // ----------------------------------------------------------
  function openDynamicModal(html) {
    document.getElementById('modal-dynamic-content').innerHTML = html;
    document.getElementById('modal-dynamic').classList.add('open');
  }
  function closeDynamicModal() {
    document.getElementById('modal-dynamic').classList.remove('open');
  }
  window.openDynamicModal = openDynamicModal;
  window.closeDynamicModal = closeDynamicModal;

  // ----------------------------------------------------------
  // Resolución de base URL (entity-agnostic).
  // El contenedor de tabla declara data-base-url="/<recurso>/".
  // Orden: data-url explícito del botón > data-base-url del
  // contenedor más cercano > primer [data-base-url] del documento.
  // ----------------------------------------------------------
  function resolveBase(btn) {
    if (btn.dataset.url) return null; // el botón usa data-url directo
    var c = btn.closest('[data-base-url]') || document.querySelector('[data-base-url]');
    return c ? c.dataset.baseUrl : null;
  }

  // ----------------------------------------------------------
  // Click handlers: data-action (CRUD genérico via modales)
  // ----------------------------------------------------------
  document.addEventListener('click', async function (e) {
    // Open modal (estático, predefinido)
    var opener = e.target.closest('[data-modal-open]');
    if (opener) {
      e.preventDefault();
      var id = opener.getAttribute('data-modal-open');
      var modal = document.getElementById(id);
      if (modal) modal.classList.add('open');
      return;
    }
    // Close modal
    var closer = e.target.closest('[data-modal-close]');
    if (closer) {
      e.preventDefault();
      var modalToClose = closer.closest('.modal-backdrop');
      if (modalToClose) modalToClose.classList.remove('open');
      return;
    }
    // Copiar al portapapeles: data-copy="#selector" copia el value/text del
    // elemento referenciado. Delegado para que funcione dentro de partials
    // inyectados en el modal dinámico (donde un <script> inline no correría).
    var copyBtn = e.target.closest('[data-copy]');
    if (copyBtn) {
      e.preventDefault();
      var target = document.querySelector(copyBtn.dataset.copy);
      if (target) {
        var text = 'value' in target ? target.value : target.textContent;
        var done = function () {
          var prev = copyBtn.textContent;
          copyBtn.textContent = 'Copiado';
          setTimeout(function () { copyBtn.textContent = prev; }, 1500);
        };
        if (navigator.clipboard && navigator.clipboard.writeText) {
          navigator.clipboard.writeText(text).then(done, function () {
            target.select && target.select();
          });
        } else if (target.select) {
          target.select();
          document.execCommand('copy');
          done();
        }
      }
      return;
    }
    // Action handlers (AJAX → modal dinámico)
    var btn = e.target.closest('[data-action]');
    if (btn) {
      e.preventDefault();
      var action = btn.dataset.action;
      var rowId = btn.dataset.id || '';
      // Base derivada del contenedor (o null si el botón trae data-url).
      // base SIEMPRE termina en "/" (ej "/usuarios/").
      var base = btn.dataset.url ? null : resolveBase(btn);
      try {
        if (action === 'modal') {
          // Acción genérica: hace fetch de data-url (o data-modal-url) e
          // inyecta el HTML en el modal dinámico. Reutilizable por cualquier
          // botón que quiera abrir un partial arbitrario en el modal.
          var modalUrl = btn.dataset.url || btn.dataset.modalUrl;
          if (!modalUrl) { alert('Falta data-url'); return; }
          var mr = await fetch(modalUrl, {
            credentials: 'same-origin',
            headers: { 'X-Requested-With': 'fetch' },
          });
          if (mr.ok) {
            openDynamicModal(await mr.text());
          } else {
            alert('Error al cargar (' + mr.status + ')');
          }
        } else if (action === 'view' || action === 'edit' || action === 'create') {
          var fetchUrl;
          if (action === 'create') {
            fetchUrl = btn.dataset.url || base + 'nuevo/?partial=1';
          } else if (action === 'view') {
            fetchUrl = btn.dataset.url || base + rowId + '/?partial=1';
          } else { // edit
            fetchUrl = btn.dataset.url || base + rowId + '/editar/?partial=1';
          }
          var r = await fetch(fetchUrl, { credentials: 'same-origin' });
          if (r.ok) {
            var html = await r.text();
            openDynamicModal(html);
          } else {
            alert('Error al cargar (' + r.status + ')');
          }
        } else if (action === 'delete') {
          // Setear action del form de delete + nombre
          document.getElementById('confirm-delete-form').action =
            btn.dataset.url || base + rowId + '/eliminar/';
          document.getElementById('confirm-delete-name').textContent =
            btn.dataset.name || 'este elemento';
          document.getElementById('modal-confirm-delete').classList.add('open');
        } else if (action === 'toggle') {
          var toggleUrl = btn.dataset.url || base + rowId + '/toggle/';
          var tr = await fetch(toggleUrl, {
            method: 'POST',
            headers: { 'X-CSRFToken': getCSRFToken(), 'X-Requested-With': 'fetch' },
            credentials: 'same-origin',
          });
          if (tr.ok) {
            window.location.reload();
          } else {
            alert('Error al cambiar estado (' + tr.status + ')');
          }
        }
      } catch (err) {
        alert('Error de red: ' + err.message);
      }
    }
  });

  // Click en backdrop cierra el modal
  document.addEventListener('click', function (e) {
    if (e.target.classList && e.target.classList.contains('modal-backdrop')) {
      e.target.classList.remove('open');
    }
  });

  // ----------------------------------------------------------
  // Form submit dentro del modal dinámico (intercepta [data-modal-form])
  // ----------------------------------------------------------
  document.addEventListener('submit', async function (e) {
    var form = e.target;
    if (!form.matches('[data-modal-form]')) return;
    e.preventDefault();

    var formData = new FormData(form);
    try {
      var r = await fetch(form.action, {
        method: 'POST',
        body: formData,
        credentials: 'same-origin',
        headers: { 'X-Requested-With': 'fetch' },
      });
      if (r.ok || r.redirected) {
        // Success: cerrar modal y recargar
        closeDynamicModal();
        window.location.reload();
      } else {
        // Error: re-renderizar el form con los errores
        var html = await r.text();
        document.getElementById('modal-dynamic-content').innerHTML = html;
      }
    } catch (err) {
      alert('Error al enviar: ' + err.message);
    }
  });

  // ----------------------------------------------------------
  // Auto-dismiss alerts (5s)
  // ----------------------------------------------------------
  document.querySelectorAll('.alert[data-auto-dismiss]').forEach(function (a) {
    setTimeout(function () {
      a.style.transition = 'opacity 0.3s';
      a.style.opacity = '0';
      setTimeout(function () { a.remove(); }, 300);
    }, 5000);
  });
})();
