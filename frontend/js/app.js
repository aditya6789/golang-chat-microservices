import {
  api,
  clearTokens,
  getAccess,
  getBase,
  jwtSub,
  setBase,
  setTokens,
  wsURL,
} from "./api.js";

/** @type {WebSocket | null} */
let socket = null;
let activeChatId = null;
/** @type {Array<{id?: string, sender_id: string, content: string, created_at?: string, _ts?: number}>} */
let messages = [];
let typingClearTimer = null;
let heartbeatTimer = null;
let typingDebounce = null;
let typingBurstOpen = false;

const $ = (sel, root = document) => root.querySelector(sel);
const $$ = (sel, root = document) => [...root.querySelectorAll(sel)];

function normId(s) {
  return String(s || "")
    .trim()
    .toLowerCase()
    .replace(/-/g, "");
}

function showView(name) {
  $$(".view").forEach((v) => v.classList.toggle("hidden", v.dataset.view !== name));
}

function fmtTime(iso) {
  if (!iso) return "";
  try {
    const d = new Date(iso);
    return d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
  } catch {
    return "";
  }
}

function chatLabel(c) {
  if (c.type === "group" && c.name) return c.name;
  if (c.type === "direct") return "Direct · " + (c.id || "").slice(0, 8) + "…";
  return (c.type || "Chat") + " · " + (c.id || "").slice(0, 8);
}

async function loadChats() {
  const r = await api("GET", "/chats");
  const list = $("#chatList");
  list.innerHTML = "";
  if (!r.ok || !r.data?.items) {
    list.innerHTML = `<li class="chat-item empty">No chats yet</li>`;
    return;
  }
  r.data.items.forEach((c) => {
    const li = document.createElement("li");
    li.className = "chat-item" + (c.id === activeChatId ? " active" : "");
    li.dataset.id = c.id;
    li.innerHTML = `<span class="chat-title">${escapeHtml(chatLabel(c))}</span><span class="chat-sub">${c.type}</span>`;
    li.addEventListener("click", () => openChat(c.id, c));
    list.appendChild(li);
  });
}

function escapeHtml(s) {
  const d = document.createElement("div");
  d.textContent = s;
  return d.innerHTML;
}

async function openChat(chatId, meta) {
  activeChatId = String(chatId || "").trim();
  $$(".chat-item").forEach((el) => {
    el.classList.toggle("active", el.dataset.id === chatId);
  });
  $("#threadEmpty").classList.add("hidden");
  $("#threadPanel").classList.remove("hidden");
  $("#threadTitle").textContent = meta ? chatLabel(meta) : "Chat";
  $("#threadSubtitle").textContent = chatId;
  messages = [];
  $("#typingBar").classList.add("hidden");
  $("#typingBar").textContent = "";

  const hist = await api(
    "GET",
    "/messages/" + encodeURIComponent(chatId) + "?limit=100&offset=0"
  );
  if (hist.ok && hist.data?.items) {
    messages = [...hist.data.items].reverse();
  }
  renderMessages();
  connectWS();
}

function renderMessages() {
  const box = $("#messageList");
  box.innerHTML = "";
  const me = jwtSub(getAccess());
  messages.forEach((m) => {
    const mine = m.sender_id === me;
    const div = document.createElement("div");
    div.className = "bubble-row " + (mine ? "mine" : "theirs");
    const t = m.created_at ? fmtTime(m.created_at) : "";
    div.innerHTML = `
      <div class="bubble">
        <div class="bubble-meta">${mine ? "You" : escapeHtml((m.sender_id || "").slice(0, 8) + "…")} · ${t}</div>
        <div class="bubble-text">${escapeHtml(m.content || "")}</div>
      </div>`;
    box.appendChild(div);
  });
  box.scrollTop = box.scrollHeight;
}

function pushMessage(m) {
  const me = jwtSub(getAccess());
  if (m.message_id && messages.some((x) => x.id === m.message_id)) return;
  if (
    m.id &&
    messages.some((x) => x.id === m.id)
  )
    return;
  const content = m.content || "";
  const sender = m.sender_id || "";
  const recent = messages.slice(-5);
  const dup = recent.some(
    (x) =>
      x.sender_id === sender &&
      x.content === content &&
      (!m.at || !x._ts || Math.abs(new Date(m.at).getTime() - x._ts) < 4000)
  );
  if (dup) return;

  messages.push({
    id: m.message_id || m.id,
    sender_id: sender,
    content,
    created_at: m.at || new Date().toISOString(),
    _ts: m.at ? new Date(m.at).getTime() : Date.now(),
  });
  renderMessages();
}

