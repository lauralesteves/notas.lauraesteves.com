import { type FormEvent, useId, useState } from 'react';
import { RAW_BASE_URL } from '../../config';
import { normalizePath, validatePath } from '../../lib/path';
import { strings as t } from '../../strings';
import { Footer } from '../Footer/Footer';
import { GithubCorner } from '../GithubCorner/GithubCorner';
import { Navbar } from '../Navbar/Navbar';

export function Home() {
  const [value, setValue] = useState('');
  const inputId = useId();
  const hintId = useId();
  const normalized = normalizePath(value);
  const error = value.trim() ? validatePath(normalized) : null;

  const onSubmit = (event: FormEvent) => {
    event.preventDefault();
    if (!normalized || error) return;
    window.location.assign(`/${normalized}`);
  };

  const example = `${RAW_BASE_URL}/${t.home.openPlaceholder}`;

  return (
    <div className="min-h-full flex flex-col">
      <Navbar />
      <GithubCorner />
      <main className="flex-1 w-full max-w-2xl mx-auto px-5 pt-16 pb-28 sm:pt-20">
        <h1 className="font-mono text-5xl font-bold tracking-tight gradient-text">
          {t.home.title}
        </h1>
        <p className="mt-2 text-lg text-text">{t.home.tagline}</p>
        <p className="mt-3 text-muted leading-relaxed">{t.home.intro}</p>

        <form onSubmit={onSubmit} className="mt-6" noValidate>
          <label
            htmlFor={inputId}
            className="block text-sm font-bold uppercase tracking-wider text-purple"
          >
            {t.home.openLabel}
          </label>
          <div className="mt-2 flex flex-col gap-2 sm:flex-row">
            <div className="flex flex-1 items-center rounded-lg border border-line bg-surface focus-within:border-cyan">
              <span className="pl-3 font-mono text-sm text-muted select-none hidden sm:inline">
                notas.lauraesteves.com/
              </span>
              <input
                id={inputId}
                type="text"
                value={value}
                onChange={(event) => setValue(event.target.value)}
                placeholder={t.home.openPlaceholder}
                aria-describedby={hintId}
                aria-invalid={error ? true : undefined}
                autoCapitalize="none"
                autoComplete="off"
                spellCheck={false}
                className="url-input min-w-0 flex-1 bg-transparent px-3 py-3 sm:pl-0.5 font-mono text-text placeholder:text-muted/60 focus:outline-none"
              />
            </div>
            <button
              type="submit"
              disabled={!normalized || error !== null}
              className="btn-primary rounded-lg px-5 py-3 font-bold"
            >
              {t.home.openButton}
            </button>
          </div>
          <p id={hintId} className="mt-2 text-sm text-muted">
            {normalized && normalized !== value.trim() ? (
              <span className="font-mono text-cyan">/{normalized} · </span>
            ) : null}
            {t.home.openHint}
          </p>
        </form>

        <section
          aria-labelledby="warning-title"
          className="mt-6 rounded-lg border border-amber/40 bg-amber/5 px-4 py-3"
        >
          <h2
            id="warning-title"
            className="text-sm font-bold uppercase tracking-wider text-amber"
          >
            {t.home.warningTitle}
          </h2>
          <ul className="mt-1.5 space-y-1 text-sm text-text leading-relaxed list-disc pl-5">
            <li>{t.home.warningPublic}</li>
            <li>{t.home.warningExpiry}</li>
          </ul>
        </section>

        <section aria-labelledby="curl-title" className="mt-6">
          <h2
            id="curl-title"
            className="text-sm font-bold uppercase tracking-wider text-purple"
          >
            {t.home.curlTitle}
          </h2>
          <pre className="mt-2 overflow-x-auto rounded-lg border border-line bg-surface px-4 py-3 font-mono text-sm leading-normal">
            <code>
              <span className="text-muted"># {t.home.curlRead}</span>
              {'\n'}curl {example}
              {'\n\n'}
              <span className="text-muted"># {t.home.curlWrite}</span>
              {'\n'}curl -X PUT --data-binary @{t.home.curlFile} {example}
              {'\n\n'}
              <span className="text-muted"># {t.home.curlPipe}</span>
              {'\n'}date | curl -X PUT --data-binary @- {example}
            </code>
          </pre>
        </section>
      </main>
      <Footer />
    </div>
  );
}
