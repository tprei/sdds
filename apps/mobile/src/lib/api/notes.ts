import createClient from 'openapi-fetch';

import { apiBaseURL } from './config';
import {
  isNoteCategorySlug,
  isNoteCitySlug,
} from '@/features/notes/metadata';
import type {
  NoteCategorySlug,
  NoteCitySlug,
} from '@/features/notes/metadata';
import type { components, paths } from './generated/schema';

export type Note = {
  body: string;
  category: NoteCategorySlug;
  city: NoteCitySlug;
  createdAt: number;
  id: string;
  title: string;
  updatedAt: number;
};

export type CreateNoteInput = {
	body: string;
	category: NoteCategorySlug;
	city: NoteCitySlug;
	title: string;
};

export type SearchNotesInput = {
	query: string;
};

type GeneratedSchemas = components['schemas'];
type CreateNoteRequest = GeneratedSchemas['CreateNoteRequest'];
type ListNotesResponse = GeneratedSchemas['ListNotesResponse'];
type NoteResponse = GeneratedSchemas['Note'];
type ParsedNoteResponse = Omit<NoteResponse, 'category_slug' | 'city_slug'> & {
  category_slug: NoteCategorySlug;
  city_slug: NoteCitySlug;
};
type SchemaKey<T> = Extract<keyof T, string>;
type SchemaKeyList<T> = readonly SchemaKey<T>[];
type ExhaustiveSchemaKeyList<T, K extends SchemaKeyList<T>> =
  Exclude<SchemaKey<T>, K[number]> extends never ? K : never;

const listNotesResponseKeys = schemaKeyList<ListNotesResponse>()(['notes']);
const noteResponseKeys = schemaKeyList<NoteResponse>()([
  'body',
  'category_slug',
  'city_slug',
  'created_at',
  'id',
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

export async function listNotes(): Promise<Note[]> {
  const { data, response } = await apiClient().GET('/v1/notes');
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
	const { data, response } = await apiClient().GET('/v1/search/notes', {
		params: {
			query: {
				q: input.query,
			},
		},
	});
	if (!response.ok) {
		throw new APIRequestError(response.status);
	}

	return parseListNotesResponse(data);
}

export async function createNote(input: CreateNoteInput): Promise<Note> {
  const request: CreateNoteRequest = {
    body: input.body,
    category_slug: input.category,
    city_slug: input.city,
    title: input.title,
  };

  const { data, response } = await apiClient().POST('/v1/notes', {
    body: request,
  });
  if (!response.ok) {
    throw new APIRequestError(response.status);
  }

  return parseNoteResponse(data);
}

function apiClient() {
  return createClient<paths>({
    baseUrl: apiBaseURL(),
    fetch: apiFetch,
  });
}

async function apiFetch(request: Request): Promise<Response> {
  const response = await fetch(request);
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
    body: value.body,
    category: value.category_slug,
    city: value.city_slug,
    createdAt: value.created_at,
    id: value.id,
    title: value.title,
    updatedAt: value.updated_at,
  };
}

function isNoteResponse(value: unknown): value is ParsedNoteResponse {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, noteResponseKeys) &&
    typeof value.id === 'string' &&
    typeof value.title === 'string' &&
    typeof value.body === 'string' &&
    typeof value.category_slug === 'string' &&
    isNoteCategorySlug(value.category_slug) &&
    typeof value.city_slug === 'string' &&
    isNoteCitySlug(value.city_slug) &&
    isUnixMillisecondTimestamp(value.created_at) &&
    isUnixMillisecondTimestamp(value.updated_at)
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
    expectedKeys.every((key) => Object.prototype.hasOwnProperty.call(value, key))
  );
}

function schemaKeyList<T>() {
  return <const K extends SchemaKeyList<T>>(keys: ExhaustiveSchemaKeyList<T, K>) => keys;
}

function isUnixMillisecondTimestamp(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value >= 0;
}
