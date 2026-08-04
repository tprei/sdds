import { describe, expect, it, vi } from 'vitest';

import { AuthAPIRequestError } from './auth';
import { createAPIClient } from './client';
import {
  apiCurrentUser,
  apiCurrentSession,
  exampleToken,
  httpStatusAccepted,
  httpStatusBadRequest,
  httpStatusNoContent,
  httpStatusUnauthorized,
  jsonResponse,
  onlyFetchCall,
  requestJSON,
  stubFetch,
  type FetchCall,
} from '@/test-support/api-fixtures';

vi.mock('react-native', () => ({ Platform: { OS: 'ios' } }));
vi.mock('expo-file-system', () => ({ File: class {} }));

describe('contact email API client', () => {
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
