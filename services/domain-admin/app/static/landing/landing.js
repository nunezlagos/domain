(function () {
  'use strict';

  var root = document.documentElement;
  var THEME_KEY = 'domain-theme';

  // el fondo de cada tema, igual que --wag-cream-soft y --wag-ink de tokens.css.
  // van literales y no leidos del fondo ya calculado porque landing.css declara
  // una transicion sobre background: leerlo tras el toggle da un valor intermedio
  var CROMO = { claro: '#faf8f2', oscuro: '#141412' };
  var themeMeta = document.querySelector('meta[name="theme-color"]');

  var writeTheme = function (value) {
    try {
      window.localStorage.setItem(THEME_KEY, value);
    } catch (error) {
      return;
    }
  };

  var pintarCromo = function (esOscuro) {
    if (themeMeta) {
      themeMeta.setAttribute('content', esOscuro ? CROMO.oscuro : CROMO.claro);
    }
  };

  pintarCromo(root.classList.contains('dark'));

  var modeToggle = document.getElementById('modeToggle');
  if (!modeToggle) {
    return;
  }

  modeToggle.setAttribute('aria-pressed', String(root.classList.contains('dark')));
  modeToggle.addEventListener('click', function () {
    root.classList.toggle('dark');
    var isDark = root.classList.contains('dark');
    writeTheme(isDark ? 'dark' : 'light');
    modeToggle.setAttribute('aria-pressed', String(isDark));
    pintarCromo(isDark);
  });
})();
