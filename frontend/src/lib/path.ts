/** Path rules from docs/api.md ("Caminhos de nota"). */

export const MAX_DEPTH = 5;
export const MAX_PATH_LENGTH = 200;
export const MAX_SEGMENT_LENGTH = 64;

const SEGMENT_RE = /^[a-z0-9_-]{1,64}$/;

export const RESERVED_SEGMENTS: ReadonlySet<string> = new Set([
  'api',
  'ws',
  'raw',
  'notes',
  'children',
  'assets',
  'static',
  'favicon.ico',
  'robots.txt',
  'sitemap.xml',
  'manifest.webmanifest',
  'humans.txt',
  'ai.txt',
  'well-known',
  'index.html',
  '404',
]);

export type PathError =
  | 'empty'
  | 'reserved'
  | 'too_deep'
  | 'too_long'
  | 'invalid_segment';

function safeDecode(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

/**
 * Turns whatever is in the address bar (or typed in the home input) into a
 * canonical note path: lowercase, accents removed (NFD), whitespace runs
 * become `-`, every other invalid character is dropped, repeated slashes are
 * merged and leading/trailing slashes are removed.
 */
export function normalizePath(raw: string): string {
  return safeDecode(raw)
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/\s+/g, '-')
    .replace(/[^a-z0-9_\-/]/g, '')
    .replace(/\/{2,}/g, '/')
    .replace(/^\/+|\/+$/g, '');
}

/** Validates an already normalized path. Returns null when it is a valid note path. */
export function validatePath(path: string): PathError | null {
  if (path === '') return 'empty';
  if (path.length > MAX_PATH_LENGTH) return 'too_long';
  const segments = path.split('/');
  if (segments.length > MAX_DEPTH) return 'too_deep';
  if (!segments.every((segment) => SEGMENT_RE.test(segment))) {
    return 'invalid_segment';
  }
  if (RESERVED_SEGMENTS.has(segments[0])) return 'reserved';
  return null;
}

export type ParsedLocation =
  | { kind: 'home'; canonicalPathname: '/' }
  | { kind: 'note'; path: string; canonicalPathname: string }
  | { kind: 'invalid'; path: string; error: PathError };

/**
 * Maps `location.pathname` to the screen the app should render. A pathname
 * that normalizes to nothing (`/`, `//`, `/!!!`) is the home.
 */
export function parseLocation(pathname: string): ParsedLocation {
  const path = normalizePath(pathname);
  if (path === '') return { kind: 'home', canonicalPathname: '/' };
  const error = validatePath(path);
  if (error) return { kind: 'invalid', path, error };
  return { kind: 'note', path, canonicalPathname: `/${path}` };
}

/** Encodes each segment for use in an API URL. Valid paths need no escaping, this is defensive. */
export function encodePath(path: string): string {
  return path.split('/').map(encodeURIComponent).join('/');
}
