import { LAURA_URL } from '../../config';
import { reportHref } from '../../lib/report';
import { strings as t } from '../../strings';

const currentYear = new Date().getFullYear();

/** Same footer as tao.lauraesteves.com (fixed at the bottom), plus the report link on a third line. */
export function Footer() {
  return (
    <footer className="fixed right-0 bottom-0 left-0 z-50 bg-bg/85 py-3 text-center text-sm text-white backdrop-blur-sm">
      <p className="font-bold uppercase">{t.footer.tagline}</p>
      <p>
        &copy; {currentYear} | {t.footer.madeWith}{' '}
        <a
          href={LAURA_URL}
          target="_blank"
          className="font-bold transition-opacity hover:opacity-80"
          rel="noreferrer"
        >
          {t.footer.name}
        </a>
      </p>
      <p>
        <a
          href={reportHref(t.footer.reportSubject)}
          className="link text-muted"
        >
          {t.footer.report}
        </a>
      </p>
    </footer>
  );
}
