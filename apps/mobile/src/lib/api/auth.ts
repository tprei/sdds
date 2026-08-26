
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

export type LoginIdentity = {
  id: string;
  kind: 'oidc' | 'password';
  provider: 'apple' | 'google' | 'local';
};

export type AuthUser = {
  author: AuthAuthor;
  email?: { address: string; verified: boolean };
  id: string;
  identities: LoginIdentity[];
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
  email?: string;
};

export type CreateAuthSessionInput = {
  password: string;
  username: string;
};

export type CreateOidcSessionInput = {
  idToken: string;
  nonce: string;
  provider: OidcProvider;
  username?: string;
};

type GeneratedSchemas = components['schemas'];
type AuthorSummaryResponse = GeneratedSchemas['AuthorSummary'];
type CreateSessionRequest = GeneratedSchemas['CreateSessionRequest'];
type CreateUserRequest = GeneratedSchemas['CreateUserRequest'];
type CreateOidcSessionRequest = GeneratedSchemas['CreateOidcSessionRequest'];
type OidcProvider = GeneratedSchemas['OidcProvider'];
type DeleteUserRequest = GeneratedSchemas['DeleteUserRequest'];
type CurrentUserResponse = GeneratedSchemas['CurrentUser'];
type SetUserEmailRequest = GeneratedSchemas['SetUserEmailRequest'];
type VerifyEmailRequest = GeneratedSchemas['VerifyEmailRequest'];
type CreatePasswordResetRequest = GeneratedSchemas['CreatePasswordResetRequest'];
type SetPasswordRequest = GeneratedSchemas['SetPasswordRequest'];
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
    email: value.email,
    id: value.id,
    identities: value.identities.map((identity) => ({
      id: identity.id,
      kind: identity.kind,
      provider: identity.provider,
    })),
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
  createAuthOidcSession(input: CreateOidcSessionInput): Promise<AuthSession>;
  getAuthSession(): Promise<CurrentAuthSession>;
  deleteAuthSession(): Promise<void>;
  deleteAuthUser(password: string): Promise<void>;
  setAuthEmail(email: string): Promise<void>;
  createAuthEmailVerification(): Promise<void>;
  verifyAuthEmail(token: string): Promise<void>;
  createAuthPasswordReset(email: string): Promise<void>;
  setAuthPassword(token: string, password: string): Promise<void>;
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
        ...(input.email ? { email: input.email } : {}),
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

    async createAuthOidcSession(input) {
      const request: CreateOidcSessionRequest = {
        id_token: input.idToken,
        nonce: input.nonce,
        provider: input.provider,
        ...(input.username !== undefined ? { username: input.username } : {}),
      };
      try {
        const { data } = await transport.POST('/v1/auth/oidc/sessions', { body: request });
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
    async deleteAuthUser(password) {
      const request: DeleteUserRequest = { password };
      try {
        await transport.DELETE('/v1/auth/users/me', { body: request });
      } catch (error) {
        rewrapAuthTransportError(error);
      }
    },
    async setAuthEmail(email) {
      const request: SetUserEmailRequest = { email };
      try {
        await transport.PUT('/v1/auth/email', { body: request });
      } catch (error) {
        rewrapAuthTransportError(error);
      }
    },

    async createAuthEmailVerification() {
      try {
        await transport.POST('/v1/auth/email/verifications');
      } catch (error) {
        rewrapAuthTransportError(error);
      }
    },

    async verifyAuthEmail(token) {
      const request: VerifyEmailRequest = { token };
      try {
        await transport.POST('/v1/auth/email/verification', { body: request });
      } catch (error) {
        rewrapAuthTransportError(error);
      }
    },

    async createAuthPasswordReset(email) {
      const request: CreatePasswordResetRequest = { email };
      try {
        await transport.POST('/v1/auth/password-resets', { body: request });
      } catch (error) {
        rewrapAuthTransportError(error);
      }
    },

    async setAuthPassword(token, password) {
      const request: SetPasswordRequest = { password, token };
      try {
        await transport.POST('/v1/auth/password', { body: request });
      } catch (error) {
        rewrapAuthTransportError(error);
      }
    },
  };
}
