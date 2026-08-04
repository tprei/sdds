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
  it('sends set auth email requests with API wire keys', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return new Response(null, { status: httpStatusAccepted });
    });

    const client = createAPIClient(exampleToken);
    const result = await client.setAuthEmail('voce@email.com');

    const request = onlyFetchCall(calls);
    expect(request.url).toBe('http://localhost:8080/v1/auth/email');
    expect(request.method).toBe('PUT');
    expect(request.headers.get('authorization')).toBe(`Bearer ${exampleToken}`);
    await expect(requestJSON(request)).resolves.toEqual({ email: 'voce@email.com' });
    expect(result).toBeUndefined();
  });

  it('raises auth request errors from a failed set auth email', async () => {
    stubFetch(async () =>
      jsonResponse(
        { code: 'invalid_auth', fields: [{ code: 'invalid', field: 'email' }] },
        httpStatusBadRequest,
      ),
    );

    const client = createAPIClient(exampleToken);
    await expect(client.setAuthEmail('voce')).rejects.toMatchObject(
      new AuthAPIRequestError(httpStatusBadRequest, {
        code: 'invalid_auth',
        fields: [{ code: 'invalid', field: 'email' }],
      }),
    );
  });

  it('resends the email verification with a bearer token and no body', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return new Response(null, { status: httpStatusAccepted });
    });

    const client = createAPIClient(exampleToken);
    const result = await client.createAuthEmailVerification();

    const request = onlyFetchCall(calls);
    expect(request.url).toBe('http://localhost:8080/v1/auth/email/verifications');
    expect(request.method).toBe('POST');
    expect(request.headers.get('authorization')).toBe(`Bearer ${exampleToken}`);
    expect(result).toBeUndefined();
  });

  it('raises auth request errors from a failed resend verification', async () => {
    stubFetch(async () => jsonResponse({ code: 'unauthenticated' }, httpStatusUnauthorized));

    const client = createAPIClient();
    await expect(client.createAuthEmailVerification()).rejects.toMatchObject(
      new AuthAPIRequestError(httpStatusUnauthorized, { code: 'unauthenticated' }),
    );
  });

  it('verifies an email token with API wire keys', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return new Response(null, { status: httpStatusNoContent });
    });

    const client = createAPIClient();
    const result = await client.verifyAuthEmail('verify-token');

    const request = onlyFetchCall(calls);
    expect(request.url).toBe('http://localhost:8080/v1/auth/email/verification');
    expect(request.method).toBe('POST');
    await expect(requestJSON(request)).resolves.toEqual({ token: 'verify-token' });
    expect(result).toBeUndefined();
  });

  it('raises auth request errors from an invalid verify email token', async () => {
    stubFetch(async () => jsonResponse({ code: 'invalid_token' }, httpStatusBadRequest));

    const client = createAPIClient();
    await expect(client.verifyAuthEmail('expired')).rejects.toMatchObject(
      new AuthAPIRequestError(httpStatusBadRequest, { code: 'invalid_token' }),
    );
  });

  it('sends password reset requests with API wire keys', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return new Response(null, { status: httpStatusAccepted });
    });

    const client = createAPIClient();
    const result = await client.createAuthPasswordReset('voce@email.com');

    const request = onlyFetchCall(calls);
    expect(request.url).toBe('http://localhost:8080/v1/auth/password-resets');
    expect(request.method).toBe('POST');
    await expect(requestJSON(request)).resolves.toEqual({ email: 'voce@email.com' });
    expect(result).toBeUndefined();
  });

  it('raises auth request errors from a failed password reset request', async () => {
    stubFetch(async () => jsonResponse({ code: 'rate_limited' }, httpStatusTooManyRequests));

    const client = createAPIClient();
    await expect(client.createAuthPasswordReset('voce@email.com')).rejects.toMatchObject(
      new AuthAPIRequestError(httpStatusTooManyRequests, { code: 'rate_limited' }),
    );
  });

  it('sets a new password with API wire keys', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return new Response(null, { status: httpStatusNoContent });
    });

    const client = createAPIClient();
    const result = await client.setAuthPassword('reset-token', 'nova-senha');

    const request = onlyFetchCall(calls);
    expect(request.url).toBe('http://localhost:8080/v1/auth/password');
    expect(request.method).toBe('POST');
    await expect(requestJSON(request)).resolves.toEqual({
      password: 'nova-senha',
      token: 'reset-token',
    });
    expect(result).toBeUndefined();
  });

  it('raises auth request errors from an invalid set password token', async () => {
    stubFetch(async () => jsonResponse({ code: 'invalid_token' }, httpStatusBadRequest));

    const client = createAPIClient();
    await expect(client.setAuthPassword('expired', 'nova-senha')).rejects.toMatchObject(
      new AuthAPIRequestError(httpStatusBadRequest, { code: 'invalid_token' }),
    );
  });

  it('maps the contact email onto the current session user', async () => {
    stubFetch(async () =>
      jsonResponse(
        apiCurrentSession({
          user: apiCurrentUser({ email: { address: 'voce@email.com', verified: false } }),
        }),
      ),
    );

    const client = createAPIClient(exampleToken);
    const session = await client.getAuthSession();

    expect(session.user.email).toEqual({ address: 'voce@email.com', verified: false });
  });
});

const httpStatusCreated = 201;
const httpStatusBadRequest = 400;
const httpStatusConflict = 409;
const httpStatusNoContent = 204;
const httpStatusAccepted = 202;
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
