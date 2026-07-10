import createClient from 'openapi-fetch';

import { apiBaseURL } from './config';
import type { components, paths } from './generated/schema';

export type AuthAuthor = {
  displayName: string;
  id: string;
};

export type AuthUser = {
  author: AuthAuthor;
  id: string;
  username: string;
};

export type AuthSession = {
  expiresAt: number;
  token: string;
  user: AuthUser;
};

export type CurrentAuthSession = {
  expiresAt: number;
  user: AuthUser;
};

export type CreateAuthUserInput = {
  displayName: string;
  password: string;
  username: string;
};

export type CreateAuthSessionInput = {
  password: string;
  username: string;
};

type GeneratedSchemas = components['schemas'];
type AuthSessionResponse = GeneratedSchemas['AuthSessionResponse'];
type AuthorSummaryResponse = GeneratedSchemas['AuthorSummary'];
type CreateSessionRequest = GeneratedSchemas['CreateSessionRequest'];
type CreateUserRequest = GeneratedSchemas['CreateUserRequest'];
type CurrentSessionResponse = GeneratedSchemas['CurrentSessionResponse'];
type CurrentUserResponse = GeneratedSchemas['CurrentUser'];
type SchemaKey<T> = Extract<keyof T, string>;
type SchemaKeyList<T> = readonly SchemaKey<T>[];
type ExhaustiveSchemaKeyList<T, K extends SchemaKeyList<T>> =
  Exclude<SchemaKey<T>, K[number]> extends never ? K : never;

const authSessionResponseKeys = schemaKeyList<AuthSessionResponse>()([
  'expires_at',
  'token',
  'user',
]);
const authorSummaryResponseKeys = schemaKeyList<AuthorSummaryResponse>()([
  'display_name',
  'id',
]);
const currentSessionResponseKeys = schemaKeyList<CurrentSessionResponse>()([
  'expires_at',
  'user',
]);
const currentUserResponseKeys = schemaKeyList<CurrentUserResponse>()([
  'author',
  'id',
  'username',
]);

export class AuthAPIRequestError extends Error {
  readonly status: number;

  constructor(status: number) {
    super('auth_api_request_failed');
    this.status = status;
  }
}

export class AuthAPIResponseError extends Error {
  constructor() {
    super('auth_api_response_invalid');
  }
}

export async function createAuthUser(
  input: CreateAuthUserInput,
): Promise<AuthSession> {
  const request: CreateUserRequest = {
    display_name: input.displayName,
    password: input.password,
    username: input.username,
  };

  const { data, response } = await apiClient().POST('/v1/auth/users', {
    body: request,
  });
  if (!response.ok) {
    throw new AuthAPIRequestError(response.status);
  }

  return parseAuthSessionResponse(data);
}

export async function createAuthSession(
  input: CreateAuthSessionInput,
): Promise<AuthSession> {
  const request: CreateSessionRequest = {
    password: input.password,
    username: input.username,
  };

  const { data, response } = await apiClient().POST('/v1/auth/sessions', {
    body: request,
  });
  if (!response.ok) {
    throw new AuthAPIRequestError(response.status);
  }

  return parseAuthSessionResponse(data);
}

export async function getAuthSession(
  token: string,
): Promise<CurrentAuthSession> {
  const { data, response } = await apiClient(token).GET('/v1/auth/session');
  if (!response.ok) {
    throw new AuthAPIRequestError(response.status);
  }

  return parseCurrentSessionResponse(data);
}

export async function deleteAuthSession(token: string): Promise<void> {
  const { response } = await apiClient(token).DELETE('/v1/auth/session');
  if (!response.ok) {
    throw new AuthAPIRequestError(response.status);
  }
}

function apiClient(token?: string) {
  return createClient<paths>({
    baseUrl: apiBaseURL(),
    fetch: (request) => apiFetch(request, token),
  });
}

async function apiFetch(request: Request, token?: string): Promise<Response> {
  const response = await fetch(authenticatedRequest(request, token));
  if (response.ok) {
    return response;
  }

  const headers = new Headers(response.headers);
  headers.delete('content-length');
  headers.delete('transfer-encoding');
  return new Response(null, {
    headers,
    status: response.status,
    statusText: response.statusText,
  });
}

function authenticatedRequest(request: Request, token?: string): Request {
  if (token === undefined) {
    return request;
  }

  const headers = new Headers(request.headers);
  headers.set('Authorization', `Bearer ${token}`);
  return new Request(request, { headers });
}

function parseAuthSessionResponse(value: unknown): AuthSession {
  if (!isAuthSessionResponse(value)) {
    throw new AuthAPIResponseError();
  }

  return {
    expiresAt: value.expires_at,
    token: value.token,
    user: parseCurrentUser(value.user),
  };
}

function parseCurrentSessionResponse(value: unknown): CurrentAuthSession {
  if (!isCurrentSessionResponse(value)) {
    throw new AuthAPIResponseError();
  }

  return {
    expiresAt: value.expires_at,
    user: parseCurrentUser(value.user),
  };
}

function parseCurrentUser(value: CurrentUserResponse): AuthUser {
  return {
    author: parseAuthorSummary(value.author),
    id: value.id,
    username: value.username,
  };
}

function parseAuthorSummary(value: AuthorSummaryResponse): AuthAuthor {
  return {
    displayName: value.display_name,
    id: value.id,
  };
}

function isAuthSessionResponse(value: unknown): value is AuthSessionResponse {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, authSessionResponseKeys) &&
    typeof value.token === 'string' &&
    isUnixMillisecondTimestamp(value.expires_at) &&
    isCurrentUserResponse(value.user)
  );
}

function isCurrentSessionResponse(
  value: unknown,
): value is CurrentSessionResponse {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, currentSessionResponseKeys) &&
    isUnixMillisecondTimestamp(value.expires_at) &&
    isCurrentUserResponse(value.user)
  );
}

function isCurrentUserResponse(value: unknown): value is CurrentUserResponse {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, currentUserResponseKeys) &&
    typeof value.id === 'string' &&
    typeof value.username === 'string' &&
    isAuthorSummaryResponse(value.author)
  );
}

function isAuthorSummaryResponse(
  value: unknown,
): value is AuthorSummaryResponse {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, authorSummaryResponseKeys) &&
    typeof value.id === 'string' &&
    typeof value.display_name === 'string'
  );
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function hasOnlyKeys(
  value: Record<string, unknown>,
  expectedKeys: readonly string[],
): boolean {
  const keys = Object.keys(value);
  return (
    keys.length === expectedKeys.length &&
    expectedKeys.every((key) =>
      Object.prototype.hasOwnProperty.call(value, key),
    )
  );
}

function schemaKeyList<T>() {
  return <const K extends SchemaKeyList<T>>(
    keys: ExhaustiveSchemaKeyList<T, K>,
  ) => keys;
}

function isUnixMillisecondTimestamp(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value >= 0;
}