function connectWS() {
  disconnectWS();
  if (!activeChatId || !getAccess()) return;
  const url = wsURL(activeChatId);
  socket = new WebSocket(url);
  $("#wsDot").className = "ws-dot connecting";
  socket.onopen = () => {
    $("#wsDot").className = "ws-dot on";
  };
  socket.onclose = () => {
    $("#wsDot").className = "ws-dot off";
    socket = null;
  };
  socket.onerror = () => {
    $("#wsDot").className = "ws-dot off";
  };
  socket.onmessage = (ev) => {
    let p;
    try {
      p = JSON.parse(ev.data);
    } catch {
      return;
    }
    if (p.chat_id && normId(p.chat_id) !== normId(activeChatId)) return;
    if (p.type === "typing") {
      const me = (jwtSub(getAccess()) || "").trim().toLowerCase();
      const them = (p.sender_id || "").trim().toLowerCase();
      if (them && them !== me) {
        const bar = $("#typingBar");
        bar.textContent = "Typing…";
        bar.dataset.peer = them;
        bar.classList.remove("hidden");
        bar.classList.add("typing-live");
        clearTimeout(typingClearTimer);
        typingClearTimer = setTimeout(() => {
          bar.classList.add("hidden");
          bar.classList.remove("typing-live");
        }, 3500);
      }
      return;
    }
    if (p.type === "message" && p.content) {
      pushMessage(p);
      return;
    }
    if (p.type === "read_receipt") {
      /* optional: could show read ticks later */
    }
  };
}

function disconnectWS() {
  if (socket) {
    try {
      socket.close();
    } catch (_) {}
    socket = null;
  }
  $("#wsDot").className = "ws-dot off";
}

async function sendText() {
  const input = $("#composerInput");
  const text = (input.value || "").trim();
  if (!text || !activeChatId) return;
  input.value = "";

  const payload = JSON.stringify({ type: "message", content: text });
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(payload);
    return;
  }
  const idem =
    (globalThis.crypto?.randomUUID?.() ||
      "web-" + Date.now() + "-" + Math.random().toString(36).slice(2));
  const r = await api("POST", "/messages", {
    chat_id: activeChatId,
    content: text,
    idempotency_key: idem,
  });
  if (r.ok && r.data) {
    messages.push({
      id: r.data.id,
      sender_id: r.data.sender_id,
      content: r.data.content,
      created_at: r.data.created_at,
    });
    renderMessages();
  }
}

function emitTyping() {
  if (!socket || socket.readyState !== WebSocket.OPEN || !activeChatId) return;
  try {
    socket.send(
      JSON.stringify({ type: "typing", chat_id: activeChatId })
    );
  } catch (_) {}
}

function sendTyping() {
  if (!socket || socket.readyState !== WebSocket.OPEN || !activeChatId) return;
  if (!typingBurstOpen) {
    typingBurstOpen = true;
    emitTyping();
    setTimeout(() => {
      typingBurstOpen = false;
    }, 2500);
  }
  clearTimeout(typingDebounce);
  typingDebounce = setTimeout(emitTyping, 180);
}

async function loadMe() {
  const sub = jwtSub(getAccess());
  if (!sub) return;
  const r = await api("GET", "/users/" + encodeURIComponent(sub));
  if (r.ok && r.data) {
    const un = r.data.username || r.data.email || "User";
    $("#sidebarUser").textContent = r.data.username || r.data.email || sub.slice(0, 8);
    $("#sidebarUserSub").textContent = r.data.email || "";
    const av = $("#sidebarAvatar");
    if (av) av.textContent = un.slice(0, 1).toUpperCase();
  } else {
    $("#sidebarUser").textContent = "Account";
    $("#sidebarUserSub").textContent = sub.slice(0, 13) + "…";
    const av = $("#sidebarAvatar");
    if (av) av.textContent = "U";
  }
}

function startHeartbeat() {
  stopHeartbeat();
  heartbeatTimer = setInterval(async () => {
    const sub = jwtSub(getAccess());
    if (!sub) return;
    await api("POST", "/users/" + encodeURIComponent(sub) + "/heartbeat", {});
  }, 45000);
}

function stopHeartbeat() {
  if (heartbeatTimer) {
    clearInterval(heartbeatTimer);
    heartbeatTimer = null;
  }
}

function openModal(id) {
  $(`#${id}`).classList.add("open");
}

function closeModals() {
  $$(".modal").forEach((m) => m.classList.remove("open"));
}

async function submitLogin(e) {
  e.preventDefault();
  const email = $("#loginEmail").value.trim();
  const password = $("#loginPass").value;
  const r = await api("POST", "/auth/login", { email, password });
  if (!r.ok) {
    $("#authError").textContent =
      typeof r.data?.error === "string"
        ? r.data.error
        : r.data?.error
          ? JSON.stringify(r.data.error)
          : "Login failed";
    return;
  }
  setTokens(r.data.access_token, r.data.refresh_token);
  $("#authError").textContent = "";
  enterApp();
}

