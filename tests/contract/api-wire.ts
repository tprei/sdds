// parser-to-wire boundary: typed guards and parsers for the public API wire
// shapes. Pure functions with no browser or HTTP dependency, so they live in a
// deterministic Vitest suite rather than Playwright.

export type AuthorSummary = {
  display_name: string;
  id: string;
};

export type CreateNoteRequest = {
  body: string;
  category_slug: string;
  client_request_id: string;
  image_upload_ids?: string[];
  title: string;
};

export type UpdateNoteRequest = {
  body?: string;
  category_slug?: string;
  title?: string;
};

export type NoteImageResponse = {
  byte_size: number;
  content_type: 'image/jpeg' | 'image/png';
  created_at: number;
  height: number;
  id: string;
  position: number;
  updated_at: number;
  url: string;
  width: number;
};

export type NoteResponse = {
  author: AuthorSummary;
  body: string;
  category_slug: string;
  created_at: number;
  id: string;
  images: NoteImageResponse[];
  title: string;
  updated_at: number;
  useful_count: number;
  useful_by_current_user?: boolean;
};

export type CommentResponse = {
  author: AuthorSummary;
  body: string;
  created_at: number;
  id: string;
  parent_comment_id: string | null;
};

export type PublicAuthorResponse = {
  display_name: string;
  id: string;
  note_count: number;
  useful_received_count: number;
};

export type AuthorNotesResponse = {
  next_cursor: string | null;
  notes: NoteResponse[];
};

export type ListNotesResponse = {
  notes: NoteResponse[];
};

export type SearchNoteResult = {
  note: NoteResponse;
  retrieval_source: 'lexical' | 'semantic' | 'hybrid';
};

export type SearchNotesResponse = {
  results: SearchNoteResult[];
  search_version: 'fts5-v1' | 'hybrid-serafim100m-fts5-v1';
};

export type ErrorResponse = {
  code: string;
  fields?: ValidationProblem[];
};

export type ValidationProblem = {
  code: string;
  field: string;
};

export const authorSummaryKeys = ['display_name', 'id'] as const;
export const commentResponseKeys = ['author', 'body', 'created_at', 'id', 'parent_comment_id'] as const;
export const searchNoteResultKeys = ['note', 'retrieval_source'] as const;
export const searchNotesResponseKeys = ['results', 'search_version'] as const;
export const listNotesResponseKeys = ['notes'] as const;
export const noteImageResponseKeys = [
  'byte_size',
  'content_type',
  'created_at',
  'height',
  'id',
  'position',
  'updated_at',
  'url',
  'width',
] as const;
export const noteResponseKeys = [
  'author',
  'body',
  'category_slug',
  'created_at',
  'id',
  'images',
  'title',
  'updated_at',
  'useful_count',
  'useful_by_current_user',
] as const;
// useful_by_current_user is optional: present for authenticated callers, absent
// for anonymous public reads. Every other note key is required.
export const requiredNoteResponseKeys = [
  'author',
  'body',
  'category_slug',
  'created_at',
  'id',
  'images',
  'title',
  'updated_at',
  'useful_count',
] as const;
export const createNoteRequestKeys = [
  'body',
  'category_slug',
  'client_request_id',
  'image_upload_ids',
  'title',
] as const;
export const updateNoteRequestKeys = ['body', 'category_slug', 'title'] as const;

export const errorCodes: Record<string, true> = {
  internal_error: true,
  forbidden: true,
  invalid_auth: true,
  invalid_comment: true,
  invalid_json: true,
  invalid_note: true,
  invalid_report: true,
  invalid_search: true,
  not_found: true,
  rate_limited: true,
  request_too_large: true,
  unauthenticated: true,
  username_taken: true,
  invalid_media: true,
  unsupported_media_type: true,
  idempotency_conflict: true,
  upload_in_progress: true,
  upload_expired: true,
  media_staging_quota_exceeded: true,
  media_storage_unavailable: true,
  media_integrity_error: true,
  too_many_images: true,
  invalid_event: true,
  invalid_event_batch: true,
  embedding_unavailable: true,
};

export const validationProblemCodes: Record<string, true> = {
  required: true,
  too_short: true,
  too_long: true,
  unknown: true,
  invalid: true,
  taken: true,
};

export const validationFields: Record<string, true> = {
  title: true,
  body: true,
  category_slug: true,
  q: true,
  username: true,
  password: true,
  display_name: true,
  limit: true,
  cursor: true,
  client_request_id: true,
  upload_request_id: true,
  image_upload_ids: true,
  file: true,
  target_type: true,
  target_id: true,
  reason: true,
  details: true,
};

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function hasOnlyKeys(
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

// (allowed/required key checks for note responses are inlined in isNoteResponse
// because useful_by_current_user is optional there and required everywhere else
// hasOnlyKeys is used.)

export function isListNotesResponse(value: unknown): value is ListNotesResponse {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, listNotesResponseKeys) &&
    Array.isArray(value.notes) &&
    value.notes.every(isNoteResponse)
  );
}

export function isSearchNotesResponse(value: unknown): value is SearchNotesResponse {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, searchNotesResponseKeys) &&
    (value.search_version === 'fts5-v1' ||
      value.search_version === 'hybrid-serafim100m-fts5-v1') &&
    Array.isArray(value.results) &&
    value.results.every(isSearchNoteResult)
  );
}

