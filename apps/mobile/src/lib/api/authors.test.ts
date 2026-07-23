import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  APIRequestError,
  APIResponseError,
} from './notes';
import {
  getPublicAuthor,
  listAuthorNotes,
} from './authors';

vi.mock('react-native', () => ({
  Platform: {
    OS: 'ios',
  },
}));

const configuredAPIBaseURLEnvName = 'EXPO_PUBLIC_SDDS_API_BASE_URL';
const authorID = 'author-id';
const noteID = 'note-id';

const exampleToken = 'session-token';

const malformedPageCases: [string, unknown][] = [
  ['missing cursor', { notes: [apiNote()] }],
  ['empty cursor', { next_cursor: '', notes: [] }],
  ['oversized cursor', { next_cursor: 'x'.repeat(513), notes: [] }],
  ['invalid cursor type', { next_cursor: 42, notes: [] }],
];

type FetchHandler = (request: Request) => Promise<Response>;

describe('authors API client', () => {
  beforeEach(() => {
    delete process.env[configuredAPIBaseURLEnvName];
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('parses public author responses without private fields', async () => {
    stubFetch(async () =>
      jsonResponse({
        display_name: 'Thiago',
        id: authorID,
        note_count: 3,
      }),
    );

    await expect(getPublicAuthor(authorID, exampleToken)).resolves.toEqual({
      displayName: 'Thiago',
      id: authorID,
      noteCount: 3,
    });
  });

  it('forwards author note pagination and parses the shared note shape', async () => {
    const requests: Request[] = [];
    stubFetch(async (request) => {
      requests.push(request);
      return jsonResponse({
        next_cursor: 'next-cursor',
        notes: [apiNote()],
      });
    });

    await expect(
      listAuthorNotes({ authorID, cursor: 'after-cursor', limit: 2 }, exampleToken),
    ).resolves.toEqual({
      nextCursor: 'next-cursor',
      notes: [expectedNote()],
    });

    const request = onlyRequest(requests);
    const url = new URL(request.url);
    expect(url.pathname).toBe(`/v1/authors/${authorID}/notes`);
    expect(url.searchParams.get('cursor')).toBe('after-cursor');
    expect(url.searchParams.get('limit')).toBe('2');
  });

  it('accepts a terminal author note page without a cursor', async () => {
    stubFetch(async () => jsonResponse({ next_cursor: null, notes: [] }));

    await expect(listAuthorNotes({ authorID }, exampleToken)).resolves.toEqual({
      nextCursor: null,
      notes: [],
    });
  });

  it('ignores extra author note page fields', async () => {
    stubFetch(async () =>
      jsonResponse({
        next_cursor: null,
        notes: [
          {
            ...apiNote(),
            author: { display_name: 'Thiago', id: authorID, user_id: 'private' },
          },
        ],
        user_id: 'private-user-id',
      }),
    );

    await expect(listAuthorNotes({ authorID }, exampleToken)).resolves.toEqual({
      nextCursor: null,
      notes: [expectedNote()],
    });
  });

  it.each(malformedPageCases)(
    'rejects malformed author note page: %s',
    async (_name, response) => {
      stubFetch(async () => jsonResponse(response));

      await expect(listAuthorNotes({ authorID }, exampleToken)).rejects.toThrow(
        APIResponseError,
      );
    },
  );

  it('ignores extra public author response fields', async () => {
    stubFetch(async () =>
      jsonResponse({
        display_name: 'Thiago',
        id: authorID,
        note_count: 3,
        username: 'private-name',
      }),
    );

    await expect(getPublicAuthor(authorID, exampleToken)).resolves.toEqual({
      displayName: 'Thiago',
      id: authorID,
      noteCount: 3,
    });
  });

  it('raises request errors for author status failures', async () => {
    stubFetch(async () => jsonResponse({ code: 'not_found' }, 404));

    await expect(getPublicAuthor(authorID, exampleToken)).rejects.toMatchObject(
      new APIRequestError(404),
    );
  });

  it('raises request errors for author note status failures', async () => {
    stubFetch(async () => jsonResponse({ code: 'invalid_note' }, 400));

    await expect(listAuthorNotes({ authorID }, exampleToken)).rejects.toMatchObject(
      new APIRequestError(400),
    );
  });

  it('raises request errors for unreadable author status bodies', async () => {
    stubFetch(async () => unreadableResponse(404));

    await expect(getPublicAuthor(authorID, exampleToken)).rejects.toMatchObject(
      new APIRequestError(404),
    );
  });

  it('raises request errors for unreadable author note status bodies', async () => {
    stubFetch(async () => unreadableResponse(500));

    await expect(listAuthorNotes({ authorID }, exampleToken)).rejects.toMatchObject(
      new APIRequestError(500),
    );
  });
});

function apiNote() {
  return {
    author: {
      display_name: 'Thiago',
      id: authorID,
    },
    body: 'Tem pão de queijo decente.',
    category_slug: 'food',
    created_at: 1782993600000,
    id: noteID,
    images: [],
    place_slug: null,
    useful_count: 0,
    useful_by_current_user: false,
    title: 'Café bom',
    updated_at: 1782993600000,
  };
}

function expectedNote() {
  return {
    author: {
      displayName: 'Thiago',
      id: authorID,
    },
    body: 'Tem pão de queijo decente.',
    categorySlug: 'food',
    createdAt: 1782993600000,
    id: noteID,
    images: [],
    placeSlug: null,
    title: 'Café bom',
    updatedAt: 1782993600000,
    usefulCount: 0,
    usefulByCurrentUser: false,
  };
}

function jsonResponse(value: unknown, status = 200): Response {
  return new Response(JSON.stringify(value), {
    headers: {
      'Content-Type': 'application/json',
    },
    status,
  });
}

function unreadableResponse(status: number): Response {
  const body = new ReadableStream({
    start(controller) {
      controller.error(new Error('body_unreadable'));
    },
  });

  return new Response(body, { status });
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
