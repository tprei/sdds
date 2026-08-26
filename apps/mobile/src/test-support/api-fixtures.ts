import { vi } from 'vitest';

import type { components } from '../lib/api/generated/schema';

type AuthSessionResponse = components['schemas']['AuthSessionResponse'];
type CurrentSessionResponse = components['schemas']['CurrentSessionResponse'];
type CurrentUserResponse = components['schemas']['CurrentUser'];

export type FetchCall = {
  request: Request;
};
export type FetchHandler = (request: Request) => Promise<Response>;

export const exampleUserID = '018ff5b8-0000-7000-8000-000000000001';
export const exampleAuthorID = '018ff5b8-0000-7000-8000-000000000002';
export const exampleToken = 'session-token';

export const httpStatusCreated = 201;
export const httpStatusBadRequest = 400;
export const httpStatusConflict = 409;
export const httpStatusNoContent = 204;
export const httpStatusAccepted = 202;
export const httpStatusTooManyRequests = 429;
export const httpStatusUnauthorized = 401;

// 2026-07-02T12:00:00Z — the fixed session-expiry instant used across tests.
export const sessionExpiresAtMs = Date.UTC(2026, 6, 2, 12, 0, 0, 0);

export function apiAuthSession(
  overrides: Partial<AuthSessionResponse> = {},
): AuthSessionResponse {
  return {
    expires_at: sessionExpiresAtMs,
    token: exampleToken,
    user: apiCurrentUser(),
    ...overrides,
  };
}

export function apiCurrentSession(
  overrides: Partial<CurrentSessionResponse> = {},
): CurrentSessionResponse {
  return {
    expires_at: sessionExpiresAtMs,
    user: apiCurrentUser(),
    ...overrides,
  };
}

export function apiCurrentUser(
  overrides: Partial<CurrentUserResponse> = {},
): CurrentUserResponse {
  return {
    author: {
      display_name: 'Thiago',
      id: exampleAuthorID,
    },
    id: exampleUserID,
    identities: [
      { id: 'identity-1', kind: 'password', provider: 'local' },
    ],
    username: 'thiago',
    ...overrides,
  };
}

export function jsonResponse(value: unknown, status = 200): Response {
  return new Response(JSON.stringify(value), {
    headers: {
      'Content-Type': 'application/json',
    },
    status,
  });
}

export function onlyFetchCall(calls: FetchCall[]): Request {
  if (calls.length !== 1) {
    throw new Error(`fetch call count = ${calls.length}, want 1`);
  }

  const call = calls[0];
  if (call === undefined) {
    throw new Error('fetch call missing');
  }

  return call.request;
}

export async function requestJSON(request: Request): Promise<unknown> {
  return request.clone().json();
}

export function stubFetch(handler: FetchHandler): void {
  vi.stubGlobal('fetch', handler);
}
