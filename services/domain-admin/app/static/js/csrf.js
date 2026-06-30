// ============================================================
// CSRF: leer el token de la cookie que Django setea via
// CsrfViewMiddleware. Expuesto global para usarse en cualquier
// fetch() con método no-seguro (POST/PUT/PATCH/DELETE).
// ============================================================
(function (global) {
  function getCSRFToken() {
    var match = document.cookie.match(/csrftoken=([^;]+)/);
    return match ? match[1] : '';
  }
  global.getCSRFToken = getCSRFToken;
})(window);
