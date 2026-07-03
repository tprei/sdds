import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  APIResponseError,
  createNote,
  listNotes,
} from './notes';
import type { components } from './generated/schema';

vi.mock('react-native', () => ({
  Platform: {
    OS: 'ios',
  },
}));

const configuredAPIBaseURLEnvName = 'EXPO_PUBLIC_SDDS_API_BASE_URL';

type FetchCall = {
  init: RequestInit | undefined;
  input: Request | URL | string;
};

type FetchHandler = (
  input: Request | URL | string,
  init?: RequestInit,
) => Promise<Response>;
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
    stubFetch(async (input, init) => {
      calls.push({ input, init });
      return jsonResponse(apiNote(), httpStatusCreated);
    });

    await createNote({
      body: 'Tem pão de queijo decente.',
      category: 'comida',
      city: 'sao-paulo',
      title: 'Café bom',
    });

    const call = onlyFetchCall(calls);
    expect(call.input).toBe('http://localhost:8080/v1/notes');
    expect(call.init?.method).toBe('POST');
    expect(call.init?.headers).toEqual({
      'Content-Type': 'application/json',
    });
    expect(requestJSON(call)).toEqual({
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
      id: '018ff5b8-0000-7000-8000-000000000000',
      title: 'Café bom',
      updatedAt: 1782993600000,
    });
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
        id: '018ff5b8-0000-7000-8000-000000000000',
        title: 'Café bom',
        updatedAt: 1782993600000,
      },
    ]);
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
            id: '018ff5b8-0000-7000-8000-000000000000',
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
    id: '018ff5b8-0000-7000-8000-000000000000',
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

function onlyFetchCall(calls: FetchCall[]): FetchCall {
  if (calls.length !== 1) {
    throw new Error(`fetch call count = ${calls.length}, want 1`);
  }

  const call = calls[0];
  if (call === undefined) {
    throw new Error('fetch call missing');
  }

  return call;
}

function requestJSON(call: FetchCall): unknown {
  if (typeof call.init?.body !== 'string') {
    throw new Error('fetch body is not JSON text');
  }

  return JSON.parse(call.init.body);
}

function stubFetch(handler: FetchHandler): void {
  vi.stubGlobal('fetch', handler);
}
