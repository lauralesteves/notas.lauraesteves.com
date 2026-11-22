import type { Note } from './api';

/** Server -> client messages from docs/api.md ("WebSocket"). */
export type SocketMessage =
  | { type: 'pong'; connectionId: string; count: number }
  | ({ type: 'update' } & Note)
  | { type: 'changed'; path: string; version: number }
  | { type: 'presence'; path: string; count: number };

export function parseSocketMessage(data: unknown): SocketMessage | null {
  if (typeof data !== 'string') return null;
  let message: unknown;
  try {
    message = JSON.parse(data);
  } catch {
    return null;
  }
  if (!message || typeof message !== 'object') return null;
  const m = message as Record<string, unknown>;
  switch (m.type) {
    case 'pong':
      return typeof m.connectionId === 'string'
        ? {
            type: 'pong',
            connectionId: m.connectionId,
            count: typeof m.count === 'number' ? m.count : 0,
          }
        : null;
    case 'update':
      return typeof m.version === 'number' && typeof m.body === 'string'
        ? {
            type: 'update',
            path: String(m.path ?? ''),
            body: m.body,
            version: m.version,
            updatedAt: (m.updatedAt as string | null) ?? null,
            expiresAt: (m.expiresAt as string | null) ?? null,
          }
        : null;
    case 'changed':
      return typeof m.version === 'number'
        ? { type: 'changed', path: String(m.path ?? ''), version: m.version }
        : null;
    case 'presence':
      return typeof m.count === 'number'
        ? { type: 'presence', path: String(m.path ?? ''), count: m.count }
        : null;
    default:
      return null;
  }
}

export const BACKOFF_BASE_MS = 1000;
export const BACKOFF_MAX_MS = 30_000;
export const PING_INTERVAL_MS = 5 * 60 * 1000;
export const POLL_INTERVAL_MS = 5000;
export const CONNECT_TIMEOUT_MS = 10_000;

/** 1s, 2s, 4s, ... capped at 30s. `attempt` starts at 0. */
export function backoffDelay(attempt: number): number {
  return Math.min(BACKOFF_BASE_MS * 2 ** Math.max(0, attempt), BACKOFF_MAX_MS);
}

export type SocketState = 'connecting' | 'open' | 'polling';

export interface NoteSocketHandlers {
  onUpdate(note: Note): void;
  /** Note changed but the body was too big for the frame: GET it. */
  onChanged(version: number): void;
  onPresence(count: number | null): void;
  onConnectionId(id: string | null): void;
  /** Socket opened (first time or after a reconnect): GET to catch up. */
  onResync(): void;
  /** Polling tick while the socket is down and the tab is visible. */
  onPoll(): void;
  onStateChange?(state: SocketState): void;
}

