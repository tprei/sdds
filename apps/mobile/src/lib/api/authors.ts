import createClient from 'openapi-fetch';

import { apiBaseURL } from './config';
import { authorNotesPageSchema, publicAuthorSchema } from './schema';
import type { paths } from './generated/schema';
import {
  APIRequestError,
  APIResponseError,
  mapNoteResponse,
} from './notes';
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

export async function getPublicAuthor(authorID: string): Promise<PublicAuthor> {
  const { data, response } = await apiClient().GET('/v1/authors/{author_id}', {
    params: { path: { author_id: authorID } },
  });
  if (!response.ok) {
    throw new APIRequestError(response.status);
  }
  const publicAuthorResponse = publicAuthorSchema.safeParse(data);
  if (!publicAuthorResponse.success) {
    throw new APIResponseError();
  }
  return {
    displayName: publicAuthorResponse.data.display_name,
    id: publicAuthorResponse.data.id,
    noteCount: publicAuthorResponse.data.note_count,
  };
}

export async function listAuthorNotes(
  input: ListAuthorNotesInput,
): Promise<AuthorNotesPage> {
  const { data, response } = await apiClient().GET(
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
  const authorNotesPageResponse = authorNotesPageSchema.safeParse(data);
  if (!authorNotesPageResponse.success) {
    throw new APIResponseError();
  }
  return {
    nextCursor: authorNotesPageResponse.data.next_cursor,
    notes: authorNotesPageResponse.data.notes.map(mapNoteResponse),
  };
}

