(function () {
  'use strict';

  var root = document.documentElement;
  var THEME_KEY = 'domain-theme';

  var writeTheme = function (value) {
    try {
      window.localStorage.setItem(THEME_KEY, value);
    } catch (error) {
      return;
    }
  };

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
  });
})();
