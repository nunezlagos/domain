(function () {
  'use strict';

  var root = document.documentElement;
  var THEME_KEY = 'domain-theme';

  var readTheme = function () {
    try {
      return window.localStorage.getItem(THEME_KEY);
    } catch (error) {
      return null;
    }
  };
  var writeTheme = function (value) {
    try {
      window.localStorage.setItem(THEME_KEY, value);
    } catch (error) {
      return;
    }
  };

  if (readTheme() === 'dark') {
    root.classList.add('dark');
  }

  var modeToggle = document.getElementById('modeToggle');
  if (modeToggle) {
    modeToggle.setAttribute('aria-pressed', String(root.classList.contains('dark')));
    modeToggle.addEventListener('click', function () {
      root.classList.toggle('dark');
      var isDark = root.classList.contains('dark');
      writeTheme(isDark ? 'dark' : 'light');
      modeToggle.setAttribute('aria-pressed', String(isDark));
    });
  }

  var navToggle = document.getElementById('navToggle');
  var navLinks = document.getElementById('navLinks');
  if (navToggle && navLinks) {
    var setNavOpen = function (open) {
      navLinks.setAttribute('data-open', String(open));
      navToggle.setAttribute('aria-expanded', String(open));
    };
    navToggle.addEventListener('click', function () {
      setNavOpen(navLinks.getAttribute('data-open') !== 'true');
    });
    navLinks.addEventListener('click', function (event) {
      if (event.target.closest('a')) {
        setNavOpen(false);
      }
    });
    document.addEventListener('keydown', function (event) {
      if (event.key === 'Escape') {
        setNavOpen(false);
      }
    });
  }

  var tabs = document.querySelectorAll('.install-tab');
  var panels = document.querySelectorAll('.install-panel');
  panels.forEach(function (panel) {
    panel.hidden = !panel.classList.contains('active');
  });
  tabs.forEach(function (tab) {
    tab.addEventListener('click', function () {
      var target = document.getElementById('tab-' + tab.dataset.tab);
      if (!target) {
        return;
      }
      tabs.forEach(function (other) {
        other.classList.remove('active');
        other.setAttribute('aria-selected', 'false');
      });
      panels.forEach(function (panel) {
        panel.classList.remove('active');
        panel.hidden = true;
      });
      tab.classList.add('active');
      tab.setAttribute('aria-selected', 'true');
      target.classList.add('active');
      target.hidden = false;
    });
  });

  var copyWithTextarea = function (text) {
    var field = document.createElement('textarea');
    field.value = text;
    field.setAttribute('readonly', '');
    field.style.position = 'fixed';
    field.style.top = '0';
    field.style.left = '-9999px';
    field.style.opacity = '0';
    field.style.pointerEvents = 'none';
    document.body.appendChild(field);
    field.select();
    var copied = false;
    try {
      copied = document.execCommand('copy') === true;
    } catch (error) {
      copied = false;
    } finally {
      document.body.removeChild(field);
    }
    return copied;
  };

  document.querySelectorAll('.btn-copy').forEach(function (button) {
    var icon = button.querySelector('i');
    var labelNode = document.createElement('span');
    var pristineLabel = button.textContent.trim();
    var pristineIcon = icon ? icon.className : '';
    labelNode.textContent = pristineLabel;
    button.textContent = '';
    if (icon) {
      button.appendChild(icon);
    }
    button.appendChild(labelNode);
    button.setAttribute('role', 'status');

    var resetTimer = null;

    var showState = function (label, iconClass, stateClass) {
      if (resetTimer !== null) {
        window.clearTimeout(resetTimer);
      }
      labelNode.textContent = label;
      if (icon) {
        icon.className = iconClass;
      }
      button.classList.remove('copied');
      button.classList.remove('copy-failed');
      button.classList.add(stateClass);
      resetTimer = window.setTimeout(function () {
        labelNode.textContent = pristineLabel;
        if (icon) {
          icon.className = pristineIcon;
        }
        button.classList.remove('copied');
        button.classList.remove('copy-failed');
        resetTimer = null;
      }, 2000);
    };

    var reportCopied = function () {
      showState('Copiado', 'fas fa-check', 'copied');
    };
    var reportFailed = function () {
      showState('Copia manual', 'fas fa-triangle-exclamation', 'copy-failed');
    };
    var fallbackCopy = function (text) {
      if (copyWithTextarea(text)) {
        reportCopied();
        return;
      }
      reportFailed();
    };

    button.addEventListener('click', function () {
      var text = button.dataset.copy;
      if (!text) {
        return;
      }
      if (navigator.clipboard && window.isSecureContext) {
        navigator.clipboard.writeText(text).then(reportCopied, function () {
          fallbackCopy(text);
        });
        return;
      }
      fallbackCopy(text);
    });
  });

  var nav = document.querySelector('.nav');
  if (nav) {
    var applyNavScroll = function () {
      nav.classList.toggle('is-scrolled', window.scrollY > 10);
    };
    applyNavScroll();
    window.addEventListener('scroll', applyNavScroll, { passive: true });
  }
})();
