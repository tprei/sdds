import { apiBaseURL } from './config';
import {
  APIRequestError as SharedAPIRequestError,
} from './request-error';
import {
  listNotesResponseSchema,
  noteSchema,
  searchNotesResponseSchema,
} from './schema';
import type { TypedTransport } from './client';
import type { APIErrorResponse } from './request-error';
import type { components } from './generated/schema';

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
  usefulCount: number;
  usefulByCurrentUser: boolean;
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
export type SearchVersion = components['schemas']['SearchVersion'];
export type RetrievalSource = components['schemas']['RetrievalSource'];

export type SearchNoteResult = {
  note: Note;
  retrievalSource: RetrievalSource;
};

export type SearchNotesResult = {
  results: readonly SearchNoteResult[];
  searchVersion: SearchVersion;
};

type GeneratedSchemas = components['schemas'];
type AuthorSummaryResponse = GeneratedSchemas['AuthorSummary'];
type CreateNoteRequest = GeneratedSchemas['CreateNoteRequest'];
type NoteResponse = GeneratedSchemas['Note'];
type NoteImageResponse = GeneratedSchemas['NoteImage'];

export class APIRequestError extends SharedAPIRequestError {
  constructor(
    status: number,
    body: APIErrorResponse | null = null,
    retryAfter?: number,
  ) {
    super(status, body, retryAfter);
  }
}

export class APIResponseError extends Error {
  constructor() {
    super('api_response_invalid');
  }
}

export type NotesAPI = {
  listNotes(input: ListNotesInput): Promise<Note[]>;
  getNote(id: string): Promise<Note>;
  markNoteUseful(noteID: string): Promise<void>;
  unmarkNoteUseful(noteID: string): Promise<void>;
  searchNotes(input: SearchNotesInput): Promise<SearchNotesResult>;
  createNote(input: CreateNoteInput): Promise<Note>;
};

function rewrapTransportError(error: unknown): never {
  if (error instanceof SharedAPIRequestError) {
    throw new APIRequestError(error.status, error.body, error.retryAfter);
  }
  throw error;
}

export function bindNotesAPI(transport: TypedTransport): NotesAPI {
  return {
    async listNotes(input) {
      try {
        const { data } = await transport.GET('/v1/notes', {
          params: { query: noteListQuery(input) },
        });
        return parseListNotesResponse(data);
      } catch (error) {
        rewrapTransportError(error);
      }
    },

    async getNote(id) {
      try {
        const { data } = await transport.GET('/v1/notes/{note_id}', {
          params: { path: { note_id: id } },
        });
        return parseNoteResponse(data);
      } catch (error) {
        rewrapTransportError(error);
      }
    },

    async markNoteUseful(noteID) {
      try {
        await transport.PUT('/v1/notes/{note_id}/useful', {
          params: { path: { note_id: noteID } },
        });
      } catch (error) {
        rewrapTransportError(error);
      }
    },

    async unmarkNoteUseful(noteID) {
      try {
        await transport.DELETE('/v1/notes/{note_id}/useful', {
          params: { path: { note_id: noteID } },
        });
      } catch (error) {
        rewrapTransportError(error);
      }
    },

    async searchNotes(input) {
      try {
        const { data } = await transport.GET('/v1/search/notes', {
          params: { query: noteSearchQuery(input) },
        });
        return parseSearchNotesResponse(data);
      } catch (error) {
        rewrapTransportError(error);
      }
    },

    async createNote(input) {
      const request: CreateNoteRequest = {
        body: input.body,
        category_slug: input.categorySlug,
        client_request_id: input.clientRequestId,
        image_upload_ids: input.imageUploadIds,
        place_slug: input.placeSlug ?? null,
        title: input.title,
      };
      try {
        const { data } = await transport.POST('/v1/notes', { body: request });
        return parseNoteResponse(data);
      } catch (error) {
        rewrapTransportError(error);
      }
    },
  };
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

function parseListNotesResponse(value: unknown): Note[] {
  const listNotesResponse = listNotesResponseSchema.safeParse(value);
  if (!listNotesResponse.success) {
    throw new APIResponseError();
  }

  return listNotesResponse.data.notes.map(mapNoteResponse);
}
export function parseSearchNotesResponse(value: unknown): SearchNotesResult {
  const searchNotesResponse = searchNotesResponseSchema.safeParse(value);
  if (!searchNotesResponse.success) {
    throw new APIResponseError();
  }

  return {
    results: searchNotesResponse.data.results.map((result) => ({
      note: mapNoteResponse(result.note),
      retrievalSource: result.retrieval_source,
    })),
    searchVersion: searchNotesResponse.data.search_version,
  };
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
    usefulCount: value.useful_count,
    usefulByCurrentUser: value.useful_by_current_user,
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
