import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  ApiError,
  NetworkError,
  type Note,
  type NotesApi,
  type SaveResult,
} from './api';
import { decideIncoming, NoteSync, retryDelay } from './noteSync';

const note = (body: string, version: number): Note => ({
  path: 'foo',
  body,
  version,
  updatedAt: version ? '2026-11-22T18:42:00Z' : null,
  expiresAt: version ? '2026-12-07T18:42:00Z' : null,
});

describe('decideIncoming', () => {
  const clean = { text: 'a', serverBody: 'a', version: 3 };
  const dirty = { text: 'a!', serverBody: 'a', version: 3 };

  it('ignores echoes and older versions', () => {
    expect(decideIncoming(clean, { body: 'a', version: 3 })).toBe('ignore');
    expect(decideIncoming(clean, { body: 'old', version: 2 })).toBe('ignore');
  });

  it('applies silently when there are no local edits', () => {
    expect(decideIncoming(clean, { body: 'b', version: 4 })).toBe('apply');
  });

  it('flags a conflict when there are local edits', () => {
    expect(decideIncoming(dirty, { body: 'b', version: 4 })).toBe('conflict');
  });

  it('adopts the version when the server already has our text', () => {
    expect(decideIncoming(dirty, { body: 'a!', version: 4 })).toBe('adopt');
  });

  it('treats a deletion (version 0) as newer', () => {
    expect(decideIncoming(clean, { body: '', version: 0 })).toBe('apply');
  });

  it('trusts an authoritative (409) note even if its version went back', () => {
    expect(decideIncoming(dirty, { body: 'new', version: 1 }, true)).toBe(
      'conflict',
    );
    expect(decideIncoming(clean, { body: 'new', version: 1 }, true)).toBe(
      'apply',
    );
  });
});

function fakeApi(initial: Note) {
  const saves: Array<{ body: string; baseVersion: number; force: boolean }> =
    [];
  let respond: (input: {
    body: string;
    baseVersion: number;
    force: boolean;
  }) => Promise<SaveResult> = async (input) => ({
    path: 'foo',
    version: input.baseVersion + 1,
    updatedAt: '2026-11-22T18:42:00Z',
    expiresAt: '2026-12-07T18:42:00Z',
  });
  const api: NotesApi = {
    getNote: vi.fn(async () => initial),
    pollNote: vi.fn(async () => ({ changed: false as const, version: 0 })),
    saveNote: vi.fn(async (_path, input, _connectionId) => {
      saves.push(input);
      return respond(input);
    }),
  };
  return {
    api,
    saves,
    setRespond(fn: typeof respond) {
      respond = fn;
    },
  };
}

async function loadedSync(initial: Note) {
  const fake = fakeApi(initial);
  const sync = new NoteSync('foo', fake.api);
  await sync.load();
  return { sync, ...fake };
}

describe('retryDelay', () => {
  it('doubles up to 30s', () => {
    expect([0, 1, 2, 3, 4, 5].map((n) => retryDelay(2000, n))).toEqual([
      2000, 4000, 8000, 16000, 30000, 30000,
    ]);
  });
});

