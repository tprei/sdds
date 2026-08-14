import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { APIRequestError, APIResponseError } from './notes';
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
const targetID = 'note-id';

const malformedReceiptCases: [string, unknown][] = [
  ['missing id', { target_type: 'note', target_id: 'note-id', reason: 'spam', details: null, created_at: 1 }],
  ['missing target_type', { id: 'report-id', target_id: 'note-id', reason: 'spam', details: null, created_at: 1 }],
  ['missing target_id', { id: 'report-id', target_type: 'note', reason: 'spam', details: null, created_at: 1 }],
  ['missing reason', { id: 'report-id', target_type: 'note', target_id: 'note-id', details: null, created_at: 1 }],
  ['missing details', { id: 'report-id', target_type: 'note', target_id: 'note-id', reason: 'spam', created_at: 1 }],
  ['missing created_at', { id: 'report-id', target_type: 'note', target_id: 'note-id', reason: 'spam', details: null }],
  ['unknown reason', apiReceipt({ reason: 'vendetta' })],
  ['unknown target_type', apiReceipt({ target_type: 'user' })],
  ['wrong reason type', apiReceipt({ reason: 42 })],
  ['wrong created_at type', apiReceipt({ created_at: 'soon' })],
  ['negative created_at', apiReceipt({ created_at: -1 })],
  ['non-integer created_at', apiReceipt({ created_at: 1.5 })],
  ['wrong details type', apiReceipt({ details: 7 })],
  ['wrong id type', apiReceipt({ id: 5 })],
];

type FetchHandler = (request: Request) => Promise<Response>;

describe('reports API client', () => {
  beforeEach(() => {
    delete process.env[configuredAPIBaseURLEnvName];
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('posts the exact JSON body with trimmed details and maps the receipt', async () => {
    const requests: Request[] = [];
    stubFetch(async (request) => {
      requests.push(request);
      return jsonResponse(apiReceipt({ details: ' spam details ' }), 201);
    });

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(
      client.createReport({
        targetType: 'note',
        targetID,
        reason: 'spam',
        details: '  spam details  ',
      }),
    ).resolves.toEqual(expectedReceipt({ details: ' spam details ' }));

    const request = onlyRequest(requests);
    const url = new URL(request.url);
    expect(request.method).toBe('POST');
    expect(url.pathname).toBe('/v1/reports');
    expect(request.headers.get('Authorization')).toBe(`Bearer ${exampleToken}`);
    await expect(request.json()).resolves.toEqual({
      target_type: 'note',
      target_id: targetID,
      reason: 'spam',
      details: 'spam details',
    });
  });

  it.each([
    ['undefined details', undefined],
    ['whitespace-only details', '   '],
  ])('omits details from the body when %s', async (_name, details) => {
    const requests: Request[] = [];
    stubFetch(async (request) => {
      requests.push(request);
      return jsonResponse(apiReceipt(), 201);
    });

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(
      client.createReport({
        targetType: 'comment',
        targetID: 'comment-id',
        reason: 'harassment',
        details,
      }),
    ).resolves.toEqual(expectedReceipt());

    const request = onlyRequest(requests);
    await expect(request.json()).resolves.toEqual({
      target_type: 'comment',
      target_id: 'comment-id',
      reason: 'harassment',
    });
  });

  it.each([
    ['note target', 'note', 'spam', 200],
    ['comment target', 'comment', 'harmful_or_misleading', 201],
    ['other reason', 'note', 'other', 200],
  ])(
    'maps both 200 and 201 payloads to the same camelCase receipt: %s',
    async (_name, targetType, reason, status) => {
      stubFetch(async () =>
        jsonResponse(apiReceipt({ target_type: targetType, reason }), status),
      );

      const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
      await expect(
        client.createReport({
          targetType: targetType as 'note' | 'comment',
          targetID,
          reason: reason as 'spam' | 'harassment' | 'harmful_or_misleading' | 'other',
        }),
      ).resolves.toEqual(expectedReceipt({ targetType, reason }));
    },
  );

  it('preserves null details and non-null details', async () => {
    stubFetch(async () => jsonResponse(apiReceipt({ details: null }), 201));

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(
      client.createReport({ targetType: 'note', targetID, reason: 'spam' }),
    ).resolves.toEqual(expectedReceipt({ details: null }));

    vi.unstubAllGlobals();
    stubFetch(async () =>
      jsonResponse(apiReceipt({ details: 'explanation' }), 201),
    );

    await expect(
      client.createReport({ targetType: 'note', targetID, reason: 'spam' }),
    ).resolves.toEqual(expectedReceipt({ details: 'explanation' }));
  });

  it.each(malformedReceiptCases)(
    'rejects malformed report receipt: %s',
    async (_name, response) => {
      stubFetch(async () => jsonResponse(response, 201));

      const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
      await expect(
        client.createReport({ targetType: 'note', targetID, reason: 'spam' }),
      ).rejects.toThrow(APIResponseError);
    },
  );

  it('retains structured invalid_report errors and validation fields', async () => {
    stubFetch(async () =>
      jsonResponse(
        {
          code: 'invalid_report',
          fields: [{ field: 'reason', code: 'invalid' }],
        },
        400,
      ),
    );

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    const error = await client
      .createReport({ targetType: 'note', targetID, reason: 'spam' })
      .catch((caught: unknown) => caught);

    expect(error).toBeInstanceOf(APIRequestError);
    expect(error).toMatchObject({
      body: {
        code: 'invalid_report',
        fields: [{ field: 'reason', code: 'invalid' }],
      },
      code: 'invalid_report',
      fields: [{ field: 'reason', code: 'invalid' }],
      status: 400,
    });
  });

  it.each([
    ['unauthenticated', 401, 'unauthenticated'],
    ['not found', 404, 'not_found'],
    ['request too large', 413, 'request_too_large'],
  ])('retains %s request errors through rewrapTransportError', async (_name, status, code) => {
    stubFetch(async () => jsonResponse({ code }, status));

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await expect(
      client.createReport({ targetType: 'note', targetID, reason: 'spam' }),
    ).rejects.toMatchObject({
      body: { code },
      code,
      status,
    });
  });

  it('injects the session token into the authorization header', async () => {
    const requests: Request[] = [];
    stubFetch(async (request) => {
      requests.push(request);
      return jsonResponse(apiReceipt(), 201);
    });

    const client = createAPIClient({ kind: 'authenticated', token: exampleToken });
    await client.createReport({ targetType: 'note', targetID, reason: 'spam' });

    const request = onlyRequest(requests);
    expect(request.headers.get('Authorization')).toBe(`Bearer ${exampleToken}`);
    expect(new URL(request.url).pathname).toBe('/v1/reports');
  });
});

function apiReceipt(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    created_at: 1782993600000,
    details: null,
    id: 'report-id',
    reason: 'spam',
    target_id: targetID,
    target_type: 'note',
    ...overrides,
  };
}

function expectedReceipt(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    createdAt: 1782993600000,
    details: null,
    id: 'report-id',
    reason: 'spam',
    targetID,
    targetType: 'note',
    ...overrides,
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
    throw new Error(`expected exactly one request, got ${requests.length}`);
  }
  return requests[0]!;
}

function stubFetch(handler: FetchHandler): void {
  vi.stubGlobal('fetch', handler);
}
