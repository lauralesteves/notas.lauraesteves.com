import { useEffect, useState } from 'react';
import { WS_URL } from '../config';
import type { NoteSync } from '../lib/noteSync';
import { NoteSocket, type SocketState } from '../lib/socket';

export interface UseNoteSocketResult {
  /** People connected to this note, including me. Null while the socket is down. */
  presence: number | null;
  connection: SocketState;
}

/**
 * Real-time channel for a note. Feeds remote changes into the sync engine and
 * falls back to polling `GET ?since=` every 5s while the socket is down.
 */
export function useNoteSocket(
  path: string,
  engine: NoteSync | null,
): UseNoteSocketResult {
  const [presence, setPresence] = useState<number | null>(null);
  const [connection, setConnection] = useState<SocketState>('connecting');

  useEffect(() => {
    if (!engine) return;
    const socket = new NoteSocket({
      url: WS_URL,
      path,
      handlers: {
        onUpdate: (note) => engine.receiveRemote(note),
        onChanged: (version) => {
          if (version !== engine.baseVersion) void engine.refresh();
        },
        onPresence: setPresence,
        onConnectionId: (id) => engine.setConnectionId(id),
        onResync: () => void engine.refresh(),
        onPoll: () => void engine.poll(),
        onStateChange: setConnection,
      },
    });
    socket.start();

    const onVisibility = () => socket.handleVisible();
    const onOnline = () => socket.reconnectNow();
    document.addEventListener('visibilitychange', onVisibility);
    window.addEventListener('online', onOnline);
    return () => {
      document.removeEventListener('visibilitychange', onVisibility);
      window.removeEventListener('online', onOnline);
      socket.stop();
    };
  }, [path, engine]);

  return { presence, connection };
}