describe('NoteSync', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('loads, debounces the save and sends baseVersion + connection id', async () => {
    const { sync, api, saves } = await loadedSync(note('hi', 7));
    sync.setConnectionId('conn=');
    expect(sync.getSnapshot()).toMatchObject({
      status: 'saved',
      text: 'hi',
      version: 7,
    });

    sync.setText('hi!');
    expect(sync.getSnapshot().status).toBe('saving');
    await vi.advanceTimersByTimeAsync(999);
    expect(saves).toHaveLength(0);
    await vi.advanceTimersByTimeAsync(1);
    expect(saves).toEqual([{ body: 'hi!', baseVersion: 7, force: false }]);
    expect(api.saveNote).toHaveBeenCalledWith(
      'foo',
      expect.anything(),
      'conn=',
    );
    expect(sync.getSnapshot()).toMatchObject({
      status: 'saved',
      version: 8,
      pending: false,
    });
  });

  it('does not save when the text goes back to the saved version', async () => {
    const { sync, saves } = await loadedSync(note('hi', 1));
    sync.setText('hi!');
    sync.setText('hi');
    await vi.advanceTimersByTimeAsync(6000);
    expect(saves).toHaveLength(0);
    expect(sync.getSnapshot().status).toBe('saved');
  });

  it('applies a remote update silently when there are no local edits', async () => {
    const { sync } = await loadedSync(note('hi', 1));
    const revision = sync.getSnapshot().remoteRevision;
    sync.receiveRemote(note('hello', 2));
    expect(sync.getSnapshot()).toMatchObject({
      text: 'hello',
      version: 2,
      conflict: null,
      remoteRevision: revision + 1,
    });
  });

  it('ignores an out-of-order older update', async () => {
    const { sync } = await loadedSync(note('v5', 5));
    sync.receiveRemote(note('v4', 4));
    expect(sync.getSnapshot()).toMatchObject({ text: 'v5', version: 5 });
  });

  it('shows a conflict when a remote update arrives over local edits', async () => {
    const { sync, saves } = await loadedSync(note('hi', 1));
    sync.setText('mine');
    sync.receiveRemote(note('theirs', 2));
    expect(sync.getSnapshot()).toMatchObject({
      status: 'conflict',
      text: 'mine',
      conflict: { body: 'theirs', version: 2 },
    });
    // saving is paused while in conflict
    sync.setText('mine, still typing');
    await vi.advanceTimersByTimeAsync(6000);
    expect(saves).toHaveLength(0);
  });

  it('"use the new version" replaces the local text', async () => {
    const { sync } = await loadedSync(note('hi', 1));
    sync.setText('mine');
    sync.receiveRemote(note('theirs', 2));
    sync.acceptRemoteVersion();
    expect(sync.getSnapshot()).toMatchObject({
      status: 'saved',
      text: 'theirs',
      version: 2,
      conflict: null,
    });
  });

  it('"overwrite" saves the local text with force: true', async () => {
    const { sync, saves } = await loadedSync(note('hi', 1));
    sync.setText('mine');
    sync.receiveRemote(note('theirs', 2));
    sync.overwriteRemote();
    await vi.advanceTimersByTimeAsync(0);
    expect(saves).toEqual([{ body: 'mine', baseVersion: 2, force: true }]);
    expect(sync.getSnapshot()).toMatchObject({
      status: 'saved',
      text: 'mine',
      version: 3,
      conflict: null,
    });
  });

  it('turns a 409 into a conflict banner when local edits differ', async () => {
    const { sync, setRespond } = await loadedSync(note('hi', 1));
    setRespond(async () => {
      throw new ApiError(409, 'conflict', 'conflict', note('theirs', 5));
    });
    sync.setText('mine');
    await vi.advanceTimersByTimeAsync(1000);
    expect(sync.getSnapshot()).toMatchObject({
      status: 'conflict',
      text: 'mine',
      conflict: { body: 'theirs', version: 5 },
    });
  });

  it('resolves a 409 silently when the server already has the same text', async () => {
    const { sync, setRespond } = await loadedSync(note('hi', 1));
    setRespond(async () => {
      throw new ApiError(409, 'conflict', 'conflict', note('same', 5));
    });
    sync.setText('same');
    await vi.advanceTimersByTimeAsync(1000);
    expect(sync.getSnapshot()).toMatchObject({
      status: 'saved',
      version: 5,
      conflict: null,
    });
  });

  it('applies a 409 silently when local edits were undone during the request', async () => {
    const { sync, setRespond } = await loadedSync(note('hi', 1));
    let reject: (error: unknown) => void = () => {};
    setRespond(
      () =>
        new Promise<SaveResult>((_, r) => {
          reject = r;
        }),
    );
    sync.setText('hi!');
    await vi.advanceTimersByTimeAsync(1000);
    sync.setText('hi');
    reject(new ApiError(409, 'conflict', 'conflict', note('theirs', 4)));
    await vi.advanceTimersByTimeAsync(0);
    expect(sync.getSnapshot()).toMatchObject({
      status: 'saved',
      text: 'theirs',
      conflict: null,
    });
  });

  it('defers remote updates received while a save is in flight', async () => {
    const { sync, setRespond } = await loadedSync(note('a', 1));
    let resolve: (result: SaveResult) => void = () => {};
    setRespond(
      () =>
        new Promise<SaveResult>((r) => {
          resolve = r;
        }),
    );
    sync.setText('ab');
    await vi.advanceTimersByTimeAsync(1000);
    // our save lands as v2; a stale broadcast of v1 and then v3 arrive meanwhile
    sync.receiveRemote(note('a', 1));
    sync.receiveRemote(note('abc', 3));
    resolve({ path: 'foo', version: 2, updatedAt: null, expiresAt: null });
    await vi.advanceTimersByTimeAsync(0);
    expect(sync.getSnapshot()).toMatchObject({ text: 'abc', version: 3 });
  });

  it('reports 413 and does not retry until the text changes', async () => {
    const { sync, saves, setRespond } = await loadedSync(note('', 0));
    setRespond(async () => {
      throw new ApiError(413, 'too_large', 'too large');
    });
    sync.setText('big');
    await vi.advanceTimersByTimeAsync(1000);
    expect(sync.getSnapshot().status).toBe('too_large');
    await vi.advanceTimersByTimeAsync(30_000);
    expect(saves).toHaveLength(1);
    sync.setText('bi');
    expect(sync.getSnapshot().status).toBe('saving');
  });

  it('refuses to send a body above 100.000 bytes', async () => {
    const { sync, saves } = await loadedSync(note('', 0));
    sync.setText('é'.repeat(50_001)); // 100.002 bytes in UTF-8
    await vi.advanceTimersByTimeAsync(1000);
    expect(saves).toHaveLength(0);
    expect(sync.getSnapshot().status).toBe('too_large');
  });

  it('goes offline on network errors and retries', async () => {
    const { sync, saves, setRespond } = await loadedSync(note('a', 1));
    setRespond(async () => {
      throw new NetworkError(new TypeError('Failed to fetch'));
    });
    sync.setText('ab');
    await vi.advanceTimersByTimeAsync(1000);
    expect(sync.getSnapshot().status).toBe('offline');
    setRespond(async (input) => ({
      path: 'foo',
      version: input.baseVersion + 1,
      updatedAt: null,
      expiresAt: null,
    }));
    await vi.advanceTimersByTimeAsync(3000);
    expect(saves).toHaveLength(2);
    expect(sync.getSnapshot()).toMatchObject({ status: 'saved', version: 2 });
  });

  it('treats any 429 as rate limited: keeps the text and retries with backoff', async () => {
    const { sync, saves, setRespond } = await loadedSync(note('a', 1));
    // API Gateway throttling body, not the contract's {"error":"rate_limited"}
    setRespond(async () => {
      throw new ApiError(429, 'unknown', 'Too Many Requests');
    });
    sync.setText('ab');
    await vi.advanceTimersByTimeAsync(1000);
    expect(saves).toHaveLength(1);
    expect(sync.getSnapshot()).toMatchObject({
      status: 'retrying',
      text: 'ab',
      pending: true,
    });

    // backoff: 2s, then 4s
    await vi.advanceTimersByTimeAsync(1999);
    expect(saves).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(saves).toHaveLength(2);
    await vi.advanceTimersByTimeAsync(3999);
    expect(saves).toHaveLength(2);

    setRespond(async (input) => ({
      path: 'foo',
      version: input.baseVersion + 1,
      updatedAt: null,
      expiresAt: null,
    }));
    await vi.advanceTimersByTimeAsync(1);
    expect(saves).toHaveLength(3);
    expect(saves[2]).toEqual({ body: 'ab', baseVersion: 1, force: false });
    expect(sync.getSnapshot()).toMatchObject({
      status: 'saved',
      version: 2,
      pending: false,
    });
  });

  it('polling applies changes reported by ?since=', async () => {
    const { sync, api } = await loadedSync(note('a', 1));
    vi.mocked(api.pollNote).mockResolvedValueOnce({
      changed: true,
      ...note('remote', 2),
    });
    await sync.poll();
    expect(api.pollNote).toHaveBeenCalledWith('foo', 1);
    expect(sync.getSnapshot()).toMatchObject({ text: 'remote', version: 2 });
  });
});
