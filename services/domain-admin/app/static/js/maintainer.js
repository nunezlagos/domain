// ============================================================
// maintainer.js — tabla server-side reusable para CUALQUIER
// mantenedor. NO hardcodea ids ni URLs: se parametriza por
// data-attrs del contenedor genérico [data-maintainer-table].
//
// Comportamiento (HU-48.2):
//   - Refresh ON-CHANGE: cada CHECK_MS chequea una "señal" barata
//     (count + version) contra [data-signal-url]. Solo re-renderiza
//     la tabla cuando la señal cambia respecto al último valor
//     embebido en [data-signal]. NO refresca a ciegas.
//   - Search as-you-type (debounce 300ms), 100% server-side.
//   - Paginación AJAX interceptando [data-page-link], server-side.
//   - Se pausa el watch mientras hay un modal abierto.
//
// Contrato de data-attrs en el contenedor [data-maintainer-table]:
//   data-base-url    (req) base REST del recurso, ej "/usuarios/"
//   data-signal-url  (req) endpoint que devuelve JSON {count, version}
//   data-signal      (req) señal embebida en el render: "<count>|<version>"
//   data-search-input (opc) selector del input de búsqueda
//                          (default: "[data-search-input]")
//
// El fragmento de tabla se pide a la MISMA URL de la página con
// ?fragment=table (+ q + page). El backend debe responder solo el
// markup de la tabla (thead/tbody + paginación).
//
// Soporta múltiples mantenedores en una misma página: inicializa
// cada [data-maintainer-table] de forma independiente.
// ============================================================
(function () {
  var CHECK_MS = 5000; // cada cuánto se chequea la señal (query barata)

  function initMaintainer(tableArea) {
    var signalURL = tableArea.dataset.signalUrl;
    var searchSelector = tableArea.dataset.searchInput || '[data-search-input]';
    var searchInput = document.querySelector(searchSelector);

    var lastSignal = tableArea.dataset.signal || ''; // señal embebida en el render
    var checkTimer = null;
    var searchTimer = null;
    var isFetching = false;
    var paused = false; // se pausa mientras hay un modal abierto

    // Construir URL para fetch de la tabla (incluye q + page + fragment)
    function buildFetchURL() {
      var params = new URLSearchParams();
      params.set('fragment', 'table');
      if (searchInput && searchInput.value.trim()) {
        params.set('q', searchInput.value.trim());
      }
      var page = new URLSearchParams(window.location.search).get('page');
      if (page) params.set('page', page);
      return window.location.pathname + '?' + params.toString();
    }

    // Fetch y reemplazar el contenido de la tabla (server-side render)
    async function refreshTable() {
      if (isFetching) return; // evitar requests concurrentes
      isFetching = true;
      try {
        var r = await fetch(buildFetchURL(), { credentials: 'same-origin' });
        if (r.ok) {
          tableArea.innerHTML = await r.text();
          bindPagination();
        }
      } catch (err) {
        console.warn('Refresh tabla falló:', err);
      } finally {
        isFetching = false;
      }
    }

    // Chequeo de señal: refresca SOLO si la BD cambió desde el último check.
    async function checkSignal() {
      if (paused || isFetching || !signalURL) return;
      try {
        var r = await fetch(signalURL, { credentials: 'same-origin' });
        if (!r.ok) return;
        var s = await r.json();
        var current = s.count + '|' + s.version;
        if (current !== lastSignal) {
          lastSignal = current;
          await refreshTable(); // hubo un cambio en BD → re-render
        }
      } catch (err) {
        console.warn('Check de señal falló:', err);
      }
    }

    function startWatch() {
      if (checkTimer) return;
      checkTimer = setInterval(checkSignal, CHECK_MS);
    }

    // Pausar el watch mientras hay un modal abierto (no refrescar mientras editás)
    document.addEventListener('click', function (e) {
      if (e.target.closest('[data-modal-open]') || e.target.closest('[data-action]')) {
        paused = true;
      }
      if (
        e.target.closest('[data-modal-close]') ||
        (e.target.classList && e.target.classList.contains('modal-backdrop'))
      ) {
        // Reanudar después de cerrar; re-sincroniza la señal en el próximo tick.
        setTimeout(function () { paused = false; }, 500);
      }
    });

    // Search as-you-type (debounce 300ms, sin botón) → server-side
    if (searchInput) {
      searchInput.addEventListener('input', function () {
        if (searchTimer) clearTimeout(searchTimer);
        searchTimer = setTimeout(function () {
          var url = new URL(window.location);
          var q = searchInput.value.trim();
          if (q) url.searchParams.set('q', q);
          else url.searchParams.delete('q');
          url.searchParams.delete('page');
          window.history.replaceState({}, '', url);
          refreshTable(); // el filtrado es server-side; refresca on-demand
        }, 300);
      });
    }

    // Paginación via AJAX (interceptar clicks en [data-page-link]) → server-side
    function bindPagination() {
      tableArea.querySelectorAll('[data-page-link]').forEach(function (a) {
        a.addEventListener('click', function (ev) {
          ev.preventDefault();
          var page = a.getAttribute('data-page-link');
          var url = new URL(window.location);
          if (page === '1') url.searchParams.delete('page');
          else url.searchParams.set('page', page);
          window.history.replaceState({}, '', url);
          refreshTable();
        });
      });
    }

    bindPagination();
    startWatch();
  }

  // Inicializar TODOS los mantenedores de la página.
  document.querySelectorAll('[data-maintainer-table]').forEach(initMaintainer);
})();
