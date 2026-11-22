import {
  type KeyboardEvent,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
} from 'react';
import { adjustSelection } from '../../lib/cursor';
import { strings as t } from '../../strings';

interface NoteEditorProps {
  text: string;
  /** Bumped by the sync engine whenever `text` did not come from this editor. */
  remoteRevision: number;
  loaded: boolean;
  onChange(text: string): void;
}

/**
 * Full-height textarea. It is uncontrolled on purpose: local keystrokes go
 * straight to the sync engine, and remote text is written in a layout effect
 * that keeps the caret and selection on the same characters.
 */
export function NoteEditor({
  text,
  remoteRevision,
  loaded,
  onChange,
}: NoteEditorProps) {
  const ref = useRef<HTMLTextAreaElement>(null);
  const escapePressed = useRef(false);
  const hintId = useId();

  // biome-ignore lint/correctness/useExhaustiveDependencies: remoteRevision forces a re-check even when text repeats
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el || el.value === text) return;
    const focused = document.activeElement === el;
    const selection = adjustSelection(el.value, text, {
      start: el.selectionStart,
      end: el.selectionEnd,
    });
    const direction = el.selectionDirection;
    const scrollTop = el.scrollTop;
    el.value = text;
    if (focused)
      el.setSelectionRange(selection.start, selection.end, direction);
    el.scrollTop = scrollTop;
  }, [text, remoteRevision]);

  useEffect(() => {
    if (loaded) ref.current?.focus({ preventScroll: true });
  }, [loaded]);

  const onKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === 'Escape') {
      escapePressed.current = true;
      return;
    }
    const wasEscaped = escapePressed.current;
    escapePressed.current = false;
    if (
      event.key !== 'Tab' ||
      event.shiftKey ||
      event.ctrlKey ||
      event.metaKey ||
      event.altKey ||
      wasEscaped
    ) {
      return;
    }
    event.preventDefault();
    const el = event.currentTarget;
    // execCommand keeps the native undo stack; setRangeText is the fallback.
    const inserted =
      typeof document.execCommand === 'function' &&
      document.execCommand('insertText', false, '\t');
    if (!inserted) {
      el.setRangeText('\t', el.selectionStart, el.selectionEnd, 'end');
      onChange(el.value);
    }
  };

  return (
    <>
      <label htmlFor="note-editor" className="sr-only">
        {t.note.editorLabel}
      </label>
      <p id={hintId} className="sr-only">
        {t.note.editorHint}
      </p>
      <textarea
        id="note-editor"
        ref={ref}
        defaultValue={text}
        readOnly={!loaded}
        aria-busy={!loaded}
        aria-describedby={hintId}
        placeholder={loaded ? '' : t.note.loading}
        onChange={(event) => onChange(event.target.value)}
        onKeyDown={onKeyDown}
        className="note-editor flex-1 w-full resize-none bg-bg px-4 py-4 sm:px-8 sm:py-6 font-mono text-[15px] leading-relaxed text-text placeholder:text-muted"
        style={{ tabSize: 4 }}
      />
    </>
  );
}
