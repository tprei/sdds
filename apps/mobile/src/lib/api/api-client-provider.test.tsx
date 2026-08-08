import { Component, type ReactNode } from 'react';
import { act, create } from 'react-test-renderer';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AuthProvider, useAuth } from '@/lib/auth/auth-provider';
import type { AuthState } from '@/lib/auth/session';
import {
  exampleAuthorID,
  exampleToken,
  exampleUserID,
  onlyFetchCall,
  stubFetch,
} from '@/test-support/api-fixtures';

import { APIClientProvider, useAPIClient } from './api-client-provider';
import type { APIClient } from './client';

// React only propagates a render error synchronously to the caller (rather
// than deferring it to an uncaught-error handler) when it recognizes the
// environment as act()-aware.
Reflect.set(globalThis, 'IS_REACT_ACT_ENVIRONMENT', true);

vi.mock('react-native', () => ({ Platform: { OS: 'web' } }));
vi.mock('expo-file-system', () => ({ File: class {} }));

const mocks = vi.hoisted(() => ({
  controller: {
    bootstrap: vi.fn<() => Promise<AuthState>>(),
    login: vi.fn(),
    logout: vi.fn(),
    signup: vi.fn(),
  },
}));

vi.mock('@/lib/auth/session', () => ({
  createAuthController: () => mocks.controller,
}));

function jsonResponse(value: unknown): Response {
  return new Response(JSON.stringify(value), {
    headers: { 'Content-Type': 'application/json' },
    status: 200,
  });
}

const authenticatedState: AuthState = {
  status: 'authenticated',
  token: exampleToken,
  user: {
    author: { displayName: 'Thiago', id: exampleAuthorID },
    id: exampleUserID,
    username: 'thiago',
  },
};

describe('APIClientProvider', () => {
  afterEach(() => {
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it('throws outside the provider', () => {
    let caught: unknown = null;

    class CaughtErrorBoundary extends Component<
      { children: ReactNode },
      { hasError: boolean }
    > {
      state = { hasError: false };
      static getDerivedStateFromError(error: Error) {
        caught = error;
        return { hasError: true };
      }
      render() {
        return this.state.hasError ? null : this.props.children;
      }
    }

    function Probe() {
      useAPIClient();
      return null;
    }

    act(() => {
      create(
        <CaughtErrorBoundary>
          <Probe />
        </CaughtErrorBoundary>,
      );
    });

    if (!(caught instanceof Error)) {
      throw new Error('render did not throw');
    }
    expect(caught.message).toBe('api_client_provider_missing');
  });

  it('sends the bearer token while authenticated and drops it after logout', async () => {
    mocks.controller.bootstrap.mockResolvedValue(authenticatedState);
    mocks.controller.logout.mockResolvedValue({ status: 'anonymous' });

    let latestClient: APIClient | undefined;
    let logout: (() => Promise<void>) | undefined;

    function Probe() {
      const { logout: doLogout } = useAuth();
      latestClient = useAPIClient();
      logout = doLogout;
      return null;
    }

    await act(async () => {
      create(
        <AuthProvider>
          <APIClientProvider>
            <Probe />
          </APIClientProvider>
        </AuthProvider>,
      );
      await Promise.resolve();
      await Promise.resolve();
    });

    const calls: { request: Request }[] = [];
    stubFetch(async (request: Request) => {
      calls.push({ request });
      return jsonResponse({ categories: [] });
    });

    await latestClient?.listCategories();
    const authenticatedRequest = onlyFetchCall(calls);
    expect(authenticatedRequest.headers.get('Authorization')).toBe(
      `Bearer ${exampleToken}`,
    );

    await act(async () => {
      await logout?.();
    });

    calls.length = 0;
    await latestClient?.listCategories();
    const anonymousRequest = onlyFetchCall(calls);
    expect(anonymousRequest.headers.get('Authorization')).toBeNull();
  });
});
