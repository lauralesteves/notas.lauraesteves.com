import { MAX_BODY_BYTES } from '../../lib/api';
import type { SyncStatus } from '../../lib/noteSync';
import { strings as t } from '../../strings';

interface StatusBarProps {
  status: SyncStatus;
  presence: number | null;
  expiresAt: string | null;
}

const dateFormat = new Intl.DateTimeFormat('pt-BR', {
  day: '2-digit',
  month: '2-digit',
});

export function formatExpiry(expiresAt: string): string | null {
  const date = new Date(expiresAt);
  return Number.isNaN(date.getTime()) ? null : dateFormat.format(date);
}

const STATUS_TONE: Record<SyncStatus, string> = {
  loading: 'text-muted',
  saved: 'text-muted',
  saving: 'text-muted',
  offline: 'text-amber',
  retrying: 'text-amber',
  conflict: 'text-danger',
  too_large: 'text-danger',
  error: 'text-amber',
};

export function statusLabel(status: SyncStatus): string {
  switch (status) {
    case 'loading':
      return t.status.loading;
    case 'saved':
      return t.status.saved;
    case 'saving':
      return t.status.saving;
    case 'offline':
      return t.status.offline;
    case 'retrying':
      return t.status.retrying;
    case 'conflict':
      return t.status.conflict;
    case 'too_large':
      return t.status.tooLarge(MAX_BODY_BYTES.toLocaleString('pt-BR'));
    case 'error':
      return t.status.error;
  }
}

/** Only states that need attention are announced to screen readers. */
const ANNOUNCED: ReadonlySet<SyncStatus> = new Set([
  'offline',
  'retrying',
  'conflict',
  'too_large',
  'error',
]);

/** Small, muted indicator in the bottom-right corner. No links, no chrome. */
export function StatusBar({ status, presence, expiresAt }: StatusBarProps) {
  const label = statusLabel(status);
  const expiry = expiresAt ? formatExpiry(expiresAt) : null;
  const parts = [
    presence !== null && presence > 0 ? t.status.people(presence) : null,
    expiry ? t.status.expiresOn(expiry) : null,
  ].filter(Boolean);
  return (
    <>
      <p className="pointer-events-none fixed right-3 bottom-2 z-10 max-w-[calc(100%-1.5rem)] truncate rounded bg-bg/85 px-2 py-0.5 text-[11px] text-muted">
        <span className="sr-only">{t.status.label}: </span>
        <span className={STATUS_TONE[status]}>{label}</span>
        {parts.map((part) => (
          <span key={part}> · {part}</span>
        ))}
      </p>
      <p role="status" className="sr-only">
        {ANNOUNCED.has(status) ? label : ''}
      </p>
    </>
  );
}