export function isSearchNoteResult(value: unknown): value is SearchNoteResult {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, searchNoteResultKeys) &&
    isNoteResponse(value.note) &&
    (value.retrieval_source === 'lexical' ||
      value.retrieval_source === 'semantic' ||
      value.retrieval_source === 'hybrid')
  );
}

export function isNoteResponse(value: unknown): value is NoteResponse {
  return (
    isRecord(value) &&
    Object.keys(value).every((key) => (noteResponseKeys as readonly string[]).includes(key)) &&
    requiredNoteResponseKeys.every((key) => Object.prototype.hasOwnProperty.call(value, key)) &&
    typeof value.id === 'string' &&
    isAuthorSummary(value.author) &&
    typeof value.title === 'string' &&
    typeof value.body === 'string' &&
    typeof value.category_slug === 'string' &&
    typeof value.created_at === 'number' &&
    typeof value.updated_at === 'number' &&
    typeof value.useful_count === 'number' &&
    Number.isInteger(value.useful_count) &&
    value.useful_count >= 0 &&
    (value.useful_by_current_user === undefined || typeof value.useful_by_current_user === 'boolean') &&
    isNoteImagesResponse(value.images)
  );
}

export function isUpdateNoteRequest(value: unknown): value is UpdateNoteRequest {
  if (!isRecord(value)) {
    return false;
  }
  const keys = Object.keys(value);
  if (keys.length === 0) {
    return false;
  }
  if (
    !keys.every((key) =>
      updateNoteRequestKeys.includes(key as (typeof updateNoteRequestKeys)[number]),
    )
  ) {
    return false;
  }
  if ('title' in value && typeof value.title !== 'string') {
    return false;
  }
  if ('body' in value && typeof value.body !== 'string') {
    return false;
  }
  if ('category_slug' in value && typeof value.category_slug !== 'string') {
    return false;
  }
  return true;
}

export function isCommentResponse(value: unknown): value is CommentResponse {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, commentResponseKeys) &&
    typeof value.id === 'string' &&
    value.id.length > 0 &&
    typeof value.body === 'string' &&
    value.body.length > 0 &&
    isAuthorSummary(value.author) &&
    typeof value.created_at === 'number' &&
    Number.isInteger(value.created_at) &&
    value.created_at >= 0 &&
    (typeof value.parent_comment_id === 'string' ||
      value.parent_comment_id === null)
  );
}

export function isNoteImagesResponse(value: unknown): value is NoteImageResponse[] {
  if (!Array.isArray(value)) {
    return false;
  }

  for (const [index, image] of value.entries()) {
    if (!isNoteImageResponse(image) || image.position !== index) {
      return false;
    }
  }
  return true;
}

export function isNoteImageResponse(value: unknown): value is NoteImageResponse {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, noteImageResponseKeys) &&
    typeof value.id === 'string' &&
    typeof value.url === 'string' &&
    value.url.length > 0 &&
    (value.content_type === 'image/jpeg' ||
      value.content_type === 'image/png') &&
    typeof value.byte_size === 'number' &&
    Number.isInteger(value.byte_size) &&
    value.byte_size > 0 &&
    typeof value.width === 'number' &&
    Number.isInteger(value.width) &&
    value.width > 0 &&
    typeof value.height === 'number' &&
    Number.isInteger(value.height) &&
    value.height > 0 &&
    typeof value.position === 'number' &&
    Number.isInteger(value.position) &&
    value.position >= 0 &&
    typeof value.created_at === 'number' &&
    Number.isInteger(value.created_at) &&
    typeof value.updated_at === 'number' &&
    Number.isInteger(value.updated_at)
  );
}

export function isAuthorSummary(value: unknown): value is AuthorSummary {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, authorSummaryKeys) &&
    typeof value.id === 'string' &&
    typeof value.display_name === 'string'
  );
}

export function isErrorResponse(value: unknown): value is ErrorResponse {
  if (!isRecord(value)) return false;
  if (!Object.keys(value).every((k) => k === 'code' || k === 'fields'))
    return false;
  if (typeof value.code !== 'string' || !(value.code in errorCodes))
    return false;
  return (
    value.fields === undefined ||
    (Array.isArray(value.fields) && value.fields.every(isValidationProblem))
  );
}

export function isValidationProblem(value: unknown): value is ValidationProblem {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, ['code', 'field']) &&
    typeof value.code === 'string' &&
    validationProblemCodes[value.code as string] !== undefined &&
    typeof value.field === 'string' &&
    validationFields[value.field as string] !== undefined
  );
}

export function parseListNotesResponse(value: unknown): ListNotesResponse {
  if (!isListNotesResponse(value)) {
    throw new Error('invalid list notes response');
  }
  return value;
}

export function parseSearchNotesResponse(value: unknown): SearchNotesResponse {
  if (!isSearchNotesResponse(value)) {
    throw new Error('invalid search notes response');
  }
  return value;
}

export function parseNoteResponse(value: unknown): NoteResponse {
  if (!isNoteResponse(value)) {
    throw new Error('invalid note response');
  }
  return value;
}

export function parseCommentResponse(value: unknown): CommentResponse {
  if (!isCommentResponse(value)) {
    throw new Error('invalid comment response');
  }
  return value;
}

export function parseErrorResponse(value: unknown): ErrorResponse {
  if (!isErrorResponse(value)) {
    throw new Error('invalid error response');
  }
  return value;
}
