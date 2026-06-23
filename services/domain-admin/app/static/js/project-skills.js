// ============================================================
// project-skills.js — excluir/re-incluir skills de un proyecto
// dentro del tab "Skills" del ver/editar, sin recargar la página.
//
// Cada botón [data-skill-toggle] POSTea {skill_id, op} a su data-url
// (/proyectos/<id>/skills/toggle/, op=exclude|include) y el server devuelve
// SOLO el pane de skills re-renderizado, que reemplaza el contenido de
// #project-skills-pane (NO todo el modal: así no se pierde el form de edición
// ni el tab activo, que viven fuera del pane).
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
        // Reemplaza SOLO el pane de skills (no todo el modal): preserva el form
        // de edición y el tab activo, que están fuera de #project-skills-pane.
        var c = document.getElementById('project-skills-pane');
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
