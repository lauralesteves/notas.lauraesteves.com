import { useEffect, useState, useSyncExternalStore } from 'react';
import { httpNotesApi, type NotesApi } from '../lib/api';
import { NoteSync, type NoteSyncState } from '../lib/noteSync';

const LOADING_STATE: NoteSyncState = {
  status: 'loading',
  loaded: false,
  text: '',
  version: 0,
  expiresAt: null,
  conflict: null,
  pending: false,
  remoteRevision: 0,
};

const noopSubscribe = () => () => {};

export interface UseNoteResult {
  state: NoteSyncState;
  /** Null until the effect creates it (keeps StrictMode's double mount safe). */
  engine: NoteSync | null;
  setText(text: string): void;
  acceptRemoteVersion(): void;
  overwriteRemote(): void;
}

/**
 * Loads a note and keeps it saved: 1s idle debounce capped at 5s, optimistic
 * versioning with `baseVersion`, conflict detection. Also flushes when the tab
 * is hidden or the network returns, and warns before leaving with unsaved text.
 */
export function useNote(
  path: string,
  api: NotesApi = httpNotesApi,
): UseNoteResult {
  const [engine, setEngine] = useState<NoteSync | null>(null);

  useEffect(() => {
    const sync = new NoteSync(path, api);
    setEngine(sync);
    void sync.load();

    const onBeforeUnload = (event: BeforeUnloadEvent) => {
      if (!sync.hasPendingChanges) return;
      void sync.flush();
      event.preventDefault();
      // Legacy browsers need returnValue set to show the prompt.
      event.returnValue = '';
    };
    const onOnline = () => {
      void sync.flush();
      void sync.refresh();
    };
    const onVisibility = () => {
      if (document.visibilityState === 'hidden') void sync.flush();
    };

    window.addEventListener('beforeunload', onBeforeUnload);
    window.addEventListener('online', onOnline);
    document.addEventListener('visibilitychange', onVisibility);
    return () => {
      window.removeEventListener('beforeunload', onBeforeUnload);
      window.removeEventListener('online', onOnline);
      document.removeEventListener('visibilitychange', onVisibility);
      sync.dispose();
    };
  }, [path, api]);

  const state = useSyncExternalStore(
    engine?.subscribe ?? noopSubscribe,
    () => engine?.getSnapshot() ?? LOADING_STATE,
  );

  return {
    state,
    engine,
    setText: (text) => engine?.setText(text),
    acceptRemoteVersion: () => engine?.acceptRemoteVersion(),
    overwriteRemote: () => engine?.overwriteRemote(),
  };
}
