// ============================================================
// clock.js — reloj en vivo del navbar (fecha + hora de Santiago
// de Chile). Reemplaza al titulo del navbar.
//
// Usa la zona horaria nombrada "America/Santiago" (Santiago de
// Chile), que resuelve sola el horario de verano/invierno (GMT-3
// en verano, GMT-4 en invierno). Asi siempre muestra la hora
// local REAL de Santiago, sin hardcodear el offset.
//
// Markup esperado (ver components/_navbar.html):
//   <div class="navbar-clock" data-clock>—</div>
// ============================================================
(function () {
  var el = document.querySelector('[data-clock]');
  if (!el) return;

  var TZ = 'America/Santiago';
  // Fecha: "lun 22 jun 2026"  ·  Hora: "17:30:45" (24h).
  var dateFmt = new Intl.DateTimeFormat('es-CL', {
    timeZone: TZ, weekday: 'short', day: '2-digit', month: 'short', year: 'numeric',
  });
  var timeFmt = new Intl.DateTimeFormat('es-CL', {
    timeZone: TZ, hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
  });

  function tick() {
    var now = new Date();
    // Capitaliza la primera letra del dia (es-CL lo da en minuscula).
    var date = dateFmt.format(now).replace(/^\w/, function (c) { return c.toUpperCase(); });
    el.innerHTML = date + ' <span class="navbar-clock-time">' +
      timeFmt.format(now) + '</span>';
  }

  tick();
  setInterval(tick, 1000);
})();
