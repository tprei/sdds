import createClient from 'openapi-fetch';

import { apiBaseURL } from './config';
import { listNotesResponseSchema, noteSchema } from './schema';
import type { components, paths } from './generated/schema';

export type Note = {
  author: NoteAuthor;
  body: string;
  categorySlug: string;
  createdAt: number;
  id: string;
  images: NoteImage[];
  placeSlug: string | null;
  title: string;
  updatedAt: number;
};

export type NoteImage = {
  byteSize: number;
  contentType: NoteImageResponse['content_type'];
  createdAt: number;
  height: number;
  id: string;
  position: number;
  updatedAt: number;
  url: string;
  width: number;
};

export type NoteAuthor = {
  displayName: string;
  id: string;
};

export type CreateNoteInput = {
  body: string;
  categorySlug: string;
  clientRequestId: string;
  imageUploadIds?: string[];
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
type NoteResponse = GeneratedSchemas['Note'];
type NoteImageResponse = GeneratedSchemas['NoteImage'];

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
    client_request_id: input.clientRequestId,
    image_upload_ids: input.imageUploadIds,
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
  const listNotesResponse = listNotesResponseSchema.safeParse(value);
  if (!listNotesResponse.success) {
    throw new APIResponseError();
  }

  return listNotesResponse.data.notes.map(mapNoteResponse);
}

export function parseNoteResponse(value: unknown): Note {
  const noteResponse = noteSchema.safeParse(value);
  if (!noteResponse.success) {
    throw new APIResponseError();
  }

  return mapNoteResponse(noteResponse.data);
}

export function mapNoteResponse(value: NoteResponse): Note {
  return {
    author: parseAuthorSummary(value.author),
    body: value.body,
    categorySlug: value.category_slug,
    createdAt: value.created_at,
    id: value.id,
    images: value.images.map(parseNoteImage),
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

function parseNoteImage(value: NoteImageResponse): NoteImage {
  return {
    byteSize: value.byte_size,
    contentType: value.content_type,
    createdAt: value.created_at,
    height: value.height,
    id: value.id,
    position: value.position,
    updatedAt: value.updated_at,
    url: resolveNoteImageURL(value.url),
    width: value.width,
  };
}

function resolveNoteImageURL(value: string): string {
  try {
    if (isAbsoluteURL(value)) {
      return value;
    }
    return new URL(value, apiBaseURL()).toString();
  } catch {
    throw new APIResponseError();
  }
}

function isAbsoluteURL(value: string): boolean {
  try {
    new URL(value);
    return true;
  } catch {
    return false;
  }
}
