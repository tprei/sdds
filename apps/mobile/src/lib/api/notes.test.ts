import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  APIRequestError,
  APIResponseError,
  createNote,
  getNote,
  listNotes,
} from './notes';
import type { components } from './generated/schema';

vi.mock('react-native', () => ({
  Platform: {
    OS: 'ios',
  },
}));

const configuredAPIBaseURLEnvName = 'EXPO_PUBLIC_SDDS_API_BASE_URL';
const exampleNoteID = '018ff5b8-0000-7000-8000-000000000000';

type FetchCall = {
  request: Request;
};

type FetchHandler = (request: Request) => Promise<Response>;
type ListNotesResponse = components['schemas']['ListNotesResponse'];
type NoteResponse = components['schemas']['Note'];

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

    await createNote({
      body: 'Tem pão de queijo decente.',
      category: 'comida',
      city: 'sao-paulo',
      title: 'Café bom',
    });

    const request = onlyFetchCall(calls);
    expect(request.url).toBe('http://localhost:8080/v1/notes');
    expect(request.method).toBe('POST');
    expect(request.headers.get('content-type')).toBe('application/json');
    await expect(requestJSON(request)).resolves.toEqual({
      body: 'Tem pão de queijo decente.',
      category_slug: 'comida',
      city_slug: 'sao-paulo',
      title: 'Café bom',
    });
  });

  it('parses created notes from the API wire shape', async () => {
    stubFetch(async () => jsonResponse(apiNote(), httpStatusCreated));

    const note = await createNote({
      body: 'Tem pão de queijo decente.',
      category: 'comida',
      city: 'sao-paulo',
      title: 'Café bom',
    });

    expect(note).toEqual({
      body: 'Tem pão de queijo decente.',
      category: 'comida',
      city: 'sao-paulo',
      createdAt: 1782993600000,
      id: exampleNoteID,
      title: 'Café bom',
      updatedAt: 1782993600000,
    });
  });

  it('raises request errors from status even when the error body fails', async () => {
    stubFetch(async () => unreadableResponse(httpStatusBadRequest));

    await expect(
      createNote({
        body: 'Tem pão de queijo decente.',
        category: 'comida',
        city: 'sao-paulo',
        title: 'Café bom',
      }),
    ).rejects.toMatchObject(new APIRequestError(httpStatusBadRequest));
  });

  it('parses listed notes from the API list response shape', async () => {
    stubFetch(async () => jsonResponse(apiListNotesResponse()));

    const notes = await listNotes();

    expect(notes).toEqual([
      {
        body: 'Tem pão de queijo decente.',
        category: 'comida',
        city: 'sao-paulo',
        createdAt: 1782993600000,
        id: exampleNoteID,
        title: 'Café bom',
        updatedAt: 1782993600000,
      },
    ]);
  });

  it('sends get note requests with the note id in the path', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return jsonResponse(apiNote());
    });

    await getNote(exampleNoteID);

    const request = onlyFetchCall(calls);
    expect(request.url).toBe(
      `http://localhost:8080/v1/notes/${exampleNoteID}`,
    );
    expect(request.method).toBe('GET');
  });

  it('parses fetched notes from the API wire shape', async () => {
    stubFetch(async () => jsonResponse(apiNote()));

    const note = await getNote(exampleNoteID);

    expect(note).toEqual({
      body: 'Tem pão de queijo decente.',
      category: 'comida',
      city: 'sao-paulo',
      createdAt: 1782993600000,
      id: exampleNoteID,
      title: 'Café bom',
      updatedAt: 1782993600000,
    });
  });

  it('raises request errors for missing fetched notes', async () => {
    stubFetch(async () => jsonResponse({ code: 'not_found' }, httpStatusNotFound));

    await expect(getNote('missing-note')).rejects.toMatchObject(
      new APIRequestError(httpStatusNotFound),
    );
  });

  it('rejects invalid fetched note response shapes', async () => {
    stubFetch(async () =>
      jsonResponse({
        ...apiNote(),
        category_slug: 'qualquer',
      }),
    );

    await expect(
      getNote(exampleNoteID),
    ).rejects.toThrow(APIResponseError);
  });

  it('rejects unexpected response shapes', async () => {
    stubFetch(async () =>
      jsonResponse({
        notes: [
          {
            body: 'Tem pão de queijo decente.',
            category: 'comida',
            city: 'sao-paulo',
            created_at: 1782993600000,
            id: exampleNoteID,
            title: 'Café bom',
            updated_at: 1782993600000,
          },
        ],
      }),
    );

    await expect(listNotes()).rejects.toThrow(APIResponseError);
  });

  it('rejects undocumented response fields', async () => {
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

    await expect(listNotes()).rejects.toThrow(APIResponseError);
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

    await expect(listNotes()).rejects.toThrow(APIResponseError);
  });

  it('rejects unknown response slugs', async () => {
    stubFetch(async () =>
      jsonResponse({
        notes: [
          {
            ...apiNote(),
            category_slug: 'qualquer',
            city_slug: 'qualquer',
          },
        ],
      }),
    );

    await expect(listNotes()).rejects.toThrow(APIResponseError);
  });
});

const httpStatusCreated = 201;
const httpStatusBadRequest = 400;
const httpStatusNotFound = 404;

function apiListNotesResponse(): ListNotesResponse {
  return {
    notes: [apiNote()],
  };
}

function apiNote(): NoteResponse {
  return {
    body: 'Tem pão de queijo decente.',
    category_slug: 'comida',
    city_slug: 'sao-paulo',
    created_at: 1782993600000,
    id: exampleNoteID,
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
