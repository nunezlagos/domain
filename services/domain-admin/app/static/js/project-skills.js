// ============================================================
// project-skills.js — enlazar/desenlazar skills de un proyecto
// dentro del modal "Skills del proyecto", sin recargar la página.
//
// Cada botón [data-skill-toggle] POSTea {skill_id, op} a su data-url
// (/proyectos/<id>/skills/toggle/) y el server devuelve el partial del
// modal re-renderizado, que reemplaza #modal-dynamic-content.
//
// Depende de csrf.js (getCSRFToken). Delegado en document para funcionar
// con el HTML inyectado en el modal dinámico.
// ============================================================
(function () {
  document.addEventListener('click', async function (e) {
    var btn = e.target.closest('[data-skill-toggle]');
    if (!btn) return;
    e.preventDefault();
    var url = btn.dataset.url;
    if (!url) return;
    btn.disabled = true;
    var body = new URLSearchParams({
      skill_id: btn.dataset.skillId || '',
      op: btn.dataset.op || '',
    });
    try {
      var r = await fetch(url, {
        method: 'POST',
        headers: {
          'X-CSRFToken': getCSRFToken(),
          'X-Requested-With': 'fetch',
          'Content-Type': 'application/x-www-form-urlencoded',
        },
        credentials: 'same-origin',
        body: body.toString(),
      });
      if (r.ok) {
        var html = await r.text();
        var c = document.getElementById('modal-dynamic-content');
        if (c) c.innerHTML = html;
      } else {
        btn.disabled = false;
        alert('Error (' + r.status + ')');
      }
    } catch (err) {
      btn.disabled = false;
      alert('Error de red: ' + err.message);
    }
  });
})();
