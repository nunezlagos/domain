// ============================================================
// project-skills.js — panes AJAX genéricos dentro del ver/editar de
// proyecto (tabs Skills y Reglas), sin recargar la página.
//
// Un "pane" es un contenedor [data-pane] con data-pane-url (endpoint GET que
// re-renderiza el contenido interno del pane). El contenido trae:
//   - [data-pane-state data-scope="..." data-page="N"]  (estado actual)
//   - [data-pane-filter]   select de filtro (name=scope) → re-fetch, page=1
//   - [data-pane-page="N"]  botones de paginación        → re-fetch page=N
//   - [data-pane-action] data-url="POST" data-params="a=1&b=2" → POST (con
//       scope+page actuales) y reemplaza el pane con la respuesta.
//
// Reemplaza SOLO el innerHTML del [data-pane] (no todo el modal): preserva el
// form de edición y el tab activo, que viven fuera del pane.
//
// Depende de csrf.js (getCSRFToken). Delegado en document para funcionar con el
// HTML inyectado en el modal dinámico.
// ============================================================
(function () {
  function paneState(pane) {
    var st = pane.querySelector('[data-pane-state]');
    var f = pane.querySelector('[data-pane-filter]');
    return {
      scope: (f && f.value) || (st && st.getAttribute('data-scope')) || 'all',
      page: (st && st.getAttribute('data-page')) || '1',
    };
  }

  async function fetchPane(pane, scope, page) {
    var params = new URLSearchParams({ scope: scope, page: String(page) });
    // El "ver" es read-only: re-fetchea sin columna/botones de gestion.
    if (pane.dataset.paneReadonly === '1') params.set('readonly', '1');
    try {
      var r = await fetch(pane.dataset.paneUrl + '?' + params.toString(), {
        credentials: 'same-origin',
        headers: { 'X-Requested-With': 'fetch' },
      });
      if (r.ok) pane.innerHTML = await r.text();
      else alert('Error (' + r.status + ')');
    } catch (err) {
      alert('Error de red: ' + err.message);
    }
  }

  // Filtro de scope: vuelve a página 1.
  document.addEventListener('change', function (e) {
    var f = e.target.closest('[data-pane-filter]');
    if (!f) return;
    var pane = f.closest('[data-pane]');
    if (pane) fetchPane(pane, f.value || 'all', 1);
  });

  // Click: paginación o acción (toggle).
  document.addEventListener('click', async function (e) {
    var pager = e.target.closest('[data-pane-page]');
    if (pager) {
      e.preventDefault();
      var pane = pager.closest('[data-pane]');
      if (pane) fetchPane(pane, paneState(pane).scope, pager.getAttribute('data-pane-page'));
      return;
    }
    var act = e.target.closest('[data-pane-action]');
    if (!act) return;
    e.preventDefault();
    var pane2 = act.closest('[data-pane]');
    if (!pane2 || !act.dataset.url) return;
    act.disabled = true;
    var st = paneState(pane2);
    var body = new URLSearchParams(act.dataset.params || '');
    body.set('scope', st.scope);
    body.set('page', st.page);
    try {
      var r = await fetch(act.dataset.url, {
        method: 'POST',
        headers: {
          'X-CSRFToken': getCSRFToken(),
          'X-Requested-With': 'fetch',
          'Content-Type': 'application/x-www-form-urlencoded',
        },
        credentials: 'same-origin',
        body: body.toString(),
      });
      if (r.ok) pane2.innerHTML = await r.text();
      else { act.disabled = false; alert('Error (' + r.status + ')'); }
    } catch (err) {
      act.disabled = false;
      alert('Error de red: ' + err.message);
    }
  });
})();
