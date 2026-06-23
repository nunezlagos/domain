// ============================================================
// sidebar.js — drawer off-canvas del sidebar en mobile.
//
// En pantallas chicas (< 768px, ver responsive.css) el sidebar
// lateral fijo se convierte en un panel deslizante (off-canvas)
// que se abre con el botón hamburguesa del navbar y se cierra
// al clickear el backdrop, un link de navegación o Escape.
//
// Markup esperado (ver base.html / _navbar.html):
//   button[data-sidebar-toggle]   — hamburguesa en el navbar
//   .sidebar                       — aside lateral (panel del drawer)
//   .sidebar-backdrop              — overlay oscuro detrás del drawer
//   body.sidebar-open              — clase que abre el drawer (CSS)
//
// Accesibilidad: el toggle expone aria-expanded; al abrir se
// mueve el foco al primer link del sidebar y al cerrar vuelve al
// botón. El drawer solo aplica visualmente en el breakpoint
// mobile; en desktop el CSS lo ignora.
// ============================================================
(function () {
  var BREAKPOINT = 768; // debe coincidir con responsive.css
  var body = document.body;

  function getToggle() { return document.querySelector('[data-sidebar-toggle]'); }
  function getSidebar() { return document.querySelector('.sidebar'); }

  function isMobile() {
    return window.matchMedia('(max-width: ' + (BREAKPOINT - 0.02) + 'px)').matches;
  }

  function isOpen() { return body.classList.contains('sidebar-open'); }

  function openSidebar() {
    var toggle = getToggle();
    var sidebar = getSidebar();
    body.classList.add('sidebar-open');
    if (toggle) toggle.setAttribute('aria-expanded', 'true');
    // Mover el foco al primer link navegable del sidebar.
    if (sidebar) {
      var firstLink = sidebar.querySelector('a[href], button');
      if (firstLink) firstLink.focus();
    }
  }

  function closeSidebar(returnFocus) {
    var toggle = getToggle();
    body.classList.remove('sidebar-open');
    if (toggle) {
      toggle.setAttribute('aria-expanded', 'false');
      if (returnFocus) toggle.focus();
    }
  }

  function toggleSidebar() {
    if (isOpen()) closeSidebar(true);
    else openSidebar();
  }

  // --- Colapso a solo-iconos (desktop). Persistido en localStorage. ---
  var COLLAPSE_KEY = 'sidebar-collapsed';

  function setCollapsed(on) {
    body.classList.toggle('sidebar-collapsed', on);
    try { localStorage.setItem(COLLAPSE_KEY, on ? '1' : '0'); } catch (e) { /* storage off */ }
  }

  function toggleCollapsed() { setCollapsed(!body.classList.contains('sidebar-collapsed')); }

  // Restaurar estado guardado (solo aplica visualmente en desktop, ver CSS).
  try {
    if (localStorage.getItem(COLLAPSE_KEY) === '1') body.classList.add('sidebar-collapsed');
  } catch (e) { /* storage off */ }

  // En modo colapsado los links quedan solo-icono: poner title con su texto
  // para que el tooltip identifique cada item al pasar el mouse.
  document.querySelectorAll('.sidebar-link').forEach(function (a) {
    if (!a.getAttribute('title')) {
      var label = (a.textContent || '').trim();
      if (label) a.setAttribute('title', label);
    }
  });

  document.addEventListener('click', function (e) {
    // Colapsar/expandir (logo o boton chevron). Solo desktop: en mobile el
    // sidebar es un drawer, no tiene sentido el modo solo-iconos.
    var collapse = e.target.closest('[data-sidebar-collapse]');
    if (collapse) {
      e.preventDefault();
      if (!isMobile()) toggleCollapsed();
      return;
    }
    // Hamburguesa
    var toggle = e.target.closest('[data-sidebar-toggle]');
    if (toggle) {
      e.preventDefault();
      toggleSidebar();
      return;
    }
    // Backdrop cierra el drawer
    if (e.target.classList && e.target.classList.contains('sidebar-backdrop')) {
      closeSidebar(true);
      return;
    }
    // Click en un link del sidebar cierra el drawer (en mobile)
    if (isOpen() && e.target.closest('.sidebar a[href]')) {
      closeSidebar(false);
      return;
    }
  });

  // Escape cierra el drawer
  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape' && isOpen()) {
      closeSidebar(true);
    }
  });

  // Si se agranda la ventana a desktop, asegurar el drawer cerrado
  window.addEventListener('resize', function () {
    if (!isMobile() && isOpen()) {
      closeSidebar(false);
    }
  });
})();
