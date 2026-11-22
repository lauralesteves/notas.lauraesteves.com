import { useNote } from '../../hooks/useNote';
import { useNoteSocket } from '../../hooks/useNoteSocket';
import { usePageMeta } from '../../hooks/usePageMeta';
import { ConflictBanner } from '../ConflictBanner/ConflictBanner';
import { NoteEditor } from '../NoteEditor/NoteEditor';
import { StatusBar } from '../StatusBar/StatusBar';

/** The note and nothing else: textarea, a corner status, and the conflict banner when needed. */
export function NotePage({ path }: { path: string }) {
  usePageMeta(path, true);
  const note = useNote(path);
  const { presence } = useNoteSocket(path, note.engine);
  const { state } = note;

  return (
    <main className="flex h-full flex-col">
      {state.conflict ? (
        <ConflictBanner
          onUseRemote={note.acceptRemoteVersion}
          onOverwrite={note.overwriteRemote}
        />
      ) : null}
      <NoteEditor
        text={state.text}
        remoteRevision={state.remoteRevision}
        loaded={state.loaded}
        onChange={note.setText}
      />
      <StatusBar
        status={state.status}
        presence={presence}
        expiresAt={state.expiresAt}
      />
    </main>
  );
}
