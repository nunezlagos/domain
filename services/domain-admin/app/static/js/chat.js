/* HU-49.3: cliente del chat IA estilo NotebookLM.
   Vanilla JS + fetch + marked.js (local). Sin dependencias npm.

   Funcionalidad:
   - Sidebar: lista conversaciones del usuario, boton nueva, search local.
   - Crear/seleccionar/eliminar conversaciones.
   - Enviar mensaje con optimistic update.
   - Polling 1.5s que para solo en completed|error.
   - Render Markdown + SourceCards inline.
   - Mobile: back button para volver a la lista.
   - Scroll: badge "Nuevo mensaje" cuando user esta arriba y llega respuesta.
   - Typing indicator animado durante el polling.
*/

(function () {
  "use strict";

  const API = "/chat/api";
  const POLL_INTERVAL_MS = 1500;
  const SCROLL_BOTTOM_THRESHOLD = 100;
  const SEARCH_DEBOUNCE_MS = 200;
  const TYPING_FRAMES = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];

  const state = {
    conversations: [],
    filteredConversations: [],
    activeId: null,
    messages: [],
    pollingId: null,
    pollingAbort: false,
    searchQuery: "",
    typingFrameIdx: 0,
    typingIntervalId: null,
  };

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
    if (opts.method && opts.method !== "GET") {
      headers["Content-Type"] = "application/json";
    }
    const res = await fetch(API + path, {
      method: opts.method || "GET",
      headers,
      credentials: "same-origin",
      body: opts.body ? JSON.stringify(opts.body) : undefined,
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error("HTTP " + res.status + ": " + text);
    }
    if (res.status === 204) return null;
    return res.json();
  }

  function relativeTime(iso) {
    if (!iso) return "";
    const d = new Date(iso);
    const now = new Date();
    const diff = (now - d) / 1000;
    if (diff < 60) return "hace instantes";
    if (diff < 3600) return "hace " + Math.floor(diff / 60) + "m";
    if (diff < 86400) return "hace " + Math.floor(diff / 3600) + "h";
    if (diff < 604800) return "hace " + Math.floor(diff / 86400) + "d";
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

  function showNewBadge(show) {
    const badge = $("chat-new-badge");
    if (!badge) return;
    badge.style.display = show ? "block" : "none";
  }

  function startTypingAnimation() {
    stopTypingAnimation();
    const el = $("typing-indicator");
    if (!el) return;
    state.typingFrameIdx = 0;
    state.typingIntervalId = setInterval(() => {
      state.typingFrameIdx = (state.typingFrameIdx + 1) % TYPING_FRAMES.length;
      el.textContent = TYPING_FRAMES[state.typingFrameIdx];
    }, 100);
  }

  function stopTypingAnimation() {
    if (state.typingIntervalId) {
      clearInterval(state.typingIntervalId);
      state.typingIntervalId = null;
    }
  }

  function renderConversations() {
    const list = $("chat-list");
    const convs = state.filteredConversations;
    if (state.conversations.length === 0) {
      list.innerHTML = '<div class="chat-list-empty">Aun no tienes conversaciones. Crea una para empezar.</div>';
      return;
    }
    if (convs.length === 0) {
      list.innerHTML = '<div class="chat-list-empty">Sin resultados para la busqueda.</div>';
      return;
    }
    const titleCounts = {};
    convs.forEach((c) => {
      const k = (c.title || "Nueva conversacion").trim().toLowerCase();
      titleCounts[k] = (titleCounts[k] || 0) + 1;
    });
    list.innerHTML = convs
      .map((c) => {
        const active = c.id === state.activeId ? " active" : "";
        const baseTitle = (c.title || "Nueva conversacion").trim();
        const dupCount = titleCounts[baseTitle.toLowerCase()] || 0;
        const title = dupCount > 1 ? `${baseTitle} (${shortTime(c.created_at)})` : baseTitle;
        const preview = (c.last_message_preview || "").slice(0, 60);
        return `
          <div class="chat-list-item${active}" data-id="${c.id}">
            <div class="title">${escapeHtml(title)}</div>
            ${preview ? `<div class="preview">${escapeHtml(preview)}</div>` : ""}
            <div class="date">${relativeTime(c.updated_at)}</div>
          </div>
        `;
      })
      .join("");
    list.querySelectorAll(".chat-list-item").forEach((el) => {
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
    if (!q) {
      state.filteredConversations = state.conversations;
    } else {
      state.filteredConversations = state.conversations.filter((c) =>
        (c.title || "").toLowerCase().includes(q) ||
        (c.last_message_preview || "").toLowerCase().includes(q)
      );
    }
  }

  function debounce(fn, ms) {
    let timer;
    return function (...args) {
      clearTimeout(timer);
      timer = setTimeout(() => fn.apply(this, args), ms);
    };
  }

  function renderSources(sources) {
    if (!sources || sources.length === 0) return "";
    return (
      '<div class="sources">' +
      sources
        .map((s) => {
          const scorePct = Math.round((s.score || 0) * 100);
          const url = s.url || "#";
          const snippet = (s.snippet || "").slice(0, 80);
          return `<a class="source-chip" href="${escapeHtml(url)}" target="_blank" rel="noopener" title="${escapeHtml(snippet)}">
            <span class="source-title">${escapeHtml(s.titulo || s.id)}</span>
            ${scorePct > 0 ? `<span class="score">${scorePct}%</span>` : ""}
          </a>`;
        })
        .join("") +
      "</div>"
    );
  }

  function renderMessage(msg) {
    const isUser = msg.role === "user";
    const isPending = msg.status === "pending";
    const isProcessing = msg.status === "processing";
    const isError = msg.status === "error";
    const content = msg.content || msg.content_partial || "";

    if (isPending) {
      return `<div class="chat-pending">
        <span class="typing-spinner" id="typing-indicator">${TYPING_FRAMES[0]}</span>
        <span>Pensando...</span>
      </div>`;
    }
    if (isProcessing && !content) {
      return `<div class="chat-pending">
        <span class="typing-spinner" id="typing-indicator">${TYPING_FRAMES[0]}</span>
        <span>Generando respuesta...</span>
      </div>`;
    }
    if (isProcessing && content) {
      const processedHtml = renderMarkdown(content);
      return `<div class="chat-bubble assistant processing">
        <div class="bubble-content">${processedHtml}</div>
        <div class="typing-footer">
          <span class="typing-spinner" id="typing-indicator">${TYPING_FRAMES[0]}</span>
          <span>generando...</span>
        </div>
      </div>`;
    }
    const bubbleClass = isUser ? "user" : isError ? "assistant error" : "assistant";
    let html;
    if (isUser) {
      html = escapeHtml(content).replace(/\n/g, "<br>");
    } else {
      html = renderMarkdown(content);
    }
    const sourcesHtml = isError ? "" : renderSources(msg.sources);
    const time = msg.created_at ? new Date(msg.created_at).toLocaleTimeString() : "";
    const meta = isUser
      ? `<div class="meta">${time}</div>`
      : (msg.tokens_in || msg.tokens_out)
      ? `<div class="meta">${time} · ${escapeHtml(msg.model || "")} · ${msg.tokens_in || 0}/${msg.tokens_out || 0} tokens</div>`
      : time
      ? `<div class="meta">${time}</div>`
      : "";
    return `<div class="chat-bubble ${bubbleClass} fade-in">${html}${sourcesHtml}${meta}</div>`;
  }

  function renderMarkdown(text) {
    if (typeof window.marked === "undefined") {
      return escapeHtml(text).replace(/\n/g, "<br>");
    }
    try {
      return window.marked.parse(text, { breaks: true, gfm: true });
    } catch (e) {
      return escapeHtml(text);
    }
  }

  function renderMessages(opts) {
    const container = $("chat-messages");
    if (!state.activeId) {
      container.innerHTML = "";
      const empty = $("chat-empty");
      if (empty) container.appendChild(empty);
      stopTypingAnimation();
      return;
    }
    const form = $("chat-form");
    form.style.display = "flex";
    const empty = $("chat-empty");
    if (empty) empty.style.display = "none";
    const wasAtBottom = isScrolledToBottom();
    container.innerHTML = state.messages.map(renderMessage).join("");
    if (opts && opts.initial) {
      scrollToBottom(false);
    } else if (wasAtBottom) {
      scrollToBottom(true);
    } else {
      showNewBadge(true);
    }
    const hasProcessing = state.messages.some(
      (m) => m.status === "pending" || m.status === "processing"
    );
    if (hasProcessing) startTypingAnimation();
    else stopTypingAnimation();
  }

  async function loadConversations() {
    try {
      const data = await apiFetch("/conversations");
      state.conversations = data.data || [];
      filterConversations();
      renderConversations();
    } catch (e) {
      console.error("loadConversations", e);
    }
  }

  async function createConversation() {
    try {
      const data = await apiFetch("/conversations/new", { method: "POST", body: {} });
      const conv = data.data;
      state.conversations.unshift(conv);
      filterConversations();
      renderConversations();
      await selectConversation(conv.id);
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

  function updateHeader() {
    const conv = state.conversations.find((c) => c.id === state.activeId);
    $("chat-header-title").textContent = conv ? conv.title || "Nueva conversacion" : "Chat IA";
    const badge = $("chat-header-badge");
    const pending = state.messages.find((m) => m.status === "pending" || m.status === "processing");
    if (pending) {
      badge.style.display = "inline-block";
      badge.textContent = pending.status === "pending" ? "esperando" : "generando";
    } else {
      badge.style.display = "none";
    }
  }

  function setSendingState(sending) {
    const input = $("chat-input");
    const btn = $("btn-send");
    if (!input || !btn) return;
    input.disabled = sending;
    btn.disabled = sending;
    if (sending) {
      input.placeholder = "Esperando respuesta del asistente...";
    } else {
      input.placeholder = "Escribi tu pregunta y presiona Enter (Shift+Enter para nueva linea)";
      input.focus();
    }
  }

  async function selectConversation(id) {
    if (state.pollingId) {
      state.pollingAbort = true;
      clearTimeout(state.pollingId);
      state.pollingId = null;
    }
    stopTypingAnimation();
    state.activeId = id;
    state.messages = [];
    showNewBadge(false);
    document.querySelectorAll(".chat-list-item").forEach((el) => {
      el.classList.toggle("active", el.dataset.id === id);
    });
    document.getElementById("chat-shell").classList.add("show-chat");
    await loadMessages(id);
    updateHeader();
  }

  function goBackToList() {
    if (state.pollingId) {
      state.pollingAbort = true;
      clearTimeout(state.pollingId);
      state.pollingId = null;
    }
    stopTypingAnimation();
    state.activeId = null;
    state.messages = [];
    document.getElementById("chat-shell").classList.remove("show-chat");
    renderMessages();
    updateHeader();
  }

  async function sendMessage() {
    const input = $("chat-input");
    const text = input.value.trim();
    if (!text || !state.activeId) return;
    input.value = "";
    input.style.height = "auto";
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
    showNewBadge(false);
    try {
      const data = await apiFetch(
        "/conversations/" + state.activeId + "/messages/new",
        { method: "POST", body: { content: text } }
      );
      const serverMsg = data.data;
      const idx = state.messages.findIndex((m) => m.id === tempMsg.id);
      if (idx >= 0) state.messages[idx] = serverMsg;
      renderMessages();
      updateHeader();
      startPolling(serverMsg.id);
    } catch (e) {
      console.error("sendMessage", e);
      alert("Error enviando mensaje: " + e.message);
      setSendingState(false);
    }
  }

  function startPolling(messageId) {
    state.pollingAbort = false;
    startTypingAnimation();
    const tick = async () => {
      if (state.pollingAbort) return;
      try {
        const data = await apiFetch("/messages/" + messageId);
        const msg = data.data;
        const idx = state.messages.findIndex((m) => m.id === msg.id);
        if (idx >= 0) state.messages[idx] = msg;
        else state.messages.push(msg);
        renderMessages();
        updateHeader();
        if (msg.status === "completed" || msg.status === "error") {
          state.pollingId = null;
          stopTypingAnimation();
          setSendingState(false);
          loadConversations();
          return;
        }
      } catch (e) {
        console.error("polling", e);
      }
      state.pollingId = setTimeout(tick, POLL_INTERVAL_MS);
    };
    state.pollingId = setTimeout(tick, POLL_INTERVAL_MS);
  }

  function autoResizeTextarea() {
    const textarea = $("chat-input");
    if (!textarea) return;
    textarea.style.height = "auto";
    textarea.style.height = Math.min(textarea.scrollHeight, 120) + "px";
  }

  function bind() {
    $("btn-new-conversation").addEventListener("click", createConversation);
    const btnEmpty = $("btn-empty-new");
    if (btnEmpty) btnEmpty.addEventListener("click", createConversation);

    const btnBack = $("btn-back-mobile");
    if (btnBack) btnBack.addEventListener("click", goBackToList);

    const btnScrollDown = $("btn-scroll-down");
    if (btnScrollDown) {
      btnScrollDown.addEventListener("click", () => {
        showNewBadge(false);
        scrollToBottom(true);
      });
    }

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
      messagesContainer.addEventListener("scroll", () => {
        if (isScrolledToBottom()) showNewBadge(false);
      });
    }

    const form = $("chat-form");
    form.addEventListener("submit", (e) => {
      e.preventDefault();
      sendMessage();
    });

    const textarea = $("chat-input");
    textarea.addEventListener("keydown", (e) => {
      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        sendMessage();
      }
    });
    textarea.addEventListener("input", autoResizeTextarea);
  }

  document.addEventListener("DOMContentLoaded", () => {
    bind();
    loadConversations();
    renderMessages();
  });
})();