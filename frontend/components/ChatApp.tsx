"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  api,
  clearTokens,
  getAccess,
  getBase,
  jwtSub,
  setBase,
  setTokens,
  signalingURL,
  wsURL,
} from "@/lib/api";
import {
  VoiceCallManager,
  type VoiceEndReason,
  type VoicePhase,
} from "@/lib/voiceCall";

function fmtVoiceDuration(ms: number): string {
  if (ms < 0 || !Number.isFinite(ms)) return "0:00";
  const s = Math.floor(ms / 1000);
  const m = Math.floor(s / 60);
  const sec = s % 60;
  const h = Math.floor(m / 60);
  if (h > 0) {
    return `${h}:${(m % 60).toString().padStart(2, "0")}:${sec.toString().padStart(2, "0")}`;
  }
  return `${m}:${sec.toString().padStart(2, "0")}`;
}

function voiceCallStatusLine(phase: VoicePhase, muted: boolean, video: boolean): string {
  const v = video ? "Video · " : "";
  switch (phase) {
    case "incoming":
      return `${v}Incoming call`;
    case "outgoing":
      return `${v}Ringing…`;
    case "connecting":
      return `${v}Connecting…`;
    case "active":
      if (video) {
        return muted ? "Video call · Mic muted" : "Video call";
      }
      return muted ? "In call · Mic muted" : "In call";
    default:
      return "";
  }
}

function voiceCallEndLine(reason: VoiceEndReason): string {
  switch (reason) {
    case "you_ended":
      return "You ended the call";
    case "peer_ended":
      return "Call ended";
    case "peer_declined":
      return "Call declined";
    case "declined_incoming":
      return "You declined";
    case "failed":
      return "Connection failed";
    case "error":
      return "Call error";
    default:
      return "Call ended";
  }
}

type ChatMeta = { id?: string; type?: string; name?: string };
type Friend = { id: string; username?: string; email?: string };
type ReplyTo = {
  id?: string;
  sender_id?: string;
  content?: string;
  created_at?: string;
};
type Reaction = { emoji: string; user_ids: string[] };
type Message = {
  id?: string;
  sender_id: string;
  message_type?: string;
  content: string;
  created_at?: string;
  _ts?: number;
  _pending?: boolean;
  _replyToId?: string | null;
  reply_to_message_id?: string;
  reply_to?: ReplyTo;
  read_by?: Array<{ user_id: string; read_at?: string }>;
  reactions?: Reaction[];
};

type FilePayload = {
  object_key?: string;
  filename?: string;
  mime_type?: string;
  size_bytes?: number;
  download_url?: string;
};

function parseFilePayload(content: string): FilePayload | null {
  try {
    const o = JSON.parse(content) as FilePayload;
    if (o && typeof o.download_url === "string" && o.download_url) return o;
    return null;
  } catch {
    return null;
  }
}

function guessMimeFromName(name: string): string {
  const n = name.toLowerCase();
  if (n.endsWith(".png")) return "image/png";
  if (n.endsWith(".jpg") || n.endsWith(".jpeg")) return "image/jpeg";
  if (n.endsWith(".gif")) return "image/gif";
  if (n.endsWith(".webp")) return "image/webp";
  if (n.endsWith(".svg")) return "image/svg+xml";
  if (n.endsWith(".pdf")) return "application/pdf";
  if (n.endsWith(".txt") || n.endsWith(".md")) return "text/plain";
  if (n.endsWith(".doc")) return "application/msword";
  if (n.endsWith(".docx")) {
    return "application/vnd.openxmlformats-officedocument.wordprocessingml.document";
  }
  if (n.endsWith(".xls")) return "application/vnd.ms-excel";
  if (n.endsWith(".xlsx")) {
    return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet";
  }
  return "";
}

function messagePreviewText(m: Pick<Message, "content" | "message_type">): string {
  if (m.message_type === "file") {
    const fp = parseFilePayload(m.content || "");
    return fp?.filename ? `📎 ${fp.filename}` : "📎 File";
  }
  return m.content || "";
}

type SearchUser = { id: string; username?: string; email?: string };

const QUICK_REACTIONS = ["👍", "❤️", "😂", "😮", "😢", "🙏"];

type OgPreviewData = {
  url: string;
  title?: string;
  description?: string;
  image?: string;
  site_name?: string;
};

const ogFetchCache = new Map<string, OgPreviewData | "err">();

function firstHttpUrl(text: string): string | null {
  const re = /\bhttps?:\/\/[^\s<>"']+/i;
  const m = re.exec(text);
  if (!m) return null;
  return m[0].replace(/[),.;!?]+$/g, "");
}

function LinkPreviewCard({ url }: { url: string }) {
  const [data, setData] = useState<OgPreviewData | null | "err">(null);

  useEffect(() => {
    const c = ogFetchCache.get(url);
    if (c === "err") {
      setData("err");
      return;
    }
    if (c && typeof c === "object" && "url" in c) {
      setData(c);
      return;
    }
    let cancelled = false;
    void api("GET", "/messages/link-preview?url=" + encodeURIComponent(url)).then((r) => {
      if (cancelled) return;
      const d = r.data as OgPreviewData & { error?: string };
      if (r.ok && d?.url) {
        ogFetchCache.set(url, d);
        setData(d);
      } else {
        ogFetchCache.set(url, "err");
        setData("err");
      }
    });
    return () => {
      cancelled = true;
    };
  }, [url]);

  if (data === "err" || data === null) return null;
  if (!data.title && !data.description && !data.image) return null;

  return (
    <a
      className="link-preview-card"
      href={data.url}
      target="_blank"
      rel="noopener noreferrer"
      onClick={(e) => e.stopPropagation()}
    >
      {data.image ? (
        <div className="link-preview-img-wrap">
          <img
            src={data.image}
            alt=""
            loading="lazy"
            onError={(e) => {
              (e.target as HTMLImageElement).style.display = "none";
            }}
          />
        </div>
      ) : null}
      <div className="link-preview-body">
        {data.site_name ? <div className="link-preview-site">{data.site_name}</div> : null}
        {data.title ? <div className="link-preview-title">{data.title}</div> : null}
        {data.description ? (
          <div className="link-preview-desc">{truncateText(data.description, 220)}</div>
        ) : null}
      </div>
    </a>
  );
}

function normId(s: string | undefined) {
  return String(s || "")
    .trim()
    .toLowerCase()
    .replace(/-/g, "");
}

function fmtTime(iso: string | undefined) {
  if (!iso) return "";
  try {
    const d = new Date(iso);
    return d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
  } catch {
    return "";
  }
}

function truncateText(s: string | null | undefined, max: number) {
  if (s == null || s === "") return "";
  if (s.length <= max) return s;
  return s.slice(0, max) + "…";
}

function chatLabel(
  c: ChatMeta,
  peerLabelCache: Record<string, string>
): string {
  if (c.type === "group" && c.name) return c.name;
  if (c.type === "direct") {
    if (c.id && peerLabelCache[c.id]) return peerLabelCache[c.id];
    return "Direct · " + (c.id || "").slice(0, 8) + "…";
  }
  return (c.type || "Chat") + " · " + (c.id || "").slice(0, 8);
}

