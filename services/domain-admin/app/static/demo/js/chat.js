// Chatbot flotante del portal (demo). Respuestas mock; acá iría la integración real (MCP).
(function () {
  const fab = document.getElementById('chatFab');
  const panel = document.getElementById('chatPanel');
  const closeBtn = document.getElementById('chatClose');
  const form = document.getElementById('chatForm');
  const input = document.getElementById('chatInput');
  const body = document.getElementById('chatBody');
  if (!fab || !panel || !form || !input || !body) return;

  function toggle(open) {
    const willOpen = typeof open === 'boolean' ? open : !panel.classList.contains('open');
    panel.classList.toggle('open', willOpen);
    fab.classList.toggle('active', willOpen);
    panel.setAttribute('aria-hidden', String(!willOpen));
    if (willOpen) setTimeout(() => input.focus(), 250);
  }

  function addMsg(text, who) {
    const el = document.createElement('div');
    el.className = 'chat-msg ' + who;
    el.textContent = text;
    body.appendChild(el);
    body.scrollTop = body.scrollHeight;
    return el;
  }

  function mockAnswer(q) {
    const t = q.toLowerCase();
    if (/(hola|buenas|buenos)/.test(t)) return '¡Hola! Soy el asistente del portal. ¿En qué te ayudo?';
    if (/(gracias|genial|perfecto)/.test(t)) return '¡De nada! 😊';
    if (/(ayuda|help|qué pod|que pod)/.test(t)) return 'Puedo responder sobre tus skills, métricas y proyectos. (demo: respuestas de ejemplo)';
    if (t.includes('?')) return 'Buena pregunta — en esta demo respondo de ejemplo; acá conectaría con el MCP para la respuesta real.';
    return 'Recibido: "' + q + '". (respuesta de ejemplo del chatbot)';
  }

  function botReply(userText) {
    const typing = addMsg('', 'bot typing');
    typing.innerHTML = '<span></span><span></span><span></span>';
    body.scrollTop = body.scrollHeight;
    setTimeout(() => {
      typing.remove();
      addMsg(mockAnswer(userText), 'bot');
    }, 700 + Math.random() * 500);
  }

  fab.addEventListener('click', () => toggle());
  closeBtn.addEventListener('click', () => toggle(false));
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && panel.classList.contains('open')) toggle(false);
  });
  form.addEventListener('submit', (e) => {
    e.preventDefault();
    const text = input.value.trim();
    if (!text) return;
    addMsg(text, 'user');
    input.value = '';
    botReply(text);
  });

  // saludo inicial
  addMsg('Hola 👋 Soy tu asistente. ¿En qué te ayudo?', 'bot');
})();
