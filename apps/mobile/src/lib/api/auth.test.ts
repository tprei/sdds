import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  AuthAPIRequestError,
  AuthAPIResponseError,
} from './auth';
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
const exampleUserID = '018ff5b8-0000-7000-8000-000000000001';
const exampleAuthorID = '018ff5b8-0000-7000-8000-000000000002';
const exampleToken = 'session-token';

type AuthSessionResponse = components['schemas']['AuthSessionResponse'];
type CurrentSessionResponse = components['schemas']['CurrentSessionResponse'];
type CurrentUserResponse = components['schemas']['CurrentUser'];
type FetchCall = {
  request: Request;
};
type FetchHandler = (request: Request) => Promise<Response>;

describe('auth API client', () => {
  beforeEach(() => {
    delete process.env[configuredAPIBaseURLEnvName];
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('sends create user requests with API wire keys', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return jsonResponse(apiAuthSession(), httpStatusCreated);
    });

    const client = createAPIClient();
    await client.createAuthUser({
      displayName: 'Thiago',
      password: 'senha-secreta',
      username: 'thiago',
    });

    const request = onlyFetchCall(calls);
    expect(request.url).toBe('http://localhost:8080/v1/auth/users');
    expect(request.method).toBe('POST');
    expect(request.headers.get('content-type')).toBe('application/json');
    await expect(requestJSON(request)).resolves.toEqual({
      display_name: 'Thiago',
      password: 'senha-secreta',
      username: 'thiago',
    });
  });

  it('sends create session requests with API wire keys', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return jsonResponse(apiAuthSession(), httpStatusCreated);
    });

    const client = createAPIClient();
    await client.createAuthSession({
      password: 'senha-secreta',
      username: 'thiago',
    });

    const request = onlyFetchCall(calls);
    expect(request.url).toBe('http://localhost:8080/v1/auth/sessions');
    expect(request.method).toBe('POST');
    expect(request.headers.get('content-type')).toBe('application/json');
    await expect(requestJSON(request)).resolves.toEqual({
      password: 'senha-secreta',
      username: 'thiago',
    });
  });

  it('parses created auth sessions from the API wire shape', async () => {
    stubFetch(async () => jsonResponse(apiAuthSession(), httpStatusCreated));

    const client = createAPIClient();
    const session = await client.createAuthSession({
      password: 'senha-secreta',
      username: 'thiago',
    });

    expect(session).toEqual({
      expiresAt: 1782993600000,
      token: exampleToken,
      user: {
        author: {
          displayName: 'Thiago',
          id: exampleAuthorID,
        },
        id: exampleUserID,
        username: 'thiago',
      },
    });
  });

  it('gets the current session with a bearer token', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return jsonResponse(apiCurrentSession());
    });

    const client = createAPIClient(exampleToken);
    const session = await client.getAuthSession();

    const request = onlyFetchCall(calls);
    expect(request.url).toBe('http://localhost:8080/v1/auth/session');
    expect(request.method).toBe('GET');
    expect(request.headers.get('authorization')).toBe(`Bearer ${exampleToken}`);
    expect(session).toEqual({
      expiresAt: 1782993600000,
      user: {
        author: {
          displayName: 'Thiago',
          id: exampleAuthorID,
        },
        id: exampleUserID,
        username: 'thiago',
      },
    });
  });

  it('deletes the current session with a bearer token', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return new Response(null, { status: httpStatusNoContent });
    });

    const client = createAPIClient(exampleToken);
    await client.deleteAuthSession();

    const request = onlyFetchCall(calls);
    expect(request.url).toBe('http://localhost:8080/v1/auth/session');
    expect(request.method).toBe('DELETE');
    expect(request.headers.get('authorization')).toBe(`Bearer ${exampleToken}`);
  });

  it('deletes the current user account with a password body', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return new Response(null, { status: httpStatusNoContent });
    });

    const client = createAPIClient(exampleToken);
    await client.deleteAuthUser('senha-secreta');

    const request = onlyFetchCall(calls);
    expect(request.url).toBe('http://localhost:8080/v1/auth/users/me');
    expect(request.method).toBe('DELETE');
    expect(request.headers.get('authorization')).toBe(`Bearer ${exampleToken}`);
    expect(request.headers.get('content-type')).toBe('application/json');
    await expect(requestJSON(request)).resolves.toEqual({ password: 'senha-secreta' });
  });

  it('surfaces a wrong-password account deletion as a forbidden request error', async () => {
    stubFetch(async () =>
      jsonResponse({ code: 'forbidden' }, httpStatusForbidden),
    );

    const client = createAPIClient(exampleToken);
    await expect(client.deleteAuthUser('wrong-password')).rejects.toMatchObject({
      status: httpStatusForbidden,
      code: 'forbidden',
    });
  });

  it('raises request errors from status even when the error body fails', async () => {
    stubFetch(async () => unreadableResponse(httpStatusUnauthorized));

    const client = createAPIClient(exampleToken);
    await expect(client.getAuthSession()).rejects.toMatchObject(
      new AuthAPIRequestError(httpStatusUnauthorized),
    );
  });

  it('preserves structured auth validation error bodies', async () => {
    stubFetch(async () =>
      jsonResponse(
        {
          code: 'invalid_auth',
          fields: [
            { code: 'too_short', field: 'password' },
            { code: 'required', field: 'display_name' },
          ],
        },
        httpStatusBadRequest,
      ),
    );

    const client = createAPIClient();
    await expect(
      client.createAuthUser({
        displayName: '',
        password: 'short',
        username: 'thiago',
      }),
    ).rejects.toMatchObject(
      new AuthAPIRequestError(httpStatusBadRequest, {
        code: 'invalid_auth',
        fields: [
          { code: 'too_short', field: 'password' },
          { code: 'required', field: 'display_name' },
        ],
      }),
    );
  });

  it('ignores extra auth error response fields', async () => {
    stubFetch(async () =>
      jsonResponse(
        {
          code: 'invalid_auth',
          fields: [{ code: 'too_short', field: 'password', request_id: 'abc' }],
          request_id: 'abc',
        },
        httpStatusBadRequest,
      ),
    );

    const client = createAPIClient();
    await expect(
      client.createAuthSession({
        password: 'short',
        username: 'thiago',
      }),
    ).rejects.toMatchObject(
      new AuthAPIRequestError(httpStatusBadRequest, {
        code: 'invalid_auth',
        fields: [{ code: 'too_short', field: 'password' }],
      }),
    );
  });

  it('preserves username-taken error bodies', async () => {
    stubFetch(async () =>
      jsonResponse(
        {
          code: 'username_taken',
          fields: [{ code: 'taken', field: 'username' }],
        },
        httpStatusConflict,
      ),
    );

    const client = createAPIClient();
    await expect(
      client.createAuthUser({
        displayName: 'Thiago',
        password: 'secret-password',
        username: 'thiago',
      }),
    ).rejects.toMatchObject(
      new AuthAPIRequestError(httpStatusConflict, {
        code: 'username_taken',
        fields: [{ code: 'taken', field: 'username' }],
      }),
    );
  });

  it('preserves rate-limit error bodies', async () => {
    const response = jsonResponse(
      { code: 'rate_limited' },
      httpStatusTooManyRequests,
    );
    response.headers.set('Retry-After', '4');
    stubFetch(async () => response);

    const client = createAPIClient();
    await expect(
      client.createAuthSession({
        password: 'secret-password',
        username: 'thiago',
      }),
    ).rejects.toMatchObject(
      new AuthAPIRequestError(
        httpStatusTooManyRequests,
        { code: 'rate_limited' },
        4,
      ),
    );
  });
  it('keeps malformed error responses as status-only request errors', async () => {
    stubFetch(async () =>
      jsonResponse(
        {
          code: 'invalid_auth',
          fields: [{ field: 'password' }],
        },
        httpStatusBadRequest,
      ),
    );

    const client = createAPIClient();
    await expect(
      client.createAuthUser({
        displayName: 'Thiago',
        password: 'short',
        username: 'thiago',
      }),
    ).rejects.toMatchObject(new AuthAPIRequestError(httpStatusBadRequest));
  });
  it('rejects malformed auth session responses', async () => {
    stubFetch(async () =>
      jsonResponse(
        {
          ...apiAuthSession(),
          expires_at: -1,
        },
        httpStatusCreated,
      ),
    );

    const client = createAPIClient();
    await expect(
      client.createAuthUser({
        displayName: 'Thiago',
        password: 'senha-secreta',
        username: 'thiago',
      }),
    ).rejects.toThrow(AuthAPIResponseError);
  });

  it('rejects malformed current session responses', async () => {
    stubFetch(async () =>
      jsonResponse({
        expires_at: 1782993600000,
        user: {
          ...apiCurrentUser(),
          author: {
            id: exampleAuthorID,
            name: 'Thiago',
          },
        },
      }),
    );

    const client = createAPIClient(exampleToken);
    await expect(client.getAuthSession()).rejects.toThrow(
      AuthAPIResponseError,
    );
  });

  it('ignores extra current session response fields', async () => {
    stubFetch(async () =>
      jsonResponse({
        ...apiCurrentSession(),
        token: exampleToken,
      }),
    );

    const client = createAPIClient(exampleToken);
    await expect(client.getAuthSession()).resolves.toEqual({
      expiresAt: 1782993600000,
      user: {
        author: {
          displayName: 'Thiago',
          id: exampleAuthorID,
        },
        id: exampleUserID,
        username: 'thiago',
      },
    });
  });
});

const httpStatusCreated = 201;
const httpStatusBadRequest = 400;
const httpStatusConflict = 409;
const httpStatusForbidden = 403;
const httpStatusNoContent = 204;
const httpStatusTooManyRequests = 429;
const httpStatusUnauthorized = 401;

function apiAuthSession(
  overrides: Partial<AuthSessionResponse> = {},
): AuthSessionResponse {
  return {
    expires_at: 1782993600000,
    token: exampleToken,
    user: apiCurrentUser(),
    ...overrides,
  };
}

function apiCurrentSession(
  overrides: Partial<CurrentSessionResponse> = {},
): CurrentSessionResponse {
  return {
    expires_at: 1782993600000,
    user: apiCurrentUser(),
    ...overrides,
  };
}

function apiCurrentUser(
  overrides: Partial<CurrentUserResponse> = {},
): CurrentUserResponse {
  return {
    author: {
      display_name: 'Thiago',
      id: exampleAuthorID,
    },
    id: exampleUserID,
    username: 'thiago',
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
