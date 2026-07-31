
import { authorNotesPageSchema, publicAuthorSchema } from './schema';
import type { TypedTransport } from './client';
import { APIRequestError as SharedAPIRequestError } from './request-error';
import { APIRequestError, APIResponseError, mapNoteResponse } from './notes';
import type { Note } from './notes';

export type PublicAuthor = {
  id: string;
  displayName: string;
  noteCount: number;
  usefulReceivedCount: number;
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
          usefulReceivedCount: publicAuthorResponse.data.useful_received_count,
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
