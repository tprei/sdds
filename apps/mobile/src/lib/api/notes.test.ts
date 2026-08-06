import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  APIRequestError,
  APIResponseError,
} from './notes';
import { createAPIClient } from './client';
import type { components } from './generated/schema';

vi.mock('react-native', () => ({
  Platform: {
    OS: 'ios',
  },
}));

vi.mock('expo-file-system', () => ({
  File: class {},
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
type SearchNotesResponse = components['schemas']['SearchNotesResponse'];

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

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await client.createNote(
      {
        body: 'Tem pao de queijo decente.',
        categorySlug: 'food',
        clientRequestId: 'mobile-create-note-wire',
        title: 'Cafe bom',
      },
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
      title: 'Cafe bom',
    });
  });

  it('parses created notes from the API wire shape', async () => {
    stubFetch(async () => jsonResponse(apiNote(), httpStatusCreated));

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    const note = await client.createNote(
      {
        body: 'Tem pao de queijo decente.',
        categorySlug: 'food',
        clientRequestId: 'mobile-create-note-response',
        title: 'Cafe bom',
      },
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
      title: 'Cafe bom',
      updatedAt: 1782993600000,
      usefulCount: 0,
      usefulByCurrentUser: false,
    });
  });

  it('raises request errors from status even when the error body fails', async () => {
    stubFetch(async () => unreadableResponse(httpStatusBadRequest));

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(
      client.createNote(
        {
          body: 'Tem pao de queijo decente.',
          categorySlug: 'food',
          clientRequestId: 'mobile-create-note-error',
          title: 'Cafe bom',
        },
      ),
    ).rejects.toMatchObject(new APIRequestError(httpStatusBadRequest));
  });

  it('sends update note requests with only the provided fields', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return jsonResponse(apiNote({ title: 'Cafe bom editado' }));
    });

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await client.updateNote({
      noteID: exampleNoteID,
      title: 'Cafe bom editado',
    });

    const request = onlyFetchCall(calls);
    expect(request.url).toBe(`http://localhost:8080/v1/notes/${exampleNoteID}`);
    expect(request.method).toBe('PATCH');
    expect(request.headers.get('content-type')).toBe('application/json');
    await expect(requestJSON(request)).resolves.toEqual({ title: 'Cafe bom editado' });
  });

  it('parses updated notes from the API wire shape', async () => {
    stubFetch(async () => jsonResponse(apiNote({ title: 'Cafe bom editado', body: 'corpo novo' })));

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    const note = await client.updateNote({
      noteID: exampleNoteID,
      body: 'corpo novo',
      title: 'Cafe bom editado',
    });

    expect(note.title).toBe('Cafe bom editado');
  });

  it('sends delete note requests', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return new Response(null, { status: 204 });
    });

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await client.deleteNote(exampleNoteID);

    const request = onlyFetchCall(calls);
    expect(request.url).toBe(`http://localhost:8080/v1/notes/${exampleNoteID}`);
    expect(request.method).toBe('DELETE');
  });

  it('raises a request error for a forbidden note delete', async () => {
    stubFetch(async () => unreadableResponse(403));

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(client.deleteNote(exampleNoteID)).rejects.toMatchObject(
      new APIRequestError(403),
    );
  });

  it('raises a request error for an unknown note update', async () => {
    stubFetch(async () => unreadableResponse(httpStatusNotFound));

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(
      client.updateNote({ noteID: exampleNoteID, title: 'sumido' }),
    ).rejects.toMatchObject(new APIRequestError(httpStatusNotFound));
  });

  it('parses listed notes from the API list response shape', async () => {
    stubFetch(async () => jsonResponse(apiListNotesResponse()));

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    const notes = await client.listNotes({});

    expect(notes).toEqual([expectedNote()]);
  });

  it('omits category filters from list note requests by default', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return jsonResponse(apiListNotesResponse());
    });

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await client.listNotes({});

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

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await client.listNotes({ categorySlug: 'food' });

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

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await client.getNote(exampleNoteID);

    const request = onlyFetchCall(calls);
    expect(request.url).toBe(`http://localhost:8080/v1/notes/${exampleNoteID}`);
    expect(request.method).toBe('GET');
  });

  it('parses fetched notes from the API wire shape', async () => {
    stubFetch(async () => jsonResponse(apiNote()));

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    const note = await client.getNote(exampleNoteID);

    expect(note).toEqual(expectedNote());
  });

  it('sends mark useful requests with bearer auth and no body parsing', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return new Response(null, { status: 204 });
    });

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await client.markNoteUseful(exampleNoteID);

    const request = onlyFetchCall(calls);
    expect(request.url).toBe(
      `http://localhost:8080/v1/notes/${exampleNoteID}/useful`,
    );
    expect(request.method).toBe('PUT');
    expect(request.headers.get('authorization')).toBe(`Bearer ${exampleToken}`);
  });

  it('sends unmark useful requests with bearer auth and no body parsing', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return new Response(null, { status: 204 });
    });

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await client.unmarkNoteUseful(exampleNoteID);

    const request = onlyFetchCall(calls);
    expect(request.url).toBe(
      `http://localhost:8080/v1/notes/${exampleNoteID}/useful`,
    );
    expect(request.method).toBe('DELETE');
    expect(request.headers.get('authorization')).toBe(`Bearer ${exampleToken}`);
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

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(client.getNote(exampleNoteID)).resolves.toMatchObject({
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

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(client.getNote(exampleNoteID)).resolves.toMatchObject({
      images: [{ url: 'https://api.example.com/v1/media/images/image-id' }],
    });
  });

  it('rejects malformed image URLs', async () => {
    stubFetch(async () =>
      jsonResponse(apiNote({ images: [apiImage({ url: 'http://[::1' })] })),
    );

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(client.getNote(exampleNoteID)).rejects.toThrow(APIResponseError);
  });

  it('rejects note responses without required images', async () => {
    const note: Record<string, unknown> = { ...apiNote() };
    delete note.images;
    stubFetch(async () => jsonResponse({ notes: [note] }));

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(client.listNotes({})).rejects.toThrow(APIResponseError);
  });

  it('raises request errors for missing fetched notes', async () => {
    stubFetch(async () =>
      jsonResponse({ code: 'not_found' }, httpStatusNotFound),
    );

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(client.getNote('missing-note')).rejects.toMatchObject(
      new APIRequestError(httpStatusNotFound, { code: 'not_found' }),
    );
  });

  it('accepts API-owned slugs without client catalog membership checks', async () => {
    stubFetch(async () =>
      jsonResponse(
        apiNote({
          category_slug: 'future-category',
        }),
      ),
    );

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(client.getNote(exampleNoteID)).resolves.toMatchObject({
      categorySlug: 'future-category',
    });
  });

  it('defaults useful_by_current_user to false when absent (anonymous read)', async () => {
    const { useful_by_current_user, ...withoutViewer } = apiNote();
    void useful_by_current_user;
    stubFetch(async () => jsonResponse(withoutViewer));

    const client = createAPIClient();
    const note = await client.getNote(exampleNoteID);

    expect(note.usefulByCurrentUser).toBe(false);
  });

  it('sends search note requests with the raw query parameter', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return jsonResponse(apiSearchNotesResponse());
    });

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await client.searchNotes({ query: 'restaurante brasileiro Dublin 12 barato' });

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
      return jsonResponse(apiSearchNotesResponse());
    });

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await client.searchNotes({ categorySlug: 'food', query: 'cafe' });

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
      return jsonResponse(apiSearchNotesResponse());
    });

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await client.searchNotes({ query: '  cafe bom  ' });

    const request = onlyFetchCall(calls);
    const url = new URL(request.url);
    expect(url.searchParams.get('q')).toBe('  cafe bom  ');
  });

  it('parses searched notes from the API list response shape', async () => {
    stubFetch(async () => jsonResponse(apiSearchNotesResponse()));

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    const notes = await client.searchNotes({ query: 'cafe' });
    expect(notes).toEqual({
      results: [
        {
          note: expectedNote(),
          retrievalSource: 'lexical',
        },
      ],
      searchVersion: 'fts5-v1',
    });
  });
  it('parses the hybrid search version from the API response', async () => {
    stubFetch(async () =>
      jsonResponse({
        ...apiSearchNotesResponse(),
        search_version: 'hybrid-serafim100m-fts5-v1',
      }),
    );

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    const notes = await client.searchNotes({ query: 'cafe' });
    expect(notes.searchVersion).toBe('hybrid-serafim100m-fts5-v1');
  });
  it('rejects missing search response wrapper fields', async () => {
    const { search_version: _searchVersion, ...response } =
      apiSearchNotesResponse();
    stubFetch(async () => jsonResponse(response));

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(client.searchNotes({ query: 'cafe' })).rejects.toThrow(
      APIResponseError,
    );
  });

  it('rejects extra search response wrapper fields', async () => {
    stubFetch(async () =>
      jsonResponse({ ...apiSearchNotesResponse(), extra: true }),
    );

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(client.searchNotes({ query: 'cafe' })).rejects.toThrow(
      APIResponseError,
    );
  });

  it('rejects unknown search version and retrieval source', async () => {
    stubFetch(async () =>
      jsonResponse({
        ...apiSearchNotesResponse(),
        search_version: 'future-v2',
        results: [
          { note: apiNote(), retrieval_source: 'unknown-source' },
        ],
      }),
    );

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(client.searchNotes({ query: 'cafe' })).rejects.toThrow(
      APIResponseError,
    );
  });

  it('raises request errors from search status codes', async () => {
    stubFetch(async () =>
      jsonResponse({ code: 'invalid_search' }, httpStatusBadRequest),
    );

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(client.searchNotes({ query: '' })).rejects.toMatchObject(
      new APIRequestError(httpStatusBadRequest, { code: 'invalid_search' }),
    );
  });

  it('rejects invalid searched note response shapes', async () => {
    stubFetch(async () =>
      jsonResponse({
        notes: [
          {
            ...apiNote(),
          },
        ],
      }),
    );

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(client.searchNotes({ query: 'cafe' })).rejects.toThrow(
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
            title: 'Cafe bom',
            updated_at: 1782993600000,
          },
        ],
      }),
    );

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(client.listNotes({})).rejects.toThrow(APIResponseError);
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

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(client.listNotes({})).resolves.toEqual([expectedNote()]);
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

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(client.listNotes({})).rejects.toThrow(APIResponseError);
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

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(client.listNotes({})).rejects.toThrow(APIResponseError);
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

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(client.listNotes({})).resolves.toEqual([expectedNote()]);
  });
});

const httpStatusCreated = 201;
const httpStatusBadRequest = 400;
const httpStatusNotFound = 404;

function apiListNotesResponse(): ListNotesResponse {
  return { notes: [apiNote()] };
}
function apiSearchNotesResponse(): SearchNotesResponse {
  return {
    results: [{ note: apiNote(), retrieval_source: 'lexical' }],
    search_version: 'fts5-v1',
  };
}

function apiNote(overrides: Partial<NoteResponse> = {}): NoteResponse {
  return {
    author: { display_name: 'Thiago', id: 'author-id' },
    body: 'Tem pao de queijo decente.',
    category_slug: 'food',
    created_at: 1782993600000,
    id: exampleNoteID,
    images: [],
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
    title: 'Cafe bom',
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
