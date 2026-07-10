import createClient from 'openapi-fetch';

import { apiBaseURL } from './config';
import type { components, paths } from './generated/schema';

export type Note = {
  author: NoteAuthor;
  body: string;
  categorySlug: string;
  createdAt: number;
  id: string;
  placeSlug: string | null;
  title: string;
  updatedAt: number;
};

export type NoteAuthor = {
  displayName: string;
  id: string;
};

export type CreateNoteInput = {
  body: string;
  categorySlug: string;
  placeSlug?: string | null;
  title: string;
};

export type ListNotesInput = {
  categorySlug?: string;
};

export type SearchNotesInput = {
  categorySlug?: string;
  query: string;
};

type GeneratedSchemas = components['schemas'];
type AuthorSummaryResponse = GeneratedSchemas['AuthorSummary'];
type CreateNoteRequest = GeneratedSchemas['CreateNoteRequest'];
type ListNotesResponse = GeneratedSchemas['ListNotesResponse'];
type NoteResponse = GeneratedSchemas['Note'];
type SchemaKey<T> = Extract<keyof T, string>;
type SchemaKeyList<T> = readonly SchemaKey<T>[];
type ExhaustiveSchemaKeyList<T, K extends SchemaKeyList<T>> =
  Exclude<SchemaKey<T>, K[number]> extends never ? K : never;

const listNotesResponseKeys = schemaKeyList<ListNotesResponse>()(['notes']);
const authorSummaryResponseKeys = schemaKeyList<AuthorSummaryResponse>()([
  'display_name',
  'id',
]);
const noteResponseKeys = schemaKeyList<NoteResponse>()([
  'author',
  'body',
  'category_slug',
  'created_at',
  'id',
  'place_slug',
  'title',
  'updated_at',
]);

export class APIRequestError extends Error {
  readonly status: number;

  constructor(status: number) {
    super('api_request_failed');
    this.status = status;
  }
}

export class APIResponseError extends Error {
  constructor() {
    super('api_response_invalid');
  }
}

export async function listNotes(input: ListNotesInput = {}): Promise<Note[]> {
  const query = noteListQuery(input);
  const { data, response } = await apiClient().GET('/v1/notes', {
    params: {
      query,
    },
  });
  if (!response.ok) {
    throw new APIRequestError(response.status);
  }

  return parseListNotesResponse(data);
}

export async function getNote(id: string): Promise<Note> {
  const { data, response } = await apiClient().GET('/v1/notes/{note_id}', {
    params: {
      path: {
        note_id: id,
      },
    },
  });
  if (!response.ok) {
    throw new APIRequestError(response.status);
  }

  return parseNoteResponse(data);
}

export async function searchNotes(input: SearchNotesInput): Promise<Note[]> {
  const query = noteSearchQuery(input);
  const { data, response } = await apiClient().GET('/v1/search/notes', {
    params: {
      query,
    },
  });
  if (!response.ok) {
    throw new APIRequestError(response.status);
  }

  return parseListNotesResponse(data);
}

function noteListQuery(input: ListNotesInput): {
  category_slug?: string;
} {
  if (input.categorySlug === undefined) {
    return {};
  }
  return { category_slug: input.categorySlug };
}

function noteSearchQuery(input: SearchNotesInput): {
  category_slug?: string;
  q: string;
} {
  if (input.categorySlug === undefined) {
    return { q: input.query };
  }
  return { category_slug: input.categorySlug, q: input.query };
}

export async function createNote(
  input: CreateNoteInput,
  token: string,
): Promise<Note> {
  const request: CreateNoteRequest = {
    body: input.body,
    category_slug: input.categorySlug,
    place_slug: input.placeSlug ?? null,
    title: input.title,
  };

  const { data, response } = await apiClient(token).POST('/v1/notes', {
    body: request,
  });
  if (!response.ok) {
    throw new APIRequestError(response.status);
  }

  return parseNoteResponse(data);
}

function apiClient(token?: string) {
  return createClient<paths>({
    baseUrl: apiBaseURL(),
    fetch: (request) => apiFetch(request, token),
  });
}

async function apiFetch(request: Request, token?: string): Promise<Response> {
  const response = await fetch(authenticatedRequest(request, token));
  if (response.ok) {
    return response;
  }

  const headers = new Headers(response.headers);
  headers.delete('content-length');
  headers.delete('transfer-encoding');
  return new Response(null, {
    headers,
    status: response.status,
    statusText: response.statusText,
  });
}

function authenticatedRequest(request: Request, token?: string): Request {
  if (token === undefined) {
    return request;
  }

  const headers = new Headers(request.headers);
  headers.set('Authorization', `Bearer ${token}`);
  return new Request(request, { headers });
}

function parseListNotesResponse(value: unknown): Note[] {
  if (!isListNotesResponse(value)) {
    throw new APIResponseError();
  }

  return value.notes.map(parseNoteResponse);
}

function parseNoteResponse(value: unknown): Note {
  if (!isNoteResponse(value)) {
    throw new APIResponseError();
  }

  return {
    author: parseAuthorSummary(value.author),
    body: value.body,
    categorySlug: value.category_slug,
    createdAt: value.created_at,
    id: value.id,
    placeSlug: value.place_slug,
    title: value.title,
    updatedAt: value.updated_at,
  };
}

function parseAuthorSummary(value: AuthorSummaryResponse): NoteAuthor {
  return {
    displayName: value.display_name,
    id: value.id,
  };
}

function isNoteResponse(value: unknown): value is NoteResponse {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, noteResponseKeys) &&
    typeof value.id === 'string' &&
    typeof value.title === 'string' &&
    typeof value.body === 'string' &&
    typeof value.category_slug === 'string' &&
    (typeof value.place_slug === 'string' || value.place_slug === null) &&
    isAuthorSummaryResponse(value.author) &&
    isUnixMillisecondTimestamp(value.created_at) &&
    isUnixMillisecondTimestamp(value.updated_at)
  );
}

function isAuthorSummaryResponse(
  value: unknown,
): value is AuthorSummaryResponse {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, authorSummaryResponseKeys) &&
    typeof value.id === 'string' &&
    typeof value.display_name === 'string'
  );
}

function isListNotesResponse(value: unknown): value is ListNotesResponse {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, listNotesResponseKeys) &&
    Array.isArray(value.notes)
  );
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function hasOnlyKeys(
  value: Record<string, unknown>,
  expectedKeys: readonly string[],
): boolean {
  const keys = Object.keys(value);
  return (
    keys.length === expectedKeys.length &&
    expectedKeys.every((key) =>
      Object.prototype.hasOwnProperty.call(value, key),
    )
  );
}

function schemaKeyList<T>() {
  return <const K extends SchemaKeyList<T>>(
    keys: ExhaustiveSchemaKeyList<T, K>,
  ) => keys;
}

function isUnixMillisecondTimestamp(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value >= 0;
}
