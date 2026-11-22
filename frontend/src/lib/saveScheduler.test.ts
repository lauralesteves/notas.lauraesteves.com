import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SaveScheduler } from './saveScheduler';

describe('SaveScheduler', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('flushes after 1s without changes', () => {
    const onFlush = vi.fn();
    const scheduler = new SaveScheduler({ onFlush });
    scheduler.schedule();
    vi.advanceTimersByTime(999);
    expect(onFlush).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(onFlush).toHaveBeenCalledTimes(1);
    expect(scheduler.pending).toBe(false);
  });

  it('restarts the idle timer on every change', () => {
    const onFlush = vi.fn();
    const scheduler = new SaveScheduler({ onFlush });
    scheduler.schedule();
    vi.advanceTimersByTime(800);
    scheduler.schedule();
    vi.advanceTimersByTime(800);
    expect(onFlush).not.toHaveBeenCalled();
    vi.advanceTimersByTime(200);
    expect(onFlush).toHaveBeenCalledTimes(1);
  });

  it('caps continuous typing at 5s', () => {
    const onFlush = vi.fn();
    const scheduler = new SaveScheduler({ onFlush });
    // a keystroke every 300ms from t=0 to t=4500, now at t=4800
    for (let elapsed = 0; elapsed < 4800; elapsed += 300) {
      scheduler.schedule();
      vi.advanceTimersByTime(300);
    }
    vi.advanceTimersByTime(199);
    expect(onFlush).toHaveBeenCalledTimes(0);
    vi.advanceTimersByTime(1);
    expect(onFlush).toHaveBeenCalledTimes(1);
    // the ceiling restarts with the next change
    scheduler.schedule();
    vi.advanceTimersByTime(1000);
    expect(onFlush).toHaveBeenCalledTimes(2);
  });

  it('cancel drops the pending flush', () => {
    const onFlush = vi.fn();
    const scheduler = new SaveScheduler({ onFlush });
    scheduler.schedule();
    scheduler.cancel();
    vi.advanceTimersByTime(10_000);
    expect(onFlush).not.toHaveBeenCalled();
  });

  it('retryIn schedules a single attempt', () => {
    const onFlush = vi.fn();
    const scheduler = new SaveScheduler({ onFlush });
    scheduler.retryIn(3000);
    vi.advanceTimersByTime(2999);
    expect(onFlush).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(onFlush).toHaveBeenCalledTimes(1);
  });
});
