import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  APIRequestError,
  APIResponseError,
} from './notes';
import { createAPIClient } from './client';

vi.mock('react-native', () => ({
  Platform: {
    OS: 'ios',
  },
}));

vi.mock('expo-file-system', () => ({
  File: class {},
}));

const configuredAPIBaseURLEnvName = 'EXPO_PUBLIC_SDDS_API_BASE_URL';
const exampleToken = 'session-token';
const noteID = 'note-id';
const authorID = 'author-id';

const malformedCommentCases: [string, unknown][] = [
  ['missing threads', { next_cursor: null }],
  ['missing cursor', { threads: [] }],
  ['empty cursor', { threads: [], next_cursor: '' }],
  ['oversized cursor', { threads: [], next_cursor: 'x'.repeat(513) }],
  ['missing comment author', { threads: [apiThread({ body: 'Oi', created_at: 1, id: 'comment-id' })], next_cursor: null }],
  ['invalid timestamp', { threads: [apiThread({ ...apiComment(), created_at: -1 })], next_cursor: null }],
];

type FetchHandler = (request: Request) => Promise<Response>;

describe('comments API client', () => {
  beforeEach(() => {
    delete process.env[configuredAPIBaseURLEnvName];
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('lists ordered comments through the authenticated client', async () => {
    const requests: Request[] = [];
    stubFetch(async (request) => {
      requests.push(request);
      return jsonResponse({
        threads: [
          apiThread(apiComment({ body: 'Primeiro comentário', id: 'comment-1' })),
          apiThread(apiComment({ body: 'Segundo comentário', id: 'comment-2' })),
        ],
        next_cursor: 'next-cursor',
      });
    });

    const client = createAPIClient(exampleToken);
    await expect(
      client.listNoteComments({ noteID, cursor: 'after-cursor', limit: 2 }),
    ).resolves.toEqual({
      threads: [
        expectedThread(expectedComment({ body: 'Primeiro comentário', id: 'comment-1' })),
        expectedThread(expectedComment({ body: 'Segundo comentário', id: 'comment-2' })),
      ],
      nextCursor: 'next-cursor',
    });

    const request = onlyRequest(requests);
    const url = new URL(request.url);
    expect(url.pathname).toBe(`/v1/notes/${noteID}/comments`);
    expect(url.searchParams.get('cursor')).toBe('after-cursor');
    expect(url.searchParams.get('limit')).toBe('2');
    expect(request.headers.get('Authorization')).toBe(`Bearer ${exampleToken}`);
  });

  it('creates a comment with the exact JSON body and maps its response', async () => {
    const requests: Request[] = [];
    stubFetch(async (request) => {
      requests.push(request);
      return jsonResponse(apiComment({ body: 'Comentário novo', id: 'comment-new' }), 201);
    });

    const client = createAPIClient(exampleToken);
    await expect(
      client.createNoteComment({ noteID, body: 'Comentário novo' }),
    ).resolves.toEqual(expectedComment({ body: 'Comentário novo', id: 'comment-new' }));

    const request = onlyRequest(requests);
    const url = new URL(request.url);
    expect(request.method).toBe('POST');
    expect(url.pathname).toBe(`/v1/notes/${noteID}/comments`);
    expect(request.headers.get('Authorization')).toBe(`Bearer ${exampleToken}`);
    await expect(request.json()).resolves.toEqual({ body: 'Comentário novo' });
  });

  it('creates a reply under a parent comment and maps the response', async () => {
    const requests: Request[] = [];
    stubFetch(async (request) => {
      requests.push(request);
      return jsonResponse(
        apiComment({ body: 'Resposta', id: 'reply-1', parent_comment_id: 'comment-1' }),
        201,
      );
    });

    const client = createAPIClient(exampleToken);
    await expect(
      client.createCommentReply({ parentCommentID: 'comment-1', body: 'Resposta' }),
    ).resolves.toEqual(
      expectedComment({ body: 'Resposta', id: 'reply-1', parentCommentID: 'comment-1' }),
    );

    const request = onlyRequest(requests);
    const url = new URL(request.url);
    expect(request.method).toBe('POST');
    expect(url.pathname).toBe('/v1/comments/comment-1/replies');
    expect(request.headers.get('Authorization')).toBe(`Bearer ${exampleToken}`);
    await expect(request.json()).resolves.toEqual({ body: 'Resposta' });
  });

  it('deletes a comment through the authenticated client', async () => {
    const requests: Request[] = [];
    stubFetch(async (request) => {
      requests.push(request);
      return new Response(null, { status: 204 });
    });

    const client = createAPIClient(exampleToken);
    await expect(
      client.deleteNoteComment({ commentID: 'comment-id', noteID }),
    ).resolves.toBeUndefined();

    const request = onlyRequest(requests);
    expect(request.method).toBe('DELETE');
    expect(new URL(request.url).pathname).toBe(
      `/v1/notes/${noteID}/comments/comment-id`,
    );
    expect(request.headers.get('Authorization')).toBe(`Bearer ${exampleToken}`);
  });

  it('preserves a terminal null cursor', async () => {
    stubFetch(async () => jsonResponse({ threads: [], next_cursor: null }));

    const client = createAPIClient(exampleToken);
    await expect(client.listNoteComments({ noteID })).resolves.toEqual({
      threads: [],
      nextCursor: null,
    });
  });

  it('accepts a response body with 1000 Unicode code points', async () => {
    const body = '😀'.repeat(1000);
    stubFetch(async () =>
      jsonResponse({ threads: [apiThread(apiComment({ body }))], next_cursor: null }),
    );

    const client = createAPIClient(exampleToken);
    await expect(client.listNoteComments({ noteID })).resolves.toEqual({
      threads: [expectedThread(expectedComment({ body }))],
      nextCursor: null,
    });
  });

  it.each(malformedCommentCases)(
    'rejects malformed comment response: %s',
    async (_name, response) => {
      stubFetch(async () => jsonResponse(response));

      const client = createAPIClient(exampleToken);
      await expect(client.listNoteComments({ noteID })).rejects.toThrow(
        APIResponseError,
      );
    },
  );

  it('retains structured invalid_comment errors and validation fields', async () => {
    stubFetch(async () =>
      jsonResponse(
        {
          code: 'invalid_comment',
          fields: [{ field: 'body', code: 'required' }],
        },
        400,
      ),
    );

    const client = createAPIClient(exampleToken);
    const error = await client
      .createNoteComment({ noteID, body: ' ' })
      .catch((caught: unknown) => caught);

    expect(error).toBeInstanceOf(APIRequestError);
    expect(error).toMatchObject({
      body: {
        code: 'invalid_comment',
        fields: [{ field: 'body', code: 'required' }],
      },
      code: 'invalid_comment',
      fields: [{ field: 'body', code: 'required' }],
      status: 400,
    });
  });

  it.each([
    ['not found', 404, 'not_found'],
    ['unauthenticated', 401, 'unauthenticated'],
    ['request too large', 413, 'request_too_large'],
  ])('retains %s request errors', async (_name, status, code) => {
    stubFetch(async () => jsonResponse({ code }, status));

    const client = createAPIClient(exampleToken);
    await expect(client.createNoteComment({ noteID, body: 'ok' })).rejects.toMatchObject({
      body: { code },
      code,
      status,
    });
  });

  it('retains an invalid-reply-target conflict from a reply request', async () => {
    stubFetch(async () => jsonResponse({ code: 'invalid_reply_target' }, 409));

    const client = createAPIClient(exampleToken);
    await expect(
      client.createCommentReply({ parentCommentID: 'comment-1', body: 'Resposta' }),
    ).rejects.toMatchObject({
      body: { code: 'invalid_reply_target' },
      code: 'invalid_reply_target',
      status: 409,
    });
    await expect(
      client.createCommentReply({ parentCommentID: 'comment-1', body: 'Resposta' }),
    ).rejects.toBeInstanceOf(APIRequestError);
  });

  it.each([
    ['forbidden', 403, 'forbidden'],
    ['not found', 404, 'not_found'],
  ])('retains %s delete errors', async (_name, status, code) => {
    stubFetch(async () => jsonResponse({ code }, status));

    const client = createAPIClient(exampleToken);
    await expect(
      client.deleteNoteComment({ commentID: 'comment-id', noteID }),
    ).rejects.toMatchObject({
      body: { code },
      code,
      status,
    });
  });
});

function apiComment(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    author: { display_name: 'Thiago', id: authorID },
    body: 'Comentário útil',
    created_at: 1782993600000,
    id: 'comment-id',
    parent_comment_id: null,
    ...overrides,
  };
}

function apiThread(comment: Record<string, unknown>) {
  return { comment, replies: [], has_more_replies: false };
}

function expectedComment(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    author: { displayName: 'Thiago', id: authorID },
    body: 'Comentário útil',
    createdAt: 1782993600000,
    id: 'comment-id',
    parentCommentID: null,
    ...overrides,
  };
}

function expectedThread(comment: Record<string, unknown>) {
  return { comment, replies: [], hasMoreReplies: false };
}

function jsonResponse(value: unknown, status = 200): Response {
  return new Response(JSON.stringify(value), {
    headers: {
      'Content-Type': 'application/json',
    },
    status,
  });
}

function onlyRequest(requests: Request[]): Request {
  if (requests.length !== 1) {
    throw new Error(`fetch call count = ${requests.length}, want 1`);
  }
  const request = requests[0];
  if (request === undefined) {
    throw new Error('fetch request missing');
  }
  return request;
}

function stubFetch(handler: FetchHandler): void {
  vi.stubGlobal('fetch', handler);
}
