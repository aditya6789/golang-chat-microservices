const LS = {
  access: "chat_app_access_token",
  refresh: "chat_app_refresh_token",
  base: "chat_app_api_base",
} as const;

const DEFAULT_BASE = "http://localhost:8080";

/** Ensures fetch/WebSocket URL helpers always get a valid absolute URL. */
export function normalizeApiBase(raw: string): string {
  let s = (raw || "").trim().replace(/\/$/, "");
  if (!s) return DEFAULT_BASE;
  if (!/^https?:\/\//i.test(s)) {
    s = "http://" + s.replace(/^\/+/, "");
  }
  try {
    const u = new URL(s);
    if (!u.hostname) return DEFAULT_BASE;
    return s.replace(/\/$/, "");
  } catch {
    return DEFAULT_BASE;
  }
}

export function getBase(): string {
  if (typeof window === "undefined") return DEFAULT_BASE;
  return normalizeApiBase(localStorage.getItem(LS.base) || DEFAULT_BASE);
}

export function setBase(url: string): void {
  localStorage.setItem(LS.base, normalizeApiBase(url));
}

export function getAccess(): string {
  if (typeof window === "undefined") return "";
  return localStorage.getItem(LS.access) || "";
}

export function getRefresh(): string {
  if (typeof window === "undefined") return "";
  return localStorage.getItem(LS.refresh) || "";
}

export function setTokens(access: string, refresh: string): void {
  if (access) localStorage.setItem(LS.access, access);
  if (refresh) localStorage.setItem(LS.refresh, refresh);
}

export function clearTokens(): void {
  localStorage.removeItem(LS.access);
  localStorage.removeItem(LS.refresh);
}

export function jwtSub(token: string | null): string | null {
  if (!token) return null;
  try {
    const p = token.split(".")[1];
    if (!p) return null;
    const pad = p.length % 4;
    const b64 =
      p.replace(/-/g, "+").replace(/_/g, "/") + (pad ? "=".repeat(4 - pad) : "");
    const o = JSON.parse(atob(b64)) as { sub?: string };
    return o.sub || null;
  } catch {
    return null;
  }
}

async function parseBody(res: Response): Promise<unknown> {
  const text = await res.text();
  if (!text) return null;
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

export type ApiResult = { ok: boolean; status: number; data: unknown };

export async function rawApi(
  method: string,
  path: string,
  body?: object,
  tokenOverride?: string
): Promise<ApiResult> {
  const base = getBase();
  const url = base + path;
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  const t = tokenOverride !== undefined ? tokenOverride : getAccess();
  if (t) headers.Authorization = "Bearer " + t;
  const opt: RequestInit = { method, headers };
  if (body !== undefined) opt.body = JSON.stringify(body);
  const res = await fetch(url, opt);
  const data = await parseBody(res);
  return { ok: res.ok, status: res.status, data };
}

export async function api(method: string, path: string, body?: object): Promise<ApiResult> {
  let r = await rawApi(method, path, body);
  if (r.status === 401 && getRefresh()) {
    const ref = await rawApi("POST", "/auth/refresh", {
      refresh_token: getRefresh(),
    });
    const refData = ref.data as { access_token?: string; refresh_token?: string } | null;
    if (ref.ok && refData?.access_token) {
      setTokens(refData.access_token, refData.refresh_token || getRefresh());
      r = await rawApi(method, path, body);
    }
  }
  return r;
}

export function wsURL(chatId: string): string {
  const base = getBase();
  try {
    const u = new URL(base);
    const proto = u.protocol === "https:" ? "wss:" : "ws:";
    const token = encodeURIComponent(getAccess());
    return `${proto}//${u.host}/ws?chat_id=${encodeURIComponent(chatId)}&access_token=${token}`;
  } catch {
    const u = new URL(DEFAULT_BASE);
    const token = encodeURIComponent(getAccess());
    return `ws://${u.host}/ws?chat_id=${encodeURIComponent(chatId)}&access_token=${token}`;
  }
}

/** WebRTC signaling (1:1 voice/video); friend-gated on the server. */
export function signalingURL(): string {
  const base = getBase();
  try {
    const u = new URL(base);
    const proto = u.protocol === "https:" ? "wss:" : "ws:";
    const token = encodeURIComponent(getAccess());
    return `${proto}//${u.host}/ws/signaling?access_token=${token}`;
  } catch {
    const u = new URL(DEFAULT_BASE);
    const token = encodeURIComponent(getAccess());
    return `ws://${u.host}/ws/signaling?access_token=${token}`;
  }
}
