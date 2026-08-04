import { describe, expect, it } from 'vitest';
import {
  hasOnlyKeys,
  isAuthorSummary,
  isCommentResponse,
  isErrorResponse,
  isListNotesResponse,
  isNoteImageResponse,
  isNoteImagesResponse,
  isNoteResponse,
  isRecord,
  isSearchNoteResult,
  isSearchNotesResponse,
  isUpdateNoteRequest,
  isValidationProblem,
  parseCommentResponse,
  parseErrorResponse,
  parseListNotesResponse,
  parseNoteResponse,
  parseSearchNotesResponse,
  type NoteImageResponse,
  type NoteResponse,
} from './api-wire';

const author = { display_name: 'Marina', id: 'author-1' };

function imageFixture(overrides: Partial<NoteImageResponse> = {}): NoteImageResponse {
  return {
    byte_size: 1024,
    content_type: 'image/jpeg',
    created_at: 1,
    height: 100,
    id: 'image-1',
    position: 0,
    updated_at: 1,
    url: 'https://example.com/image.jpg',
    width: 100,
    ...overrides,
  };
}

function noteFixture(overrides: Partial<NoteResponse> = {}): NoteResponse {
  return {
    author,
    body: 'corpo',
    category_slug: 'food',
    created_at: 1,
    id: 'note-1',
    images: [],
    title: 'titulo',
    updated_at: 1,
    useful_by_current_user: false,
    useful_count: 0,
    ...overrides,
  };
}

describe('isRecord', () => {
  it('accepts a plain object', () => {
    expect(isRecord({})).toBe(true);
  });
  it.each([
    ['null', null],
    ['array', [1]],
    ['number', 1],
    ['string', 'x'],
    ['undefined', undefined],
  ])('rejects %s', (_label, value) => {
    expect(isRecord(value)).toBe(false);
  });
});

describe('hasOnlyKeys', () => {
  it('accepts an exact key set regardless of order', () => {
    expect(hasOnlyKeys({ b: 1, a: 1 }, ['a', 'b'])).toBe(true);
  });
  it('rejects an extra key', () => {
    expect(hasOnlyKeys({ a: 1, b: 1, c: 1 }, ['a', 'b'])).toBe(false);
  });
  it('rejects a missing key', () => {
    expect(hasOnlyKeys({ a: 1 }, ['a', 'b'])).toBe(false);
  });
});

describe('isAuthorSummary', () => {
  it('accepts a valid summary', () => {
    expect(isAuthorSummary(author)).toBe(true);
  });
  it('rejects an extra key', () => {
    expect(isAuthorSummary({ ...author, note_count: 0 })).toBe(false);
  });
});

describe('isNoteResponse', () => {
  it('accepts a valid note', () => {
    expect(isNoteResponse(noteFixture())).toBe(true);
  });
  it('rejects an extra key', () => {
    expect(isNoteResponse({ ...noteFixture(), extra: 1 })).toBe(false);
  });
  it('rejects a missing key', () => {
    const { body, ...missing } = noteFixture();
    void body;
    expect(isNoteResponse(missing)).toBe(false);
  });
  it('rejects a non-integer useful_count', () => {
    expect(isNoteResponse(noteFixture({ useful_count: 1.5 }))).toBe(false);
  });
  it('rejects a negative useful_count', () => {
    expect(isNoteResponse(noteFixture({ useful_count: -1 }))).toBe(false);
  });
});

describe('isUpdateNoteRequest', () => {
  it('accepts a single field', () => {
    expect(isUpdateNoteRequest({ title: 'novo' })).toBe(true);
  });
  it('accepts all fields', () => {
    expect(isUpdateNoteRequest({ body: 'b', category_slug: 'food', title: 't' })).toBe(true);
  });
  it('rejects an empty object', () => {
    expect(isUpdateNoteRequest({})).toBe(false);
  });
  it('rejects an unknown key', () => {
    expect(isUpdateNoteRequest({ body: 'b', image_upload_ids: ['x'] })).toBe(false);
  });
});

describe('isNoteImagesResponse', () => {
  it('accepts images whose position matches their index', () => {
    expect(
      isNoteImagesResponse([imageFixture({ position: 0 }), imageFixture({ id: 'image-2', position: 1 })]),
    ).toBe(true);
  });
  it('rejects an array whose position does not equal its index', () => {
    expect(isNoteImagesResponse([imageFixture({ position: 1 })])).toBe(false);
  });
});

describe('isNoteImageResponse', () => {
  it('accepts a valid image', () => {
    expect(isNoteImageResponse(imageFixture())).toBe(true);
  });
  it.each(['image/gif', 'image/png+xml', 'application/json'])(
    'rejects content_type %s',
    (contentType) => {
      expect(isNoteImageResponse(imageFixture({ content_type: contentType as NoteImageResponse['content_type'] }))).toBe(false);
    },
  );
  it('rejects a non-positive byte_size', () => {
    expect(isNoteImageResponse(imageFixture({ byte_size: 0 }))).toBe(false);
  });
  it('rejects a non-positive width', () => {
    expect(isNoteImageResponse(imageFixture({ width: 0 }))).toBe(false);
  });
});

