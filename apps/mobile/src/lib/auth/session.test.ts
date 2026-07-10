import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { AuthSession, AuthUser, CurrentAuthSession } from '@/lib/api/auth';
import {
  AuthAPIRequestError,
  createAuthSession,
  createAuthUser,
  deleteAuthSession,
  getAuthSession,
} from '@/lib/api/auth';

import { createAuthController } from './session';
import {
  clearSessionToken,
  readSessionToken,
  saveSessionToken,
} from './session-storage';

vi.mock('@/lib/api/auth', () => ({
  AuthAPIRequestError: class AuthAPIRequestError extends Error {
    readonly status: number;

    constructor(status: number) {
      super('auth_api_request_failed');
      this.status = status;
    }
  },
  createAuthSession: vi.fn(),
  createAuthUser: vi.fn(),
  deleteAuthSession: vi.fn(),
  getAuthSession: vi.fn(),
}));

vi.mock('./session-storage', () => ({
  clearSessionToken: vi.fn(),
  readSessionToken: vi.fn(),
  saveSessionToken: vi.fn(),
}));

describe('auth session controller', () => {
  beforeEach(() => {
    vi.mocked(clearSessionToken).mockReset();
    vi.mocked(createAuthSession).mockReset();
    vi.mocked(createAuthUser).mockReset();
    vi.mocked(deleteAuthSession).mockReset();
    vi.mocked(getAuthSession).mockReset();
    vi.mocked(readSessionToken).mockReset();
    vi.mocked(saveSessionToken).mockReset();
  });

  it('boots anonymous when no token is stored', async () => {
    vi.mocked(readSessionToken).mockResolvedValue(null);

    await expect(createAuthController().bootstrap()).resolves.toEqual({
      status: 'anonymous',
    });
    expect(getAuthSession).not.toHaveBeenCalled();
  });

  it('boots authenticated when the stored token is valid', async () => {
    vi.mocked(readSessionToken).mockResolvedValue('stored-token');
    vi.mocked(getAuthSession).mockResolvedValue(apiCurrentSession());

    await expect(createAuthController().bootstrap()).resolves.toEqual({
      status: 'authenticated',
      token: 'stored-token',
      user: apiUser(),
    });
    expect(getAuthSession).toHaveBeenCalledWith('stored-token');
    expect(clearSessionToken).not.toHaveBeenCalled();
  });

  it('clears invalid stored tokens during boot', async () => {
    vi.mocked(readSessionToken).mockResolvedValue('expired-token');
    vi.mocked(getAuthSession).mockRejectedValue(new AuthAPIRequestError(401));
    vi.mocked(clearSessionToken).mockResolvedValue(undefined);

    await expect(createAuthController().bootstrap()).resolves.toEqual({
      status: 'anonymous',
    });
    expect(clearSessionToken).toHaveBeenCalledOnce();
  });

  it('keeps storage intact when boot fails without an auth rejection', async () => {
    vi.mocked(readSessionToken).mockResolvedValue('stored-token');
    vi.mocked(getAuthSession).mockRejectedValue(new AuthAPIRequestError(500));

    await expect(createAuthController().bootstrap()).resolves.toEqual({
      status: 'error',
    });
    expect(clearSessionToken).not.toHaveBeenCalled();
  });

  it('logs in and persists the returned token', async () => {
    vi.mocked(createAuthSession).mockResolvedValue(apiAuthSession());
    vi.mocked(saveSessionToken).mockResolvedValue(undefined);

    await expect(
      createAuthController().login({
        password: 'senha-secreta',
        username: 'thiago',
      }),
    ).resolves.toEqual({
      status: 'authenticated',
      token: 'session-token',
      user: apiUser(),
    });
    expect(createAuthSession).toHaveBeenCalledWith({
      password: 'senha-secreta',
      username: 'thiago',
    });
    expect(saveSessionToken).toHaveBeenCalledWith('session-token');
  });

  it('signs up and persists the returned token', async () => {
    vi.mocked(createAuthUser).mockResolvedValue(apiAuthSession());
    vi.mocked(saveSessionToken).mockResolvedValue(undefined);

    await expect(
      createAuthController().signup({
        displayName: 'Thiago',
        password: 'senha-secreta',
        username: 'thiago',
      }),
    ).resolves.toEqual({
      status: 'authenticated',
      token: 'session-token',
      user: apiUser(),
    });
    expect(createAuthUser).toHaveBeenCalledWith({
      displayName: 'Thiago',
      password: 'senha-secreta',
      username: 'thiago',
    });
    expect(saveSessionToken).toHaveBeenCalledWith('session-token');
  });

  it('logs out by revoking and clearing an authenticated token', async () => {
    vi.mocked(deleteAuthSession).mockResolvedValue(undefined);
    vi.mocked(clearSessionToken).mockResolvedValue(undefined);

    await expect(
      createAuthController().logout({
        status: 'authenticated',
        token: 'session-token',
        user: apiUser(),
      }),
    ).resolves.toEqual({ status: 'anonymous' });
    expect(deleteAuthSession).toHaveBeenCalledWith('session-token');
    expect(clearSessionToken).toHaveBeenCalledOnce();
  });

  it('clears local state when logout finds an already invalid token', async () => {
    vi.mocked(deleteAuthSession).mockRejectedValue(new AuthAPIRequestError(401));
    vi.mocked(clearSessionToken).mockResolvedValue(undefined);

    await expect(
      createAuthController().logout({
        status: 'authenticated',
        token: 'session-token',
        user: apiUser(),
      }),
    ).resolves.toEqual({ status: 'anonymous' });
    expect(clearSessionToken).toHaveBeenCalledOnce();
  });
});

function apiAuthSession(): AuthSession {
  return {
    expiresAt: 1782993600000,
    token: 'session-token',
    user: apiUser(),
  };
}

function apiCurrentSession(): CurrentAuthSession {
  return {
    expiresAt: 1782993600000,
    user: apiUser(),
  };
}

function apiUser(): AuthUser {
  return {
    author: {
      displayName: 'Thiago',
      id: 'author-id',
    },
    id: 'private-user-id',
    username: 'thiago',
  };
}
