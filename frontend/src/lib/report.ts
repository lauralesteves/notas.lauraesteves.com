const REPORT_EMAIL = 'lauralesteves@gmail.com';

/** mailto link for reporting a note. With a note URL, it goes pre-filled in the body. */
export function reportHref(subject: string, noteUrl?: string): string {
  const body = noteUrl
    ? `Endereço da nota: ${noteUrl}\n\nMotivo: `
    : 'Endereço da nota: ';
  return `mailto:${REPORT_EMAIL}?subject=${encodeURIComponent(subject)}&body=${encodeURIComponent(body)}`;
}
