/**
 * Endpoints come from VITE_API_URL / VITE_WS_URL when set. Otherwise the
 * production build uses the public endpoints from docs/api.md and every other
 * mode (dev, test) uses the local dev server. The defaults live here instead
 * of a `.env.production` file because the root .gitignore ignores `.env.*`.
 */
const PRODUCTION = {
  api: 'https://api.notas.lauraesteves.com',
  ws: 'wss://ws.notas.lauraesteves.com',
} as const;

const LOCAL = {
  api: 'http://localhost:8080',
  ws: 'ws://localhost:8080/ws',
} as const;

const defaults = import.meta.env.PROD ? PRODUCTION : LOCAL;

const trimSlash = (url: string) => url.replace(/\/+$/, '');

export const API_URL = trimSlash(import.meta.env.VITE_API_URL || defaults.api);
export const WS_URL = trimSlash(import.meta.env.VITE_WS_URL || defaults.ws);

/** Base URL shown in the curl examples. In production `/raw/*` is served by CloudFront on the site's own domain. */
export const RAW_BASE_URL = import.meta.env.PROD
  ? 'https://notas.lauraesteves.com/raw'
  : `${API_URL}/raw`;

export const REPO_URL =
  'https://github.com/lauralesteves/notas.lauraesteves.com';
export const LAURA_URL = 'https://lauraesteves.com';
