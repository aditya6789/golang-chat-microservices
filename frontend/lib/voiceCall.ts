const ICE: RTCIceServer[] = [{ urls: "stun:stun.l.google.com:19302" }];

export type VoicePhase = "idle" | "outgoing" | "incoming" | "connecting" | "active";

/** Shown briefly when the call session ends (passed with phase `idle`). */
export type VoiceEndReason =
  | "you_ended"
  | "peer_ended"
  | "peer_declined"
  | "declined_incoming"
  | "failed"
  | "error";

export type VoicePhaseDetail = {
  peerId?: string;
  callId?: string;
  peerLabel?: string;
  /** True when this session includes camera (1:1 video call). */
  video?: boolean;
  endReason?: VoiceEndReason;
};

const VIDEO_CONSTRAINTS: MediaTrackConstraints = {
  facingMode: "user",
  width: { ideal: 1280 },
  height: { ideal: 720 },
};

export class VoiceCallManager {
  private phase: VoicePhase = "idle";
  private callId: string | null = null;
  private peerId: string | null = null;
  private pc: RTCPeerConnection | null = null;
  private localStream: MediaStream | null = null;
  private pendingIce: RTCIceCandidateInit[] = [];
  /** Whether this call uses the camera (negotiated via `video` on invite). */
  private videoCall = false;

  constructor(
    private sendRaw: (o: Record<string, unknown>) => void,
    private onRemoteStream: (s: MediaStream | null) => void,
    private onPhase: (p: VoicePhase, d?: VoicePhaseDetail) => void,
    private onError: (msg: string) => void,
    private onLocalStream?: (s: MediaStream | null) => void
  ) {}

  getPhase(): VoicePhase {
    return this.phase;
  }

  getLocalStream(): MediaStream | null {
    return this.localStream;
  }

  /** When phase is `incoming`, returns ids for the incoming call UI. */
  getPendingIncoming(): { callId: string; peerId: string; video: boolean } | null {
    if (this.phase !== "incoming" || !this.callId || !this.peerId) return null;
    return { callId: this.callId, peerId: this.peerId, video: this.videoCall };
  }

  isVideoCall(): boolean {
    return this.videoCall;
  }

  setMuted(muted: boolean): void {
    this.localStream?.getAudioTracks().forEach((t) => {
      t.enabled = !muted;
    });
  }

  /** Turn local camera on/off (video calls only). */
  setCameraEnabled(enabled: boolean): void {
    this.localStream?.getVideoTracks().forEach((t) => {
      t.enabled = enabled;
    });
  }

  private emitLocal(): void {
    this.onLocalStream?.(this.localStream);
  }

  private cleanup(endReason?: VoiceEndReason): void {
    const lastPeer = this.peerId;
    if (this.pc) {
      this.pc.ontrack = null;
      this.pc.onicecandidate = null;
      this.pc.onconnectionstatechange = null;
      this.pc.close();
      this.pc = null;
    }
    if (this.localStream) {
      this.localStream.getTracks().forEach((t) => t.stop());
      this.localStream = null;
    }
    this.pendingIce = [];
    this.callId = null;
    this.peerId = null;
    this.phase = "idle";
    this.videoCall = false;
    this.onRemoteStream(null);
    this.emitLocal();
    this.onPhase("idle", endReason ? { endReason, peerId: lastPeer || undefined } : undefined);
  }

  async startOutgoing(peerId: string, peerLabel?: string, video = false): Promise<void> {
    if (this.phase !== "idle") return;
    const callId = crypto.randomUUID();
    this.callId = callId;
    this.peerId = peerId;
    this.videoCall = video;
    this.phase = "outgoing";
    this.onPhase("outgoing", { peerId, callId, peerLabel, video });
    try {
      this.localStream = await navigator.mediaDevices.getUserMedia({
        audio: true,
        video: video ? VIDEO_CONSTRAINTS : false,
      });
    } catch (e) {
      this.cleanup("failed");
      this.onError(
        e instanceof Error ? e.message : video ? "Camera / mic unavailable" : "Microphone unavailable"
      );
      throw e;
    }
    this.emitLocal();
    this.pc = new RTCPeerConnection({ iceServers: ICE });
    this.localStream.getTracks().forEach((t) => this.pc!.addTrack(t, this.localStream!));
    this.pc.ontrack = (ev) => {
      if (ev.streams[0]) this.onRemoteStream(ev.streams[0]);
    };
    this.wirePcState();
    this.wireIce(peerId, callId);
    const invite: Record<string, unknown> = {
      type: "call_invite",
      call_id: callId,
      to_user_id: peerId,
    };
    if (video) invite.video = true;
    this.sendRaw(invite);
  }

  private wirePcState(): void {
    this.pc!.onconnectionstatechange = () => {
      const st = this.pc?.connectionState;
      if (st === "connected" && this.phase !== "idle") {
        this.phase = "active";
        this.onPhase("active", {
          peerId: this.peerId!,
          callId: this.callId!,
          video: this.videoCall,
        });
      }
      if (st === "failed" || st === "closed") {
        this.cleanup("failed");
      }
    };
  }

  private wireIce(peerId: string, callId: string): void {
    this.pc!.onicecandidate = (ev) => {
      if (!ev.candidate || !this.callId) return;
      this.sendRaw({
        type: "call_ice",
        call_id: callId,
        to_user_id: peerId,
        candidate: ev.candidate.toJSON(),
      });
    };
  }

