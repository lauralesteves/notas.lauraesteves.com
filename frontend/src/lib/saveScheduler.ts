export interface SaveSchedulerOptions {
  /** Save after this much time without changes. */
  idleMs?: number;
  /** Never wait longer than this since the first unsaved change, even while typing non-stop. */
  maxWaitMs?: number;
  onFlush: () => void;
}

export const SAVE_IDLE_MS = 1000;
export const SAVE_MAX_WAIT_MS = 5000;

/** Debounce with a ceiling: 1s of idle time, capped at 5s of continuous typing. */
export class SaveScheduler {
  private readonly idleMs: number;
  private readonly maxWaitMs: number;
  private readonly onFlush: () => void;
  private idleTimer: ReturnType<typeof setTimeout> | null = null;
  private maxTimer: ReturnType<typeof setTimeout> | null = null;

  constructor({
    idleMs = SAVE_IDLE_MS,
    maxWaitMs = SAVE_MAX_WAIT_MS,
    onFlush,
  }: SaveSchedulerOptions) {
    this.idleMs = idleMs;
    this.maxWaitMs = maxWaitMs;
    this.onFlush = onFlush;
  }

  get pending(): boolean {
    return this.idleTimer !== null || this.maxTimer !== null;
  }

  /** Call on every change. */
  schedule(): void {
    if (this.idleTimer !== null) clearTimeout(this.idleTimer);
    this.idleTimer = setTimeout(() => this.flush(), this.idleMs);
    if (this.maxTimer === null) {
      this.maxTimer = setTimeout(() => this.flush(), this.maxWaitMs);
    }
  }

  /** Schedules a single attempt after `ms`, ignoring the idle/max logic (used for retries). */
  retryIn(ms: number): void {
    this.cancel();
    this.idleTimer = setTimeout(() => this.flush(), ms);
  }

  flush(): void {
    this.cancel();
    this.onFlush();
  }

  cancel(): void {
    if (this.idleTimer !== null) clearTimeout(this.idleTimer);
    if (this.maxTimer !== null) clearTimeout(this.maxTimer);
    this.idleTimer = null;
    this.maxTimer = null;
  }
}
