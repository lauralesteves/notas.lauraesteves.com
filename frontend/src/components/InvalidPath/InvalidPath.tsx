import { usePageMeta } from '../../hooks/usePageMeta';
import { MAX_DEPTH, MAX_PATH_LENGTH, type PathError } from '../../lib/path';
import { strings as t } from '../../strings';

function describe(error: PathError, path: string): string {
  switch (error) {
    case 'reserved':
      return t.invalid.reserved(path.split('/')[0]);
    case 'too_deep':
      return t.invalid.tooDeep(MAX_DEPTH);
    case 'too_long':
      return t.invalid.tooLong(MAX_PATH_LENGTH);
    case 'invalid_segment':
      return t.invalid.invalidSegment;
    case 'empty':
      return t.invalid.empty;
  }
}

export function InvalidPath({
  path,
  error,
}: {
  path: string;
  error: PathError;
}) {
  usePageMeta(t.invalid.title, true);
  return (
    <main className="mx-auto w-full max-w-2xl px-5 pt-24">
      <p className="break-all font-mono text-sm text-muted">/{path}</p>
      <h1 className="mt-2 text-2xl font-bold text-text">{t.invalid.title}</h1>
      <p className="mt-2 text-muted">{describe(error, path)}</p>
      <a
        href="/"
        className="btn-primary mt-6 inline-block rounded-lg px-4 py-2 font-bold"
      >
        {t.invalid.back}
      </a>
    </main>
  );
}