export default function ChatApp() {
  const [view, setView] = useState<"auth" | "main">("auth");
  const [authTab, setAuthTab] = useState<"login" | "register">("login");
  const [authError, setAuthError] = useState("");
  const [loginEmail, setLoginEmail] = useState("");
  const [loginPass, setLoginPass] = useState("");
  const [regEmail, setRegEmail] = useState("");
  const [regUser, setRegUser] = useState("");
  const [regPass, setRegPass] = useState("");

  const [chats, setChats] = useState<ChatMeta[]>([]);
  const [friends, setFriends] = useState<Friend[]>([]);
  const [activeChatId, setActiveChatId] = useState<string | null>(null);
  const [activeChatMeta, setActiveChatMeta] = useState<ChatMeta | null>(null);
  const [directPeerUserId, setDirectPeerUserId] = useState<string | null>(null);
  const [threadTitle, setThreadTitle] = useState("Chat");
  const [threadSubtitle, setThreadSubtitle] = useState("");
  const [showThread, setShowThread] = useState(false);

  const [messages, setMessages] = useState<Message[]>([]);
  const messagesRef = useRef<Message[]>([]);
  useEffect(() => {
    messagesRef.current = messages;
  }, [messages]);

  const activeChatIdRef = useRef<string | null>(null);
  useEffect(() => {
    activeChatIdRef.current = activeChatId;
  }, [activeChatId]);

  const [replyingTo, setReplyingTo] = useState<{
    id: string;
    fromLabel: string;
    reply_to: { id: string; sender_id: string; content: string; created_at: string };
  } | null>(null);

  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [modalSettings, setModalSettings] = useState(false);
  const [modalDirect, setModalDirect] = useState(false);
  const [modalGroup, setModalGroup] = useState(false);
  const [modalAddMember, setModalAddMember] = useState(false);
  const [threadMenuOpen, setThreadMenuOpen] = useState(false);
  const [wsStatus, setWsStatus] = useState<"off" | "connecting" | "on">("off");

  const [typingBar, setTypingBar] = useState({ show: false, live: false, text: "" });

  const [sidebarUser, setSidebarUser] = useState("…");
  const [sidebarUserSub, setSidebarUserSub] = useState("");
  const [sidebarAvatar, setSidebarAvatar] = useState("");

  const [apiBaseField, setApiBaseField] = useState("");
  const [directInput, setDirectInput] = useState("");
  const [directHint, setDirectHint] = useState("Start typing to search.");
  const [directResults, setDirectResults] = useState<SearchUser[]>([]);

  const [groupName, setGroupName] = useState("");
  const [groupPickFriends, setGroupPickFriends] = useState<Friend[]>([]);
  const [groupPickHint, setGroupPickHint] = useState("");
  const [groupChecked, setGroupChecked] = useState<Record<string, boolean>>({});

  const [addMemberCandidates, setAddMemberCandidates] = useState<Friend[]>([]);

  const [voicePhase, setVoicePhase] = useState<VoicePhase>("idle");
  const [voicePeerLabel, setVoicePeerLabel] = useState("");
  const [voiceMuted, setVoiceMuted] = useState(false);
  const [voiceToast, setVoiceToast] = useState("");
  const [voiceCallEndedReason, setVoiceCallEndedReason] = useState<VoiceEndReason | null>(null);
  const [voiceEndedDurationSnapshot, setVoiceEndedDurationSnapshot] = useState<string | null>(null);
  const [voiceTimerTick, setVoiceTimerTick] = useState(0);
  const [callVideoMode, setCallVideoMode] = useState(false);
  const [voiceCameraOff, setVoiceCameraOff] = useState(false);
  const voiceRingStartRef = useRef<number | null>(null);
  const voiceActiveStartRef = useRef<number | null>(null);
  const voiceEndedDismissTimerRef = useRef<number | null>(null);

  const peerLabelCacheRef = useRef<Record<string, string>>({});
  const signalingRef = useRef<WebSocket | null>(null);
  const voiceRef = useRef<VoiceCallManager | null>(null);
  const remoteAudioRef = useRef<HTMLAudioElement | null>(null);
  const remoteVideoRef = useRef<HTMLVideoElement | null>(null);
  const localVideoRef = useRef<HTMLVideoElement | null>(null);
  const socketRef = useRef<WebSocket | null>(null);
  const typingClearTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const heartbeatTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const typingDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const typingBurstOpenRef = useRef(false);
  const directSearchTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const messageListRef = useRef<HTMLDivElement>(null);

  const scrollMessagesToEnd = useCallback(() => {
    const el = messageListRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, []);

  useEffect(() => {
    queueMicrotask(scrollMessagesToEnd);
  }, [messages, scrollMessagesToEnd]);

  const disconnectWS = useCallback(() => {
    const s = socketRef.current;
    if (s) {
      try {
        s.close();
      } catch {
        /* ignore */
      }
      socketRef.current = null;
    }
    setWsStatus("off");
  }, []);

  const friendsRef = useRef(friends);
  useEffect(() => {
    friendsRef.current = friends;
  }, [friends]);

  const disconnectSignaling = useCallback(() => {
    if (voiceEndedDismissTimerRef.current) {
      clearTimeout(voiceEndedDismissTimerRef.current);
      voiceEndedDismissTimerRef.current = null;
    }
    voiceRef.current?.endCall();
    voiceRef.current = null;
    const sig = signalingRef.current;
    if (sig) {
      try {
        sig.close();
      } catch {
        /* ignore */
      }
      signalingRef.current = null;
    }
    voiceRingStartRef.current = null;
    voiceActiveStartRef.current = null;
    setVoicePhase((p) => (p === "idle" ? p : "idle"));
    setVoicePeerLabel((l) => (l === "" ? l : ""));
    setVoiceMuted((m) => (m === false ? m : false));
    setCallVideoMode(false);
    setVoiceCameraOff(false);
    const ra = remoteAudioRef.current;
    if (ra?.srcObject) ra.srcObject = null;
    const rv = remoteVideoRef.current;
    if (rv?.srcObject) rv.srcObject = null;
    const lv = localVideoRef.current;
    if (lv?.srcObject) lv.srcObject = null;
  }, []);

  useEffect(() => {
    if (view !== "main" || !jwtSub(getAccess())) {
      return;
    }
    try {
      voiceRef.current = new VoiceCallManager(
        (o) => {
          const ws = signalingRef.current;
          if (ws?.readyState === WebSocket.OPEN) ws.send(JSON.stringify(o));
        },
        (stream) => {
          const vEl = remoteVideoRef.current;
          const aEl = remoteAudioRef.current;
          if (!stream) {
            if (vEl) vEl.srcObject = null;
            if (aEl) aEl.srcObject = null;
            return;
          }
          const hasVideo = stream.getVideoTracks().length > 0;
          if (hasVideo && vEl) {
            vEl.srcObject = stream;
            void vEl.play().catch(() => {});
            if (aEl) aEl.srcObject = null;
          } else if (aEl) {
            aEl.srcObject = stream;
            if (vEl) vEl.srcObject = null;
          }
        },
        (phase, d) => {
          if (phase === "idle") {
            setVoicePhase("idle");
            if (d?.endReason) {
              const now = Date.now();
              let snap = "0:00";
              if (voiceActiveStartRef.current != null) {
                snap = fmtVoiceDuration(now - voiceActiveStartRef.current);
              } else if (voiceRingStartRef.current != null) {
                snap = fmtVoiceDuration(now - voiceRingStartRef.current);
              }
              setVoiceEndedDurationSnapshot(snap);
              setVoiceCallEndedReason(d.endReason);
              if (voiceEndedDismissTimerRef.current) {
                clearTimeout(voiceEndedDismissTimerRef.current);
              }
              voiceEndedDismissTimerRef.current = window.setTimeout(() => {
                voiceEndedDismissTimerRef.current = null;
                setVoiceCallEndedReason(null);
                setVoiceEndedDurationSnapshot(null);
                setVoicePeerLabel("");
              }, 4500);
            } else {
              setVoiceCallEndedReason(null);
              setVoiceEndedDurationSnapshot(null);
              setVoicePeerLabel("");
            }
            voiceRingStartRef.current = null;
            voiceActiveStartRef.current = null;
            setCallVideoMode(false);
            setVoiceCameraOff(false);
            return;
          }
          setVoicePhase(phase);
          setVoiceCallEndedReason(null);
          setVoiceEndedDurationSnapshot(null);
          setCallVideoMode(!!d?.video);
          setVoiceCameraOff(false);
          const pid = d?.peerId;
          if (d?.peerLabel) {
            setVoicePeerLabel(d.peerLabel);
          } else if (pid) {
            const f = friendsRef.current.find((x) => x.id === pid);
            setVoicePeerLabel(f?.username || f?.email || pid.slice(0, 8));
          }
          if (phase === "outgoing" || phase === "incoming") {
            voiceRingStartRef.current = Date.now();
            voiceActiveStartRef.current = null;
          }
          if (phase === "active") {
            voiceActiveStartRef.current = Date.now();
          }
        },
        (msg) => {
          setVoiceToast(msg);
          window.setTimeout(() => setVoiceToast(""), 5000);
        },
        (local) => {
          const el = localVideoRef.current;
          if (el) el.srcObject = local;
          if (local) void el?.play().catch(() => {});
        }
      );
      const ws = new WebSocket(signalingURL());
      signalingRef.current = ws;
      ws.onmessage = (ev) => {
        let p: Record<string, unknown>;
        try {
          p = JSON.parse(ev.data as string);
        } catch {
          return;
        }
        void voiceRef.current?.handleSignal(p);
      };
      ws.onclose = () => {
        if (signalingRef.current === ws) signalingRef.current = null;
      };
    } catch (err) {
      console.error("signaling init failed", err);
      disconnectSignaling();
      setVoiceToast("Voice signaling unavailable (check API base URL in Settings)");
      window.setTimeout(() => setVoiceToast(""), 6000);
    }
    return () => {
      disconnectSignaling();
    };
  }, [view, disconnectSignaling]);

  useEffect(() => {
    voiceRef.current?.setMuted(voiceMuted);
  }, [voiceMuted]);

  useEffect(() => {
    voiceRef.current?.setCameraEnabled(!voiceCameraOff);
  }, [voiceCameraOff]);

  /** Attach local camera to PiP when the video element mounts after the stream exists. */
  useEffect(() => {
    if (!callVideoMode || voicePhase === "idle") return;
    const id = window.setInterval(() => {
      const el = localVideoRef.current;
      const s = voiceRef.current?.getLocalStream();
      if (el && s && el.srcObject !== s) {
        el.srcObject = s;
        void el.play().catch(() => {});
      }
    }, 400);
    return () => clearInterval(id);
  }, [callVideoMode, voicePhase]);

  useEffect(() => {
    if (voicePhase === "idle" && !voiceCallEndedReason) return;
    const id = window.setInterval(() => setVoiceTimerTick((n) => n + 1), 1000);
    return () => clearInterval(id);
  }, [voicePhase, voiceCallEndedReason]);

  const mergeReactionEvent = useCallback((p: Record<string, unknown>) => {
    const mid = p.message_id as string;
    const uid = p.sender_id as string;
    const emoji = p.emoji as string;
    const act = p.reaction_action === "remove" ? "remove" : "add";
    if (!mid || !uid || !emoji) return;
    setMessages((prev) => {
      const msg = prev.find((x) => String(x.id) === String(mid));
      if (!msg) return prev;
      let reactions = [...(msg.reactions || [])];
      const bucket = reactions.find((r) => r.emoji === emoji);
      if (act === "remove") {
        if (!bucket) return prev;
        const newIds = (bucket.user_ids || []).filter((u) => u !== uid);
        if (newIds.length === 0) {
          reactions = reactions.filter((r) => r.emoji !== emoji);
        } else {
          reactions = reactions.map((r) =>
            r.emoji === emoji ? { ...r, user_ids: newIds } : r
          );
        }
      } else {
        if (!bucket) {
          reactions.push({ emoji, user_ids: [uid] });
        } else if (!bucket.user_ids.includes(uid)) {
          reactions = reactions.map((r) =>
            r.emoji === emoji ? { ...r, user_ids: [...r.user_ids, uid] } : r
          );
        }
      }
      return prev.map((m) => (String(m.id) === String(mid) ? { ...m, reactions } : m));
    });
  }, []);

  const pushMessage = useCallback((m: Record<string, unknown>) => {
    const me = jwtSub(getAccess());
    const realId = (m.message_id || m.id) as string | undefined;
    const content = (m.content as string) || "";
    const sender = (m.sender_id as string) || "";
    const wireType = ((m.message_type as string) || "text").trim() || "text";
    const wireReplyId =
      (m.reply_to_message_id as string) || (m.reply_to as ReplyTo | undefined)?.id || "";
    const wireReplyTo = (m.reply_to as ReplyTo | null) || null;

    setMessages((msgs) => {
      if (realId && sender === me) {
        const pi = msgs.findIndex(
          (x) =>
            x._pending &&
            x.sender_id === sender &&
            x.content === content &&
            (x.message_type || "text") === wireType &&
            (x._replyToId || "") === (wireReplyId || "")
        );
        if (pi >= 0) {
          const copy = [...msgs];
          copy[pi] = {
            id: realId,
            sender_id: sender,
            message_type: wireType,
            content,
            created_at: (m.at as string) || (m.created_at as string) || copy[pi].created_at,
            _ts: m.at ? new Date(m.at as string).getTime() : Date.now(),
            read_by: (m.read_by as Message["read_by"]) || copy[pi].read_by || [],
            reply_to_message_id: wireReplyId || undefined,
            reply_to: wireReplyTo || copy[pi].reply_to,
            reactions: (m.reactions as Reaction[]) || copy[pi].reactions || [],
          };
          return copy;
        }
      }

      if (realId && msgs.some((x) => x.id === realId)) return msgs;

      const recent = msgs.slice(-5);
      const dup = recent.some(
        (x) =>
          x.sender_id === sender &&
          x.content === content &&
          (x.message_type || "text") === wireType &&
          (x.reply_to_message_id || x.reply_to?.id || "") === (wireReplyId || "") &&
          (!m.at ||
            !x._ts ||
            Math.abs(new Date(m.at as string).getTime() - x._ts) < 4000)
      );
      if (dup) return msgs;

      return [
        ...msgs,
        {
          id: realId,
          sender_id: sender,
          message_type: wireType,
          content,
          created_at:
            (m.at as string) || (m.created_at as string) || new Date().toISOString(),
          _ts: m.at ? new Date(m.at as string).getTime() : Date.now(),
          read_by: (m.read_by as Message["read_by"]) || [],
          reply_to_message_id: wireReplyId || undefined,
          reply_to: wireReplyTo || undefined,
          reactions: (m.reactions as Reaction[]) || [],
        },
      ];
    });

    if (sender && sender !== me && realId) {
      queueMicrotask(() => {
        const sock = socketRef.current;
        if (sock?.readyState === WebSocket.OPEN) {
          try {
            sock.send(JSON.stringify({ type: "read_receipt", message_id: realId }));
          } catch {
            /* ignore */
          }
        }
      });
    }
  }, []);

  const acknowledgePeerReads = useCallback(() => {
    const sock = socketRef.current;
    if (!sock || sock.readyState !== WebSocket.OPEN) return;
    const me = jwtSub(getAccess());
    for (const m of messagesRef.current) {
      if (m.sender_id === me || !m.id || String(m.id).startsWith("pending-")) continue;
      try {
        sock.send(JSON.stringify({ type: "read_receipt", message_id: m.id }));
      } catch {
        /* ignore */
      }
    }
  }, []);

  useEffect(() => {
    if (!activeChatId || !getAccess()) {
      disconnectWS();
      return;
    }
    disconnectWS();
    const url = wsURL(activeChatId);
    const socket = new WebSocket(url);
    socketRef.current = socket;
    setWsStatus("connecting");

    socket.onopen = () => {
      setWsStatus("on");
      acknowledgePeerReads();
    };
    socket.onclose = () => {
      setWsStatus("off");
      socketRef.current = null;
    };
    socket.onerror = () => setWsStatus("off");
    socket.onmessage = (ev) => {
      let p: Record<string, unknown>;
      try {
        p = JSON.parse(ev.data);
      } catch {
        return;
      }
      const cid = activeChatIdRef.current;
      if (p.chat_id && normId(String(p.chat_id)) !== normId(cid || undefined)) return;
      if (p.type === "typing") {
        const meSub = (jwtSub(getAccess()) || "").trim().toLowerCase();
        const them = String(p.sender_id || "")
          .trim()
          .toLowerCase();
        if (them && them !== meSub) {
          if (typingClearTimerRef.current) clearTimeout(typingClearTimerRef.current);
          setTypingBar({ show: true, live: true, text: "Typing…" });
          typingClearTimerRef.current = setTimeout(() => {
            setTypingBar((b) => ({ ...b, show: false, live: false }));
          }, 3500);
        }
        return;
      }
      if (p.type === "message" && p.content) {
        pushMessage(p);
        return;
      }
      if (p.type === "reaction") {
        mergeReactionEvent(p);
        return;
      }
      if (p.type === "read_receipt") {
        const mid = p.message_id as string;
        const reader = p.sender_id as string;
        if (!mid || !reader) return;
        const meSub = jwtSub(getAccess());
        setMessages((msgs) => {
          const msg = msgs.find((x) => x.id === mid);
          if (!msg || msg.sender_id !== meSub) return msgs;
          const read_by = [...(msg.read_by || [])];
          if (read_by.some((r) => r.user_id === reader)) return msgs;
          read_by.push({
            user_id: reader,
            read_at: (p.at as string) || new Date().toISOString(),
          });
          return msgs.map((x) => (x.id === mid ? { ...x, read_by } : x));
        });
      }
    };

    return () => {
      try {
        socket.close();
      } catch {
        /* ignore */
      }
      socketRef.current = null;
      setWsStatus("off");
    };
  }, [activeChatId, disconnectWS, acknowledgePeerReads, pushMessage, mergeReactionEvent]);

  const enrichDirectLabels = useCallback(async (list: ChatMeta[]) => {
    const me = jwtSub(getAccess());
    const directs = (list || []).filter(
      (c) => c.type === "direct" && c.id && !peerLabelCacheRef.current[c.id]
    );
    await Promise.all(
      directs.map(async (c) => {
        if (!c.id) return;
        const m = await api("GET", "/chats/" + encodeURIComponent(c.id) + "/members");
        const data = m.data as { items?: Array<{ user_id: string; username?: string; email?: string }> };
        if (!m.ok || !data?.items) return;
        const peer = data.items.find((x) => x.user_id !== me);
        if (peer?.username) peerLabelCacheRef.current[c.id] = peer.username;
        else if (peer?.email) peerLabelCacheRef.current[c.id] = peer.email;
      })
    );
  }, []);

  const loadChats = useCallback(async () => {
    const r = await api("GET", "/chats");
    const data = r.data as { items?: ChatMeta[] };
    if (!r.ok || !data?.items) {
      setChats([]);
      return;
    }
    await enrichDirectLabels(data.items);
    setChats(data.items);
  }, [enrichDirectLabels]);

  const loadFriends = useCallback(async () => {
    const r = await api("GET", "/users/friends");
    const data = r.data as { items?: Friend[] };
    if (!r.ok || !data?.items?.length) {
      setFriends([]);
      return;
    }
    setFriends(data.items);
  }, []);

  const startCall = useCallback((peerId: string, label: string, video = false) => {
    setVoicePeerLabel(label);
    void voiceRef.current?.startOutgoing(peerId, label, video);
  }, []);

  const closeModals = useCallback(() => {
    setModalSettings(false);
    setModalDirect(false);
    setModalGroup(false);
    setModalAddMember(false);
  }, []);

  const openChat = useCallback(
    async (chatId: string, meta: ChatMeta | null) => {
      const id = String(chatId || "").trim();
      setActiveChatId(id);
      setActiveChatMeta(meta);
      setDirectPeerUserId(null);
      setReplyingTo(null);
      setShowThread(true);
      setSidebarOpen(false);

      let title = meta ? chatLabel(meta, peerLabelCacheRef.current) : "Chat";
      if (meta?.type === "direct" && meta?.name) title = meta.name;
      if (meta?.type === "direct" && peerLabelCacheRef.current[id])
        title = peerLabelCacheRef.current[id];
      setThreadTitle(title);
      setThreadSubtitle(chatId);
      setMessages([]);
      setTypingBar({ show: false, live: false, text: "" });

      const hist = await api(
        "GET",
        "/messages/" + encodeURIComponent(id) + "?limit=100&offset=0"
      );
      const histData = hist.data as { items?: Message[] };
      if (hist.ok && histData?.items) {
        setMessages([...histData.items].reverse());
      }

      if (meta?.type === "direct") {
        const mm = await api("GET", "/chats/" + encodeURIComponent(id) + "/members");
        const mmData = mm.data as {
          items?: Array<{ user_id: string; username?: string; email?: string }>;
        };
        if (mm.ok && mmData?.items) {
          const me = jwtSub(getAccess());
          const peer = mmData.items.find((x) => x.user_id !== me);
          if (peer?.user_id) setDirectPeerUserId(peer.user_id);
          if (peer?.username) {
            peerLabelCacheRef.current[id] = peer.username;
            setThreadTitle(peer.username);
          } else if (peer?.email) {
            peerLabelCacheRef.current[id] = peer.email;
            setThreadTitle(peer.email);
          }
        }
      }
    },
    []
  );

  const openDirectWithFriend = useCallback(
    async (otherUserId: string, displayLabel: string) => {
      const dm = await api("POST", "/chats/direct", { other_user_id: otherUserId });
      const dmData = dm.data as { id?: string; error?: string };
      if (!dm.ok) {
        alert(dmData?.error || "Could not open chat (are you friends?)");
        return;
      }
      if (dmData?.id && displayLabel) peerLabelCacheRef.current[dmData.id] = displayLabel;
      closeModals();
      await loadChats();
      if (dmData.id) {
        await openChat(dmData.id, {
          id: dmData.id,
          type: "direct",
          name: displayLabel,
        });
      }
    },
    [closeModals, loadChats, openChat]
  );

  const addFriendAndOpenChat = useCallback(
    async (userId: string, username: string | undefined, email: string | undefined) => {
      const fr = await api("POST", "/users/friends", { user_id: userId });
      const frData = fr.data as { error?: string };
      if (!fr.ok) {
        alert(frData?.error || "Could not add friend");
        return;
      }
      const label = username || email || userId.slice(0, 8);
      const dm = await api("POST", "/chats/direct", { other_user_id: userId });
      const dmData = dm.data as { id?: string; error?: string };
      if (!dm.ok) {
        alert(dmData?.error || "Friend added but chat failed");
        await loadFriends();
        return;
      }
      if (dmData?.id) peerLabelCacheRef.current[dmData.id] = label;
      setDirectInput("");
      setDirectResults([]);
      setDirectHint("Start typing to search.");
      closeModals();
      await loadFriends();
      await loadChats();
      if (dmData.id) {
        await openChat(dmData.id, { id: dmData.id, type: "direct", name: label });
      }
    },
    [closeModals, loadChats, loadFriends, openChat]
  );

  const runDirectSearch = useCallback(
    (q: string) => {
      const t = q.trim();
      setDirectResults([]);
      if (t.length < 2) {
        setDirectHint("Type at least 2 characters.");
        return;
      }
      setDirectHint("Searching…");
      void api("GET", "/users/search?q=" + encodeURIComponent(t)).then((r) => {
        const data = r.data as { items?: SearchUser[]; error?: string };
        if (!r.ok) {
          setDirectHint("Search failed.");
          return;
        }
        const items = data?.items || [];
        setDirectHint(items.length ? `${items.length} result(s)` : "No matches.");
        setDirectResults(items);
      });
    },
    []
  );

  const populateGroupFriendPick = useCallback(() => {
    void api("GET", "/users/friends").then((r) => {
      const data = r.data as { items?: Friend[] };
      const items = r.ok && data?.items ? data.items : [];
      setGroupChecked({});
      if (!items.length) {
        setGroupPickFriends([]);
        setGroupPickHint("No friends yet — find people and add them first.");
        return;
      }
      setGroupPickFriends(items);
      setGroupPickHint("Select who should be in the group.");
    });
  }, []);

  const populateAddMemberPick = useCallback(async () => {
    if (!activeChatId) return;
    const [fr, mem] = await Promise.all([
      api("GET", "/users/friends"),
      api("GET", "/chats/" + encodeURIComponent(activeChatId) + "/members"),
    ]);
    const frData = fr.data as { items?: Friend[] };
    const memData = mem.data as { items?: Array<{ user_id: string }> };
    const friendList = fr.ok && frData?.items ? frData.items : [];
    const memberIds = new Set(
      mem.ok && memData?.items ? memData.items.map((m) => m.user_id) : []
    );
    const candidates = friendList.filter((f) => !memberIds.has(f.id));
    setAddMemberCandidates(candidates);
  }, [activeChatId]);

  const composerTextRef = useRef("");
  const [composerTick, setComposerTick] = useState(0);

  const sendReaction = useCallback(
    (messageId: string, emoji: string, add: boolean) => {
      if (!messageId || !emoji || !activeChatId) return;
      const payload = JSON.stringify({
        type: "reaction",
        message_id: messageId,
        emoji,
        reaction_action: add ? "add" : "remove",
      });
      const sock = socketRef.current;
      if (sock?.readyState === WebSocket.OPEN) {
        try {
          sock.send(payload);
        } catch {
          /* ignore */
        }
        return;
      }
      void api("POST", "/messages/" + encodeURIComponent(messageId) + "/reactions", {
        emoji,
        add,
      }).then((r) => {
        if (r.ok)
          mergeReactionEvent({
            message_id: messageId,
            sender_id: jwtSub(getAccess()),
            emoji,
            reaction_action: add ? "add" : "remove",
          });
      });
    },
    [activeChatId, mergeReactionEvent]
  );

  const sendText = useCallback(async () => {
    const text = composerTextRef.current.trim();
    if (!text || !activeChatId) return;
    composerTextRef.current = "";
    setComposerTick((n) => n + 1);

    const replySnap = replyingTo;
    setReplyingTo(null);

    const wsBody: Record<string, unknown> = { type: "message", content: text };
    if (replySnap?.id) wsBody.reply_to_message_id = replySnap.id;

    const sock = socketRef.current;
    if (sock && sock.readyState === WebSocket.OPEN) {
      const me = jwtSub(getAccess());
      const pendingId =
        "pending-" + (globalThis.crypto?.randomUUID?.() || String(Date.now()));
      setMessages((msgs) => [
        ...msgs,
        {
          id: pendingId,
          sender_id: me!,
          message_type: "text",
          content: text,
          created_at: new Date().toISOString(),
          _pending: true,
          _replyToId: replySnap?.id || null,
          read_by: [],
          reactions: [],
          reply_to_message_id: replySnap?.id,
          reply_to: replySnap?.reply_to,
        },
      ]);
      sock.send(JSON.stringify(wsBody));
      return;
    }
    const idem =
      globalThis.crypto?.randomUUID?.() ||
      "web-" + Date.now() + "-" + Math.random().toString(36).slice(2);
    const body: Record<string, unknown> = {
      chat_id: activeChatId,
      content: text,
      idempotency_key: idem,
    };
    if (replySnap?.id) body.reply_to_message_id = replySnap.id;
    const r = await api("POST", "/messages", body);
    const d = r.data as Message | { error?: string };
    if (r.ok && d && "sender_id" in d) {
      setMessages((msgs) => [
        ...msgs,
        {
          id: d.id,
          sender_id: d.sender_id,
          message_type: d.message_type || "text",
          content: d.content,
          created_at: d.created_at,
          read_by: d.read_by || [],
          reply_to_message_id: d.reply_to_message_id,
          reply_to: d.reply_to,
          reactions: d.reactions || [],
        },
      ]);
    }
  }, [activeChatId, replyingTo]);

  const fileInputRef = useRef<HTMLInputElement>(null);
  const [fileBusy, setFileBusy] = useState(false);
  const [attachmentDraft, setAttachmentDraft] = useState<{
    file: File;
    previewUrl: string | null;
  } | null>(null);

  const clearAttachmentDraft = useCallback(() => {
    setAttachmentDraft((prev) => {
      if (prev?.previewUrl) URL.revokeObjectURL(prev.previewUrl);
      return null;
    });
    if (fileInputRef.current) fileInputRef.current.value = "";
  }, []);

  useEffect(() => {
    setAttachmentDraft((prev) => {
      if (prev?.previewUrl) URL.revokeObjectURL(prev.previewUrl);
      return null;
    });
    if (fileInputRef.current) fileInputRef.current.value = "";
  }, [activeChatId]);

  const sendSelectedFile = useCallback(
    async (file: File) => {
      if (!activeChatId) return;
      const mime = file.type?.trim() || guessMimeFromName(file.name);
      if (!mime) {
        alert("Could not detect file type; try a common image or document format.");
        return;
      }
      setFileBusy(true);
      try {
        const replySnap = replyingTo;
        setReplyingTo(null);
        const idem =
          globalThis.crypto?.randomUUID?.() ||
          "file-" + Date.now() + "-" + Math.random().toString(36).slice(2);
        const pres = await api(
          "POST",
          "/chats/" + encodeURIComponent(activeChatId) + "/attachments/presign",
          {
            filename: file.name,
            content_type: mime,
            size_bytes: file.size,
          }
        );
        const pe = pres.data as {
          error?: string;
          upload_url?: string;
          object_key?: string;
          headers?: Record<string, string>;
        };
        if (!pres.ok) {
          alert(typeof pe?.error === "string" ? pe.error : "Could not start upload.");
          return;
        }
        const h = new Headers();
        if (pe.headers) {
          for (const [k, v] of Object.entries(pe.headers)) {
            if (v) h.set(k, v);
          }
        }
        const put = await fetch(pe.upload_url!, { method: "PUT", headers: h, body: file });
        if (!put.ok) {
          alert("Upload failed (" + put.status + ").");
          return;
        }
        const body: Record<string, unknown> = {
          chat_id: activeChatId,
          message_type: "file",
          file: {
            object_key: pe.object_key,
            filename: file.name,
            mime_type: mime,
            size_bytes: file.size,
          },
          idempotency_key: idem,
        };
        if (replySnap?.id) body.reply_to_message_id = replySnap.id;
        const msg = await api("POST", "/messages", body);
        const d = msg.data as Message | { error?: string };
        if (msg.ok && d && "sender_id" in d) {
          clearAttachmentDraft();
          setMessages((msgs) => [
            ...msgs,
            {
              id: d.id,
              sender_id: d.sender_id,
              message_type: d.message_type || "file",
              content: d.content,
              created_at: d.created_at,
              read_by: d.read_by || [],
              reply_to_message_id: d.reply_to_message_id,
              reply_to: d.reply_to,
              reactions: d.reactions || [],
            },
          ]);
        } else {
          alert((d as { error?: string })?.error || "Could not save attachment message.");
        }
      } finally {
        setFileBusy(false);
      }
    },
    [activeChatId, replyingTo, clearAttachmentDraft]
  );

  const emitTyping = useCallback(() => {
    const sock = socketRef.current;
    if (!sock || sock.readyState !== WebSocket.OPEN || !activeChatId) return;
    try {
      sock.send(JSON.stringify({ type: "typing", chat_id: activeChatId }));
    } catch {
      /* ignore */
    }
  }, [activeChatId]);

  const sendTyping = useCallback(() => {
    const sock = socketRef.current;
    if (!sock || sock.readyState !== WebSocket.OPEN || !activeChatId) return;
    if (!typingBurstOpenRef.current) {
      typingBurstOpenRef.current = true;
      emitTyping();
      setTimeout(() => {
        typingBurstOpenRef.current = false;
      }, 2500);
    }
    if (typingDebounceRef.current) clearTimeout(typingDebounceRef.current);
    typingDebounceRef.current = setTimeout(emitTyping, 180);
  }, [activeChatId, emitTyping]);

  const loadMe = useCallback(async () => {
    const sub = jwtSub(getAccess());
    if (!sub) return;
    const r = await api("GET", "/users/" + encodeURIComponent(sub));
    const d = r.data as { username?: string; email?: string };
    if (r.ok && d) {
      const un = d.username || d.email || "User";
      setSidebarUser(d.username || d.email || sub.slice(0, 8));
      setSidebarUserSub(d.email || "");
      setSidebarAvatar(un.slice(0, 1).toUpperCase());
    } else {
      setSidebarUser("Account");
      setSidebarUserSub(sub.slice(0, 13) + "…");
      setSidebarAvatar("U");
    }
  }, []);

  const stopHeartbeat = useCallback(() => {
    if (heartbeatTimerRef.current) {
      clearInterval(heartbeatTimerRef.current);
      heartbeatTimerRef.current = null;
    }
  }, []);

  const startHeartbeat = useCallback(() => {
    stopHeartbeat();
    heartbeatTimerRef.current = setInterval(async () => {
      const sub = jwtSub(getAccess());
      if (!sub) return;
      await api("POST", "/users/" + encodeURIComponent(sub) + "/heartbeat", {});
    }, 45000);
  }, [stopHeartbeat]);

  const enterApp = useCallback(() => {
    setView("main");
    void loadMe();
    void loadFriends();
    void loadChats();
    startHeartbeat();
  }, [loadChats, loadFriends, loadMe, startHeartbeat]);

  const leaveApp = useCallback(() => {
    stopHeartbeat();
    disconnectWS();
    disconnectSignaling();
    clearTokens();
    setActiveChatId(null);
    setActiveChatMeta(null);
    setDirectPeerUserId(null);
    setReplyingTo(null);
    setMessages([]);
    setShowThread(false);
    setSidebarOpen(false);
    setVoiceCallEndedReason(null);
    setVoiceEndedDurationSnapshot(null);
    setVoicePeerLabel("");
    setVoiceMuted(false);
    setView("auth");
  }, [disconnectWS, disconnectSignaling, stopHeartbeat]);

  useEffect(() => {
    setApiBaseField(getBase());
    if (getAccess() && jwtSub(getAccess())) {
      enterApp();
    }
    return () => stopHeartbeat();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- mount only
  }, []);

  const submitLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    const r = await api("POST", "/auth/login", {
      email: loginEmail.trim(),
      password: loginPass,
    });
    const d = r.data as { access_token?: string; refresh_token?: string; error?: unknown };
    if (!r.ok) {
      setAuthError(
        typeof d?.error === "string"
          ? d.error
          : d?.error
            ? JSON.stringify(d.error)
            : "Login failed"
      );
      return;
    }
    if (!d.access_token) {
      setAuthError("Invalid login response from server");
      return;
    }
    setTokens(d.access_token, d.refresh_token || "");
    setAuthError("");
    enterApp();
  };

  const submitRegister = async (e: React.FormEvent) => {
    e.preventDefault();
    const r = await api("POST", "/auth/register", {
      email: regEmail.trim(),
      username: regUser.trim(),
      password: regPass,
    });
    const d = r.data as { access_token?: string; refresh_token?: string; error?: unknown };
    if (!r.ok) {
      setAuthError(
        typeof d?.error === "string"
          ? d.error
          : d?.error
            ? JSON.stringify(d.error)
            : "Register failed"
      );
      return;
    }
    if (!d.access_token) {
      setAuthError("Invalid register response from server");
      return;
    }
    setTokens(d.access_token, d.refresh_token || "");
    setAuthError("");
    enterApp();
  };

  const setReplyTo = useCallback((msg: Message) => {
    if (!msg?.id || msg._pending || String(msg.id).startsWith("pending-")) return;
    const me = jwtSub(getAccess());
    const fromLabel =
      msg.sender_id === me ? "You" : (msg.sender_id || "").slice(0, 8) + "…";
    setReplyingTo({
      id: String(msg.id),
      fromLabel,
      reply_to: {
        id: String(msg.id),
        sender_id: msg.sender_id,
        content: truncateText(messagePreviewText(msg), 280),
        created_at: msg.created_at || new Date().toISOString(),
      },
    });
  }, []);

  const meSub = jwtSub(getAccess());

  const voiceDurationLabel = useMemo(() => {
    const now = Date.now();
    if (voicePhase === "active" && voiceActiveStartRef.current != null) {
      return fmtVoiceDuration(now - voiceActiveStartRef.current);
    }
    if (
      (voicePhase === "outgoing" ||
        voicePhase === "incoming" ||
        voicePhase === "connecting") &&
      voiceRingStartRef.current != null
    ) {
      return fmtVoiceDuration(now - voiceRingStartRef.current);
    }
    return "0:00";
  }, [voicePhase, voiceTimerTick]);

  const showVoiceCallOverlay =
    voicePhase !== "idle" || voiceCallEndedReason !== null;
  const voicePeerInitial =
    (voicePeerLabel || "?").trim().slice(0, 1).toUpperCase() || "?";

  const receiptMarkup = (m: Message) => {
    if (m._pending || !m.id || String(m.id).startsWith("pending-")) {
      return <span className="msg-rcpt pending" title="Sending">…</span>;
    }
    const others = (m.read_by || []).filter((r) => r.user_id && r.user_id !== meSub);
    const isDirect = activeChatMeta?.type === "direct";
    const read =
      isDirect && directPeerUserId
        ? others.some((r) => r.user_id === directPeerUserId)
        : others.length > 0;
    const title = read ? "Read" : "Delivered";
    const sym = read ? "✓✓" : "✓";
    const cls = read ? "msg-rcpt read" : "msg-rcpt delivered";
    return (
      <span className={cls} title={title}>
        {sym}
      </span>
    );
  };

  const replyQuote = (m: Message) => {
    const rt = m.reply_to;
    if (!rt || !rt.id) return null;
    const who =
      rt.sender_id === meSub ? "You" : (rt.sender_id || "").slice(0, 8) + "…";
    return (
      <div className="bubble-quote">
        <span className="bubble-quote-author">{who}</span>
        <span className="bubble-quote-text">{truncateText(rt.content || "", 200)}</span>
      </div>
    );
  };

  const onSubmitGroup = async (e: React.FormEvent) => {
    e.preventDefault();
    const name = groupName.trim() || "Group";
    const member_ids = Object.entries(groupChecked)
      .filter(([, v]) => v)
      .map(([k]) => k);
    const r = await api("POST", "/chats", {
      type: "group",
      name,
      member_ids,
    });
    const d = r.data as { id?: string; error?: string };
    if (!r.ok) {
      alert(d?.error || "Could not create group");
      return;
    }
    closeModals();
    setGroupName("");
    setGroupChecked({});
    await loadChats();
    if (d.id) {
      await openChat(d.id, { id: d.id, type: "group", name });
    }
  };

  return (
    <>
      <div className={view === "auth" ? "view" : "view hidden"} data-view="auth">
        <div className="auth-layout">
          <div className="auth-brand">
            <div className="logo-mark">◉</div>
            <h1>Orbit Chat</h1>
            <p>Realtime messaging · JWT · WebSockets</p>
          </div>
          <div className="auth-card">
            <div className="auth-tabs">
              <button
                type="button"
                className={"auth-tab" + (authTab === "login" ? " active" : "")}
                onClick={() => setAuthTab("login")}
              >
                Sign in
              </button>
              <button
                type="button"
                className={"auth-tab" + (authTab === "register" ? " active" : "")}
                onClick={() => setAuthTab("register")}
              >
                Create account
              </button>
            </div>
            <p className="auth-error" role="alert">
              {authError}
            </p>
            <form
              className={"auth-panel" + (authTab !== "login" ? " hidden" : "")}
              onSubmit={submitLogin}
            >
              <label>Email</label>
              <input
                type="email"
                required
                autoComplete="email"
                placeholder="you@example.com"
                value={loginEmail}
                onChange={(e) => setLoginEmail(e.target.value)}
              />
              <label>Password</label>
              <input
                type="password"
                required
                autoComplete="current-password"
                placeholder="••••••••"
                value={loginPass}
                onChange={(e) => setLoginPass(e.target.value)}
              />
              <button type="submit" className="btn primary full">
                Sign in
              </button>
            </form>
            <form
              className={"auth-panel" + (authTab !== "register" ? " hidden" : "")}
              onSubmit={submitRegister}
            >
              <label>Email</label>
              <input
                type="email"
                required
                autoComplete="email"
                value={regEmail}
                onChange={(e) => setRegEmail(e.target.value)}
              />
              <label>Username</label>
              <input
                type="text"
                required
                autoComplete="username"
                value={regUser}
                onChange={(e) => setRegUser(e.target.value)}
              />
              <label>Password (min 8)</label>
              <input
                type="password"
                required
                minLength={8}
                autoComplete="new-password"
                value={regPass}
                onChange={(e) => setRegPass(e.target.value)}
              />
              <button type="submit" className="btn primary full">
                Create account
              </button>
            </form>
            <p className="auth-hint">
              Gateway: <code>http://localhost:8080</code> — override in Settings after sign-in.
            </p>
          </div>
        </div>
      </div>

      <div className={view === "main" ? "view" : "view hidden"} data-view="main">
        <audio ref={remoteAudioRef} autoPlay playsInline className="hidden" aria-hidden />
        {voiceToast ? (
          <div className="voice-toast" role="status">
            {voiceToast}
          </div>
        ) : null}
        {showVoiceCallOverlay ? (
          <div
            className="voice-modal-backdrop"
            role="dialog"
            aria-modal="true"
            aria-label="Voice call"
          >
            <div className="voice-modal voice-modal-lg">
              {callVideoMode && !voiceCallEndedReason ? (
                <div className="voice-call-video-stage">
                  <video
                    ref={remoteVideoRef}
                    className="voice-remote-video"
                    autoPlay
                    playsInline
                  />
                  <video
                    ref={localVideoRef}
                    className="voice-local-pip"
                    autoPlay
                    playsInline
                    muted
                  />
                </div>
              ) : null}
              <div className="voice-call-hero">
                {!callVideoMode || voiceCallEndedReason ? (
                  <div className="voice-avatar-xl" aria-hidden>
                    {voicePeerInitial}
                  </div>
                ) : null}
                <h2 className="voice-call-name">{voicePeerLabel || "Unknown"}</h2>
                {voiceCallEndedReason ? (
                  <>
                    <p className="voice-call-status voice-call-status-ended">
                      {voiceCallEndLine(voiceCallEndedReason)}
                    </p>
                    <p className="voice-call-timer-label">Duration</p>
                    <p className="voice-call-timer voice-call-timer-muted">
                      {voiceEndedDurationSnapshot ?? voiceDurationLabel}
                    </p>
                    <div className="voice-modal-actions voice-modal-actions-center">
                      <button
                        type="button"
                        className="btn primary"
                        onClick={() => {
                          if (voiceEndedDismissTimerRef.current) {
                            clearTimeout(voiceEndedDismissTimerRef.current);
                            voiceEndedDismissTimerRef.current = null;
                          }
                          setVoiceCallEndedReason(null);
                          setVoiceEndedDurationSnapshot(null);
                          setVoicePeerLabel("");
                        }}
                      >
                        Dismiss
                      </button>
                    </div>
                  </>
                ) : (
                  <>
                    <p className="voice-call-status">
                      {voiceCallStatusLine(voicePhase, voiceMuted, callVideoMode)}
                    </p>
                    <p className="voice-call-timer-label">
                      {voicePhase === "active" ? "Call duration" : "Time"}
                    </p>
                    <p className="voice-call-timer">{voiceDurationLabel}</p>
                    <div className="voice-modal-actions voice-modal-actions-spread">
                      {voicePhase === "incoming" ? (
                        <>
                          <button
                            type="button"
                            className="btn voice-btn-decline"
                            onClick={() => {
                              const m = voiceRef.current?.getPendingIncoming();
                              if (m) voiceRef.current?.rejectIncoming(m.callId, m.peerId);
                            }}
                          >
                            Decline
                          </button>
                          <button
                            type="button"
                            className="btn voice-btn-accept"
                            onClick={() => {
                              const m = voiceRef.current?.getPendingIncoming();
                              if (m) {
                                void voiceRef.current?.acceptIncoming(
                                  m.callId,
                                  m.peerId,
                                  voicePeerLabel
                                );
                              }
                            }}
                          >
                            Accept
                          </button>
                        </>
                      ) : null}
                      {voicePhase === "outgoing" || voicePhase === "connecting" ? (
                        <button
                          type="button"
                          className="btn voice-btn-end wide"
                          onClick={() => voiceRef.current?.endCall()}
                        >
                          End call
                        </button>
                      ) : null}
                      {voicePhase === "active" ? (
                        <>
                          <button
                            type="button"
                            className={
                              "btn voice-btn-mute" + (voiceMuted ? " muted-on" : "")
                            }
                            onClick={() => setVoiceMuted((m) => !m)}
                          >
                            {voiceMuted ? "Unmute" : "Mute"}
                          </button>
                          {callVideoMode ? (
                            <button
                              type="button"
                              className={
                                "btn voice-btn-mute" + (voiceCameraOff ? " muted-on" : "")
                              }
                              onClick={() => setVoiceCameraOff((v) => !v)}
                            >
                              {voiceCameraOff ? "Cam on" : "Cam off"}
                            </button>
                          ) : null}
                          <button 
                            type="button"
                            className="btn voice-btn-end"
                            onClick={() => voiceRef.current?.endCall()}
                          >
                            End
                          </button>
                        </>
                      ) : null}
                    </div>
                  </>
                )}
              </div>
            </div>
          </div>
        ) : null}
        <div className="app-shell">
          <button
            type="button"
            className="sidebar-toggle"
            aria-label="Toggle chats"
            onClick={() => setSidebarOpen((o) => !o)}
          >
            ☰
          </button>

          <aside className={"sidebar" + (sidebarOpen ? " open" : "")}>
            <header className="sidebar-head">
              <div className="sidebar-brand">
                <span className="logo-mark sm">◉</span>
                <span>Chats</span>
              </div>
              <div className="sidebar-actions">
                <button
                  type="button"
                  className="btn icon"
                  title="Refresh"
                  onClick={() => void loadChats()}
                >
                  ↻
                </button>
                <button
                  type="button"
                  className="btn icon"
                  title="Settings"
                  onClick={() => setModalSettings(true)}
                >
                  ⚙
                </button>
              </div>
            </header>

            <div className="sidebar-user">
              <div className="avatar" aria-hidden>
                {sidebarAvatar}
              </div>
              <div className="sidebar-user-text">
                <strong>{sidebarUser}</strong>
                <span>{sidebarUserSub}</span>
              </div>
              <button type="button" className="btn text" onClick={leaveApp}>
                Log out
              </button>
            </div>

            <div className="new-chat-row">
              <button
                type="button"
                className="btn secondary small"
                onClick={() => {
                  setDirectInput("");
                  setDirectResults([]);
                  setDirectHint("Start typing to search.");
                  setModalDirect(true);
                }}
              >
                + Chat
              </button>
              <button
                type="button"
                className="btn secondary small"
                onClick={() => {
                  populateGroupFriendPick();
                  setModalGroup(true);
                }}
              >
                + Group
              </button>
            </div>

            <div className="friends-panel">
              <div className="friends-head">
                <span>Friends</span>
                <button
                  type="button"
                  className="btn icon tiny"
                  title="Refresh friends"
                  onClick={() => void loadFriends()}
                >
                  ↻
                </button>
              </div>
              <ul className="friend-list">
                {friends.map((f) => {
                  const label = f.username || f.email || f.id.slice(0, 8);
                  return (
                    <li key={f.id}>
                      <div className="friend-meta">
                        <strong>{f.username || "User"}</strong>
                        <span>{f.email || ""}</span>
                      </div>
                      <div className="friend-actions">
                        <button
                          type="button"
                          className="btn secondary small"
                          title="Voice call"
                          onClick={() => startCall(f.id, label, false)}
                        >
                          Call
                        </button>
                        <button
                          type="button"
                          className="btn secondary small"
                          title="Video call"
                          onClick={() => startCall(f.id, label, true)}
                        >
                          Video
                        </button>
                        <button
                          type="button"
                          className="btn secondary small"
                          onClick={() => void openDirectWithFriend(f.id, label)}
                        >
                          Message
                        </button>
                      </div>
                    </li>
                  );
                })}
              </ul>
              <p className={"friends-empty muted" + (friends.length ? " hidden" : "")}>
                No friends yet — use Find people.
              </p>
            </div>

            <ul className="chat-list">
              {chats.length === 0 ? (
                <li className="chat-item empty">No chats yet</li>
              ) : (
                chats.map((c) => (
                  <li
                    key={c.id}
                    className={"chat-item" + (c.id === activeChatId ? " active" : "")}
                    onClick={() => c.id && void openChat(c.id, c)}
                  >
                    <span className="chat-title">
                      {chatLabel(c, peerLabelCacheRef.current)}
                    </span>
                    <span className="chat-sub">{c.type}</span>
                  </li>
                ))
              )}
            </ul>
          </aside>

          <main
            className="main-pane"
            onClick={() => {
              if (typeof window !== "undefined" && window.matchMedia("(max-width: 900px)").matches) {
                setSidebarOpen(false);
              }
            }}
          >
            <div className={showThread ? "thread-empty hidden" : "thread-empty"}>
              <div className="empty-illu">💬</div>
              <h2>Select a conversation</h2>
              <p>
                Use <strong>+ Chat</strong> to find people by email or username, add friends, and open
                DMs. Groups can include your friends only.
              </p>
            </div>

            <div className={showThread ? "thread-panel" : "thread-panel hidden"}>
              <header className="thread-header">
                <div className="thread-title-block">
                  <h2>{threadTitle}</h2>
                  <p className="thread-sub">{threadSubtitle}</p>
                  <p
                    className={
                      "typing-bar" +
                      (!typingBar.show ? " hidden" : "") +
                      (typingBar.live ? " typing-live" : "")
                    }
                  >
                    {typingBar.text}
                  </p>
                </div>
                <div className="thread-header-right">
                  {directPeerUserId && activeChatMeta?.type === "direct" ? (
                    <>
                      <button
                        type="button"
                        className="btn secondary small thread-call-btn"
                        title="Voice call"
                        onClick={() =>
                          startCall(directPeerUserId, threadTitle || "Friend", false)
                        }
                      >
                        Call
                      </button>
                      <button
                        type="button"
                        className="btn secondary small thread-call-btn"
                        title="Video call"
                        onClick={() =>
                          startCall(directPeerUserId, threadTitle || "Friend", true)
                        }
                      >
                        Video
                      </button>
                    </>
                  ) : null}
                  <span
                    className={"ws-dot " + wsStatus}
                    title="Realtime connection"
                  />
                  <div className="thread-menu-wrap">
                    <button
                      type="button"
                      className="btn icon"
                      aria-haspopup="true"
                      onClick={() => setThreadMenuOpen((o) => !o)}
                    >
                      ⋮
                    </button>
                    <div className={"dropdown" + (threadMenuOpen ? " open" : "")}>
                      <button
                        type="button"
                        style={{ display: activeChatMeta?.type === "group" ? undefined : "none" }}
                        onClick={async () => {
                          if (!activeChatId) return;
                          if (activeChatMeta?.type !== "group") {
                            alert("Add member is only for group chats.");
                            return;
                          }
                          await populateAddMemberPick();
                          setModalAddMember(true);
                          setThreadMenuOpen(false);
                        }}
                      >
                        Add friend to group
                      </button>
                    </div>
                  </div>
                </div>
              </header>

              <div className="message-scroll">
                <div
                  className="message-list"
                  ref={messageListRef}
                  onClick={(e) => {
                    const t = e.target as HTMLElement;
                    const pick = t.closest("[data-react-pick]");
                    if (pick) {
                      const mid = pick.getAttribute("data-msg-id");
                      const em = pick.getAttribute("data-react-pick");
                      if (mid && em) sendReaction(mid, em, true);
                      return;
                    }
                    const chip = t.closest("[data-react-emoji]");
                    if (chip) {
                      const mid = chip.getAttribute("data-msg-id");
                      const em = chip.getAttribute("data-react-emoji");
                      const me = jwtSub(getAccess());
                      const msg = messages.find((x) => String(x.id) === String(mid));
                      const r = msg?.reactions?.find((x) => x.emoji === em);
                      const mine = me && r?.user_ids?.includes(me);
                      if (mid && em) sendReaction(mid, em, !mine);
                      return;
                    }
                    const btn = t.closest("[data-reply-id]");
                    if (!btn) return;
                    const id = btn.getAttribute("data-reply-id");
                    const msg = messages.find((x) => String(x.id) === String(id));
                    if (msg) setReplyTo(msg);
                  }}
                >
                  {messages.map((m) => {
                    const mine = m.sender_id === meSub;
                    const t = m.created_at ? fmtTime(m.created_at) : "";
                    const canReply =
                      Boolean(m.id) && !m._pending && !String(m.id).startsWith("pending-");
                    const list = m.reactions || [];
                    const linkPreviewUrl =
                      m.message_type === "file" || parseFilePayload(m.content || "")
                        ? null
                        : firstHttpUrl(m.content || "");
                    return (
                      <div key={String(m.id) + (m._pending ? "-p" : "")} className={"bubble-row " + (mine ? "mine" : "theirs")}>
                        <div className="bubble">
                          <div className="bubble-meta">
                            {mine ? "You" : (m.sender_id || "").slice(0, 8) + "…"} · {t}
                            {mine ? receiptMarkup(m) : null}
                            {canReply ? (
                              <button
                                type="button"
                                className="reply-pill"
                                data-reply-id={String(m.id)}
                              >
                                Reply
                              </button>
                            ) : null}
                          </div>
                          {replyQuote(m)}
                          {m.message_type === "file" || parseFilePayload(m.content || "") ? (
                            (() => {
                              const fp = parseFilePayload(m.content || "");
                              if (!fp) {
                                return <div className="bubble-text">{m.content || ""}</div>;
                              }
                              const isImg = (fp.mime_type || "").startsWith("image/");
                              return (
                                <div className="bubble-file">
                                  {isImg ? (
                                    <a
                                      href={fp.download_url}
                                      target="_blank"
                                      rel="noreferrer"
                                      className="bubble-file-thumb"
                                    >
                                      <img src={fp.download_url} alt={fp.filename || "attachment"} />
                                    </a>
                                  ) : null}
                                  <div className="bubble-file-row">
                                    <a
                                      href={fp.download_url}
                                      target="_blank"
                                      rel="noreferrer"
                                      className="bubble-file-link"
                                    >
                                      📎 {fp.filename || "Download"}
                                    </a>
                                    {fp.size_bytes != null ? (
                                      <span className="bubble-file-meta">
                                        {" "}
                                        · {(fp.size_bytes / 1024).toFixed(1)} KB
                                      </span>
                                    ) : null}
                                  </div>
                                </div>
                              );
                            })()
                          ) : (
                            <>
                              <div className="bubble-text">{m.content || ""}</div>
                              {linkPreviewUrl ? (
                                <LinkPreviewCard key={String(m.id) + "-og"} url={linkPreviewUrl} />
                              ) : null}
                            </>
                          )}
                          {m.id && !String(m.id).startsWith("pending-") ? (
                            <div className="bubble-reactions">
                              {list.map((r) => {
                                const n = (r.user_ids || []).length;
                                const mineR = meSub && (r.user_ids || []).includes(meSub);
                                return (
                                  <button
                                    key={r.emoji}
                                    type="button"
                                    className={"react-chip" + (mineR ? " has-mine" : "")}
                                    data-react-emoji={r.emoji}
                                    data-msg-id={String(m.id)}
                                  >
                                    {r.emoji}
                                    <span className="react-n">{n}</span>
                                  </button>
                                );
                              })}
                              {QUICK_REACTIONS.filter((em) => !list.some((x) => x.emoji === em)).map(
                                (em) => (
                                  <button
                                    key={em}
                                    type="button"
                                    className="react-chip pick"
                                    data-react-pick={em}
                                    data-msg-id={String(m.id)}
                                  >
                                    {em}
                                  </button>
                                )
                              )}
                            </div>
                          ) : null}
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>

              <footer className="composer">
                <div
                  className={"composer-reply" + (replyingTo ? "" : " hidden")}
                  aria-live="polite"
                >
                  <div className="composer-reply-line">
                    <div className="composer-reply-text">
                      <span className="composer-reply-label">
                        {replyingTo ? "Replying to " + replyingTo.fromLabel : ""}
                      </span>
                      <span className="composer-reply-snippet">
                        {replyingTo ? truncateText(replyingTo.reply_to.content, 100) : ""}
                      </span>
                    </div>
                    <button type="button" className="btn text tiny" onClick={() => setReplyingTo(null)}>
                      Cancel
                    </button>
                  </div>
                </div>
                <div
                  className={
                    "composer-attachment-draft" + (attachmentDraft ? "" : " hidden")
                  }
                  aria-live="polite"
                >
                  {attachmentDraft ? (
                    <>
                      <div className="composer-attachment-preview">
                        {attachmentDraft.previewUrl ? (
                          <img
                            src={attachmentDraft.previewUrl}
                            alt=""
                            className="composer-attachment-img"
                          />
                        ) : (
                          <div className="composer-attachment-file">
                            <span className="composer-attachment-name">
                              📎 {attachmentDraft.file.name}
                            </span>
                            <span className="composer-attachment-size muted small">
                              {(attachmentDraft.file.size / 1024).toFixed(1)} KB
                            </span>
                          </div>
                        )}
                      </div>
                      <div className="composer-attachment-actions">
                        <button
                          type="button"
                          className="btn secondary"
                          disabled={fileBusy}
                          onClick={clearAttachmentDraft}
                        >
                          Cancel
                        </button>
                        <button
                          type="button"
                          className="btn primary"
                          disabled={fileBusy}
                          onClick={() => void sendSelectedFile(attachmentDraft.file)}
                        >
                          {fileBusy ? "Sending…" : "Send"}
                        </button>
                      </div>
                    </>
                  ) : null}
                </div>
                <div className="composer-row">
                  <input
                    ref={fileInputRef}
                    type="file"
                    className="visually-hidden"
                    aria-hidden
                    tabIndex={-1}
                    onChange={(e) => {
                      const f = e.target.files?.[0];
                      if (!f) return;
                      setAttachmentDraft((prev) => {
                        if (prev?.previewUrl) URL.revokeObjectURL(prev.previewUrl);
                        const isImg =
                          (f.type && f.type.startsWith("image/")) ||
                          /\.(png|jpe?g|gif|webp|bmp|svg)$/i.test(f.name);
                        return {
                          file: f,
                          previewUrl: isImg ? URL.createObjectURL(f) : null,
                        };
                      });
                    }}
                  />
                  <button
                    type="button"
                    className="btn secondary icon-attach"
                    disabled={!activeChatId || fileBusy || !!attachmentDraft}
                    title="Attach file"
                    onClick={() => fileInputRef.current?.click()}
                  >
                    +
                  </button>
                  <textarea
                    key={composerTick}
                    defaultValue=""
                    rows={1}
                    placeholder="Message… (Enter to send, Shift+Enter newline)"
                    onChange={(e) => {
                      composerTextRef.current = e.target.value;
                    }}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" && !e.shiftKey) {
                        e.preventDefault();
                        void sendText();
                        return;
                      }
                      if (e.key === "Backspace" || e.key.length === 1) sendTyping();
                    }}
                    onInput={() => sendTyping()}
                  />
                  <button type="button" className="btn primary" onClick={() => void sendText()}>
                    Send
                  </button>
                </div>
              </footer>
            </div>
          </main>
        </div>
      </div>

      <div className={"modal" + (modalSettings ? " open" : "")}>
        <div className="modal-backdrop" onClick={closeModals} />
        <div className="modal-card">
          <h3>Connection</h3>
          <p className="muted">API Gateway base URL</p>
          <input
            type="url"
            className="modal-input"
            value={apiBaseField}
            onChange={(e) => setApiBaseField(e.target.value)}
          />
          <div className="modal-actions">
            <button type="button" className="btn secondary" onClick={closeModals}>
              Cancel
            </button>
            <button
              type="button"
              className="btn primary"
              onClick={() => {
                setBase(apiBaseField);
                closeModals();
              }}
            >
              Save
            </button>
          </div>
        </div>
      </div>

      <div className={"modal" + (modalDirect ? " open" : "")}>
        <div className="modal-backdrop" onClick={closeModals} />
        <div className="modal-card modal-wide">
          <h3>Find people</h3>
          <p className="muted">
            Search by email or username (at least 2 characters). Add them as a friend — your direct
            chat opens automatically.
          </p>
          <input
            type="search"
            className="modal-input"
            autoComplete="off"
            placeholder="e.g. alex or alex@mail.com"
            value={directInput}
            onChange={(e) => {
              const v = e.target.value;
              setDirectInput(v);
              if (directSearchTimerRef.current) clearTimeout(directSearchTimerRef.current);
              directSearchTimerRef.current = setTimeout(() => runDirectSearch(v), 320);
            }}
          />
          <p className="muted small">{directHint}</p>
          <ul className="search-results" aria-live="polite">
            {directResults.map((u) => (
              <li key={u.id}>
                <div className="peer-meta">
                  <strong>@{u.username || ""}</strong>
                  <span>{u.email || ""}</span>
                </div>
                <button
                  type="button"
                  className="btn primary small"
                  onClick={() => void addFriendAndOpenChat(u.id, u.username, u.email)}
                >
                  Add &amp; chat
                </button>
              </li>
            ))}
          </ul>
          <div className="modal-actions">
            <button type="button" className="btn secondary" onClick={closeModals}>
              Close
            </button>
          </div>
        </div>
      </div>

      <div className={"modal" + (modalGroup ? " open" : "")}>
        <div className="modal-backdrop" onClick={closeModals} />
        <div className="modal-card modal-wide">
          <h3>New group</h3>
          <form onSubmit={onSubmitGroup}>
            <label>Group name</label>
            <input
              type="text"
              className="modal-input"
              placeholder="Team"
              value={groupName}
              onChange={(e) => setGroupName(e.target.value)}
            />
            <label>Friends in this group</label>
            <p className="muted small">{groupPickHint}</p>
            <div className="friend-pick-list">
              {groupPickFriends.map((f) => {
                if (f.id === meSub) return null;
                return (
                  <label key={f.id} className="friend-pick-row">
                    <input
                      type="checkbox"
                      name="groupFriend"
                      checked={!!groupChecked[f.id]}
                      onChange={(e) =>
                        setGroupChecked((prev) => ({ ...prev, [f.id]: e.target.checked }))
                      }
                    />
                    <span>
                      @{f.username || ""} · {f.email || ""}
                    </span>
                  </label>
                );
              })}
            </div>
            <div className="modal-actions">
              <button type="button" className="btn secondary" onClick={closeModals}>
                Cancel
              </button>
              <button type="submit" className="btn primary">
                Create
              </button>
            </div>
          </form>
        </div>
      </div>

      <div className={"modal" + (modalAddMember ? " open" : "")}>
        <div className="modal-backdrop" onClick={closeModals} />
        <div className="modal-card modal-wide">
          <h3>Add to group</h3>
          <p className="muted">Only friends can be added.</p>
          <div className="friend-pick-list">
            {addMemberCandidates.length === 0 ? (
              <p className="muted small">
                Everyone is already in this group or you have no other friends.
              </p>
            ) : (
              addMemberCandidates.map((f) => (
                <div key={f.id} className="friend-pick-row" style={{ cursor: "pointer" }}>
                  <span>
                    @{f.username || ""} · {f.email || ""}
                  </span>
                  <button
                    type="button"
                    className="btn primary small"
                    onClick={async () => {
                      if (!activeChatId) return;
                      const r = await api(
                        "POST",
                        "/chats/" + encodeURIComponent(activeChatId) + "/members",
                        { user_id: f.id }
                      );
                      const d = r.data as { error?: string };
                      if (!r.ok) alert(d?.error || "Failed");
                      else {
                        closeModals();
                        void loadChats();
                      }
                    }}
                  >
                    Add
                  </button>
                </div>
              ))
            )}
          </div>
          <div className="modal-actions">
            <button type="button" className="btn secondary" onClick={closeModals}>
              Cancel
            </button>
          </div>
        </div>
      </div>

      <DocumentClickCloseMenu
        open={threadMenuOpen}
        onClose={() => setThreadMenuOpen(false)}
      />
    </>
  );
}

function DocumentClickCloseMenu({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  useEffect(() => {
    if (!open) return;
    const h = (e: MouseEvent) => {
      const t = e.target as HTMLElement;
      if (!t.closest(".thread-menu-wrap")) onClose();
    };
    document.addEventListener("click", h);
    return () => document.removeEventListener("click", h);
  }, [open, onClose]);
  return null;
}
