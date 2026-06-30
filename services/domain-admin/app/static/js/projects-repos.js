// ============================================================
// projects-repos.js — filas dinámicas de "Repositorios git" en el
// form de proyecto. El + clona la fila template y la agrega; el −
// quita su fila. Delegado en document para funcionar dentro del
// modal dinámico (donde un <script> inline no correría).
//
// Markup esperado (templates/projects/_repo_rows.html):
//   [data-repo-list]      contenedor de filas .repo-row
//   [data-repo-template]  <template> con una fila vacía para clonar
//   [data-repo-add]       botón +
//   [data-repo-remove]    botón − dentro de cada fila
// ============================================================
(function () {
  document.addEventListener('click', function (e) {
    // Agregar fila
    var addBtn = e.target.closest('[data-repo-add]');
    if (addBtn) {
      e.preventDefault();
      var form = addBtn.closest('form') || document;
      var list = form.querySelector('[data-repo-list]');
      var tpl = form.querySelector('[data-repo-template]');
      if (!list || !tpl) return;
      var clone = tpl.content.firstElementChild.cloneNode(true);
      list.appendChild(clone);
      var firstInput = clone.querySelector('input');
      if (firstInput) firstInput.focus();
      return;
    }

    // Quitar fila. Si es la última, la vaciamos en vez de borrarla
    // (siempre queda al menos una fila para tipear).
    var removeBtn = e.target.closest('[data-repo-remove]');
    if (removeBtn) {
      e.preventDefault();
      var row = removeBtn.closest('.repo-row');
      if (!row) return;
      var listEl = row.parentElement;
      if (listEl && listEl.querySelectorAll('.repo-row').length > 1) {
        row.remove();
      } else {
        row.querySelectorAll('input').forEach(function (i) { i.value = ''; });
      }
      return;
    }
  });
})();
