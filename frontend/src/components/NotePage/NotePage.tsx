import { useNote } from '../../hooks/useNote';
import { useNoteSocket } from '../../hooks/useNoteSocket';
import { usePageMeta } from '../../hooks/usePageMeta';
import { reportHref } from '../../lib/report';
import { strings as t } from '../../strings';
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
      <a
        href={reportHref(t.footer.reportSubject, window.location.href)}
        className="link fixed right-3 bottom-2 z-10 rounded bg-bg/85 px-2 py-0.5 text-[11px] text-muted"
      >
        {t.note.report}
      </a>
    </main>
  );
}
