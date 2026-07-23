import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { AuthSession, AuthUser, CurrentAuthSession } from '@/lib/api/auth';
import { AuthAPIRequestError } from '@/lib/api/auth';

import { createAuthController } from './session';
import {
  clearSessionToken,
  readSessionToken,
  saveSessionToken,
} from './session-storage';

const mocks = vi.hoisted(() => {
  const mockClient = {
    createAuthSession: vi.fn(),
    createAuthUser: vi.fn(),
    deleteAuthSession: vi.fn(),
    getAuthSession: vi.fn(),
  };
  return {
    createAPIClient: vi.fn(() => mockClient),
    mockClient,
  };
});

vi.mock('@/lib/api/auth', () => ({
  AuthAPIRequestError: class AuthAPIRequestError extends Error {
    readonly status: number;

    constructor(status: number) {
      super('auth_api_request_failed');
      this.status = status;
    }
  },
}));

vi.mock('@/lib/api/client', () => ({
  createAPIClient: mocks.createAPIClient,
}));

vi.mock('./session-storage', () => ({
  clearSessionToken: vi.fn(),
  readSessionToken: vi.fn(),
  saveSessionToken: vi.fn(),
}));

const { createAPIClient, mockClient } = mocks;

describe('auth session controller', () => {
  beforeEach(() => {
    createAPIClient.mockClear();
    mockClient.createAuthSession.mockReset();
    mockClient.createAuthUser.mockReset();
    mockClient.deleteAuthSession.mockReset();
    mockClient.getAuthSession.mockReset();
    vi.mocked(clearSessionToken).mockReset();
    vi.mocked(readSessionToken).mockReset();
    vi.mocked(saveSessionToken).mockReset();
  });

  it('boots anonymous when no token is stored', async () => {
    vi.mocked(readSessionToken).mockResolvedValue(null);

    await expect(createAuthController().bootstrap()).resolves.toEqual({
      status: 'anonymous',
    });
    expect(mockClient.getAuthSession).not.toHaveBeenCalled();
  });

  it('boots authenticated when the stored token is valid', async () => {
    vi.mocked(readSessionToken).mockResolvedValue('stored-token');
    mockClient.getAuthSession.mockResolvedValue(apiCurrentSession());

    await expect(createAuthController().bootstrap()).resolves.toEqual({
      status: 'authenticated',
      token: 'stored-token',
      user: apiUser(),
    });
    expect(createAPIClient).toHaveBeenCalledWith('stored-token');
    expect(clearSessionToken).not.toHaveBeenCalled();
  });

  it('clears invalid stored tokens during boot', async () => {
    vi.mocked(readSessionToken).mockResolvedValue('expired-token');
    mockClient.getAuthSession.mockRejectedValue(new AuthAPIRequestError(401));
    vi.mocked(clearSessionToken).mockResolvedValue(undefined);

    await expect(createAuthController().bootstrap()).resolves.toEqual({
      status: 'anonymous',
    });
    expect(clearSessionToken).toHaveBeenCalledOnce();
  });

  it('boots error when the stored token cannot be read', async () => {
    vi.mocked(readSessionToken).mockRejectedValue(new Error('storage failed'));

    await expect(createAuthController().bootstrap()).resolves.toEqual({
      status: 'error',
    });
    expect(mockClient.getAuthSession).not.toHaveBeenCalled();
    expect(clearSessionToken).not.toHaveBeenCalled();
  });

  it('boots error when an invalid stored token cannot be cleared', async () => {
    vi.mocked(readSessionToken).mockResolvedValue('expired-token');
    mockClient.getAuthSession.mockRejectedValue(new AuthAPIRequestError(401));
    vi.mocked(clearSessionToken).mockRejectedValue(new Error('storage failed'));

    await expect(createAuthController().bootstrap()).resolves.toEqual({
      status: 'error',
    });
    expect(clearSessionToken).toHaveBeenCalledOnce();
  });

  it('boots the replacement token when storage changes during boot', async () => {
    vi.mocked(readSessionToken)
      .mockResolvedValueOnce('expired-token')
      .mockResolvedValueOnce('fresh-token');
    mockClient.getAuthSession
      .mockRejectedValueOnce(new AuthAPIRequestError(401))
      .mockResolvedValueOnce(apiCurrentSession());

    await expect(createAuthController().bootstrap()).resolves.toEqual({
      status: 'authenticated',
      token: 'fresh-token',
      user: apiUser(),
    });
    expect(clearSessionToken).not.toHaveBeenCalled();
    expect(createAPIClient).toHaveBeenNthCalledWith(1, 'expired-token');
    expect(createAPIClient).toHaveBeenNthCalledWith(2, 'fresh-token');
  });

  it('does not clear storage when the token is removed during boot', async () => {
    vi.mocked(readSessionToken)
      .mockResolvedValueOnce('expired-token')
      .mockResolvedValueOnce(null);
    mockClient.getAuthSession.mockRejectedValue(new AuthAPIRequestError(401));

    await expect(createAuthController().bootstrap()).resolves.toEqual({
      status: 'anonymous',
    });
    expect(clearSessionToken).not.toHaveBeenCalled();
    expect(mockClient.getAuthSession).toHaveBeenCalledOnce();
  });

  it('keeps storage intact when boot fails without an auth rejection', async () => {
    vi.mocked(readSessionToken).mockResolvedValue('stored-token');
    mockClient.getAuthSession.mockRejectedValue(new AuthAPIRequestError(500));

    await expect(createAuthController().bootstrap()).resolves.toEqual({
      status: 'error',
    });
    expect(clearSessionToken).not.toHaveBeenCalled();
  });

  it('serializes login storage writes after bootstrap clears an expired token', async () => {
    const bootSession = deferred<CurrentAuthSession>();
    vi.mocked(readSessionToken)
      .mockResolvedValueOnce('expired-token')
      .mockResolvedValueOnce('expired-token');
    mockClient.getAuthSession.mockReturnValueOnce(bootSession.promise);
    vi.mocked(clearSessionToken).mockResolvedValue(undefined);
    mockClient.createAuthSession.mockResolvedValue(
      apiAuthSession({ token: 'fresh-token' }),
    );
    vi.mocked(saveSessionToken).mockResolvedValue(undefined);

    const controller = createAuthController();
    const bootstrap = controller.bootstrap();
    const login = controller.login({
      password: 'senha-secreta',
      username: 'thiago',
    });

    await flushQueuedMutation();
    expect(mockClient.createAuthSession).not.toHaveBeenCalled();

    bootSession.reject(new AuthAPIRequestError(401));

    await expect(bootstrap).resolves.toEqual({ status: 'anonymous' });
    await expect(login).resolves.toEqual({
      status: 'authenticated',
      token: 'fresh-token',
      user: apiUser(),
    });
    expect(clearSessionToken).toHaveBeenCalledBefore(
      vi.mocked(saveSessionToken),
    );
    expect(saveSessionToken).toHaveBeenCalledWith('fresh-token');
  });

  it('logs in and persists the returned token', async () => {
    mockClient.createAuthSession.mockResolvedValue(apiAuthSession());
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
    expect(mockClient.createAuthSession).toHaveBeenCalledWith({
      password: 'senha-secreta',
      username: 'thiago',
    });
    expect(saveSessionToken).toHaveBeenCalledWith('session-token');
  });

  it('serializes overlapping logins before persisting tokens', async () => {
    const firstSession = deferred<AuthSession>();
    const secondSession = deferred<AuthSession>();
    mockClient.createAuthSession
      .mockReturnValueOnce(firstSession.promise)
      .mockReturnValueOnce(secondSession.promise);
    vi.mocked(saveSessionToken).mockResolvedValue(undefined);

    const controller = createAuthController();
    const firstLogin = controller.login({
      password: 'senha-secreta',
      username: 'ana',
    });
    const secondLogin = controller.login({
      password: 'senha-secreta',
      username: 'bia',
    });

    await flushQueuedMutation();

    expect(mockClient.createAuthSession).toHaveBeenCalledOnce();
    firstSession.resolve(apiAuthSession({ token: 'ana-token' }));

    await expect(firstLogin).resolves.toEqual({
      status: 'authenticated',
      token: 'ana-token',
      user: apiUser(),
    });
    await flushQueuedMutation();

    expect(mockClient.createAuthSession).toHaveBeenCalledTimes(2);
    expect(mockClient.createAuthSession).toHaveBeenNthCalledWith(2, {
      password: 'senha-secreta',
      username: 'bia',
    });
    expect(saveSessionToken).toHaveBeenNthCalledWith(1, 'ana-token');

    secondSession.resolve(apiAuthSession({ token: 'bia-token' }));

    await expect(secondLogin).resolves.toEqual({
      status: 'authenticated',
      token: 'bia-token',
      user: apiUser(),
    });
    expect(saveSessionToken).toHaveBeenNthCalledWith(2, 'bia-token');
  });

  it('signs up and persists the returned token', async () => {
    mockClient.createAuthUser.mockResolvedValue(apiAuthSession());
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
    expect(mockClient.createAuthUser).toHaveBeenCalledWith({
      displayName: 'Thiago',
      password: 'senha-secreta',
      username: 'thiago',
    });
    expect(saveSessionToken).toHaveBeenCalledWith('session-token');
  });

  it('logs out by revoking and clearing an authenticated token', async () => {
    mockClient.deleteAuthSession.mockResolvedValue(undefined);
    vi.mocked(clearSessionToken).mockResolvedValue(undefined);

    await expect(
      createAuthController().logout({
        status: 'authenticated',
        token: 'session-token',
        user: apiUser(),
      }),
    ).resolves.toEqual({ status: 'anonymous' });
    expect(createAPIClient).toHaveBeenCalledWith('session-token');
    expect(clearSessionToken).toHaveBeenCalledOnce();
  });

  it('clears local state when logout finds an already invalid token', async () => {
    mockClient.deleteAuthSession.mockRejectedValue(new AuthAPIRequestError(401));
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

  it('logs out locally when server revocation fails after clearing the token', async () => {
    mockClient.deleteAuthSession.mockRejectedValue(new AuthAPIRequestError(500));
    vi.mocked(clearSessionToken).mockResolvedValue(undefined);

    await expect(
      createAuthController().logout({
        status: 'authenticated',
        token: 'session-token',
        user: apiUser(),
      }),
    ).resolves.toEqual({ status: 'anonymous' });
    expect(createAPIClient).toHaveBeenCalledWith('session-token');
    expect(clearSessionToken).toHaveBeenCalledOnce();
  });

  it('keeps logout rejected when a failed revocation cannot clear the token', async () => {
    const clearError = new Error('storage failed');
    mockClient.deleteAuthSession.mockRejectedValue(new AuthAPIRequestError(500));
    vi.mocked(clearSessionToken).mockRejectedValue(clearError);

    await expect(
      createAuthController().logout({
        status: 'authenticated',
        token: 'session-token',
        user: apiUser(),
      }),
    ).rejects.toBe(clearError);
    expect(createAPIClient).toHaveBeenCalledWith('session-token');
    expect(clearSessionToken).toHaveBeenCalledOnce();
  });

  it('keeps logout rejected when a revoked token cannot be cleared', async () => {
    const clearError = new Error('storage failed');
    mockClient.deleteAuthSession.mockResolvedValue(undefined);
    vi.mocked(clearSessionToken).mockRejectedValue(clearError);

    await expect(
      createAuthController().logout({
        status: 'authenticated',
        token: 'session-token',
        user: apiUser(),
      }),
    ).rejects.toBe(clearError);
    expect(createAPIClient).toHaveBeenCalledWith('session-token');
    expect(clearSessionToken).toHaveBeenCalledOnce();
  });
});

type Deferred<T> = {
  promise: Promise<T>;
  reject(error: unknown): void;
  resolve(value: T): void;
};

function deferred<T>(): Deferred<T> {
  let rejectDeferred: (error: unknown) => void = () => undefined;
  let resolveDeferred: (value: T) => void = () => undefined;
  const promise = new Promise<T>((resolve, reject) => {
    rejectDeferred = reject;
    resolveDeferred = resolve;
  });
  return {
    promise,
    reject: rejectDeferred,
    resolve: resolveDeferred,
  };
}

async function flushQueuedMutation(): Promise<void> {
  await Promise.resolve();
  await Promise.resolve();
}

function apiAuthSession(options: { token?: string } = {}): AuthSession {
  return {
    expiresAt: 1782993600000,
    token: options.token ?? 'session-token',
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
