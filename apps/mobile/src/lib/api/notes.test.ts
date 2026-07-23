import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  APIRequestError,
  APIResponseError,
  createNote,
  getNote,
  listNotes,
  searchNotes,
} from './notes';
import type { components } from './generated/schema';

vi.mock('react-native', () => ({
  Platform: {
    OS: 'ios',
  },
}));

const configuredAPIBaseURLEnvName = 'EXPO_PUBLIC_SDDS_API_BASE_URL';
const exampleNoteID = '018ff5b8-0000-7000-8000-000000000000';
const exampleToken = 'session-token';
type FetchCall = {
  request: Request;
};
type FetchHandler = (request: Request) => Promise<Response>;
type NoteResponse = components['schemas']['Note'];
type NoteImageResponse = components['schemas']['NoteImage'];
type ListNotesResponse = components['schemas']['ListNotesResponse'];

describe('notes API client', () => {
  beforeEach(() => {
    delete process.env[configuredAPIBaseURLEnvName];
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });
  it('sends create note requests with API wire keys', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return jsonResponse(apiNote(), httpStatusCreated);
    });

    await createNote(
      {
        body: 'Tem pao de queijo decente.',
        categorySlug: 'food',
        clientRequestId: 'mobile-create-note-wire',
        placeSlug: 'sao-paulo',
        title: 'Cafe bom',
      },
      exampleToken,
    );

    const request = onlyFetchCall(calls);
    expect(request.url).toBe('http://localhost:8080/v1/notes');
    expect(request.method).toBe('POST');
    expect(request.headers.get('authorization')).toBe(`Bearer ${exampleToken}`);
    expect(request.headers.get('content-type')).toBe('application/json');
    await expect(requestJSON(request)).resolves.toEqual({
      body: 'Tem pao de queijo decente.',
      category_slug: 'food',
      client_request_id: 'mobile-create-note-wire',
      place_slug: 'sao-paulo',
      title: 'Cafe bom',
    });
  });

  it('sends null place when create note input omits place', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return jsonResponse(apiNote({ place_slug: null }), httpStatusCreated);
    });

    await createNote(
      {
        body: 'Tem pao de queijo decente.',
        categorySlug: 'food',
        clientRequestId: 'mobile-create-note-without-place',
        title: 'Cafe bom',
      },
      exampleToken,
    );

    await expect(requestJSON(onlyFetchCall(calls))).resolves.toMatchObject({
      place_slug: null,
    });
  });

  it('parses created notes from the API wire shape', async () => {
    stubFetch(async () => jsonResponse(apiNote(), httpStatusCreated));

    const note = await createNote(
      {
        body: 'Tem pao de queijo decente.',
        categorySlug: 'food',
        clientRequestId: 'mobile-create-note-response',
        placeSlug: 'sao-paulo',
        title: 'Cafe bom',
      },
      exampleToken,
    );

    expect(note).toEqual({
      author: {
        displayName: 'Thiago',
        id: 'author-id',
      },
      body: 'Tem pao de queijo decente.',
      categorySlug: 'food',
      createdAt: 1782993600000,
      id: exampleNoteID,
      images: [],
      placeSlug: 'sao-paulo',
      title: 'Cafe bom',
      updatedAt: 1782993600000,
    });
  });

  it('parses notes without a place', async () => {
    stubFetch(async () => jsonResponse(apiNote({ place_slug: null })));

    await expect(getNote(exampleNoteID, exampleToken)).resolves.toMatchObject({
      placeSlug: null,
    });
  });
  it('raises request errors from status even when the error body fails', async () => {
    stubFetch(async () => unreadableResponse(httpStatusBadRequest));

    await expect(
      createNote(
        {
          body: 'Tem pao de queijo decente.',
          categorySlug: 'food',
          clientRequestId: 'mobile-create-note-error',
          placeSlug: 'sao-paulo',
          title: 'Cafe bom',
        },
        exampleToken,
      ),
    ).rejects.toMatchObject(new APIRequestError(httpStatusBadRequest));
  });

  it('parses listed notes from the API list response shape', async () => {
    stubFetch(async () => jsonResponse(apiListNotesResponse()));

    const notes = await listNotes({}, exampleToken);

    expect(notes).toEqual([expectedNote()]);
  });

  it('omits category filters from list note requests by default', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return jsonResponse(apiListNotesResponse());
    });

    await listNotes({}, exampleToken);

    const request = onlyFetchCall(calls);
    const url = new URL(request.url);
    expect(url.pathname).toBe('/v1/notes');
    expect(url.searchParams.has('category_slug')).toBe(false);
    expect(request.method).toBe('GET');
  });

  it('sends category filters on list note requests', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return jsonResponse(apiListNotesResponse());
    });

    await listNotes({ categorySlug: 'food' }, exampleToken);

    const request = onlyFetchCall(calls);
    const url = new URL(request.url);
    expect(url.pathname).toBe('/v1/notes');
    expect(url.searchParams.get('category_slug')).toBe('food');
    expect(request.method).toBe('GET');
  });

  it('sends get note requests with the note id in the path', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return jsonResponse(apiNote());
    });

    await getNote(exampleNoteID, exampleToken);

    const request = onlyFetchCall(calls);
    expect(request.url).toBe(`http://localhost:8080/v1/notes/${exampleNoteID}`);
    expect(request.method).toBe('GET');
  });

  it('parses fetched notes from the API wire shape', async () => {
    stubFetch(async () => jsonResponse(apiNote()));

    const note = await getNote(exampleNoteID, exampleToken);

    expect(note).toEqual(expectedNote());
  });

  it('resolves root-relative image URLs and preserves absolute URLs', async () => {
    stubFetch(async () =>
      jsonResponse(
        apiNote({
          images: [
            apiImage(),
            apiImage({
              id: 'image-id-2',
              position: 1,
              url: 'https://cdn.example.com/image-id-2.png',
            }),
          ],
        }),
      ),
    );

    await expect(getNote(exampleNoteID, exampleToken)).resolves.toMatchObject({
      images: [
        {
          byteSize: 481234,
          contentType: 'image/jpeg',
          createdAt: 1782993600000,
          height: 900,
          id: 'image-id',
          position: 0,
          updatedAt: 1782993600000,
          url: 'http://localhost:8080/v1/media/images/image-id',
          width: 1200,
        },
        {
          id: 'image-id-2',
          position: 1,
          url: 'https://cdn.example.com/image-id-2.png',
        },
      ],
    });
  });

  it('resolves root-relative image URLs against a configured API base', async () => {
    process.env[configuredAPIBaseURLEnvName] =
      'https://api.example.com/mobile/';
    stubFetch(async () =>
      jsonResponse(
        apiNote({
          images: [apiImage({ url: '/v1/media/images/image-id' })],
        }),
      ),
    );

    await expect(getNote(exampleNoteID, exampleToken)).resolves.toMatchObject({
      images: [{ url: 'https://api.example.com/v1/media/images/image-id' }],
    });
  });

  it('rejects malformed image URLs', async () => {
    stubFetch(async () =>
      jsonResponse(apiNote({ images: [apiImage({ url: 'http://[::1' })] })),
    );

    await expect(getNote(exampleNoteID, exampleToken)).rejects.toThrow(APIResponseError);
  });

  it('rejects note responses without required images', async () => {
    const note: Record<string, unknown> = { ...apiNote() };
    delete note.images;
    stubFetch(async () => jsonResponse({ notes: [note] }));

    await expect(listNotes({}, exampleToken)).rejects.toThrow(APIResponseError);
  });

  it('raises request errors for missing fetched notes', async () => {
    stubFetch(async () =>
      jsonResponse({ code: 'not_found' }, httpStatusNotFound),
    );

    await expect(getNote('missing-note', exampleToken)).rejects.toMatchObject(
      new APIRequestError(httpStatusNotFound, { code: 'not_found' }),
    );
  });

  it('accepts API-owned slugs without client catalog membership checks', async () => {
    stubFetch(async () =>
      jsonResponse(
        apiNote({
          category_slug: 'future-category',
          place_slug: 'future-place',
        }),
      ),
    );

    await expect(getNote(exampleNoteID, exampleToken)).resolves.toMatchObject({
      categorySlug: 'future-category',
      placeSlug: 'future-place',
    });
  });

  it('sends search note requests with the raw query parameter', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return jsonResponse(apiListNotesResponse());
    });

    await searchNotes({ query: 'restaurante brasileiro Dublin 12 barato' }, exampleToken);

    const request = onlyFetchCall(calls);
    const url = new URL(request.url);
    expect(url.origin).toBe('http://localhost:8080');
    expect(url.pathname).toBe('/v1/search/notes');
    expect(url.searchParams.get('q')).toBe(
      'restaurante brasileiro Dublin 12 barato',
    );
    expect(url.searchParams.has('category_slug')).toBe(false);
    expect(request.method).toBe('GET');
  });

  it('sends category filters on search note requests', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return jsonResponse(apiListNotesResponse());
    });

    await searchNotes({ categorySlug: 'food', query: 'cafe' }, exampleToken);

    const request = onlyFetchCall(calls);
    const url = new URL(request.url);
    expect(url.pathname).toBe('/v1/search/notes');
    expect(url.searchParams.get('q')).toBe('cafe');
    expect(url.searchParams.get('category_slug')).toBe('food');
    expect(request.method).toBe('GET');
  });

  it('sends accented and spaced search text without client-side parsing', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return jsonResponse(apiListNotesResponse());
    });

    await searchNotes({ query: '  cafe bom  ' }, exampleToken);

    const request = onlyFetchCall(calls);
    const url = new URL(request.url);
    expect(url.searchParams.get('q')).toBe('  cafe bom  ');
  });

  it('parses searched notes from the API list response shape', async () => {
    stubFetch(async () => jsonResponse(apiListNotesResponse()));

    const notes = await searchNotes({ query: 'cafe' }, exampleToken);

    expect(notes).toEqual([expectedNote()]);
  });

  it('raises request errors from search status codes', async () => {
    stubFetch(async () =>
      jsonResponse({ code: 'invalid_search' }, httpStatusBadRequest),
    );

    await expect(searchNotes({ query: '' }, exampleToken)).rejects.toMatchObject(
      new APIRequestError(httpStatusBadRequest, { code: 'invalid_search' }),
    );
  });

  it('rejects invalid searched note response shapes', async () => {
    stubFetch(async () =>
      jsonResponse({
        notes: [
          {
            ...apiNote(),
            place_slug: 42,
          },
        ],
      }),
    );

    await expect(searchNotes({ query: 'cafe' }, exampleToken)).rejects.toThrow(
      APIResponseError,
    );
  });

  it('rejects unexpected response shapes', async () => {
    stubFetch(async () =>
      jsonResponse({
        notes: [
          {
            body: 'Tem pao de queijo decente.',
            category: 'food',
            created_at: 1782993600000,
            id: exampleNoteID,
            place: 'sao-paulo',
            title: 'Cafe bom',
            updated_at: 1782993600000,
          },
        ],
      }),
    );

    await expect(listNotes({}, exampleToken)).rejects.toThrow(APIResponseError);
  });

  it('ignores extra note response fields', async () => {
    stubFetch(async () =>
      jsonResponse({
        notes: [
          {
            ...apiNote(),
            summary: 'curto',
          },
        ],
      }),
    );

    await expect(listNotes({}, exampleToken)).resolves.toEqual([expectedNote()]);
  });

  it('rejects invalid timestamp values', async () => {
    stubFetch(async () =>
      jsonResponse({
        notes: [
          {
            ...apiNote(),
            created_at: 1.5,
            updated_at: -1,
          },
        ],
      }),
    );

    await expect(listNotes({}, exampleToken)).rejects.toThrow(APIResponseError);
  });

  it('rejects invalid author response shapes', async () => {
    stubFetch(async () =>
      jsonResponse({
        notes: [
          {
            ...apiNote(),
            author: {
              display_name: 'Thiago',
              user_id: 'private-user-id',
            },
          },
        ],
      }),
    );

    await expect(listNotes({}, exampleToken)).rejects.toThrow(APIResponseError);
  });

  it('ignores extra legacy city slug response fields', async () => {
    stubFetch(async () =>
      jsonResponse({
        notes: [
          {
            ...apiNote(),
            city_slug: 'sao-paulo',
          },
        ],
      }),
    );

    await expect(listNotes({}, exampleToken)).resolves.toEqual([expectedNote()]);
  });
});

