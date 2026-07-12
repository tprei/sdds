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

    await expect(getPublicAuthor(authorID)).resolves.toEqual({
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
      listAuthorNotes({ authorID, cursor: 'after-cursor', limit: 2 }),
    ).resolves.toMatchObject({
      nextCursor: 'next-cursor',
      notes: [
        expect.objectContaining({
          author: { displayName: 'Thiago', id: authorID },
          id: noteID,
        }),
      ],
    });

    const request = onlyRequest(requests);
    const url = new URL(request.url);
    expect(url.pathname).toBe(`/v1/authors/${authorID}/notes`);
    expect(url.searchParams.get('cursor')).toBe('after-cursor');
    expect(url.searchParams.get('limit')).toBe('2');
  });

  it('rejects private fields in public author responses', async () => {
    stubFetch(async () =>
      jsonResponse({
        display_name: 'Thiago',
        id: authorID,
        note_count: 3,
        username: 'private-name',
      }),
    );

    await expect(getPublicAuthor(authorID)).rejects.toThrow(APIResponseError);
  });

  it('raises request errors for author status failures', async () => {
    stubFetch(async () => jsonResponse({ code: 'not_found' }, 404));

    await expect(getPublicAuthor(authorID)).rejects.toMatchObject(
      new APIRequestError(404),
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
    place_slug: null,
    title: 'Café bom',
    updated_at: 1782993600000,
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
