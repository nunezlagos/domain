/* HU-49.3: cliente del chat IA estilo LLM moderno (ChatGPT/Claude).

   Funcionalidad:
   - Sidebar colapsable (toggle con Ctrl+B o boton hamburguesa)
   - Command palette (Ctrl+K) para nueva conversacion
   - Busqueda con debounce en sidebar
   - Mobile: sidebar overlay con backdrop
   - Sugerencias de bienvenida (click llena el input)
   - Scroll inteligente + badge "ir al fondo"
   - Polling cada 1.5s mientras el bot procesa
   - Typing indicator animado (spinner CSS)
   - Fade-in animation para mensajes nuevos
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
    sidebarCollapsed: window.innerWidth >= 1024 ? false : true,
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

  function isMobile() {
    return window.innerWidth < 768;
  }

  function setSidebarCollapsed(collapsed) {
    state.sidebarCollapsed = collapsed;
    const shell = $("llm-shell");
    if (isMobile()) {
      shell.classList.toggle("mobile-open", !collapsed);
    } else {
      shell.classList.toggle("collapsed", collapsed);
    }
  }

  function renderConversations() {
    const list = $("chat-list");
    const convs = state.filteredConversations;
    if (state.conversations.length === 0) {
      list.innerHTML = '<div class="llm-list-empty">Aun no tienes conversaciones.<br>Pulsa <kbd>Ctrl</kbd>+<kbd>K</kbd> o el boton "+ Nueva".</div>';
      return;
    }
    if (convs.length === 0) {
      list.innerHTML = '<div class="llm-list-empty">Sin resultados.</div>';
      return;
    }
    const titleCounts = {};
    convs.forEach((c) => {
      const k = (c.title || "Nueva conversacion").trim().toLowerCase();
      titleCounts[k] = (titleCounts[k] || 0) + 1;
    });
    list.innerHTML = convs.map((c) => {
      const active = c.id === state.activeId ? " active" : "";
      const baseTitle = (c.title || "Nueva conversacion").trim();
      const dupCount = titleCounts[baseTitle.toLowerCase()] || 0;
      const title = dupCount > 1 ? `${baseTitle} (${shortTime(c.created_at)})` : baseTitle;
      const preview = (c.last_message_preview || "").slice(0, 60);
      return `
        <div class="llm-item${active}" data-id="${c.id}">
          <div class="llm-item-title">${escapeHtml(title)}</div>
          ${preview ? `<div class="llm-item-preview">${escapeHtml(preview)}</div>` : ""}
          <div class="llm-item-time">${relativeTime(c.updated_at)}</div>
        </div>`;
    }).join("");
    list.querySelectorAll(".llm-item").forEach((el) => {
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

  function renderSources(sources) {
    if (!sources || sources.length === 0) return "";
    return (
      '<div class="llm-msg-sources">' +
      sources.map((s) => {
        const scorePct = Math.round((s.score || 0) * 100);
        const url = s.url || "#";
        return `<a class="llm-source" href="${escapeHtml(url)}" target="_blank" rel="noopener">
          ${escapeHtml(s.titulo || s.id)} ${scorePct > 0 ? `<span class="llm-source-score">${scorePct}%</span>` : ""}
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

  function renderMessage(msg) {
    const isUser = msg.role === "user";
    const isPending = msg.status === "pending";
    const isProcessing = msg.status === "processing";
    const isError = msg.status === "error";
    const content = msg.content || msg.content_partial || "";

    if (isPending) {
      return `<div class="llm-pending">
        <div class="llm-msg-avatar" style="background:var(--llm-primary);color:white">B</div>
        <div class="llm-spinner"></div>
        <span>Pensando...</span>
      </div>`;
    }
    if (isProcessing && !content) {
      return `<div class="llm-pending">
        <div class="llm-msg-avatar" style="background:var(--llm-primary);color:white">B</div>
        <div class="llm-spinner"></div>
        <span>Generando respuesta...</span>
      </div>`;
    }

    const cls = isUser ? "llm-msg-user" : isError ? "llm-msg-assistant llm-error" : "llm-msg-assistant";
    const avatar = isUser ? "U" : "B";
    const avatarBg = isUser ? "var(--llm-text-muted)" : "var(--llm-primary)";
    const avatarColor = isUser ? "var(--llm-bg-elevated)" : "white";

    let html;
    if (isUser) {
      html = escapeHtml(content).replace(/\n/g, "<br>");
    } else {
      html = renderMarkdown(content);
    }
    const sourcesHtml = isError ? "" : renderSources(msg.sources);
    const time = msg.created_at ? new Date(msg.created_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }) : "";
    const metaParts = [];
    if (time) metaParts.push(time);
    if (msg.model) metaParts.push(escapeHtml(msg.model));
    if (msg.tokens_in || msg.tokens_out) metaParts.push(`${msg.tokens_in || 0} / ${msg.tokens_out || 0} tokens`);
    const meta = metaParts.length ? `<div class="llm-msg-meta">${metaParts.join('<span class="llm-msg-meta-sep">·</span>')}</div>` : "";

    return `<div class="llm-msg ${cls}">
      <div class="llm-msg-avatar" style="background:${avatarBg};color:${avatarColor}">${avatar}</div>
      <div class="llm-msg-content">${html}${sourcesHtml}${meta}</div>
    </div>`;
  }

  function renderMessages(opts) {
    const container = $("chat-messages");
    if (!state.activeId) {
      container.innerHTML = "";
      const welcome = $("chat-empty");
      if (welcome) container.appendChild(welcome);
      return;
    }
    const form = $("chat-form");
    form.style.display = "block";
    const welcome = $("chat-empty");
    if (welcome) welcome.remove();
    const wasAtBottom = isScrolledToBottom();
    container.innerHTML = state.messages.map(renderMessage).join("");
    const btnDown = $("btn-scroll-down");
    if (opts && opts.initial) {
      scrollToBottom(false);
      if (btnDown) btnDown.style.display = "none";
    } else if (wasAtBottom) {
      scrollToBottom(true);
      if (btnDown) btnDown.style.display = "none";
    } else {
      if (btnDown) btnDown.style.display = "flex";
    }
  }

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
      if (isMobile()) setSidebarCollapsed(true);
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
      input.placeholder = "Esperando respuesta del bot...";
    } else {
      input.placeholder = "Preguntale algo al bot...";
      input.focus();
    }
  }

  async function selectConversation(id) {
    if (state.pollingId) {
      state.pollingAbort = true;
      clearTimeout(state.pollingId);
      state.pollingId = null;
    }
    state.activeId = id;
    state.messages = [];
    document.querySelectorAll(".llm-item").forEach((el) => {
      el.classList.toggle("active", el.dataset.id === id);
    });
    if (isMobile()) setSidebarCollapsed(true);
    await loadMessages(id);
    updateHeader();
  }

  async function sendMessage() {
    const input = $("chat-input");
    const text = input.value.trim();
    if (!text || !state.activeId) return;
    input.value = "";
    autoResizeTextarea();
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
          setSendingState(false);
          loadConversations();
          return;
        }
      } catch (e) { console.error("polling", e); }
      state.pollingId = setTimeout(tick, POLL_INTERVAL_MS);
    };
    state.pollingId = setTimeout(tick, POLL_INTERVAL_MS);
  }

  function autoResizeTextarea() {
    const textarea = $("chat-input");
    if (!textarea) return;
    textarea.style.height = "auto";
    textarea.style.height = Math.min(textarea.scrollHeight, 200) + "px";
  }

  function handleSuggestionClick(e) {
    const btn = e.target.closest(".llm-suggestion");
    if (!btn) return;
    const q = btn.dataset.q;
    if (!q) return;
    if (!state.activeId) {
      createConversation().then(() => {
        const input = $("chat-input");
        if (input) {
          input.value = q;
          input.focus();
          autoResizeTextarea();
        }
      });
    } else {
      const input = $("chat-input");
      if (input) {
        input.value = q;
        input.focus();
        autoResizeTextarea();
      }
    }
  }

  function bind() {
    $("btn-new-chat").addEventListener("click", createConversation);

    $("btn-sidebar-toggle").addEventListener("click", () => {
      setSidebarCollapsed(!state.sidebarCollapsed);
    });
    $("btn-sidebar-open").addEventListener("click", () => {
      setSidebarCollapsed(false);
    });

    const backdrop = $("llm-backdrop");
    if (backdrop) backdrop.addEventListener("click", () => setSidebarCollapsed(true));

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

    const messages = $("chat-messages");
    if (messages) messages.addEventListener("click", handleSuggestionClick);

    const form = $("chat-form");
    form.addEventListener("submit", (e) => {
      e.preventDefault();
      sendMessage();
    });

    const textarea = $("chat-input");
    textarea.addEventListener("keydown", (e) => {
      if (e.key === "Enter" && !e.shiftKey && !e.ctrlKey && !e.metaKey) {
        e.preventDefault();
        sendMessage();
      }
    });
    textarea.addEventListener("input", autoResizeTextarea);

    document.addEventListener("keydown", (e) => {
      const isMod = e.ctrlKey || e.metaKey;
      if (isMod && e.key === "b") {
        e.preventDefault();
        setSidebarCollapsed(!state.sidebarCollapsed);
      }
      if (isMod && e.key === "k") {
        e.preventDefault();
        createConversation();
      }
      if (e.key === "Escape" && !state.sidebarCollapsed && isMobile()) {
        setSidebarCollapsed(true);
      }
    });

    window.addEventListener("resize", () => {
      if (isMobile()) {
        $("llm-shell").classList.remove("collapsed");
      }
    });
  }

  document.addEventListener("DOMContentLoaded", () => {
    bind();
    loadConversations();
    renderMessages();
    setTimeout(() => {
      const input = $("chat-input");
      if (input && !state.activeId) input.focus();
    }, 200);
  });
})();