const httpStatusCreated = 201;
const httpStatusBadRequest = 400;
const httpStatusNotFound = 404;

function apiListNotesResponse(): ListNotesResponse {
  return { notes: [apiNote()] };
}

function apiNote(overrides: Partial<NoteResponse> = {}): NoteResponse {
  return {
    author: { display_name: 'Thiago', id: 'author-id' },
    body: 'Tem pao de queijo decente.',
    category_slug: 'food',
    created_at: 1782993600000,
    id: exampleNoteID,
    images: [],
    place_slug: 'sao-paulo',
    useful_count: 0,
    useful_by_current_user: false,
    title: 'Cafe bom',
    updated_at: 1782993600000,
    ...overrides,
  };
}

function apiImage(
  overrides: Partial<NoteImageResponse> = {},
): NoteImageResponse {
  return {
    byte_size: 481234,
    content_type: 'image/jpeg',
    created_at: 1782993600000,
    height: 900,
    id: 'image-id',
    position: 0,
    updated_at: 1782993600000,
    url: '/v1/media/images/image-id',
    width: 1200,
    ...overrides,
  };
}
function expectedNote() {
  return {
    author: {
      displayName: 'Thiago',
      id: 'author-id',
    },
    body: 'Tem pao de queijo decente.',
    categorySlug: 'food',
    createdAt: 1782993600000,
    id: exampleNoteID,
    images: [],
    placeSlug: 'sao-paulo',
    title: 'Cafe bom',
    updatedAt: 1782993600000,
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
function onlyFetchCall(calls: FetchCall[]): Request {
  if (calls.length !== 1) {
    throw new Error(`fetch call count = ${calls.length}, want 1`);
  }

  const call = calls[0];
  if (call === undefined) {
    throw new Error('fetch call missing');
  }

  return call.request;
}
async function requestJSON(request: Request): Promise<unknown> {
  return request.clone().json();
}
function stubFetch(handler: FetchHandler): void {
  vi.stubGlobal('fetch', handler);
}
