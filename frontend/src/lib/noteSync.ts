import {
  ApiError,
  byteLength,
  MAX_BODY_BYTES,
  NetworkError,
  type Note,
  type NotesApi,
} from './api';
import { SaveScheduler, type SaveSchedulerOptions } from './saveScheduler';

export type SyncStatus =
  | 'loading'
  | 'saved'
  | 'saving'
  | 'offline'
  | 'retrying'
  | 'conflict'
  | 'too_large'
  | 'error';

export type SyncError = 'offline' | 'too_large' | 'rate_limited' | 'server';

export interface NoteSyncState {
  status: SyncStatus;
  loaded: boolean;
  /** Local text, what the textarea shows. */
  text: string;
  /** Server version the local text is based on (`baseVersion` of the next save). */
  version: number;
  expiresAt: string | null;
  /** Remote version that clashed with unsaved local edits. */
  conflict: Note | null;
  /** Unsaved local edits or a save in flight. */
  pending: boolean;
  /**
   * Bumped every time `text` is replaced by something that did not come from
   * the editor (load, remote update, "use the new version"), so the editor
   * knows it must write the value into the textarea.
   */
  remoteRevision: number;
}

export interface LocalSnapshot {
  /** What the editor currently has. */
  text: string;
  /** Body of the server version `version` (what we last loaded or saved). */
  serverBody: string;
  version: number;
}

export type IncomingDecision = 'ignore' | 'apply' | 'adopt' | 'conflict';

/**
 * What to do with a note that came from the server (WS update, poll, GET on
 * reconnect, or the `note` of a 409).
 *
 * - `ignore`: nothing new (our own echo) or an out-of-order older version.
 * - `adopt`: the server already has exactly our text; only take its version.
 * - `apply`: no unsaved local edits, replace the text silently.
 * - `conflict`: someone else changed the note while we have unsaved edits.
 *
 * A 409 is `authoritative`: its note is the current state even if the
 * version went backwards (the note expired or was emptied and restarted at 1).
 * Version 0 means the note was deleted, which is always newer than what we have.
 */
export function decideIncoming(
  local: LocalSnapshot,
  remote: Pick<Note, 'body' | 'version'>,
  authoritative = false,
): IncomingDecision {
  if (remote.version === local.version && remote.body === local.serverBody) {
    return 'ignore';
  }
  if (
    !authoritative &&
    remote.version !== 0 &&
    remote.version < local.version
  ) {
    return 'ignore';
  }
  if (remote.body === local.text) return 'adopt';
  return local.text === local.serverBody ? 'apply' : 'conflict';
}

export function deriveStatus(input: {
  loaded: boolean;
  conflict: boolean;
  error: SyncError | null;
  pending: boolean;
}): SyncStatus {
  if (!input.loaded) return input.error === 'offline' ? 'offline' : 'loading';
  if (input.conflict) return 'conflict';
  if (input.error === 'too_large') return 'too_large';
  if (input.error === 'offline') return 'offline';
  if (input.error === 'rate_limited') return 'retrying';
  if (input.error === 'server') return 'error';
  return input.pending ? 'saving' : 'saved';
}

export const RETRY_OFFLINE_MS = 3000;
export const RETRY_RATE_LIMITED_MS = 2000;
export const RETRY_SERVER_MS = 5000;
export const RETRY_MAX_MS = 30_000;

/** Exponential backoff for failed saves: base, 2x, 4x... capped at 30s. `attempt` starts at 0. */
export function retryDelay(baseMs: number, attempt: number): number {
  return Math.min(baseMs * 2 ** Math.max(0, attempt), RETRY_MAX_MS);
}

export interface NoteSyncOptions {
  scheduler?: Omit<SaveSchedulerOptions, 'onFlush'>;
}

/**
 * Framework-free sync engine for one note: initial load, debounced saves with
 * optimistic versioning, conflict detection and resolution. `useNote` wraps it
 * for React; tests drive it directly with a fake `NotesApi`.
 */
export class NoteSync {
  private readonly path: string;
  private readonly api: NotesApi;
  private readonly scheduler: SaveScheduler;
  private readonly listeners = new Set<() => void>();

