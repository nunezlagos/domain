/* HU-49.3: cliente del chat widget estilo burbuja (ChatGPT-style).

   - Boton flotante visible en todas las paginas del admin
   - Click -> abre/cierra popup con animacion pop-in/out
   - Click fuera o X -> cerrar
   - Ctrl+K abre nueva conversacion
   - Sidebar colapsable con search
   - Sugerencias clickeables completan el input
   - Polling cada 1.5s mientras el bot procesa
   - Typing indicator: 3 dots animados + shimmer (estilo ChatGPT)
   - Boton toggle: send (paper plane) -> stop (cuadrado rojo) durante polling
   - Burbuja con badge de no leidos + pulse animation
   - Auto-resize textarea hasta 120px
   - Scroll inteligente + boton "ir al fondo"
*/

(function () {
  "use strict";

  const API = "/chat/api";
  const POLL_INTERVAL_MS = 1500;
  const SCROLL_BOTTOM_THRESHOLD = 100;
  const SEARCH_DEBOUNCE_MS = 200;

  const state = {
    conversations: [],
    filteredConversations: [],
    activeId: null,
    messages: [],
    pollingId: null,
    pollingAbort: false,
    searchQuery: "",
    open: false,
    unread: 0,
    // HU-52.1: feedback enviado por message_id -> rating (1 / -1). Persiste en
    // memoria durante la sesion para no re-mostrar los botones tras votar.
    feedback: {},
  };

  // HU-52.1: endpoint del feedback loop (view del Django, session-auth).
  const FEEDBACK_URL = "/feedback/api/submit";

  const $ = (id) => document.getElementById(id);

  function escapeHtml(s) {
    return String(s || "")
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#39;");
  }

  async function apiFetch(path, opts = {}) {
    const headers = { "X-Requested-With": "fetch", ...(opts.headers || {}) };
    if (opts.method && opts.method !== "GET") headers["Content-Type"] = "application/json";
    const res = await fetch(API + path, {
      method: opts.method || "GET",
      headers,
      credentials: "same-origin",
      body: opts.body ? JSON.stringify(opts.body) : undefined,
    });
    if (!res.ok) throw new Error("HTTP " + res.status + ": " + await res.text());
    if (res.status === 204) return null;
    return res.json();
  }

  function relativeTime(iso) {
    if (!iso) return "";
    const d = new Date(iso);
    const diff = (Date.now() - d.getTime()) / 1000;
    if (diff < 60) return "ahora";
    if (diff < 3600) return Math.floor(diff / 60) + "m";
    if (diff < 86400) return Math.floor(diff / 3600) + "h";
    if (diff < 604800) return Math.floor(diff / 86400) + "d";
    return d.toLocaleDateString();
  }

  function isScrolledToBottom() {
    const c = $("chat-messages");
    if (!c) return true;
    return c.scrollHeight - c.scrollTop - c.clientHeight < SCROLL_BOTTOM_THRESHOLD;
  }

  function scrollToBottom(smooth) {
    const c = $("chat-messages");
    if (!c) return;
    c.scrollTo({ top: c.scrollHeight, behavior: smooth ? "smooth" : "auto" });
  }

  function openWidget() {
    const widget = $("llm-widget");
    const backdrop = $("llm-widget-backdrop");
    if (!widget) return;
    state.open = true;
    widget.classList.remove("closing");
    widget.classList.add("open");
    widget.setAttribute("aria-hidden", "false");
    if (backdrop) backdrop.classList.add("active");
    clearUnread();
    setTimeout(() => {
      const input = $("chat-input");
      if (input && !input.disabled) input.focus();
    }, 350);
  }

  function closeWidget() {
    const widget = $("llm-widget");
    const backdrop = $("llm-widget-backdrop");
    if (!widget || !state.open) return;
    widget.classList.add("closing");
    widget.setAttribute("aria-hidden", "true");
    if (backdrop) backdrop.classList.remove("active");
    setTimeout(() => {
      widget.classList.remove("open", "closing");
      state.open = false;
    }, 300);
  }

  function toggleWidget() {
    if (state.open) closeWidget();
    else openWidget();
  }

  function incrementUnread() {
    if (state.open) return;
    state.unread += 1;
    const badge = $("llm-bubble-badge");
    if (badge) {
      badge.style.display = "flex";
      badge.textContent = state.unread > 9 ? "9+" : state.unread;
    }
    const bubble = $("llm-bubble");
    if (bubble) bubble.classList.add("has-unread");
  }

  function clearUnread() {
    state.unread = 0;
    const badge = $("llm-bubble-badge");
    if (badge) badge.style.display = "none";
    const bubble = $("llm-bubble");
    if (bubble) bubble.classList.remove("has-unread");
  }

  /* ============================================================
     RENDER: lista de conversaciones
     ============================================================ */

  function dayBucket(iso) {
    const d = new Date(iso);
    const now = new Date();
    const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const dStart = new Date(d.getFullYear(), d.getMonth(), d.getDate());
    const diffDays = Math.floor((todayStart - dStart) / 86400000);
    if (diffDays === 0) return "Hoy";
    if (diffDays === 1) return "Ayer";
    if (diffDays < 7) return "Esta semana";
    if (diffDays < 30) return "Este mes";
    return "Mas antiguo";
  }

  function renderConversations() {
    const list = $("chat-list");
    if (!list) return;
    const convs = state.filteredConversations;
    if (state.conversations.length === 0) {
      list.innerHTML = '<div class="llm-widget-list-empty">Aun no tienes conversaciones.<br>Empezá una nueva con el boton +.</div>';
      return;
    }
    if (convs.length === 0) {
      list.innerHTML = '<div class="llm-widget-list-empty">Sin resultados para la busqueda.</div>';
      return;
    }

    const titleCounts = {};
    convs.forEach((c) => {
      const k = (c.title || "Nueva conversacion").trim().toLowerCase();
      titleCounts[k] = (titleCounts[k] || 0) + 1;
    });

    const groups = { "Hoy": [], "Ayer": [], "Esta semana": [], "Este mes": [], "Mas antiguo": [] };
    convs.forEach((c) => {
      const bucket = dayBucket(c.updated_at || c.created_at);
      groups[bucket].push(c);
    });

    const html = Object.entries(groups)
      .filter(([_, items]) => items.length > 0)
      .map(([bucket, items]) => {
        const itemsHtml = items.map((c) => {
          const active = c.id === state.activeId ? " active" : "";
          const baseTitle = (c.title || "").trim();
          const isEmpty = !baseTitle || baseTitle.toLowerCase() === "nueva conversacion";
          const dupCount = titleCounts[baseTitle.toLowerCase()] || 0;
          const titleText = isEmpty
            ? "Conversacion vacia"
            : (dupCount > 1 ? baseTitle : baseTitle);
          const preview = (c.last_message_preview || "").slice(0, 60);
          const isEmptyClass = isEmpty ? " llm-widget-item-empty" : "";
          return `
            <div class="llm-widget-item${active}${isEmptyClass}" data-id="${c.id}" title="${escapeHtml(c.title || 'Nueva conversacion')}">
              <div class="llm-widget-item-title">
                <span class="llm-widget-item-dot"></span>
                <span class="llm-widget-item-title-text">${escapeHtml(titleText)}</span>
              </div>
              ${preview ? `<div class="llm-widget-item-preview">${escapeHtml(preview)}</div>` : ""}
              <div class="llm-widget-item-time">${relativeTime(c.updated_at)}</div>
            </div>`;
        }).join("");
        return `
          <div class="llm-widget-list-group">
            <div class="llm-widget-list-group-title">${bucket}</div>
            ${itemsHtml}
          </div>`;
      })
      .join("");

    list.innerHTML = html;
    list.querySelectorAll(".llm-widget-item").forEach((el) => {
      el.addEventListener("click", () => selectConversation(el.dataset.id));
    });
  }

  function shortTime(iso) {
    if (!iso) return "";
    const d = new Date(iso);
    return d.toLocaleString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" });
  }

  function filterConversations() {
    const q = state.searchQuery.toLowerCase().trim();
    state.filteredConversations = !q
      ? state.conversations
      : state.conversations.filter((c) =>
          (c.title || "").toLowerCase().includes(q) ||
          (c.last_message_preview || "").toLowerCase().includes(q)
        );
  }

  function debounce(fn, ms) {
    let t;
    return function (...args) {
      clearTimeout(t);
      t = setTimeout(() => fn.apply(this, args), ms);
    };
  }

  /* ============================================================
     RENDER: mensajes + typing indicator
     ============================================================ */

  function renderSources(sources) {
    if (!sources || sources.length === 0) return "";
    return (
      '<div class="llm-widget-msg-sources">' +
      sources.map((s) => {
        const scorePct = Math.round((s.score || 0) * 100);
        const url = s.url || "#";
        return `<a class="llm-widget-source" href="${escapeHtml(url)}" target="_blank" rel="noopener">
          ${escapeHtml(s.titulo || s.id)} ${scorePct > 0 ? `· ${scorePct}%` : ""}
        </a>`;
      }).join("") +
      "</div>"
    );
  }

  function renderMarkdown(text) {
    if (typeof window.marked === "undefined") return escapeHtml(text).replace(/\n/g, "<br>");
    try {
      return window.marked.parse(text, { breaks: true, gfm: true });
    } catch (e) {
      return escapeHtml(text);
    }
  }

  function renderTypingBubble(text) {
    return `<div class="llm-widget-pending">
      <div class="llm-widget-msg-avatar" style="background: var(--color-accent); color: var(--color-text-inverse);">B</div>
      <div class="llm-widget-pending-bubble">
        <span class="llm-widget-pending-dots">
          <span class="llm-widget-pending-dot"></span>
          <span class="llm-widget-pending-dot"></span>
          <span class="llm-widget-pending-dot"></span>
        </span>
        ${text ? `<span class="llm-widget-pending-text">${escapeHtml(text)}</span>` : ""}
      </div>
    </div>`;
  }

  /* ============================================================
     HU-52.1: feedback loop (👍/👎) bajo cada respuesta del assistant
     ============================================================ */

  // Deriva el skill_slug del mensaje desde sus sources. Las sources guardan
  // tabla/id/titulo/snippet; el chunk (snippet) embebe "slug=<slug>" cuando la
  // fuente es un skill. Si no se puede derivar, devuelve "" (NULL en la DB:
  // el backend lo agrupa en el bucket "sin skill").
  function skillSlugFromMessage(msg) {
    const sources = msg.sources || [];
    const skillSrc = sources.find((s) => (s.tabla || s.table) === "skill");
    if (!skillSrc) return "";
    const snippet = skillSrc.snippet || "";
    const m = snippet.match(/slug=([^|\s]+)/);
    if (m) return m[1].trim();
    return "";
  }

  // Renderiza los botones (o el "gracias" si ya se voto). Solo para respuestas
  // del assistant ya completas y persistidas (id > 0, no placeholders temp).
  function renderFeedback(msg) {
    if (msg.role !== "assistant") return "";
    if (msg.status !== "completed") return "";
    if (typeof msg.id !== "number" || msg.id <= 0) return "";

    const voted = state.feedback[msg.id];
    if (voted === 1 || voted === -1) {
      return `<div class="llm-feedback llm-feedback-done">
        <span class="llm-feedback-thanks">Gracias por tu feedback ${voted === 1 ? "👍" : "👎"}</span>
      </div>`;
    }
    return `<div class="llm-feedback" data-msg-id="${msg.id}">
      <button type="button" class="llm-feedback-btn llm-feedback-up" data-rating="1" title="Respuesta util" aria-label="Me gusta">
        <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M14 9V5a3 3 0 0 0-3-3l-4 9v11h11.28a2 2 0 0 0 2-1.7l1.38-9a2 2 0 0 0-2-2.3zM7 22H4a2 2 0 0 1-2-2v-7a2 2 0 0 1 2-2h3"/>
        </svg>
      </button>
      <button type="button" class="llm-feedback-btn llm-feedback-down" data-rating="-1" title="Respuesta poco util" aria-label="No me gusta">
        <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M10 15v4a3 3 0 0 0 3 3l4-9V2H5.72a2 2 0 0 0-2 1.7l-1.38 9a2 2 0 0 0 2 2.3zm7-13h2.67A2.31 2.31 0 0 1 22 4v7a2.31 2.31 0 0 1-2.33 2H17"/>
        </svg>
      </button>
    </div>`;
  }

  // Envia el voto al backend. rating: 1 (👍) | -1 (👎). comment opcional.
  async function submitFeedback(messageId, rating, comment, skillSlug) {
    const res = await fetch(FEEDBACK_URL, {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Requested-With": "fetch" },
      credentials: "same-origin",
      body: JSON.stringify({
        message_id: messageId,
        rating: rating,
        comment: comment || "",
        skill_slug: skillSlug || "",
      }),
    });
    if (!res.ok) throw new Error("HTTP " + res.status);
    return res.json();
  }

  // Maneja clicks en los botones de feedback (delegado en #chat-messages).
  async function handleFeedbackClick(e) {
    const btn = e.target.closest(".llm-feedback-btn");
    if (!btn) return;
    e.stopPropagation();
    const wrap = btn.closest(".llm-feedback");
    if (!wrap) return;
    const msgId = parseInt(wrap.dataset.msgId, 10);
    const rating = parseInt(btn.dataset.rating, 10);
    if (!msgId || (rating !== 1 && rating !== -1)) return;

    const msg = state.messages.find((m) => m.id === msgId);
    const skillSlug = msg ? skillSlugFromMessage(msg) : "";

    // 👎 -> ofrecer textbox inline opcional antes de enviar.
    if (rating === -1 && !wrap.querySelector(".llm-feedback-comment")) {
      showFeedbackComment(wrap, msgId, skillSlug);
      return;
    }
    await sendAndLock(wrap, msgId, rating, "", skillSlug);
  }

  function showFeedbackComment(wrap, msgId, skillSlug) {
    const box = document.createElement("div");
    box.className = "llm-feedback-comment";
    box.innerHTML = `
      <textarea class="llm-feedback-comment-input" rows="2" maxlength="1000"
        placeholder="¿Qué falló? (opcional)"></textarea>
      <div class="llm-feedback-comment-actions">
        <button type="button" class="llm-feedback-comment-send">Enviar</button>
        <button type="button" class="llm-feedback-comment-skip">Omitir</button>
      </div>`;
    wrap.appendChild(box);
    const ta = box.querySelector(".llm-feedback-comment-input");
    if (ta) ta.focus();
    box.querySelector(".llm-feedback-comment-send").addEventListener("click", () => {
      sendAndLock(wrap, msgId, -1, ta ? ta.value.trim() : "", skillSlug);
    });
    box.querySelector(".llm-feedback-comment-skip").addEventListener("click", () => {
      sendAndLock(wrap, msgId, -1, "", skillSlug);
    });
  }

  async function sendAndLock(wrap, msgId, rating, comment, skillSlug) {
    wrap.querySelectorAll("button, textarea").forEach((el) => (el.disabled = true));
    try {
      await submitFeedback(msgId, rating, comment, skillSlug);
      state.feedback[msgId] = rating;
      wrap.classList.add("llm-feedback-done");
      wrap.innerHTML = `<span class="llm-feedback-thanks">Gracias por tu feedback ${rating === 1 ? "👍" : "👎"}</span>`;
    } catch (err) {
      console.error("feedback", err);
      wrap.querySelectorAll("button, textarea").forEach((el) => (el.disabled = false));
      const errSpan = document.createElement("span");
      errSpan.className = "llm-feedback-error";
      errSpan.textContent = "No se pudo enviar. Reintentá.";
      wrap.appendChild(errSpan);
    }
  }

  function renderMessage(msg) {
    const isUser = msg.role === "user";
    const isPending = msg.status === "pending";
    const isProcessing = msg.status === "processing";
    const isError = msg.status === "error";
    const content = msg.content || msg.content_partial || "";

    if (isPending) {
      return renderTypingBubble("Pensando...");
    }
    if (isProcessing && !content) {
      return renderTypingBubble("Generando respuesta...");
    }
    if (isProcessing && content) {
      const processedHtml = renderMarkdown(content);
      return `<div class="llm-widget-msg llm-widget-msg-assistant">
        <div class="llm-widget-msg-avatar" style="background: var(--color-accent); color: var(--color-text-inverse);">B</div>
        <div>
          <div class="llm-widget-msg-bubble">${processedHtml}</div>
          <div style="margin-top: 4px;">
            <span class="llm-widget-pending-dots">
              <span class="llm-widget-pending-dot"></span>
              <span class="llm-widget-pending-dot"></span>
              <span class="llm-widget-pending-dot"></span>
            </span>
          </div>
        </div>
      </div>`;
    }

    const cls = "llm-widget-msg-" + (isUser ? "user" : "assistant");
    const avatar = isUser ? "U" : "B";
    const errorCls = isError ? " llm-widget-msg-error" : "";

    let html;
    if (isUser) {
      html = escapeHtml(content).replace(/\n/g, "<br>");
    } else {
      html = renderMarkdown(content);
    }
    const sourcesHtml = isError ? "" : renderSources(msg.sources);
    const time = msg.created_at ? new Date(msg.created_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }) : "";
    const meta = time ? `<div class="llm-widget-msg-meta">${time}${msg.model ? " · " + escapeHtml(msg.model) : ""}</div>` : "";
    // HU-52.1: feedback solo bajo respuestas del assistant (no del user).
    const feedbackHtml = isUser ? "" : renderFeedback(msg);

    return `<div class="llm-widget-msg ${cls}">
      <div class="llm-widget-msg-avatar">${avatar}</div>
      <div>
        <div class="llm-widget-msg-bubble${errorCls}">${html}${sourcesHtml}${meta}</div>
        ${feedbackHtml}
      </div>
    </div>`;
  }

  function renderMessages(opts) {
    const container = $("chat-messages");
    if (!container) return;
    if (!state.activeId) {
      container.innerHTML = "";
      const welcome = $("chat-empty");
      if (welcome) container.appendChild(welcome);
      return;
    }
    const form = $("chat-form");
    if (form) form.style.display = "block";
    const welcome = $("chat-empty");
    if (welcome) welcome.remove();
    const wasAtBottom = isScrolledToBottom();
    const prevCount = container.children.length;
    container.innerHTML = state.messages.map(renderMessage).join("");
    const btnDown = $("btn-scroll-down");
    const newCount = container.children.length;
    const lastMsg = state.messages[state.messages.length - 1];
    const lastIsAssistant = lastMsg && lastMsg.role === "assistant";

    if (opts && opts.initial) {
      scrollToBottom(false);
      if (btnDown) btnDown.style.display = "none";
    } else if (wasAtBottom) {
      scrollToBottom(true);
      if (btnDown) btnDown.style.display = "none";
    } else {
      if (btnDown) btnDown.style.display = "flex";
    }

    if (lastIsAssistant && !state.open && newCount > prevCount) {
      incrementUnread();
    }
  }

  /* ============================================================
     API: conversaciones + mensajes
     ============================================================ */

  async function loadConversations() {
    try {
      const data = await apiFetch("/conversations");
      state.conversations = data.data || [];
      filterConversations();
      renderConversations();
    } catch (e) { console.error("loadConversations", e); }
  }

  async function createConversation() {
    try {
      const data = await apiFetch("/conversations/new", { method: "POST", body: {} });
      const conv = data.data;
      state.conversations.unshift(conv);
      filterConversations();
      renderConversations();
      await selectConversation(conv.id);
      openWidget();
    } catch (e) {
      console.error("createConversation", e);
      alert("Error creando conversacion: " + e.message);
    }
  }

  async function loadMessages(convId) {
    try {
      const data = await apiFetch("/conversations/" + convId + "/messages");
      state.messages = data.data || [];
      renderMessages({ initial: true });
    } catch (e) {
      console.error("loadMessages", e);
      state.messages = [];
      renderMessages();
    }
  }

  /* ============================================================
     STATE: enviar / stop
     ============================================================ */

  function setSendingState(sending) {
    const input = $("chat-input");
    const btnSend = $("btn-send");
    const btnStop = $("btn-stop");
    const shell = $("chat-input-shell");
    if (!input || !btnSend || !btnStop || !shell) return;

    input.disabled = sending;
    if (sending) {
      input.placeholder = "Esperando respuesta...";
      btnSend.style.display = "none";
      btnStop.style.display = "flex";
      shell.classList.add("processing");
    } else {
      input.placeholder = "Escribi tu pregunta...";
      btnSend.style.display = "flex";
      btnStop.style.display = "none";
      shell.classList.remove("processing");
      input.focus();
    }
  }

  async function selectConversation(id) {
    stopPolling();
    state.activeId = id;
    state.messages = [];
    document.querySelectorAll(".llm-widget-item").forEach((el) => {
      el.classList.toggle("active", el.dataset.id === id);
    });
    await loadMessages(id);
  }

  async function sendMessage() {
    const input = $("chat-input");
    const text = input.value.trim();
    if (!text || !state.activeId) return;
    input.value = "";
    autoResizeTextarea();
    updateSendButton();
    setSendingState(true);
    const tempMsg = {
      id: -Date.now(),
      role: "user",
      content: text,
      content_partial: null,
      status: "completed",
      sources: [],
      created_at: new Date().toISOString(),
    };
    state.messages.push(tempMsg);
    renderMessages();
    const btnDown = $("btn-scroll-down");
    if (btnDown) btnDown.style.display = "none";
    try {
      const data = await apiFetch(
        "/conversations/" + state.activeId + "/messages/new",
        { method: "POST", body: { content: text } }
      );
      const serverMsg = data.data;
      const idx = state.messages.findIndex((m) => m.id === tempMsg.id);
      if (idx >= 0) state.messages[idx] = serverMsg;
      renderMessages();
      startPolling(serverMsg.id);
    } catch (e) {
      console.error("sendMessage", e);
      alert("Error enviando mensaje: " + e.message);
      setSendingState(false);
    }
  }

  function startPolling(messageId) {
    state.pollingAbort = false;
    const tick = async () => {
      if (state.pollingAbort) return;
      try {
        const data = await apiFetch("/messages/" + messageId);
        const msg = data.data;
        const idx = state.messages.findIndex((m) => m.id === msg.id);
        if (idx >= 0) state.messages[idx] = msg;
        else state.messages.push(msg);
        renderMessages();
        if (msg.status === "completed" || msg.status === "error") {
          stopPolling();
          setSendingState(false);
          loadConversations();
          return;
        }
      } catch (e) { console.error("polling", e); }
      state.pollingId = setTimeout(tick, POLL_INTERVAL_MS);
    };
    state.pollingId = setTimeout(tick, POLL_INTERVAL_MS);
  }

  function stopPolling() {
    if (state.pollingId) {
      state.pollingAbort = true;
      clearTimeout(state.pollingId);
      state.pollingId = null;
    }
  }

  function handleStop() {
    stopPolling();
    setSendingState(false);
    if (state.messages.length > 0) {
      const lastMsg = state.messages[state.messages.length - 1];
      if (lastMsg && (lastMsg.status === "pending" || lastMsg.status === "processing")) {
        lastMsg.status = "error";
        lastMsg.content_partial = null;
        lastMsg.content = "Generacion detenida por el usuario.";
        lastMsg.error_message = "stopped_by_user";
        renderMessages();
      }
    }
  }

  function autoResizeTextarea() {
    const textarea = $("chat-input");
    if (!textarea) return;
    textarea.style.height = "auto";
    textarea.style.height = Math.min(textarea.scrollHeight, 120) + "px";
  }

  function updateSendButton() {
    const input = $("chat-input");
    const btn = $("btn-send");
    if (!input || !btn) return;
    if (input.disabled) return;
    btn.disabled = !input.value.trim();
  }

  function handleSuggestionClick(e) {
    const btn = e.target.closest(".llm-widget-suggestion");
    if (!btn) return;
    const q = btn.dataset.q;
    if (!q) return;
    if (!state.activeId) {
      createConversation().then(() => fillInput(q));
    } else {
      fillInput(q);
    }
  }

  function fillInput(q) {
    const input = $("chat-input");
    if (!input) return;
    input.value = q;
    input.focus();
    autoResizeTextarea();
    updateSendButton();
  }

  /* ============================================================
     BIND
     ============================================================ */

  function bind() {
    const bubble = $("llm-bubble");
    if (bubble) bubble.addEventListener("click", toggleWidget);

    const closeBtn = $("btn-close-widget");
    if (closeBtn) closeBtn.addEventListener("click", closeWidget);

    const minimizeBtn = $("btn-minimize-widget");
    if (minimizeBtn) minimizeBtn.addEventListener("click", closeWidget);

    const newBtn = $("btn-new-chat-widget");
    if (newBtn) newBtn.addEventListener("click", createConversation);

    const stopBtn = $("btn-stop");
    if (stopBtn) stopBtn.addEventListener("click", handleStop);

    const toggleListBtn = $("btn-toggle-list");
    const sidebar = $("llm-widget-sidebar");
    if (toggleListBtn && sidebar) {
      toggleListBtn.addEventListener("click", () => {
        sidebar.classList.toggle("collapsed");
        const isCollapsed = sidebar.classList.contains("collapsed");
        toggleListBtn.title = isCollapsed ? "Mostrar lista" : "Ocultar lista";
      });
    }

    document.addEventListener("click", (e) => {
      if (!state.open) return;
      const widget = $("llm-widget");
      const bubbleEl = $("llm-bubble");
      const backdrop = $("llm-widget-backdrop");
      if (widget && widget.contains(e.target)) return;
      if (bubbleEl && bubbleEl.contains(e.target)) return;
      if (backdrop && e.target === backdrop) {
        closeWidget();
        return;
      }
      if (window.innerWidth < 600) {
        closeWidget();
      }
    });

    const searchInput = $("chat-search-input");
    if (searchInput) {
      const onSearch = debounce((e) => {
        state.searchQuery = e.target.value;
        filterConversations();
        renderConversations();
      }, SEARCH_DEBOUNCE_MS);
      searchInput.addEventListener("input", onSearch);
    }

    const messagesContainer = $("chat-messages");
    if (messagesContainer) {
      messagesContainer.addEventListener("click", handleSuggestionClick);
      messagesContainer.addEventListener("click", handleFeedbackClick);  // HU-52.1
      messagesContainer.addEventListener("scroll", () => {
        if (isScrolledToBottom()) {
          const btnDown = $("btn-scroll-down");
          if (btnDown) btnDown.style.display = "none";
        }
      });
    }

    const btnDown = $("btn-scroll-down");
    if (btnDown) {
      btnDown.addEventListener("click", () => {
        btnDown.style.display = "none";
        scrollToBottom(true);
      });
    }

    const form = $("chat-form");
    if (form) {
      form.addEventListener("submit", (e) => {
        e.preventDefault();
        sendMessage();
      });
    }

    const textarea = $("chat-input");
    if (textarea) {
      textarea.addEventListener("keydown", (e) => {
        if (e.key === "Enter" && !e.shiftKey && !e.ctrlKey && !e.metaKey) {
          e.preventDefault();
          if (!textarea.disabled) sendMessage();
        }
      });
      textarea.addEventListener("input", () => {
        autoResizeTextarea();
        updateSendButton();
      });
    }

    document.addEventListener("keydown", (e) => {
      const isMod = e.ctrlKey || e.metaKey;
      if (isMod && e.key === "k") {
        e.preventDefault();
        createConversation();
      }
      if (e.key === "Escape" && state.open) {
        closeWidget();
      }
    });
  }

  document.addEventListener("DOMContentLoaded", () => {
    bind();
    loadConversations();
  });
})();