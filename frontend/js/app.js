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
/** @type {{ type?: string, name?: string } | null} */
let activeChatMeta = null;
/** @type {string | null} other participant in active direct chat (for read ticks) */
let directPeerUserId = null;

/** @type {Array<{id?: string, sender_id: string, content: string, created_at?: string, _ts?: number, _pending?: boolean, read_by?: Array<{user_id: string, read_at?: string}>}>} */
let messages = [];
let typingClearTimer = null;
let heartbeatTimer = null;
let typingDebounce = null;
let typingBurstOpen = false;

/** @type {Record<string, string>} chatId -> display label for direct chats */
const peerLabelCache = {};
let directSearchTimer = null;

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

/** Receipt ticks for outgoing bubbles only (HTML). */
function receiptMarkup(m) {
  const me = jwtSub(getAccess());
  if (m._pending || !m.id || String(m.id).startsWith("pending-")) {
    return ` <span class="msg-rcpt pending" title="Sending">…</span>`;
  }
  const others = (m.read_by || []).filter((r) => r.user_id && r.user_id !== me);
  const isDirect = activeChatMeta?.type === "direct";
  const read =
    isDirect && directPeerUserId
      ? others.some((r) => r.user_id === directPeerUserId)
      : others.length > 0;
  const title = read ? "Read" : "Delivered";
  const sym = read ? "✓✓" : "✓";
  const cls = read ? "msg-rcpt read" : "msg-rcpt delivered";
  return ` <span class="${cls}" title="${escapeHtml(title)}">${sym}</span>`;
}

function acknowledgePeerReads() {
  if (!socket || socket.readyState !== WebSocket.OPEN) return;
  const me = jwtSub(getAccess());
  for (const m of messages) {
    if (m.sender_id === me || !m.id || String(m.id).startsWith("pending-")) continue;
    try {
      socket.send(JSON.stringify({ type: "read_receipt", message_id: m.id }));
    } catch (_) {}
  }
}

function chatLabel(c) {
  if (c.type === "group" && c.name) return c.name;
  if (c.type === "direct") {
    if (c.id && peerLabelCache[c.id]) return peerLabelCache[c.id];
    return "Direct · " + (c.id || "").slice(0, 8) + "…";
  }
  return (c.type || "Chat") + " · " + (c.id || "").slice(0, 8);
}

async function enrichDirectLabels(chats) {
  const me = jwtSub(getAccess());
  const directs = (chats || []).filter((c) => c.type === "direct" && c.id && !peerLabelCache[c.id]);
  await Promise.all(
    directs.map(async (c) => {
      const m = await api("GET", "/chats/" + encodeURIComponent(c.id) + "/members");
      if (!m.ok || !m.data?.items) return;
      const peer = m.data.items.find((x) => x.user_id !== me);
      if (peer?.username) peerLabelCache[c.id] = peer.username;
      else if (peer?.email) peerLabelCache[c.id] = peer.email;
    })
  );
}

async function loadChats() {
  const r = await api("GET", "/chats");
  const list = $("#chatList");
  list.innerHTML = "";
  if (!r.ok || !r.data?.items) {
    list.innerHTML = `<li class="chat-item empty">No chats yet</li>`;
    return;
  }
  await enrichDirectLabels(r.data.items);
  r.data.items.forEach((c) => {
    const li = document.createElement("li");
    li.className = "chat-item" + (c.id === activeChatId ? " active" : "");
    li.dataset.id = c.id;
    li.innerHTML = `<span class="chat-title">${escapeHtml(chatLabel(c))}</span><span class="chat-sub">${c.type}</span>`;
    li.addEventListener("click", () => openChat(c.id, c));
    list.appendChild(li);
  });
}