  private loaded = false;
  private text = '';
  private serverBody = '';
  private version = 0;
  private expiresAt: string | null = null;
  private conflict: Note | null = null;
  private error: SyncError | null = null;
  private inFlight = false;
  private forceNext = false;
  private queuedRemote: Note | null = null;
  private remoteRevision = 0;
  private retryAttempt = 0;
  private connectionId: string | null = null;
  private loadTimer: ReturnType<typeof setTimeout> | null = null;
  private disposed = false;
  private snapshot: NoteSyncState;

  constructor(path: string, api: NotesApi, options: NoteSyncOptions = {}) {
    this.path = path;
    this.api = api;
    this.scheduler = new SaveScheduler({
      ...options.scheduler,
      onFlush: () => {
        void this.flush();
      },
    });
    this.snapshot = this.buildSnapshot();
  }

  // --- store API (useSyncExternalStore) ---

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  getSnapshot = (): NoteSyncState => this.snapshot;

  get baseVersion(): number {
    return this.version;
  }

  get hasPendingChanges(): boolean {
    return this.inFlight || this.text !== this.serverBody;
  }

  private buildSnapshot(): NoteSyncState {
    const pending = this.hasPendingChanges;
    return {
      status: deriveStatus({
        loaded: this.loaded,
        conflict: this.conflict !== null,
        error: this.error,
        pending,
      }),
      loaded: this.loaded,
      text: this.text,
      version: this.version,
      expiresAt: this.expiresAt,
      conflict: this.conflict,
      pending,
      remoteRevision: this.remoteRevision,
    };
  }

  private emit(): void {
    if (this.disposed) return;
    this.snapshot = this.buildSnapshot();
    for (const listener of this.listeners) listener();
  }

  // --- lifecycle ---

  async load(): Promise<void> {
    if (this.loadTimer !== null) clearTimeout(this.loadTimer);
    this.loadTimer = null;
    try {
      const note = await this.api.getNote(this.path);
      if (this.disposed) return;
      if (this.loaded) {
        this.receiveRemote(note);
        return;
      }
      this.loaded = true;
      this.text = note.body;
      this.serverBody = note.body;
      this.version = note.version;
      this.expiresAt = note.expiresAt;
      this.error = null;
      this.remoteRevision++;
      this.emit();
    } catch (error) {
      if (this.disposed) return;
      this.handleFetchError(error);
      if (!this.loaded) {
        this.loadTimer = setTimeout(() => void this.load(), RETRY_OFFLINE_MS);
      }
    }
  }

  dispose(): void {
    this.disposed = true;
    this.scheduler.cancel();
    if (this.loadTimer !== null) clearTimeout(this.loadTimer);
    this.listeners.clear();
  }

  setConnectionId(id: string | null): void {
    this.connectionId = id;
  }

  // --- local edits ---

  setText(text: string): void {
    if (text === this.text) return;
    this.text = text;
    if (this.error === 'too_large') this.error = null;
    if (this.conflict) {
      // Saving is paused until the person picks a side.
      if (text === this.conflict.body) this.adopt(this.conflict);
    } else if (text === this.serverBody && !this.inFlight) {
      this.scheduler.cancel();
    } else {
      this.scheduler.schedule();
    }
    this.emit();
  }

  /** Saves right away (skipping the debounce), e.g. when the network comes back. */
  async flush(): Promise<void> {
    if (!this.loaded || this.conflict || this.inFlight || this.disposed) return;
    if (this.text === this.serverBody && !this.forceNext) {
      if (this.error !== 'offline') this.error = null;
      this.emit();
      return;
    }
    if (byteLength(this.text) > MAX_BODY_BYTES) {
      this.error = 'too_large';
      this.emit();
      return;
    }
    await this.save();
  }

  private async save(): Promise<void> {
    const body = this.text;
    const force = this.forceNext;
    this.inFlight = true;
    this.emit();
    try {
      const result = await this.api.saveNote(
        this.path,
        { body, baseVersion: this.version, force },
        this.connectionId,
      );
      if (this.disposed) return;
      this.version = result.version;
      this.serverBody = body;
      this.expiresAt = result.expiresAt;
      this.forceNext = false;
      this.error = null;
      this.retryAttempt = 0;
    } catch (error) {
      if (this.disposed) return;
      this.handleSaveError(error);
    } finally {
      this.inFlight = false;
    }
    if (this.disposed) return;
    const queued = this.queuedRemote;
    this.queuedRemote = null;
    if (queued) this.receiveRemote(queued);
    if (
      !this.conflict &&
      this.error === null &&
      this.text !== this.serverBody
    ) {
      // Typed while the request was in flight.
      this.scheduler.schedule();
    }
    this.emit();
  }

