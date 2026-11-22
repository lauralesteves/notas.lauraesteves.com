import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  backoffDelay,
  NoteSocket,
  type NoteSocketHandlers,
  parseSocketMessage,
} from './socket';

describe('backoffDelay', () => {
  it('doubles from 1s up to 30s', () => {
    expect([0, 1, 2, 3, 4, 5, 6, 10].map(backoffDelay)).toEqual([
      1000, 2000, 4000, 8000, 16000, 30000, 30000, 30000,
    ]);
  });
});

describe('parseSocketMessage', () => {
  it('parses the contract messages', () => {
    expect(
      parseSocketMessage('{"type":"pong","connectionId":"abc=","count":2}'),
    ).toEqual({ type: 'pong', connectionId: 'abc=', count: 2 });
    expect(
      parseSocketMessage(
        '{"type":"update","path":"foo","version":0,"body":"","updatedAt":null,"expiresAt":null}',
      ),
    ).toMatchObject({ type: 'update', version: 0, body: '' });
    expect(
      parseSocketMessage('{"type":"changed","path":"foo","version":8}'),
    ).toEqual({ type: 'changed', path: 'foo', version: 8 });
  });

  it('drops garbage', () => {
    expect(parseSocketMessage('nope')).toBeNull();
    expect(parseSocketMessage('{"type":"other"}')).toBeNull();
    expect(parseSocketMessage(42)).toBeNull();
  });
});

class FakeSocket {
  static instances: FakeSocket[] = [];
  onopen: ((event: Event) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  readyState = 0;
  sent: string[] = [];
  constructor(readonly url: string) {
    FakeSocket.instances.push(this);
  }
  send(data: string) {
    this.sent.push(data);
  }
  close() {
    this.readyState = 3;
    this.onclose?.({} as CloseEvent);
  }
  open() {
    this.readyState = 1;
    this.onopen?.({} as Event);
  }
  receive(data: unknown) {
    this.onmessage?.({ data: JSON.stringify(data) } as MessageEvent);
  }
}

function handlers(): NoteSocketHandlers {
  return {
    onUpdate: vi.fn(),
    onChanged: vi.fn(),
    onPresence: vi.fn(),
    onConnectionId: vi.fn(),
    onResync: vi.fn(),
    onPoll: vi.fn(),
  };
}

describe('NoteSocket', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    FakeSocket.instances = [];
  });
  afterEach(() => vi.useRealTimers());

  const create = (h: NoteSocketHandlers, visible = () => true) =>
    new NoteSocket({
      url: 'ws://x/ws',
      path: 'a/b',
      handlers: h,
      createSocket: (url) => new FakeSocket(url),
      isVisible: visible,
    });

  it('pings on open and every 5 minutes, and routes messages', () => {
    const h = handlers();
    create(h).start();
    const socket = FakeSocket.instances[0];
    expect(socket.url).toBe('ws://x/ws?path=a%2Fb');
    socket.open();
    expect(socket.sent).toEqual(['{"action":"ping"}']);
    expect(h.onResync).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(5 * 60 * 1000);
    expect(socket.sent).toHaveLength(2);

    socket.receive({ type: 'pong', connectionId: 'c=', count: 2 });
    expect(h.onConnectionId).toHaveBeenCalledWith('c=');
    expect(h.onPresence).toHaveBeenCalledWith(2);
    socket.receive({
      type: 'update',
      path: 'a/b',
      version: 3,
      body: 'x',
      updatedAt: null,
      expiresAt: null,
    });
    expect(h.onUpdate).toHaveBeenCalledWith(
      expect.objectContaining({ body: 'x', version: 3 }),
    );
    socket.receive({ type: 'changed', path: 'a/b', version: 9 });
    expect(h.onChanged).toHaveBeenCalledWith(9);
  });

  it('reconnects with backoff, polls while down and re-syncs on reconnect', () => {
    const h = handlers();
    const ns = create(h);
    ns.start();
    FakeSocket.instances[0].close(); // never opened
    expect(ns.state).toBe('polling');

    vi.advanceTimersByTime(1000);
    expect(FakeSocket.instances).toHaveLength(2);
    FakeSocket.instances[1].close();
    vi.advanceTimersByTime(1999);
    expect(FakeSocket.instances).toHaveLength(2);
    vi.advanceTimersByTime(1);
    expect(FakeSocket.instances).toHaveLength(3);

    // polling ran every 5s meanwhile (t = 3s so far)
    vi.advanceTimersByTime(2000);
    expect(h.onPoll).toHaveBeenCalledTimes(1);

    FakeSocket.instances[2].open();
    expect(ns.state).toBe('open');
    expect(h.onResync).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(20_000);
    expect(h.onPoll).toHaveBeenCalledTimes(1);
  });

  it('only polls while the tab is visible', () => {
    const h = handlers();
    let visible = false;
    create(h, () => visible).start();
    FakeSocket.instances[0].close();
    vi.advanceTimersByTime(5000);
    expect(h.onPoll).not.toHaveBeenCalled();
    visible = true;
    vi.advanceTimersByTime(5000);
    expect(h.onPoll).toHaveBeenCalledTimes(1);
  });

  it('stop closes the socket without reconnecting', () => {
    const h = handlers();
    const ns = create(h);
    ns.start();
    FakeSocket.instances[0].open();
    ns.stop();
    vi.advanceTimersByTime(60_000);
    expect(FakeSocket.instances).toHaveLength(1);
  });
});
