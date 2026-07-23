import createClient from 'openapi-fetch';

import { apiBaseURL } from './config';
import { authorNotesPageSchema, publicAuthorSchema } from './schema';
import type { paths } from './generated/schema';
import type { TypedTransport } from './client';
import { APIRequestError as SharedAPIRequestError } from './request-error';
import { APIRequestError, APIResponseError, mapNoteResponse } from './notes';
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

function apiClient(token: string) {
  return createClient<paths>({
    baseUrl: apiBaseURL(),
    fetch: (request) => apiFetch(request, token),
  });
}

async function apiFetch(request: Request, token: string): Promise<Response> {
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

function authenticatedRequest(request: Request, token: string): Request {
  const headers = new Headers(request.headers);
  headers.set('Authorization', `Bearer ${token}`);
  return new Request(request, { headers });
}

export async function getPublicAuthor(
  authorID: string,
  token: string,
): Promise<PublicAuthor> {
  const { data, response } = await apiClient(token).GET('/v1/authors/{author_id}', {
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
  token: string,
): Promise<AuthorNotesPage> {
  const { data, response } = await apiClient(token).GET(
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


export type AuthorsAPI = {
  getPublicAuthor(authorID: string): Promise<PublicAuthor>;
  listAuthorNotes(input: ListAuthorNotesInput): Promise<AuthorNotesPage>;
};

function rewrapAuthorsTransportError(error: unknown): never {
  if (error instanceof SharedAPIRequestError) {
    throw new APIRequestError(error.status);
  }
  throw error;
}

export function bindAuthorsAPI(transport: TypedTransport): AuthorsAPI {
  return {
    async getPublicAuthor(authorID) {
      try {
        const { data } = await transport.GET('/v1/authors/{author_id}', {
          params: { path: { author_id: authorID } },
        });
        const publicAuthorResponse = publicAuthorSchema.safeParse(data);
        if (!publicAuthorResponse.success) {
          throw new APIResponseError();
        }
        return {
          displayName: publicAuthorResponse.data.display_name,
          id: publicAuthorResponse.data.id,
          noteCount: publicAuthorResponse.data.note_count,
        };
      } catch (error) {
        rewrapAuthorsTransportError(error);
      }
    },

    async listAuthorNotes(input) {
      try {
        const { data } = await transport.GET(
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
        const authorNotesPageResponse = authorNotesPageSchema.safeParse(data);
        if (!authorNotesPageResponse.success) {
          throw new APIResponseError();
        }
        return {
          nextCursor: authorNotesPageResponse.data.next_cursor,
          notes: authorNotesPageResponse.data.notes.map(mapNoteResponse),
        };
      } catch (error) {
        rewrapAuthorsTransportError(error);
      }
    },
  };
}
