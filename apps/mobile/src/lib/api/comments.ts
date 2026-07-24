import {
  commentSchema,
  listNoteCommentsResponseSchema,
} from './schema';
import type { TypedTransport } from './client';
import { APIRequestError as SharedAPIRequestError } from './request-error';
import { APIRequestError, APIResponseError } from './notes';
import type { components } from './generated/schema';

export type CommentAuthor = {
  id: string;
  displayName: string;
};

export type Comment = {
  id: string;
  body: string;
  author: CommentAuthor;
  createdAt: number;
};

export type CommentPage = {
  comments: Comment[];
  nextCursor: string | null;
};

export type ListNoteCommentsInput = {
  noteID: string;
  limit?: number;
  cursor?: string;
};

export type CreateNoteCommentInput = {
  noteID: string;
  body: string;
};

export type DeleteNoteCommentInput = {
  noteID: string;
  commentID: string;
};

type GeneratedSchemas = components['schemas'];
type CommentResponse = GeneratedSchemas['Comment'];
type CreateCommentRequest = GeneratedSchemas['CreateCommentRequest'];


export type CommentsAPI = {
  listNoteComments(input: ListNoteCommentsInput): Promise<CommentPage>;
  createNoteComment(input: CreateNoteCommentInput): Promise<Comment>;
  deleteNoteComment(input: DeleteNoteCommentInput): Promise<void>;
};

export function bindCommentsAPI(transport: TypedTransport): CommentsAPI {
  return {
    async listNoteComments(input) {
      try {
        const { data } = await transport.GET('/v1/notes/{note_id}/comments', {
          params: {
            path: { note_id: input.noteID },
            query: commentListQuery(input),
          },
        });
        return parseCommentPage(data);
      } catch (error) {
        rewrapTransportError(error);
      }
    },

    async createNoteComment(input) {
      const request: CreateCommentRequest = { body: input.body };
      try {
        const { data } = await transport.POST('/v1/notes/{note_id}/comments', {
          params: { path: { note_id: input.noteID } },
          body: request,
        });
        return parseCommentResponse(data);
      } catch (error) {
        rewrapTransportError(error);
      }
    },

    async deleteNoteComment(input) {
      try {
        await transport.DELETE('/v1/notes/{note_id}/comments/{comment_id}', {
          params: {
            path: {
              comment_id: input.commentID,
              note_id: input.noteID,
            },
          },
        });
      } catch (error) {
        rewrapTransportError(error);
      }
    },
  };
}

function rewrapTransportError(error: unknown): never {
  if (error instanceof SharedAPIRequestError) {
    throw new APIRequestError(error.status, error.body, error.retryAfter);
  }
  throw error;
}

function commentListQuery(input: ListNoteCommentsInput): {
  limit?: number;
  cursor?: string;
} {
  return {
    ...(input.limit === undefined ? {} : { limit: input.limit }),
    ...(input.cursor === undefined ? {} : { cursor: input.cursor }),
  };
}

function parseCommentPage(value: unknown): CommentPage {
  const response = listNoteCommentsResponseSchema.safeParse(value);
  if (!response.success) {
    throw new APIResponseError();
  }
  return {
    comments: response.data.comments.map(mapCommentResponse),
    nextCursor: response.data.next_cursor,
  };
}

function parseCommentResponse(value: unknown): Comment {
  const response = commentSchema.safeParse(value);
  if (!response.success) {
    throw new APIResponseError();
  }
  return mapCommentResponse(response.data);
}

function mapCommentResponse(value: CommentResponse): Comment {
  return {
    id: value.id,
    body: value.body,
    author: {
      id: value.author.id,
      displayName: value.author.display_name,
    },
    createdAt: value.created_at,
  };
}