async function loadFriends() {
  const ul = $("#friendListSidebar");
  const empty = $("#friendsEmpty");
  if (!ul) return;
  ul.innerHTML = "";
  const r = await api("GET", "/users/friends");
  if (!r.ok || !r.data?.items?.length) {
    empty?.classList.remove("hidden");
    return;
  }
  empty?.classList.add("hidden");
  r.data.items.forEach((f) => {
    const li = document.createElement("li");
    const label = f.username || f.email || f.id.slice(0, 8);
    li.innerHTML = `
      <div class="friend-meta">
        <strong>${escapeHtml(f.username || "User")}</strong>
        <span>${escapeHtml(f.email || "")}</span>
      </div>
      <button type="button" class="btn secondary small" data-open-dm="${escapeHtml(f.id)}" data-dm-label="${escapeHtml(label)}">Message</button>`;
    ul.appendChild(li);
  });
  ul.querySelectorAll("[data-open-dm]").forEach((btn) => {
    btn.addEventListener("click", async (e) => {
      const id = e.currentTarget.getAttribute("data-open-dm");
      const lab = e.currentTarget.getAttribute("data-dm-label") || "";
      if (!id) return;
      await openDirectWithFriend(id, lab);
    });
  });
}

async function openDirectWithFriend(otherUserId, displayLabel) {
  const dm = await api("POST", "/chats/direct", { other_user_id: otherUserId });
  if (!dm.ok) {
    alert((dm.data && dm.data.error) || "Could not open chat (are you friends?)");
    return;
  }
  if (dm.data?.id && displayLabel) peerLabelCache[dm.data.id] = displayLabel;
  closeModals();
  await loadChats();
  await openChat(dm.data.id, {
    id: dm.data.id,
    type: "direct",
    name: displayLabel,
  });
}

async function addFriendAndOpenChat(userId, username, email) {
  const fr = await api("POST", "/users/friends", { user_id: userId });
  if (!fr.ok) {
    alert((fr.data && fr.data.error) || "Could not add friend");
    return;
  }
  const label = username || email || userId.slice(0, 8);
  const dm = await api("POST", "/chats/direct", { other_user_id: userId });
  if (!dm.ok) {
    alert((dm.data && dm.data.error) || "Friend added but chat failed");
    await loadFriends();
    return;
  }
  if (dm.data?.id) peerLabelCache[dm.data.id] = label;
  $("#directSearchInput").value = "";
  $("#directSearchResults").innerHTML = "";
  $("#directSearchHint").textContent = "Start typing to search.";
  closeModals();
  await loadFriends();
  await loadChats();
  await openChat(dm.data.id, { id: dm.data.id, type: "direct", name: label });
}

function runDirectSearch() {
  const q = ($("#directSearchInput").value || "").trim();
  const hint = $("#directSearchHint");
  const box = $("#directSearchResults");
  if (!box) return;
  box.innerHTML = "";
  if (q.length < 2) {
    hint.textContent = "Type at least 2 characters.";
    return;
  }
  hint.textContent = "Searching…";
  api("GET", "/users/search?q=" + encodeURIComponent(q)).then((r) => {
    if (!r.ok) {
      hint.textContent = "Search failed.";
      return;
    }
    const items = r.data?.items || [];
    hint.textContent = items.length ? `${items.length} result(s)` : "No matches.";
    items.forEach((u) => {
      const li = document.createElement("li");
      li.innerHTML = `
        <div class="peer-meta">
          <strong>@${escapeHtml(u.username || "")}</strong>
          <span>${escapeHtml(u.email || "")}</span>
        </div>
        <button type="button" class="btn primary small">Add & chat</button>`;
      li.querySelector("button").addEventListener("click", () =>
        addFriendAndOpenChat(u.id, u.username, u.email)
      );
      box.appendChild(li);
    });
  });
}

function populateGroupFriendPick() {
  const host = $("#groupFriendsPick");
  const hint = $("#groupFriendsHint");
  if (!host) return;
  host.innerHTML = "";
  api("GET", "/users/friends").then((r) => {
    const items = r.ok && r.data?.items ? r.data.items : [];
    if (!items.length) {
      hint.textContent = "No friends yet — find people and add them first.";
      return;
    }
    hint.textContent = "Select who should be in the group.";
    const me = jwtSub(getAccess());
    items.forEach((f) => {
      if (f.id === me) return;
      const row = document.createElement("label");
      row.className = "friend-pick-row";
      row.innerHTML = `<input type="checkbox" name="groupFriend" value="${escapeHtml(f.id)}" />
        <span>@${escapeHtml(f.username || "")} · ${escapeHtml(f.email || "")}</span>`;
      host.appendChild(row);
    });
  });
}