describe('isCommentResponse', () => {
  it('accepts a top-level comment with parent_comment_id null', () => {
    expect(isCommentResponse({ author, body: 'corpo', created_at: 1, id: 'comment-1', parent_comment_id: null })).toBe(true);
  });
  it('accepts a reply with a parent UUID', () => {
    expect(isCommentResponse({ author, body: 'corpo', created_at: 1, id: 'comment-1', parent_comment_id: '11111111-1111-1111-1111-111111111111' })).toBe(true);
  });
  it('rejects an empty id', () => {
    expect(isCommentResponse({ author, body: 'corpo', created_at: 1, id: '', parent_comment_id: null })).toBe(false);
  });
  it('rejects a non-integer created_at', () => {
    expect(isCommentResponse({ author, body: 'corpo', created_at: 1.5, id: 'comment-1', parent_comment_id: null })).toBe(false);
  });
  it('rejects a non-string, non-null parent_comment_id', () => {
    expect(isCommentResponse({ author, body: 'corpo', created_at: 1, id: 'comment-1', parent_comment_id: 1234 })).toBe(false);
  });
  it('rejects a comment missing parent_comment_id', () => {
    expect(isCommentResponse({ author, body: 'corpo', created_at: 1, id: 'comment-1' })).toBe(false);
  });
});

describe('isSearchNoteResult', () => {
  it('accepts a lexical result', () => {
    expect(isSearchNoteResult({ note: noteFixture(), retrieval_source: 'lexical' })).toBe(true);
  });
  it.each(['semantic', 'hybrid'])('accepts retrieval_source %s', (source) => {
    expect(isSearchNoteResult({ note: noteFixture(), retrieval_source: source })).toBe(true);
  });
  it('rejects an unknown retrieval_source', () => {
    expect(isSearchNoteResult({ note: noteFixture(), retrieval_source: 'vector' })).toBe(false);
  });
});

describe('isSearchNotesResponse', () => {
  it('accepts a valid response', () => {
    expect(
      isSearchNotesResponse({ results: [{ note: noteFixture(), retrieval_source: 'lexical' }], search_version: 'fts5-v1' }),
    ).toBe(true);
  });

  it('accepts the hybrid search version', () => {
    expect(
      isSearchNotesResponse({
        results: [],
        search_version: 'hybrid-serafim100m-fts5-v1',
      }),
    ).toBe(true);
  });
  it('rejects a wrong search_version', () => {
    expect(isSearchNotesResponse({ results: [], search_version: 'fts5-v2' })).toBe(false);
  });
});

describe('isListNotesResponse', () => {
  it('accepts a valid response', () => {
    expect(isListNotesResponse({ notes: [noteFixture()] })).toBe(true);
  });
  it('rejects a non-array notes field', () => {
    expect(isListNotesResponse({ notes: 'nope' })).toBe(false);
  });
});

describe('isErrorResponse', () => {
  it('accepts a code-only error', () => {
    expect(isErrorResponse({ code: 'invalid_note' })).toBe(true);
  });
  it('accepts an error with validation fields', () => {
    expect(isErrorResponse({ code: 'invalid_note', fields: [{ code: 'required', field: 'title' }] })).toBe(true);
  });
  it('accepts embedding_unavailable', () => {
    expect(isErrorResponse({ code: 'embedding_unavailable' })).toBe(true);
  });
  it('rejects an unknown error code', () => {
    expect(isErrorResponse({ code: 'invalid' })).toBe(false);
  });
  it('rejects a non-string code', () => {
    expect(isErrorResponse({ code: 1 })).toBe(false);
  });
  it('rejects extra keys', () => {
    expect(isErrorResponse({ code: 'invalid_note', extra: true })).toBe(false);
  });
});

describe('isValidationProblem', () => {
  it('accepts a known code and field', () => {
    expect(isValidationProblem({ code: 'required', field: 'title' })).toBe(true);
  });
  it('rejects an unknown code', () => {
    expect(isValidationProblem({ code: 'unknown_code', field: 'title' })).toBe(false);
  });
  it('rejects an unknown field', () => {
    expect(isValidationProblem({ code: 'required', field: 'unknown_field' })).toBe(false);
  });
  it('rejects a missing field', () => {
    expect(isValidationProblem({ code: 'required' })).toBe(false);
  });
});

describe('parsers throw on invalid input and narrow on valid input', () => {
  it('parseNoteResponse', () => {
    expect(parseNoteResponse(noteFixture())).toEqual(noteFixture());
    expect(() => parseNoteResponse({})).toThrowError('invalid note response');
  });
  it('parseCommentResponse', () => {
    expect(parseCommentResponse({ author, body: 'b', created_at: 1, id: 'c1', parent_comment_id: null })).toMatchObject({ id: 'c1' });
    expect(() => parseCommentResponse({})).toThrowError('invalid comment response');
  });
  it('parseListNotesResponse', () => {
    expect(parseListNotesResponse({ notes: [] })).toEqual({ notes: [] });
    expect(() => parseListNotesResponse({})).toThrowError('invalid list notes response');
  });
  it('parseSearchNotesResponse', () => {
    expect(parseSearchNotesResponse({ results: [], search_version: 'fts5-v1' })).toEqual({
      results: [],
      search_version: 'fts5-v1',
    });
    expect(() => parseSearchNotesResponse({})).toThrowError('invalid search notes response');
  });
  it('parseErrorResponse', () => {
    expect(parseErrorResponse({ code: 'invalid_note' })).toEqual({ code: 'invalid_note' });
    expect(() => parseErrorResponse({})).toThrowError('invalid error response');
  });
});
