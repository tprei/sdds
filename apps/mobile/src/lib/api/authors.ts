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

const maxAuthorNotesCursorLength = 512;

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
    Array.isArray(value.notes) &&
    (value.next_cursor === null ||
      isAuthorNotesCursor(value.next_cursor))
  );
}

function isAuthorNotesCursor(value: unknown): value is string {
  return (
    typeof value === 'string' &&
    value.length > 0 &&
    value.length <= maxAuthorNotesCursorLength
  );
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
