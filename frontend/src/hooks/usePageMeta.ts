import { useEffect } from 'react';

/** Sets the document title and, for notes, `<meta name="robots" content="noindex">`. */
export function usePageMeta(title: string, noindex: boolean): void {
  useEffect(() => {
    const previousTitle = document.title;
    document.title = title;
    let meta: HTMLMetaElement | null = null;
    if (noindex) {
      meta = document.createElement('meta');
      meta.name = 'robots';
      meta.content = 'noindex';
      document.head.appendChild(meta);
    }
    return () => {
      document.title = previousTitle;
      meta?.remove();
    };
  }, [title, noindex]);
}
