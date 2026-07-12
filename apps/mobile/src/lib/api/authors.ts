import createClient from 'openapi-fetch';

import { apiBaseURL } from './config';
import type { components, paths } from './generated/schema';
import { APIRequestError, APIResponseError, parseNoteResponse } from './notes';
import type { Note } from './notes';

export type PublicAuthor = {
  id: string;
  displayName: string;
  noteCount: number;
};

export type AuthorNotesPage = {
  notes: Note[];
  nextCursor: string | null;
};

export type ListAuthorNotesInput = {
  authorID: string;
  limit?: number;
  cursor?: string;
};

type GeneratedSchemas = components['schemas'];
type PublicAuthorResponse = GeneratedSchemas['PublicAuthor'];
type AuthorNotesPageResponse = GeneratedSchemas['AuthorNotesPage'];
type SchemaKey<T> = Extract<keyof T, string>;
type SchemaKeyList<T> = readonly SchemaKey<T>[];
type ExhaustiveSchemaKeyList<T, K extends SchemaKeyList<T>> =
  Exclude<SchemaKey<T>, K[number]> extends never ? K : never;

const publicAuthorResponseKeys = schemaKeyList<PublicAuthorResponse>()([
  'display_name',
  'id',
  'note_count',
]);
const authorNotesPageResponseKeys = schemaKeyList<AuthorNotesPageResponse>()([
  'next_cursor',
  'notes',
]);

const client = () => createClient<paths>({ baseUrl: apiBaseURL() });

export async function getPublicAuthor(authorID: string): Promise<PublicAuthor> {
  const { data, response } = await client().GET('/v1/authors/{author_id}', {
    params: { path: { author_id: authorID } },
  });
  if (!response.ok) {
    throw new APIRequestError(response.status);
  }
  if (!isPublicAuthorResponse(data)) {
    throw new APIResponseError();
  }
  return {
    displayName: data.display_name,
    id: data.id,
    noteCount: data.note_count,
  };
}

export async function listAuthorNotes(
  input: ListAuthorNotesInput,
): Promise<AuthorNotesPage> {
  const { data, response } = await client().GET(
    '/v1/authors/{author_id}/notes',
    {
      params: {
        path: { author_id: input.authorID },
        query: {
          ...(input.limit === undefined ? {} : { limit: input.limit }),
          ...(input.cursor === undefined ? {} : { cursor: input.cursor }),
        },
      },
    },
  );
  if (!response.ok) {
    throw new APIRequestError(response.status);
  }
  if (!isAuthorNotesPageResponse(data)) {
    throw new APIResponseError();
  }
  return {
    nextCursor: data.next_cursor,
    notes: data.notes.map(parseNoteResponse),
  };
}

function isPublicAuthorResponse(value: unknown): value is PublicAuthorResponse {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, publicAuthorResponseKeys) &&
    typeof value.id === 'string' &&
    typeof value.display_name === 'string' &&
    typeof value.note_count === 'number' &&
    Number.isInteger(value.note_count) &&
    value.note_count >= 0
  );
}

function isAuthorNotesPageResponse(
  value: unknown,
): value is AuthorNotesPageResponse {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, authorNotesPageResponseKeys) &&
    Array.isArray(value.notes) &&
    (typeof value.next_cursor === 'string' || value.next_cursor === null)
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
