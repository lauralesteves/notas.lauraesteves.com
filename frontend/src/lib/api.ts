import { API_URL } from '../config';
import { encodePath } from './path';

/** `Note` from docs/api.md. Missing/expired notes come back with version 0 and null dates. */
export interface Note {
  path: string;
  body: string;
  version: number;
  updatedAt: string | null;
  expiresAt: string | null;
}

export interface SaveResult {
  path: string;
  version: number;
  updatedAt: string | null;
  expiresAt: string | null;
}

export type PollResult =
  | { changed: false; version: number }
  | ({ changed: true } & Note);

export type ApiErrorCode =
  | 'invalid_path'
  | 'invalid_body'
  | 'too_large'
  | 'conflict'
  | 'not_found'
  | 'rate_limited'
  | 'internal';

export class ApiError extends Error {
  readonly status: number;
  readonly code: ApiErrorCode | 'unknown';
  /** Present on 409: the current note on the server. */
  readonly note?: Note;

  constructor(
    status: number,
    code: ApiErrorCode | 'unknown',
    message: string,
    note?: Note,
  ) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.note = note;
  }
}

/** fetch() rejected: offline, DNS, CORS, server down. */
export class NetworkError extends Error {
  constructor(cause: unknown) {
    super('network error', { cause });
    this.name = 'NetworkError';
  }
}

export const MAX_BODY_BYTES = 100_000;

const encoder = new TextEncoder();
export const byteLength = (text: string) => encoder.encode(text).length;

const STATUS_CODES: Partial<Record<number, ApiErrorCode>> = {
  409: 'conflict',
  413: 'too_large',
  429: 'rate_limited',
};

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await fetch(url, init);
  } catch (error) {
    throw new NetworkError(error);
  }
  const data = await response.json().catch(() => null);
  if (!response.ok) {
    // API Gateway's own errors (e.g. throttling: {"message":"Too Many Requests"})
    // have no `error` field, so fall back to the status.
    const code = data?.error ?? STATUS_CODES[response.status] ?? 'unknown';
    throw new ApiError(
      response.status,
      code,
      data?.message ?? response.statusText,
      data?.note,
    );
  }
  return data as T;
}

export interface NotesApi {
  getNote(path: string, signal?: AbortSignal): Promise<Note>;
  pollNote(
    path: string,
    since: number,
    signal?: AbortSignal,
  ): Promise<PollResult>;
  saveNote(
    path: string,
    input: { body: string; baseVersion: number; force: boolean },
    connectionId: string | null,
  ): Promise<SaveResult>;
}

export const httpNotesApi: NotesApi = {
  getNote(path, signal) {
    return request<Note>(`${API_URL}/notes/${encodePath(path)}`, { signal });
  },
  pollNote(path, since, signal) {
    return request<PollResult>(
      `${API_URL}/notes/${encodePath(path)}?since=${since}`,
      { signal },
    );
  },
  saveNote(path, input, connectionId) {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    if (connectionId) headers['X-Connection-Id'] = connectionId;
    return request<SaveResult>(`${API_URL}/notes/${encodePath(path)}`, {
      method: 'PUT',
      headers,
      body: JSON.stringify(input),
    });
  },
};
