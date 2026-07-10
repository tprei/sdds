import type { AuthSession, AuthUser } from '@/lib/api/auth';
import {
  AuthAPIRequestError,
  createAuthSession,
  createAuthUser,
  deleteAuthSession,
  getAuthSession,
} from '@/lib/api/auth';

import {
  clearSessionToken,
  readSessionToken,
  saveSessionToken,
} from './session-storage';

export type AuthState =
  | { status: 'loading' }
  | { status: 'anonymous' }
  | { status: 'error' }
  | { status: 'authenticated'; token: string; user: AuthUser };

export type LoginInput = {
  password: string;
  username: string;
};

export type SignupInput = {
  displayName: string;
  password: string;
  username: string;
};

export type AuthController = {
  bootstrap(): Promise<AuthState>;
  login(input: LoginInput): Promise<AuthState>;
  logout(state: AuthState): Promise<AuthState>;
  signup(input: SignupInput): Promise<AuthState>;
};

export function createAuthController(): AuthController {
  return {
    async bootstrap() {
      const token = await readStoredSessionToken();
      if (token === undefined) {
        return { status: 'error' };
      }

      return bootstrapToken(token);
    },
    async login(input) {
      const session = await createAuthSession(input);
      return persistSession(session);
    },
    async logout(state) {
      if (state.status === 'authenticated') {
        try {
          await deleteAuthSession(state.token);
        } catch (error: unknown) {
          if (!isUnauthenticatedRequest(error)) {
            await clearSessionToken();
            throw error;
          }
        }
      }
      await clearSessionToken();
      return { status: 'anonymous' };
    },
    async signup(input) {
      const session = await createAuthUser(input);
      return persistSession(session);
    },
  };
}

const unauthenticatedStatus = 401;

async function bootstrapToken(token: string | null): Promise<AuthState> {
  if (token === null) {
    return { status: 'anonymous' };
  }

  try {
    const session = await getAuthSession(token);
    return {
      status: 'authenticated',
      token,
      user: session.user,
    };
  } catch (error: unknown) {
    if (!isUnauthenticatedRequest(error)) {
      return { status: 'error' };
    }

    const currentToken = await readStoredSessionToken();
    if (currentToken === undefined) {
      return { status: 'error' };
    }
    if (currentToken !== token) {
      return bootstrapToken(currentToken);
    }

    try {
      await clearSessionToken();
      return { status: 'anonymous' };
    } catch {
      return { status: 'error' };
    }
  }
}

async function readStoredSessionToken(): Promise<string | null | undefined> {
  try {
    return await readSessionToken();
  } catch {
    return undefined;
  }
}

function isUnauthenticatedRequest(error: unknown): boolean {
  return (
    error instanceof AuthAPIRequestError && error.status === unauthenticatedStatus
  );
}

async function persistSession(session: AuthSession): Promise<AuthState> {
  await saveSessionToken(session.token);
  return {
    status: 'authenticated',
    token: session.token,
    user: session.user,
  };
}
