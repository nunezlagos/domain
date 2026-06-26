/* HU-49.3: cliente del chat widget estilo burbuja.

   Comportamiento:
   - Boton flotante visible en todas las paginas del admin
   - Click en burbuja -> abre/cierra popup
   - Click fuera o X -> cerrar
   - Ctrl+K abre nueva conversacion (y abre el popup si esta cerrado)
   - Sidebar colapsable dentro del popup
   - Sugerencias clickeables que completan el input
   - Polling cada 1.5s mientras el bot procesa
   - Typing indicator animado
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

  function openWidget() {
    const widget = $("llm-widget");
    if (!widget) return;
    state.open = true;
    widget.classList.add("open");
    widget.classList.remove("closing");
    clearUnread();
    setTimeout(() => {
      const input = $("chat-input");
      if (input) input.focus();
    }, 300);
  }

  function closeWidget() {
    const widget = $("llm-widget");
    if (!widget || !state.open) return;
    widget.classList.add("closing");
    setTimeout(() => {
      widget.classList.remove("open", "closing");
      state.open = false;
    }, 200);
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

  function renderConversations() {
    const list = $("chat-list");
    if (!list) return;
    const convs = state.filteredConversations;
    if (state.conversations.length === 0) {
      list.innerHTML = '<div class="llm-widget-list-empty">Aun no tienes conversaciones.</div>';
      return;
    }
    if (convs.length === 0) {
      list.innerHTML = '<div class="llm-widget-list-empty">Sin resultados.</div>';
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
      const preview = (c.last_message_preview || "").slice(0, 50);
      return `
        <div class="llm-widget-item${active}" data-id="${c.id}">
          <div class="llm-widget-item-title">${escapeHtml(title)}</div>
          ${preview ? `<div class="llm-widget-item-preview">${escapeHtml(preview)}</div>` : ""}
          <div class="llm-widget-item-time">${relativeTime(c.updated_at)}</div>
        </div>`;
    }).join("");
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

  function renderMessage(msg) {
    const isUser = msg.role === "user";
    const isPending = msg.status === "pending";
    const isProcessing = msg.status === "processing";
    const isError = msg.status === "error";
    const content = msg.content || msg.content_partial || "";

    if (isPending) {
      return `<div class="llm-widget-pending">
        <div class="llm-widget-spinner"></div>
        <span>Pensando...</span>
      </div>`;
    }
    if (isProcessing && !content) {
      return `<div class="llm-widget-pending">
        <div class="llm-widget-spinner"></div>
        <span>Generando respuesta...</span>
      </div>`;
    }

    const cls = isUser ? "llm-widget-msg-user" : isError ? "llm-widget-msg-assistant" : "llm-widget-msg-assistant";
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

    return `<div class="llm-widget-msg ${cls}">
      <div class="llm-widget-msg-avatar">${avatar}</div>
      <div class="llm-widget-msg-bubble${errorCls}">${html}${sourcesHtml}${meta}</div>
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

  function setSendingState(sending) {
    const input = $("chat-input");
    const btn = $("btn-send");
    if (!input || !btn) return;
    input.disabled = sending;
    btn.disabled = sending;
    if (sending) input.placeholder = "Esperando respuesta...";
    else input.placeholder = "Escribi tu pregunta...";
  }

  async function selectConversation(id) {
    if (state.pollingId) {
      state.pollingAbort = true;
      clearTimeout(state.pollingId);
      state.pollingId = null;
    }
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
    textarea.style.height = Math.min(textarea.scrollHeight, 120) + "px";
  }

  function handleSuggestionClick(e) {
    const btn = e.target.closest(".llm-widget-suggestion");
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
          updateSendButton();
        }
      });
    } else {
      const input = $("chat-input");
      if (input) {
        input.value = q;
        input.focus();
        autoResizeTextarea();
        updateSendButton();
      }
    }
  }

  function updateSendButton() {
    const input = $("chat-input");
    const btn = $("btn-send");
    if (!input || !btn) return;
    btn.disabled = !input.value.trim();
  }

  function bind() {
    const bubble = $("llm-bubble");
    if (bubble) bubble.addEventListener("click", toggleWidget);

    const closeBtn = $("btn-close-widget");
    if (closeBtn) closeBtn.addEventListener("click", closeWidget);

    const minimizeBtn = $("btn-minimize-widget");
    if (minimizeBtn) minimizeBtn.addEventListener("click", closeWidget);

    const newBtn = $("btn-new-chat-widget");
    if (newBtn) newBtn.addEventListener("click", createConversation);

    const toggleListBtn = $("btn-toggle-list");
    const sidebar = $("llm-widget-sidebar");
    if (toggleListBtn && sidebar) {
      toggleListBtn.addEventListener("click", () => {
        sidebar.classList.toggle("collapsed");
        const isCollapsed = sidebar.classList.contains("collapsed");
        toggleListBtn.innerHTML = isCollapsed
          ? '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 18l6-6-6-6"/></svg> Mostrar lista'
          : '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><path d="M15 18l-6-6 6-6"/></svg> Ocultar lista';
      });
    }

    document.addEventListener("click", (e) => {
      if (!state.open) return;
      const widget = $("llm-widget");
      const bubbleEl = $("llm-bubble");
      if (widget && widget.contains(e.target)) return;
      if (bubbleEl && bubbleEl.contains(e.target)) return;
      closeWidget();
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
          sendMessage();
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