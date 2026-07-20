import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  AuthAPIRequestError,
  AuthAPIResponseError,
  createAuthSession,
  createAuthUser,
  deleteAuthSession,
  getAuthSession,
} from './auth';
import type { components } from './generated/schema';

vi.mock('react-native', () => ({
  Platform: {
    OS: 'ios',
  },
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

    await createAuthUser({
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

    await createAuthSession({
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

    const session = await createAuthSession({
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

    const session = await getAuthSession(exampleToken);

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

    await deleteAuthSession(exampleToken);

    const request = onlyFetchCall(calls);
    expect(request.url).toBe('http://localhost:8080/v1/auth/session');
    expect(request.method).toBe('DELETE');
    expect(request.headers.get('authorization')).toBe(`Bearer ${exampleToken}`);
  });

  it('raises request errors from status even when the error body fails', async () => {
    stubFetch(async () => unreadableResponse(httpStatusUnauthorized));

    await expect(getAuthSession(exampleToken)).rejects.toMatchObject(
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

    await expect(
      createAuthUser({
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

    await expect(
      createAuthSession({
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

    await expect(
      createAuthUser({
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

    await expect(
      createAuthSession({
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

    await expect(
      createAuthUser({
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

    await expect(
      createAuthUser({
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

    await expect(getAuthSession(exampleToken)).rejects.toThrow(
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

    await expect(getAuthSession(exampleToken)).resolves.toEqual({
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
