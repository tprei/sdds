import createClient from 'openapi-fetch';

import { apiBaseURL } from './config';
import {
  authSessionResponseSchema,
  currentSessionResponseSchema,
  errorResponseSchema,
} from './schema';
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
type AuthorSummaryResponse = GeneratedSchemas['AuthorSummary'];
type CreateSessionRequest = GeneratedSchemas['CreateSessionRequest'];
type CreateUserRequest = GeneratedSchemas['CreateUserRequest'];
type CurrentUserResponse = GeneratedSchemas['CurrentUser'];
type ErrorCode = GeneratedSchemas['ErrorCode'];
type ErrorResponse = GeneratedSchemas['ErrorResponse'];
type ValidationProblemResponse = GeneratedSchemas['ValidationProblem'];

export type AuthAPIErrorCode = ErrorCode;
export type AuthAPIErrorBody = ErrorResponse;
export type AuthAPIErrorField = ValidationProblemResponse;

export class AuthAPIRequestError extends Error {
  readonly code: AuthAPIErrorCode | undefined;

  readonly fields: readonly AuthAPIErrorField[] | undefined;

  readonly status: number;

  constructor(status: number, errorResponse: ErrorResponse | null = null) {
    super('auth_api_request_failed');
    this.code = errorResponse?.code;
    this.fields = errorResponse?.fields?.map((field) => ({
      code: field.code,
      field: field.field,
    }));
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

  const { data, error, response } = await apiClient().POST('/v1/auth/users', {
    body: request,
  });
  if (!response.ok) {
    throw new AuthAPIRequestError(response.status, parseErrorResponse(error));
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

  const { data, error, response } = await apiClient().POST(
    '/v1/auth/sessions',
    {
      body: request,
    },
  );
  if (!response.ok) {
    throw new AuthAPIRequestError(response.status, parseErrorResponse(error));
  }

  return parseAuthSessionResponse(data);
}

export async function getAuthSession(
  token: string,
): Promise<CurrentAuthSession> {
  const { data, error, response } =
    await apiClient(token).GET('/v1/auth/session');
  if (!response.ok) {
    throw new AuthAPIRequestError(response.status, parseErrorResponse(error));
  }

  return parseCurrentSessionResponse(data);
}

export async function deleteAuthSession(token: string): Promise<void> {
  const { error, response } = await apiClient(token).DELETE('/v1/auth/session');
  if (!response.ok) {
    throw new AuthAPIRequestError(response.status, parseErrorResponse(error));
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

  let body: string | null;
  try {
    body = await response.text();
  } catch (error: unknown) {
    if (!(error instanceof Error)) {
      throw error;
    }
    body = null;
  }

  const headers = new Headers(response.headers);
  headers.delete('content-length');
  headers.delete('transfer-encoding');
  return new Response(body, {
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
  const authSessionResponse = authSessionResponseSchema.safeParse(value);
  if (!authSessionResponse.success) {
    throw new AuthAPIResponseError();
  }

  return {
    expiresAt: authSessionResponse.data.expires_at,
    token: authSessionResponse.data.token,
    user: parseCurrentUser(authSessionResponse.data.user),
  };
}

function parseCurrentSessionResponse(value: unknown): CurrentAuthSession {
  const currentSessionResponse = currentSessionResponseSchema.safeParse(value);
  if (!currentSessionResponse.success) {
    throw new AuthAPIResponseError();
  }

  return {
    expiresAt: currentSessionResponse.data.expires_at,
    user: parseCurrentUser(currentSessionResponse.data.user),
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

function parseErrorResponse(value: unknown): ErrorResponse | null {
  const errorResponse = errorResponseSchema.safeParse(value);
  if (!errorResponse.success) {
    return null;
  }

  return errorResponse.data;
}
