import { apiBaseURL } from './config';
import { apiRoutes } from './routes';
import {
  isNoteCategorySlug,
  isNoteCitySlug,
} from '@/features/notes/metadata';
import type {
  NoteCategorySlug,
  NoteCitySlug,
} from '@/features/notes/metadata';
import type { components } from './generated/schema';

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
type AssertNoMissingKeys<T extends never> = T;

const listNotesResponseKeys = ['notes'] as const satisfies SchemaKeyList<ListNotesResponse>;
const noteResponseKeys = [
  'body',
  'category_slug',
  'city_slug',
  'created_at',
  'id',
  'title',
  'updated_at',
] as const satisfies SchemaKeyList<NoteResponse>;

type MissingListNotesResponseKeys = AssertNoMissingKeys<
  Exclude<SchemaKey<ListNotesResponse>, (typeof listNotesResponseKeys)[number]>
>;
type MissingNoteResponseKeys = AssertNoMissingKeys<
  Exclude<SchemaKey<NoteResponse>, (typeof noteResponseKeys)[number]>
>;

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
  const response = await fetch(`${apiBaseURL()}${apiRoutes.notes}`);
  if (!response.ok) {
    throw new APIRequestError(response.status);
  }

  const body: unknown = await response.json();
  return parseListNotesResponse(body);
}

export async function createNote(input: CreateNoteInput): Promise<Note> {
  const request: CreateNoteRequest = {
    body: input.body,
    category_slug: input.category,
    city_slug: input.city,
    title: input.title,
  };

  const response = await fetch(`${apiBaseURL()}${apiRoutes.notes}`, {
    body: JSON.stringify(request),
    headers: {
      'Content-Type': 'application/json',
    },
    method: 'POST',
  });
  if (!response.ok) {
    throw new APIRequestError(response.status);
  }

  const body: unknown = await response.json();
  return parseNoteResponse(body);
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

function isUnixMillisecondTimestamp(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value >= 0;
}