interface SocketLike {
  onopen: ((event: Event) => void) | null;
  onclose: ((event: CloseEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
  onmessage: ((event: MessageEvent) => void) | null;
  readonly readyState: number;
  send(data: string): void;
  close(): void;
}

export interface NoteSocketOptions {
  url: string;
  path: string;
  handlers: NoteSocketHandlers;
  createSocket?: (url: string) => SocketLike;
  isVisible?: () => boolean;
}

const OPEN = 1;

/**
 * WebSocket client for one note: ping on open and every 5 minutes, reconnect
 * with exponential backoff, and poll over HTTP every 5s (visible tab only)
 * while the socket is down.
 */
export class NoteSocket {
  private readonly url: string;
  private readonly handlers: NoteSocketHandlers;
  private readonly createSocket: (url: string) => SocketLike;
  private readonly isVisible: () => boolean;

  private socket: SocketLike | null = null;
  private attempt = 0;
  private stopped = true;
  private pingTimer: ReturnType<typeof setInterval> | null = null;
  private pollTimer: ReturnType<typeof setInterval> | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private connectTimer: ReturnType<typeof setTimeout> | null = null;
  state: SocketState = 'connecting';

  constructor({
    url,
    path,
    handlers,
    createSocket = (u) => new WebSocket(u),
    isVisible = () =>
      typeof document === 'undefined' || document.visibilityState === 'visible',
  }: NoteSocketOptions) {
    this.url = `${url}?path=${encodeURIComponent(path)}`;
    this.handlers = handlers;
    this.createSocket = createSocket;
    this.isVisible = isVisible;
  }

  start(): void {
    if (!this.stopped) return;
    this.stopped = false;
    this.connect();
  }

  stop(): void {
    this.stopped = true;
    this.clearTimers();
    this.stopPolling();
    const socket = this.socket;
    this.socket = null;
    if (socket) {
      socket.onopen = null;
      socket.onclose = null;
      socket.onerror = null;
      socket.onmessage = null;
      socket.close();
    }
  }

  /** Skip the backoff wait (e.g. the browser went back online). */
  reconnectNow(): void {
    if (this.stopped || this.socket) return;
    if (this.reconnectTimer !== null) clearTimeout(this.reconnectTimer);
    this.reconnectTimer = null;
    this.attempt = 0;
    this.connect();
  }

  /** Tab became visible: poll right away if we are in fallback mode. */
  handleVisible(): void {
    if (this.state === 'polling' && this.isVisible()) this.handlers.onPoll();
  }

  private setState(state: SocketState): void {
    if (this.state === state) return;
    this.state = state;
    this.handlers.onStateChange?.(state);
  }

  private connect(): void {
    let socket: SocketLike;
    try {
      socket = this.createSocket(this.url);
    } catch {
      this.handleClose();
      return;
    }
    this.socket = socket;
    if (this.state !== 'polling') this.setState('connecting');
    this.connectTimer = setTimeout(() => {
      if (socket.readyState !== OPEN) socket.close();
    }, CONNECT_TIMEOUT_MS);

    socket.onopen = () => {
      if (this.connectTimer !== null) clearTimeout(this.connectTimer);
      this.connectTimer = null;
      this.attempt = 0;
      this.stopPolling();
      this.setState('open');
      this.ping();
      this.pingTimer = setInterval(() => this.ping(), PING_INTERVAL_MS);
      // Catch up on anything saved between the last GET and this subscription.
      this.handlers.onResync();
    };
    socket.onmessage = (event) => this.handleMessage(event.data);
    socket.onclose = () => {
      if (this.socket === socket) this.handleClose();
    };
    socket.onerror = () => {
      // A close event always follows.
    };
  }

  private ping(): void {
    if (this.socket?.readyState === OPEN) {
      this.socket.send(JSON.stringify({ action: 'ping' }));
    }
  }

  private handleMessage(data: unknown): void {
    const message = parseSocketMessage(data);
    if (!message) return;
    switch (message.type) {
      case 'pong':
        this.handlers.onConnectionId(message.connectionId);
        this.handlers.onPresence(message.count);
        break;
      case 'presence':
        this.handlers.onPresence(message.count);
        break;
      case 'update': {
        const { type: _type, ...note } = message;
        this.handlers.onUpdate(note);
        break;
      }
      case 'changed':
        this.handlers.onChanged(message.version);
        break;
    }
  }

  /**
   * Any failure ends here: network errors, a handshake the server refused
   * (invalid path, or 429 when the note already has 50 connections; browsers
   * only expose these as a close event), timeouts and normal drops. All of them
   * fall back to polling and keep retrying with backoff.
   */
  private handleClose(): void {
    this.clearTimers();
    this.socket = null;
    this.handlers.onConnectionId(null);
    this.handlers.onPresence(null);
    if (this.stopped) return;
    this.startPolling();
    const delay = backoffDelay(this.attempt);
    this.attempt++;
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, delay);
  }

  private startPolling(): void {
    this.setState('polling');
    if (this.pollTimer !== null) return;
    this.pollTimer = setInterval(() => {
      if (this.isVisible()) this.handlers.onPoll();
    }, POLL_INTERVAL_MS);
  }

  private stopPolling(): void {
    if (this.pollTimer !== null) clearInterval(this.pollTimer);
    this.pollTimer = null;
  }

  private clearTimers(): void {
    if (this.pingTimer !== null) clearInterval(this.pingTimer);
    if (this.reconnectTimer !== null) clearTimeout(this.reconnectTimer);
    if (this.connectTimer !== null) clearTimeout(this.connectTimer);
    this.pingTimer = null;
    this.reconnectTimer = null;
    this.connectTimer = null;
  }
}