async function populateAddMemberPick() {
  const host = $("#addMemberPick");
  if (!host || !activeChatId) return;
  host.innerHTML = "";
  const [fr, mem] = await Promise.all([
    api("GET", "/users/friends"),
    api("GET", "/chats/" + encodeURIComponent(activeChatId) + "/members"),
  ]);
  const friends = fr.ok && fr.data?.items ? fr.data.items : [];
  const memberIds = new Set(
    mem.ok && mem.data?.items ? mem.data.items.map((m) => m.user_id) : []
  );
  const candidates = friends.filter((f) => !memberIds.has(f.id));
  if (!candidates.length) {
    host.innerHTML = `<p class="muted small">Everyone is already in this group or you have no other friends.</p>`;
    return;
  }
  candidates.forEach((f) => {
    const row = document.createElement("div");
    row.className = "friend-pick-row";
    row.style.cursor = "pointer";
    row.innerHTML = `<span>@${escapeHtml(f.username || "")} · ${escapeHtml(f.email || "")}</span>
      <button type="button" class="btn primary small">Add</button>`;
    row.querySelector("button").addEventListener("click", async () => {
      const r = await api(
        "POST",
        "/chats/" + encodeURIComponent(activeChatId) + "/members",
        { user_id: f.id }
      );
      if (!r.ok) alert((r.data && r.data.error) || "Failed");
      else {
        closeModals();
        loadChats();
      }
    });
    host.appendChild(row);
  });
}

function escapeHtml(s) {
  const d = document.createElement("div");
  d.textContent = s;
  return d.innerHTML;
}

async function openChat(chatId, meta) {
  activeChatId = String(chatId || "").trim();
  activeChatMeta = meta || null;
  directPeerUserId = null;
  $$(".chat-item").forEach((el) => {
    el.classList.toggle("active", el.dataset.id === chatId);
  });
  $("#threadEmpty").classList.add("hidden");
  $("#threadPanel").classList.remove("hidden");
  let title = meta ? chatLabel(meta) : "Chat";
  if (meta?.type === "direct" && meta?.name) title = meta.name;
  if (meta?.type === "direct" && peerLabelCache[activeChatId])
    title = peerLabelCache[activeChatId];
  $("#threadTitle").textContent = title;
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
  const addBtn = $("#btnAddMember");
  if (addBtn) addBtn.style.display = meta?.type === "group" ? "" : "none";

  if (meta?.type === "direct") {
    const mm = await api("GET", "/chats/" + encodeURIComponent(chatId) + "/members");
    if (mm.ok && mm.data?.items) {
      const me = jwtSub(getAccess());
      const peer = mm.data.items.find((x) => x.user_id !== me);
      if (peer?.user_id) directPeerUserId = peer.user_id;
      if (peer?.username) {
        peerLabelCache[chatId] = peer.username;
        $("#threadTitle").textContent = peer.username;
      } else if (peer?.email) {
        peerLabelCache[chatId] = peer.email;
        $("#threadTitle").textContent = peer.email;
      }
    }
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
    const rcpt = mine ? receiptMarkup(m) : "";
    div.innerHTML = `
      <div class="bubble">
        <div class="bubble-meta">${mine ? "You" : escapeHtml((m.sender_id || "").slice(0, 8) + "…")} · ${t}${rcpt}</div>
        <div class="bubble-text">${escapeHtml(m.content || "")}</div>
      </div>`;
    box.appendChild(div);
  });
  box.scrollTop = box.scrollHeight;
}

