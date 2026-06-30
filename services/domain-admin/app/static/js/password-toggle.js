// ============================================================
// password-toggle.js — botón "ojito" que alterna ver/ocultar un
// input de contraseña. Delegado en document para funcionar con
// los forms inyectados en el modal dinámico (mismo patrón que
// slug-autofill.js).
//
// Markup: un .btn-password-toggle con data-target="<id del input>"
// dentro de un .password-input-group junto al input.
// ============================================================
(function () {
  document.addEventListener('click', function (e) {
    var btn = e.target.closest('.btn-password-toggle');
    if (!btn) return;
    e.preventDefault();
    var input = document.getElementById(btn.dataset.target);
    if (!input) return;
    var show = input.type === 'password';
    input.type = show ? 'text' : 'password';
    btn.classList.toggle('is-visible', show);
  });
})();