async function submitRegister(e) {
  e.preventDefault();
  const email = $("#regEmail").value.trim();
  const username = $("#regUser").value.trim();
  const password = $("#regPass").value;
  const r = await api("POST", "/auth/register", {
    email,
    username,
    password,
  });
  if (!r.ok) {
    $("#authError").textContent =
      typeof r.data?.error === "string"
        ? r.data.error
        : r.data?.error
          ? JSON.stringify(r.data.error)
          : "Register failed";
    return;
  }
  setTokens(r.data.access_token, r.data.refresh_token);
  $("#authError").textContent = "";
  enterApp();
}

function enterApp() {
  showView("main");
  loadMe();
  loadChats();
  startHeartbeat();
}

function leaveApp() {
  stopHeartbeat();
  disconnectWS();
  clearTokens();
  activeChatId = null;
  messages = [];
  $("#threadPanel").classList.add("hidden");
  $("#threadEmpty").classList.remove("hidden");
  $("#sidebar").classList.remove("open");
  showView("auth");
}

function initAuthTabs() {
  $$(".auth-tab").forEach((btn) => {
    btn.addEventListener("click", () => {
      const tab = btn.dataset.tab;
      $$(".auth-tab").forEach((b) => b.classList.toggle("active", b === btn));
      $$(".auth-panel").forEach((p) =>
        p.classList.toggle("hidden", p.dataset.panel !== tab)
      );
    });
  });
}

export function init() {
  $("#apiBaseField").value = getBase();

  initAuthTabs();

  if (getAccess() && jwtSub(getAccess())) {
    enterApp();
  } else {
    showView("auth");
  }

  $("#formLogin").addEventListener("submit", submitLogin);
  $("#formRegister").addEventListener("submit", submitRegister);
  $("#btnLogout").addEventListener("click", leaveApp);

  $("#btnSaveApi").addEventListener("click", () => {
    setBase($("#apiBaseField").value);
    closeModals();
  });

  $("#btnOpenSettings").addEventListener("click", () => openModal("modalSettings"));
  $("#btnOpenNewDirect").addEventListener("click", () => openModal("modalDirect"));
  $("#btnOpenNewGroup").addEventListener("click", () => openModal("modalGroup"));
  $("#btnRefreshChats").addEventListener("click", loadChats);

  $$("[data-close-modal]").forEach((el) =>
    el.addEventListener("click", closeModals)
  );

  $("#formDirect").addEventListener("submit", async (e) => {
    e.preventDefault();
    const other = $("#directOtherId").value.trim();
    const r = await api("POST", "/chats", { type: "direct", member_ids: [other] });
    if (!r.ok) {
      alert((r.data && r.data.error) || "Could not create chat");
      return;
    }
    closeModals();
    $("#directOtherId").value = "";
    await loadChats();
    if (r.data.id) {
      await openChat(r.data.id, { id: r.data.id, type: "direct" });
    }
  });

  $("#formGroup").addEventListener("submit", async (e) => {
    e.preventDefault();
    const name = $("#groupNameInput").value.trim() || "Group";
    const raw = $("#groupMembersInput").value.trim();
    const member_ids = raw
      ? raw.split(",").map((s) => s.trim()).filter(Boolean)
      : [];
    const r = await api("POST", "/chats", {
      type: "group",
      name,
      member_ids,
    });
    if (!r.ok) {
      alert((r.data && r.data.error) || "Could not create group");
      return;
    }
    closeModals();
    $("#groupNameInput").value = "";
    $("#groupMembersInput").value = "";
    await loadChats();
    if (r.data.id) {
      await openChat(r.data.id, { id: r.data.id, type: "group", name });
    }
  });

  $("#btnSend").addEventListener("click", () => sendText());
  $("#composerInput").addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendText();
      return;
    }
    if (e.key === "Backspace" || e.key.length === 1) {
      sendTyping();
    }
  });
  $("#composerInput").addEventListener("input", sendTyping);

  $("#btnAddMember").addEventListener("click", async () => {
    if (!activeChatId) return;
    const uid = prompt("User UUID to add");
    if (!uid) return;
    const r = await api(
      "POST",
      "/chats/" + encodeURIComponent(activeChatId) + "/members",
      { user_id: uid.trim() }
    );
    if (!r.ok) alert((r.data && r.data.error) || "Failed");
    else loadChats();
  });

  $("#btnThreadMenu").addEventListener("click", () => {
    $("#threadMenu").classList.toggle("open");
  });
  document.addEventListener("click", (e) => {
    if (!e.target.closest(".thread-menu-wrap")) $("#threadMenu").classList.remove("open");
  });

  $("#btnSidebarToggle").addEventListener("click", () => {
    $("#sidebar").classList.toggle("open");
  });
  $(".main-pane").addEventListener("click", () => {
    if (window.matchMedia("(max-width: 900px)").matches) {
      $("#sidebar").classList.remove("open");
    }
  });
}

init();
