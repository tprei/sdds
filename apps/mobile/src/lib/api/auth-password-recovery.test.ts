import { describe, expect, it, vi } from 'vitest';

import { AuthAPIRequestError } from './auth';
import { createAPIClient } from './client';
import {
  httpStatusAccepted,
  httpStatusBadRequest,
  httpStatusNoContent,
  httpStatusTooManyRequests,
  jsonResponse,
  onlyFetchCall,
  requestJSON,
  stubFetch,
  type FetchCall,
} from '@/test-support/api-fixtures';

vi.mock('react-native', () => ({ Platform: { OS: 'ios' } }));
vi.mock('expo-file-system', () => ({ File: class {} }));

describe('password recovery API client', () => {
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
});