  async acceptIncoming(callId: string, fromUserId: string, peerLabel?: string): Promise<void> {
    if (this.phase !== "incoming" || this.callId !== callId) return;
    this.peerId = fromUserId;
    this.phase = "connecting";
    this.onPhase("connecting", {
      peerId: fromUserId,
      callId,
      peerLabel,
      video: this.videoCall,
    });
    try {
      this.localStream = await navigator.mediaDevices.getUserMedia({
        audio: true,
        video: this.videoCall ? VIDEO_CONSTRAINTS : false,
      });
    } catch (e) {
      this.sendRaw({ type: "call_reject", call_id: callId, to_user_id: fromUserId });
      this.cleanup("failed");
      this.onError(
        e instanceof Error ? e.message : this.videoCall ? "Camera / mic unavailable" : "Microphone unavailable"
      );
      throw e;
    }
    this.emitLocal();
    this.pc = new RTCPeerConnection({ iceServers: ICE });
    this.localStream.getTracks().forEach((t) => this.pc!.addTrack(t, this.localStream!));
    this.pc.ontrack = (ev) => {
      if (ev.streams[0]) this.onRemoteStream(ev.streams[0]);
    };
    this.wirePcState();
    this.wireIce(fromUserId, callId);
    this.sendRaw({ type: "call_accept", call_id: callId, to_user_id: fromUserId });
  }

  rejectIncoming(callId: string, fromUserId: string): void {
    if (this.phase !== "incoming" || this.callId !== callId) return;
    this.sendRaw({ type: "call_reject", call_id: callId, to_user_id: fromUserId });
    this.cleanup("declined_incoming");
  }

  endCall(): void {
    if (this.phase === "idle") return;
    const cid = this.callId;
    const pid = this.peerId;
    if (cid && pid && this.phase !== "incoming") {
      this.sendRaw({ type: "call_end", call_id: cid, to_user_id: pid });
    } else if (cid && pid && this.phase === "incoming") {
      this.sendRaw({ type: "call_reject", call_id: cid, to_user_id: pid });
    }
    this.cleanup("you_ended");
  }

  private async flushPendingIce(): Promise<void> {
    if (!this.pc) return;
    const copy = this.pendingIce.splice(0);
    for (const c of copy) {
      try {
        await this.pc.addIceCandidate(c);
      } catch {
        /* ignore */
      }
    }
  }

  private async addIceCandidate(c: RTCIceCandidateInit): Promise<void> {
    if (!this.pc) return;
    if (!this.pc.remoteDescription) {
      this.pendingIce.push(c);
      return;
    }
    try {
      await this.pc.addIceCandidate(c);
    } catch {
      /* ignore */
    }
  }

  async handleSignal(msg: Record<string, unknown>): Promise<void> {
    const type = msg.type as string;
    const from = msg.from_user_id as string | undefined;
    if (type === "error") {
      const m = (msg.message as string) || "Signaling error";
      this.onError(m);
      if (this.phase !== "idle") this.cleanup("error");
      return;
    }
    if (!from) return;

    if (type === "call_invite") {
      const cid = msg.call_id as string;
      if (!cid || this.phase !== "idle") return;
      this.videoCall = msg.video === true;
      this.phase = "incoming";
      this.callId = cid;
      this.peerId = from;
      this.onPhase("incoming", { peerId: from, callId: cid, video: this.videoCall });
      return;
    }

    const cid = msg.call_id as string;
    if (!cid || cid !== this.callId) return;

    if (type === "call_accept") {
      if (this.phase !== "outgoing" || from !== this.peerId || !this.pc) return;
      try {
        const offer = await this.pc.createOffer();
        await this.pc.setLocalDescription(offer);
        this.sendRaw({
          type: "call_offer",
          call_id: this.callId,
          to_user_id: this.peerId,
          sdp: { type: offer.type, sdp: offer.sdp || "" },
        });
      } catch {
        this.onError("Could not start media negotiation");
        this.cleanup("failed");
      }
      return;
    }

    if (type === "call_offer") {
      if (this.phase !== "connecting" || from !== this.peerId || !this.pc) return;
      const sdp = msg.sdp as RTCSessionDescriptionInit | undefined;
      if (!sdp?.sdp) return;
      try {
        await this.pc.setRemoteDescription(new RTCSessionDescription(sdp));
        await this.flushPendingIce();
        const answer = await this.pc.createAnswer();
        await this.pc.setLocalDescription(answer);
        this.sendRaw({
          type: "call_answer",
          call_id: this.callId,
          to_user_id: this.peerId,
          sdp: { type: answer.type, sdp: answer.sdp || "" },
        });
      } catch {
        this.onError("Could not answer call");
        this.cleanup("failed");
      }
      return;
    }

    if (type === "call_answer") {
      if ((this.phase !== "outgoing" && this.phase !== "connecting") || from !== this.peerId || !this.pc)
        return;
      const sdpAns = msg.sdp as RTCSessionDescriptionInit | undefined;
      if (!sdpAns?.sdp) return;
      try {
        await this.pc.setRemoteDescription(new RTCSessionDescription(sdpAns));
        await this.flushPendingIce();
      } catch {
        this.onError("Could not complete call");
        this.cleanup("failed");
      }
      return;
    }

    if (type === "call_ice") {
      if (from !== this.peerId) return;
      const cand = msg.candidate as RTCIceCandidateInit | undefined;
      if (!cand) return;
      await this.addIceCandidate(cand);
      return;
    }

    if (type === "call_reject") {
      if (from !== this.peerId) return;
      this.cleanup("peer_declined");
      return;
    }
    if (type === "call_end") {
      if (from !== this.peerId) return;
      this.cleanup("peer_ended");
      return;
    }
  }
}
