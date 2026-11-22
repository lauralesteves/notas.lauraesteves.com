import { LAURA_URL } from '../../config';
import { strings as t } from '../../strings';

const REPORT_EMAIL = 'lauralesteves@gmail.com';
const currentYear = new Date().getFullYear();

function reportHref(): string {
  const subject = encodeURIComponent(t.footer.reportSubject);
  const body = encodeURIComponent('Endereço da nota: ');
  return `mailto:${REPORT_EMAIL}?subject=${subject}&body=${body}`;
}

/** Same shape as the family footer (vaicorinthians.lauraesteves.com), plus the report link. */
export function Footer() {
  return (
    <footer className="py-4 text-center text-sm text-text">
      <p>
        &copy; {currentYear} |{' '}
        <a
          href={LAURA_URL}
          target="_blank"
          className="font-bold transition-opacity hover:opacity-80"
          rel="noreferrer"
        >
          {t.footer.name}
        </a>
      </p>
      <p className="mt-1 text-xs">
        <a href={reportHref()} className="link text-muted">
          {t.footer.report}
        </a>
      </p>
    </footer>
  );
}