function pushMessage(m) {
  const me = jwtSub(getAccess());
  const realId = m.message_id || m.id;
  const content = m.content || "";
  const sender = m.sender_id || "";

  if (realId && sender === me) {
    const pi = messages.findIndex(
      (x) => x._pending && x.sender_id === sender && x.content === content
    );
    if (pi >= 0) {
      messages[pi] = {
        id: realId,
        sender_id: sender,
        content,
        created_at: m.at || m.created_at || messages[pi].created_at,
        _ts: m.at ? new Date(m.at).getTime() : Date.now(),
        read_by: m.read_by || messages[pi].read_by || [],
      };
      renderMessages();
      return;
    }
  }

  if (realId && messages.some((x) => x.id === realId)) return;

  const recent = messages.slice(-5);
  const dup = recent.some(
    (x) =>
      x.sender_id === sender &&
      x.content === content &&
      (!m.at || !x._ts || Math.abs(new Date(m.at).getTime() - x._ts) < 4000)
  );
  if (dup) return;

  messages.push({
    id: realId,
    sender_id: sender,
    content,
    created_at: m.at || m.created_at || new Date().toISOString(),
    _ts: m.at ? new Date(m.at).getTime() : Date.now(),
    read_by: m.read_by || [],
  });
  renderMessages();

  if (sender && sender !== me && realId) {
    queueMicrotask(() => {
      if (socket?.readyState === WebSocket.OPEN) {
        try {
          socket.send(JSON.stringify({ type: "read_receipt", message_id: realId }));
        } catch (_) {}
      }
    });
  }
}

function connectWS() {
  disconnectWS();
  if (!activeChatId || !getAccess()) return;
  const url = wsURL(activeChatId);
  socket = new WebSocket(url);
  $("#wsDot").className = "ws-dot connecting";
  socket.onopen = () => {
    $("#wsDot").className = "ws-dot on";
    acknowledgePeerReads();
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
      const mid = p.message_id;
      const reader = p.sender_id;
      if (!mid || !reader) return;
      const meSub = jwtSub(getAccess());
      const msg = messages.find((x) => x.id === mid);
      if (!msg || msg.sender_id !== meSub) return;
      if (!msg.read_by) msg.read_by = [];
      if (!msg.read_by.some((r) => r.user_id === reader)) {
        msg.read_by.push({
          user_id: reader,
          read_at: p.at || new Date().toISOString(),
        });
        renderMessages();
      }
      return;
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
    const me = jwtSub(getAccess());
    const pendingId =
      "pending-" + (globalThis.crypto?.randomUUID?.() || String(Date.now()));
    messages.push({
      id: pendingId,
      sender_id: me,
      content: text,
      created_at: new Date().toISOString(),
      _pending: true,
      read_by: [],
    });
    renderMessages();
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
      read_by: r.data.read_by || [],
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
  loadFriends();
  loadChats();
  startHeartbeat();
}

function leaveApp() {
  stopHeartbeat();
  disconnectWS();
  clearTokens();
  activeChatId = null;
  activeChatMeta = null;
  directPeerUserId = null;
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
  $("#btnOpenNewDirect").addEventListener("click", () => {
    $("#directSearchInput").value = "";
    $("#directSearchResults").innerHTML = "";
    $("#directSearchHint").textContent = "Start typing to search.";
    openModal("modalDirect");
    setTimeout(() => $("#directSearchInput").focus(), 100);
  });
  $("#btnOpenNewGroup").addEventListener("click", () => {
    populateGroupFriendPick();
    openModal("modalGroup");
  });
  $("#btnRefreshChats").addEventListener("click", loadChats);
  $("#btnRefreshFriends").addEventListener("click", loadFriends);

  $("#directSearchInput").addEventListener("input", () => {
    clearTimeout(directSearchTimer);
    directSearchTimer = setTimeout(runDirectSearch, 320);
  });

  $$("[data-close-modal]").forEach((el) =>
    el.addEventListener("click", closeModals)
  );

  $("#formGroup").addEventListener("submit", async (e) => {
    e.preventDefault();
    const name = $("#groupNameInput").value.trim() || "Group";
    const member_ids = $$('#groupFriendsPick input[name="groupFriend"]:checked').map(
      (inp) => inp.value
    );
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
    $$('#groupFriendsPick input[type="checkbox"]').forEach((inp) => {
      inp.checked = false;
    });
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
    if (activeChatMeta?.type !== "group") {
      alert("Add member is only for group chats.");
      return;
    }
    await populateAddMemberPick();
    openModal("modalAddMember");
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
