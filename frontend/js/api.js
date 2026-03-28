const LS = {
  access: "chat_app_access_token",
  refresh: "chat_app_refresh_token",
  base: "chat_app_api_base",
};

export function getBase() {
  return (localStorage.getItem(LS.base) || "http://localhost:8080").replace(/\/$/, "");
}

export function setBase(url) {
  localStorage.setItem(LS.base, (url || "").trim().replace(/\/$/, ""));
}

export function getAccess() {
  return localStorage.getItem(LS.access) || "";
}

export function getRefresh() {
  return localStorage.getItem(LS.refresh) || "";
}

export function setTokens(access, refresh) {
  if (access) localStorage.setItem(LS.access, access);
  if (refresh) localStorage.setItem(LS.refresh, refresh);
}

export function clearTokens() {
  localStorage.removeItem(LS.access);
  localStorage.removeItem(LS.refresh);
}

export function jwtSub(token) {
  if (!token) return null;
  try {
    const p = token.split(".")[1];
    if (!p) return null;
    const pad = p.length % 4;
    const b64 =
      p.replace(/-/g, "+").replace(/_/g, "/") + (pad ? "=".repeat(4 - pad) : "");
    const o = JSON.parse(atob(b64));
    return o.sub || null;
  } catch {
    return null;
  }
}

async function parseBody(res) {
  const text = await res.text();
  if (!text) return null;
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

/**
 * @param {string} method
 * @param {string} path
 * @param {object|undefined} body
 * @param {string|undefined} tokenOverride Bearer token (optional)
 */
export async function rawApi(method, path, body, tokenOverride) {
  const base = getBase();
  const url = base + path;
  const headers = { "Content-Type": "application/json" };
  const t = tokenOverride !== undefined ? tokenOverride : getAccess();
  if (t) headers.Authorization = "Bearer " + t;
  const opt = { method, headers };
  if (body !== undefined) opt.body = JSON.stringify(body);
  const res = await fetch(url, opt);
  const data = await parseBody(res);
  return { ok: res.ok, status: res.status, data };
}

export async function api(method, path, body) {
  let r = await rawApi(method, path, body);
  if (r.status === 401 && getRefresh()) {
    const ref = await rawApi("POST", "/auth/refresh", {
      refresh_token: getRefresh(),
    });
    if (ref.ok && ref.data?.access_token) {
      setTokens(ref.data.access_token, ref.data.refresh_token || getRefresh());
      r = await rawApi(method, path, body);
    }
  }
  return r;
}

export function wsURL(chatId) {
  const base = getBase();
  const u = new URL(base);
  const proto = u.protocol === "https:" ? "wss:" : "ws:";
  const token = encodeURIComponent(getAccess());
  return `${proto}//${u.host}/ws?chat_id=${encodeURIComponent(chatId)}&access_token=${token}`;
}
