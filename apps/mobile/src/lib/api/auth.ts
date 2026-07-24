
import {
  APIRequestError as SharedAPIRequestError,
} from './request-error';
import {
  authSessionResponseSchema,
  currentSessionResponseSchema,
} from './schema';
import type { APIErrorResponse, APIValidationProblem } from './request-error';
import type { components } from './generated/schema';
import type { TypedTransport } from './client';
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
export type AuthAPIErrorField = APIValidationProblem;

export class AuthAPIRequestError extends SharedAPIRequestError {
  constructor(
    status: number,
    body: APIErrorResponse | null = null,
    retryAfter?: number,
  ) {
    super(status, body, retryAfter);
    this.message = 'auth_api_request_failed';
  }
}

export class AuthAPIResponseError extends Error {
  constructor() {
    super('auth_api_response_invalid');
  }
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


export type AuthAPI = {
  createAuthUser(input: CreateAuthUserInput): Promise<AuthSession>;
  createAuthSession(input: CreateAuthSessionInput): Promise<AuthSession>;
  getAuthSession(): Promise<CurrentAuthSession>;
  deleteAuthSession(): Promise<void>;
};

function rewrapAuthTransportError(error: unknown): never {
  if (error instanceof SharedAPIRequestError) {
    throw new AuthAPIRequestError(error.status, error.body, error.retryAfter);
  }
  throw error;
}

export function bindAuthAPI(transport: TypedTransport): AuthAPI {
  return {
    async createAuthUser(input) {
      const request: CreateUserRequest = {
        display_name: input.displayName,
        password: input.password,
        username: input.username,
      };
      try {
        const { data } = await transport.POST('/v1/auth/users', { body: request });
        return parseAuthSessionResponse(data);
      } catch (error) {
        rewrapAuthTransportError(error);
      }
    },

    async createAuthSession(input) {
      const request: CreateSessionRequest = {
        password: input.password,
        username: input.username,
      };
      try {
        const { data } = await transport.POST('/v1/auth/sessions', { body: request });
        return parseAuthSessionResponse(data);
      } catch (error) {
        rewrapAuthTransportError(error);
      }
    },

    async getAuthSession() {
      try {
        const { data } = await transport.GET('/v1/auth/session');
        return parseCurrentSessionResponse(data);
      } catch (error) {
        rewrapAuthTransportError(error);
      }
    },

    async deleteAuthSession() {
      try {
        await transport.DELETE('/v1/auth/session');
      } catch (error) {
        rewrapAuthTransportError(error);
      }
    },
  };
}
