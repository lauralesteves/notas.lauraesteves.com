import { strings as t } from '../../strings';

interface ConflictBannerProps {
  onUseRemote(): void;
  onOverwrite(): void;
}

export function ConflictBanner({
  onUseRemote,
  onOverwrite,
}: ConflictBannerProps) {
  return (
    <div
      role="alert"
      className="flex flex-col gap-3 border-b border-danger/40 bg-danger/10 px-4 py-3 sm:flex-row sm:items-center sm:px-8"
    >
      <div className="flex-1 text-sm">
        <p className="font-bold text-danger">{t.conflict.title}</p>
        <p className="text-text">{t.conflict.description}</p>
      </div>
      <div className="flex flex-wrap gap-2">
        <button
          type="button"
          onClick={onUseRemote}
          className="btn-primary rounded-md px-3 py-1.5 text-sm font-bold"
        >
          {t.conflict.useRemote}
        </button>
        <button
          type="button"
          onClick={onOverwrite}
          className="btn-ghost rounded-md px-3 py-1.5 text-sm font-bold"
        >
          {t.conflict.overwrite}
        </button>
      </div>
    </div>
  );
}