  private retry(baseMs: number): void {
    this.scheduler.retryIn(retryDelay(baseMs, this.retryAttempt));
    this.retryAttempt++;
  }

  /** The pending text is never dropped: every failure keeps it and retries, except 413 and 4xx. */
  private handleSaveError(error: unknown): void {
    if (error instanceof ApiError) {
      // Any 429 counts, whatever the body: API Gateway throttling answers
      // {"message":"Too Many Requests"} instead of {"error":"rate_limited"}.
      if (error.status === 429) {
        this.error = 'rate_limited';
        this.retry(RETRY_RATE_LIMITED_MS);
        return;
      }
      if (error.status === 409 && error.note) {
        this.receiveRemote(error.note, true);
        this.error = null;
        return;
      }
      if (error.status === 409) {
        void this.load();
        return;
      }
      if (error.status === 413 || error.code === 'too_large') {
        this.error = 'too_large';
        return;
      }
      this.error = 'server';
      if (error.status >= 500) this.retry(RETRY_SERVER_MS);
      return;
    }
    this.error = 'offline';
    this.retry(RETRY_OFFLINE_MS);
  }

  private handleFetchError(error: unknown): void {
    if (error instanceof NetworkError) {
      this.error = 'offline';
    } else if (!this.loaded) {
      this.error = 'server';
    }
    this.emit();
  }

  // --- remote changes ---

  /** A note from the server: WebSocket `update`, poll result or GET after reconnect. */
  receiveRemote(note: Note, authoritative = false): void {
    if (this.disposed) return;
    // load() is still running or retrying; it will fetch the latest anyway.
    if (!this.loaded) return;
    if (this.inFlight && !authoritative) {
      // The save response decides what this means; look at it afterwards.
      if (!this.queuedRemote || note.version >= this.queuedRemote.version) {
        this.queuedRemote = note;
      }
      return;
    }
    const decision = decideIncoming(
      { text: this.text, serverBody: this.serverBody, version: this.version },
      note,
      authoritative,
    );
    switch (decision) {
      case 'ignore':
        return;
      case 'adopt':
        this.adopt(note);
        break;
      case 'apply':
        this.replaceText(note);
        break;
      case 'conflict':
        this.conflict = note;
        this.scheduler.cancel();
        break;
    }
    this.emit();
  }

  /** GET the note and treat it as a remote change (reconnect, `changed` message). */
  async refresh(): Promise<void> {
    // Before the first load succeeds, load() itself is retrying.
    if (!this.loaded) return;
    try {
      const note = await this.api.getNote(this.path);
      if (this.error === 'offline') this.error = null;
      this.receiveRemote(note);
      this.emit();
    } catch (error) {
      if (!this.disposed) this.handleFetchError(error);
    }
  }

  /** Polling fallback: `GET ?since=version`. */
  async poll(): Promise<void> {
    if (!this.loaded) return;
    try {
      const result = await this.api.pollNote(this.path, this.version);
      if (this.error === 'offline') this.error = null;
      if (result.changed) {
        const { changed: _changed, ...note } = result;
        this.receiveRemote(note);
      }
      this.emit();
    } catch (error) {
      if (!this.disposed) this.handleFetchError(error);
    }
  }

  // --- conflict resolution ---

  /** Discard local edits and take the remote version. */
  acceptRemoteVersion(): void {
    if (!this.conflict) return;
    this.replaceText(this.conflict);
    this.emit();
  }

  /** Keep local edits and overwrite the server (`force: true`). */
  overwriteRemote(): void {
    const conflict = this.conflict;
    if (!conflict) return;
    this.conflict = null;
    this.serverBody = conflict.body;
    this.version = conflict.version;
    this.expiresAt = conflict.expiresAt;
    this.forceNext = true;
    this.emit();
    void this.flush();
  }

  private adopt(note: Note): void {
    this.serverBody = note.body;
    this.version = note.version;
    this.expiresAt = note.expiresAt;
    this.conflict = null;
    if (this.text === this.serverBody) this.scheduler.cancel();
  }

  private replaceText(note: Note): void {
    this.text = note.body;
    this.adopt(note);
    this.forceNext = false;
    this.remoteRevision++;
  }
}
