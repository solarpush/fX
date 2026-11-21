import { Injectable, OnDestroy, signal } from '@angular/core';
import { Observable, Subject } from 'rxjs';
import { environment } from '../../../environments/environment';
import { WSPreviewRequest, WSPreviewResponse } from '../models/studio.models';

export type WSConnectionState = 'disconnected' | 'connecting' | 'connected' | 'reconnecting';

interface LegacyPreviewMessage {
  type: string;
  data?: unknown;
  error?: string;
}

/**
 * Upgraded WebSocket client for fX v2 Live Preview (`/api/v1/ws/preview`).
 * Features requestId correlation, automatic exponential-backoff reconnect,
 * optional bearer token query support, heartbeat keep-alive, and pending
 * preview coalescing so rapid edits never drop the latest frame.
 */
@Injectable({
  providedIn: 'root',
})
export class Websocket implements OnDestroy {
  private socket: WebSocket | null = null;
  private readonly messageSubject = new Subject<WSPreviewResponse>();
  private readonly state = signal<WSConnectionState>('disconnected');

  private activeUrl = '';
  private token: string | null = null;
  private intentionalClose = false;
  private reconnectAttempts = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private pendingPreview: WSPreviewRequest | null = null;
  private reqCounter = 0;

  readonly connectionState = this.state.asReadonly();

  private resolveDefaultPreviewUrl(): string {
    if (typeof window === 'undefined') {
      return environment.wsUrl;
    }
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    return `${protocol}//${host}/api/v1/ws/preview`;
  }

  private buildUrlWithToken(baseUrl: string, token?: string | null): string {
    const t = token?.trim();
    if (!t) {
      return baseUrl;
    }
    const sep = baseUrl.includes('?') ? '&' : '?';
    return `${baseUrl}${sep}token=${encodeURIComponent(t)}`;
  }

  connect(url?: string, token?: string | null): void {
    const resolvedBase = url || this.resolveDefaultPreviewUrl();
    if (token !== undefined) {
      this.token = token;
    }

    const fullUrl = this.buildUrlWithToken(resolvedBase, this.token);
    if (
      this.socket &&
      this.activeUrl === fullUrl &&
      (this.socket.readyState === WebSocket.OPEN || this.socket.readyState === WebSocket.CONNECTING)
    ) {
      return;
    }

    this.clearTimers();
    if (this.socket) {
      this.intentionalClose = true;
      try {
        this.socket.close();
      } catch {
        // ignore
      }
      this.socket = null;
    }

    this.activeUrl = fullUrl;
    this.intentionalClose = false;
    this.state.set(this.reconnectAttempts > 0 ? 'reconnecting' : 'connecting');

    try {
      const ws = new WebSocket(fullUrl);
      this.socket = ws;

      ws.onopen = () => {
        if (this.socket !== ws) return;
        this.reconnectAttempts = 0;
        this.state.set('connected');
        this.startHeartbeat();

        if (this.pendingPreview) {
          const queued = this.pendingPreview;
          this.pendingPreview = null;
          this.sendRaw(queued);
        }
      };

      ws.onmessage = (event) => {
        if (this.socket !== ws) return;
        try {
          const msg = JSON.parse(event.data as string) as WSPreviewResponse;
          if (msg.type === 'pong') {
            return;
          }
          this.messageSubject.next(msg);
        } catch {
          // Ignore malformed frame
        }
      };

      ws.onerror = () => {
        if (this.socket !== ws) return;
        // Let onclose handle reconnect gracefully without noisy fatal errors
      };

      ws.onclose = () => {
        if (this.socket !== ws) return;
        this.socket = null;
        this.stopHeartbeat();

        if (this.intentionalClose) {
          this.state.set('disconnected');
          return;
        }
        this.scheduleReconnect();
      };
    } catch {
      this.scheduleReconnect();
    }
  }

  disconnect(): void {
    this.intentionalClose = true;
    this.pendingPreview = null;
    this.clearTimers();
    if (this.socket) {
      try {
        this.socket.close();
      } catch {
        // ignore
      }
      this.socket = null;
    }
    this.state.set('disconnected');
  }

  isConnected(): boolean {
    return this.socket?.readyState === WebSocket.OPEN;
  }

  nextRequestId(): string {
    this.reqCounter += 1;
    return `req-${Date.now()}-${this.reqCounter}`;
  }

  /**
   * Sends a v2 live preview request. Returns the assigned requestId and
   * whether it was sent immediately over an open socket (false means queued
   * while reconnecting or offline so caller may optionally HTTP-fallback).
   */
  sendPreview(req: Omit<WSPreviewRequest, 'type'>): { requestId: string; sent: boolean } {
    const requestId = req.requestId ?? this.nextRequestId();
    const payload: WSPreviewRequest = {
      ...req,
      type: 'preview',
      requestId,
    };

    if (this.socket?.readyState === WebSocket.OPEN) {
      this.sendRaw(payload);
      return { requestId, sent: true };
    }

    this.pendingPreview = payload;
    if (!this.socket && !this.intentionalClose) {
      this.connect(this.activeUrl || undefined);
    }
    return { requestId, sent: false };
  }

  send(message: WSPreviewRequest | LegacyPreviewMessage): void {
    this.sendRaw(message);
  }

  onMessage(): Observable<WSPreviewResponse> {
    return this.messageSubject.asObservable();
  }

  /** Legacy helper retained for backward compatibility. */
  compilePreview(typstCode: string): void {
    this.sendPreview({
      mode: 'pdf',
      kind: 'typst',
      template: typstCode,
    });
  }

  ngOnDestroy(): void {
    this.disconnect();
    this.messageSubject.complete();
  }

  private sendRaw(payload: unknown): void {
    if (this.socket?.readyState === WebSocket.OPEN) {
      try {
        this.socket.send(JSON.stringify(payload));
      } catch {
        // ignore broken pipe; onclose triggers reconnect
      }
    }
  }

  private startHeartbeat(): void {
    this.stopHeartbeat();
    this.heartbeatTimer = setInterval(() => {
      if (this.socket?.readyState === WebSocket.OPEN) {
        this.sendRaw({ type: 'ping' });
      }
    }, 25000);
  }

  private stopHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
  }

  private scheduleReconnect(): void {
    if (this.intentionalClose || !this.activeUrl) {
      this.state.set('disconnected');
      return;
    }
    if (this.reconnectAttempts >= 10) {
      this.state.set('disconnected');
      return;
    }
    this.reconnectAttempts += 1;
    const delayMs = Math.min(15000, 1000 * Math.pow(1.7, this.reconnectAttempts - 1));
    this.state.set('reconnecting');

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      if (!this.intentionalClose) {
        this.connect(this.activeUrl);
      }
    }, delayMs);
  }

  private clearTimers(): void {
    this.stopHeartbeat();
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }
}
