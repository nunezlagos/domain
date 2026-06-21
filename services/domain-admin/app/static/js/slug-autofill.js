// ============================================================
// slug-autofill.js — autogenera el campo `slug` a partir del
// campo fuente (`name`/`title`/`nombre`) en cualquier form de
// mantenedor, manteniéndolo EDITABLE por el usuario.
//
// Convención (cero cambios en los templates de form): dentro de
// un mismo <form>, si existe un input [name="slug"] y un campo
// fuente [name="name"|"title"|"nombre"], al tipear en la fuente
// se rellena el slug con su versión slugificada.
//
// Reglas de "ownership" del slug (vía dataset.slugAuto en el input):
//   - Si el slug está vacío y nunca lo tocó nadie → el primer
//     tipeo en la fuente lo autogenera (slugAuto="1").
//   - Mientras slugAuto="1" se sigue regenerando con cada tipeo.
//   - Si el usuario edita el slug a mano → slugAuto se limpia y
//     la fuente deja de pisarlo.
//   - En edición, el slug viene precargado (slugAuto sin setear)
//     → NO se pisa salvo que el usuario lo edite explícitamente.
//
// Delegado en document → funciona con los forms inyectados en el
// modal dinámico (donde un <script> inline no correría).
// ============================================================
(function () {
  var SOURCE_NAMES = ['name', 'title', 'nombre'];

  // Slugify: sin acentos, sin signos, minúsculas, kebab-case.
  // "Áéí Qué-Tal!" → "aei-que-tal"
  function slugify(value) {
    return value
      .normalize('NFD').replace(/[̀-ͯ]/g, '') // separa y quita diacríticos
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-') // no-alfanumérico → guion
      .replace(/-{2,}/g, '-')      // colapsa guiones repetidos
      .replace(/^-+|-+$/g, '');    // recorta guiones de los extremos
  }

  function slugFieldOf(el) {
    var form = el.form || el.closest('form');
    return form ? form.querySelector('input[name="slug"]') : null;
  }

  document.addEventListener('input', function (e) {
    var t = e.target;
    if (!t || !t.name) return;

    // El usuario tipea directo en el slug → toma ownership (deja de autogenerarse).
    if (t.name === 'slug') {
      t.dataset.slugAuto = '';
      return;
    }

    // Tipeo en un campo fuente → autogenera el slug si corresponde.
    if (SOURCE_NAMES.indexOf(t.name) === -1) return;
    var slug = slugFieldOf(t);
    if (!slug) return;

    var autogenerable = slug.dataset.slugAuto === '1' || slug.value === '';
    if (!autogenerable) return; // slug precargado (edición) o propiedad del usuario

    slug.value = slugify(t.value);
    slug.dataset.slugAuto = '1';
  });
})();
