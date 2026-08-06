import type { AuthSession, AuthUser } from '@/lib/api/auth';
import { AuthAPIRequestError } from '@/lib/api/auth';
import { createAPIClient } from '@/lib/api/client';

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
  email?: string;
  password: string;
  username: string;
};

export type AuthController = {
  bootstrap(): Promise<AuthState>;
  login(input: LoginInput): Promise<AuthState>;
  logout(state: AuthState): Promise<AuthState>;
  refresh(state: AuthState): Promise<AuthState>;
  deleteAccount(state: AuthState, password: string): Promise<AuthState>;
  signup(input: SignupInput): Promise<AuthState>;
};

export function createAuthController(): AuthController {
  let mutationQueue: Promise<void> = Promise.resolve();

  async function runAuthMutation(
    operation: () => Promise<AuthState>,
  ): Promise<AuthState> {
    const result = mutationQueue.then(operation, operation);
    mutationQueue = result.then(
      () => undefined,
      () => undefined,
    );
    return result;
  }

  return {
    async bootstrap() {
      return runAuthMutation(async () => {
        const token = await readStoredSessionToken();
        if (token === undefined) {
          return { status: 'error' };
        }

        return bootstrapToken(token);
      });
    },
    async login(input) {
      return runAuthMutation(async () => {
        const session = await createAPIClient().createAuthSession(input);
        return persistSession(session);
      });
    },
    async logout(state) {
      return runAuthMutation(async () => {
        if (state.status === 'authenticated') {
          try {
            await createAPIClient(state.token).deleteAuthSession();
          } catch (error: unknown) {
            if (!isUnauthenticatedRequest(error)) {
              await clearSessionToken();
              return { status: 'anonymous' };
            }
          }
        }
        await clearSessionToken();
        return { status: 'anonymous' };
      });
    },
    async refresh(state) {
      return runAuthMutation(async () => {
        if (state.status !== 'authenticated') {
          return state;
        }
        return bootstrapToken(state.token);
      });
    },
    async signup(input) {
      return runAuthMutation(async () => {
        const normalizedEmail = input.email?.trim();
        const session = await createAPIClient().createAuthUser({
          displayName: input.displayName,
          password: input.password,
          username: input.username,
          ...(normalizedEmail ? { email: normalizedEmail } : {}),
        });
        return persistSession(session);
      });
    },
    async deleteAccount(state, password) {
      return runAuthMutation(async () => {
        if (state.status !== 'authenticated') {
          return state;
        }
        await createAPIClient(state.token).deleteAuthUser(password);
        await clearSessionToken();
        return { status: 'anonymous' };
      });
    },
  };
}

const unauthenticatedStatus = 401;

async function bootstrapToken(token: string | null): Promise<AuthState> {
  if (token === null) {
    return { status: 'anonymous' };
  }

  try {
    const session = await createAPIClient(token).getAuthSession();
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
