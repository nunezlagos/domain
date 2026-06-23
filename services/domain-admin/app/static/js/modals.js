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
  function openDynamicModal(html, srcUrl) {
    var content = document.getElementById('modal-dynamic-content');
    content.innerHTML = html;
    // Modal GRANDE: si el contenido inyectado trae [data-modal-lg], el modal usa
    // la variante ancha (.modal-dynamic--lg). Se resetea si no lo trae.
    content.classList.toggle('modal-dynamic--lg', !!content.querySelector('[data-modal-lg]'));
    var m = document.getElementById('modal-dynamic');
    // Guardamos la URL fuente del modal para poder REFRESCAR su contenido tras
    // una accion interna (ej. borrar una API key dentro del detalle de usuario)
    // sin navegar al redirect del recurso borrado.
    if (srcUrl !== undefined) m.dataset.src = srcUrl || '';
    m.classList.add('open');
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
    // Seleccionar todo el texto al hacer click en un input readonly
    // ([data-select-on-click]). Reemplaza el onclick="this.select()" inline
    // para que NO haya JS en el HTML; funciona en partials inyectados al modal.
    var selectable = e.target.closest('[data-select-on-click]');
    if (selectable && typeof selectable.select === 'function') {
      selectable.select();
      // no return: dejar que otros handlers (copy, etc.) sigan si aplica
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
            // Pasar modalUrl como src para que un delete dentro de ESTE modal lo
            // refresque a si mismo (antes quedaba el src viejo del "ver usuario").
            openDynamicModal(await mr.text(), modalUrl);
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
            openDynamicModal(html, fetchUrl);
          } else {
            alert('Error al cargar (' + r.status + ')');
          }
        } else if (action === 'delete') {
          // Setear action del form de delete + nombre
          var delForm = document.getElementById('confirm-delete-form');
          delForm.action = btn.dataset.url || base + rowId + '/eliminar/';
          document.getElementById('confirm-delete-name').textContent =
            btn.dataset.name || 'este elemento';
          // Si el delete se dispara DESDE un modal dinamico abierto (ej. detalle
          // de usuario), guardamos su URL fuente para refrescarlo tras borrar en
          // vez de navegar al redirect del recurso (que sacaba del modal).
          var dynM = document.getElementById('modal-dynamic');
          delForm.dataset.refreshSrc =
            (dynM && dynM.classList.contains('open') && dynM.dataset.src) ? dynM.dataset.src : '';
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

  // NOTA: el click en el backdrop NO cierra el modal (a pedido). Solo se cierra
  // con el boton X, Cancelar o Cerrar (data-modal-close), para evitar cierres
  // accidentales al hacer click afuera.

  // Filtros DENTRO de un modal ([data-modal-filter]): al cambiar, re-fetch del
  // contenido del modal (su src) con los params de todos los filtros + re-inject.
  document.addEventListener('change', async function (e) {
    if (!e.target.closest('[data-modal-filter]')) return;
    var modal = document.getElementById('modal-dynamic');
    if (!modal || !modal.dataset.src) return;
    var base = modal.dataset.src.split('?')[0];
    var params = new URLSearchParams();
    modal.querySelectorAll('[data-modal-filter]').forEach(function (el) {
      var name = el.getAttribute('name') || el.dataset.modalFilter;
      if (name && el.value) params.set(name, el.value);
    });
    var url = base + '?' + params.toString();
    try {
      var r = await fetch(url, { credentials: 'same-origin', headers: { 'X-Requested-With': 'fetch' } });
      if (r.ok) openDynamicModal(await r.text(), url);
    } catch (err) {
      console.warn('Filtro de modal fallo:', err);
    }
  });

  // Paginacion DENTRO de un modal ([data-modal-page]): re-fetch del contenido
  // del modal con ?page=N + los filtros activos ([data-modal-filter]), re-inject.
  document.addEventListener('click', async function (e) {
    var pager = e.target.closest('[data-modal-page]');
    if (!pager) return;
    e.preventDefault();
    var modal = document.getElementById('modal-dynamic');
    if (!modal || !modal.dataset.src) return;
    var base = modal.dataset.src.split('?')[0];
    var params = new URLSearchParams();
    modal.querySelectorAll('[data-modal-filter]').forEach(function (el) {
      var name = el.getAttribute('name') || el.dataset.modalFilter;
      if (name && el.value) params.set(name, el.value);
    });
    params.set('page', pager.getAttribute('data-modal-page'));
    var url = base + '?' + params.toString();
    try {
      var r = await fetch(url, { credentials: 'same-origin', headers: { 'X-Requested-With': 'fetch' } });
      if (r.ok) openDynamicModal(await r.text(), url);
    } catch (err) {
      console.warn('Paginacion de modal fallo:', err);
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
  // Submit del form de confirmacion de delete: por fetch (no nativo) para NO
  // seguir el redirect del recurso. Si el delete salio de un modal dinamico
  // abierto (data-refresh-src), refrescamos su contenido y nos quedamos en el
  // modal; si no, recargamos la pagina (delete desde la lista).
  // ----------------------------------------------------------
  document.addEventListener('submit', async function (e) {
    var form = e.target;
    if (form.id !== 'confirm-delete-form') return;
    e.preventDefault();
    // Recordar la tab activa del modal (si tiene tabs) para restaurarla tras el
    // refresh — asi un delete dentro de la tab API Keys no salta a Informacion.
    var activeTab = document.querySelector('#modal-dynamic input[name="utab"]:checked');
    var activeTabId = activeTab ? activeTab.id : null;
    try {
      var r = await fetch(form.action, {
        method: 'POST',
        body: new FormData(form),
        credentials: 'same-origin',
        headers: { 'X-Requested-With': 'fetch' },
      });
      document.getElementById('modal-confirm-delete').classList.remove('open');
      if (!(r.ok || r.redirected)) {
        alert('Error al eliminar (' + r.status + ')');
        return;
      }
      var refresh = form.dataset.refreshSrc;
      if (refresh) {
        var rr = await fetch(refresh, {
          credentials: 'same-origin',
          headers: { 'X-Requested-With': 'fetch' },
        });
        if (rr.ok) {
          openDynamicModal(await rr.text(), refresh);
          // Restaurar la tab activa (el HTML re-inyectado vuelve a la 1ra).
          if (activeTabId) {
            var t = document.getElementById(activeTabId);
            if (t) t.checked = true;
          }
          return;
        }
      }
      closeDynamicModal();
      window.location.reload();
    } catch (err) {
      alert('Error al eliminar: ' + err.message);
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
