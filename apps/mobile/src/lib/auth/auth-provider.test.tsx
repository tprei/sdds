import { act, create } from 'react-test-renderer';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AuthProvider, useAuth } from './auth-provider';
import type { AuthState } from './session';

const mocks = vi.hoisted(() => ({
  controller: {
    bootstrap: vi.fn<() => Promise<AuthState>>(),
    login: vi.fn(),
    logout: vi.fn(),
    refresh: vi.fn(),
    signInWithOIDC: vi.fn(),
    signup: vi.fn(),
  },
}));

vi.mock('./session', () => ({
  createAuthController: () => mocks.controller,
}));

describe('AuthProvider', () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it('leaves loading after a failed login while bootstrap is pending', async () => {
    const bootstrap = deferred<AuthState>();
    mocks.controller.bootstrap.mockReturnValue(bootstrap.promise);
    mocks.controller.login.mockRejectedValue(new Error('invalid credentials'));

    let latestState: AuthState | undefined;
    let login: (() => Promise<void>) | undefined;

    function Probe() {
      const auth = useAuth();
      latestState = auth.state;
      login = () => auth.login({ password: 'senha-secreta', username: 'thiago' });
      return null;
    }

    await act(async () => {
      create(
        <AuthProvider>
          <Probe />
        </AuthProvider>,
      );
    });

    expect(latestState).toEqual({ status: 'loading' });

    await act(async () => {
      await expect(login?.()).rejects.toThrow('invalid credentials');
    });

    bootstrap.resolve({ status: 'anonymous' });
    await act(async () => {
      await bootstrap.promise;
    });

    expect(latestState).toEqual({ status: 'anonymous' });
  });

  it('exposes OIDC sign-in through the mutation queue', async () => {
    mocks.controller.bootstrap.mockResolvedValue({ status: 'anonymous' });
    mocks.controller.signInWithOIDC.mockResolvedValue({ status: 'anonymous' });

    let signInWithOIDC: (() => Promise<void>) | undefined;
    function Probe() {
      const auth = useAuth();
      signInWithOIDC = () =>
        auth.signInWithOIDC({
          idToken: 'provider-id-token',
          nonce: 'nonce-value',
          provider: 'google',
        });
      return null;
    }

    await act(async () => {
      create(
        <AuthProvider>
          <Probe />
        </AuthProvider>,
      );
    });

    await act(async () => {
      await signInWithOIDC?.();
    });

    expect(mocks.controller.signInWithOIDC).toHaveBeenCalledWith({
      idToken: 'provider-id-token',
      nonce: 'nonce-value',
      provider: 'google',
    });
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
