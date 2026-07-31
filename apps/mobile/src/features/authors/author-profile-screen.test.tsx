import * as React from 'react';
import { act, create } from 'react-test-renderer';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import AuthorProfileScreen from '@/app/authors/[id]';

const { createElement } = React;
type ReactNode = React.ReactNode;
type NativeProps = {
  children?: ReactNode | ((state: { pressed: boolean }) => ReactNode);
  [key: string]: unknown;
};

type AuthStateMock =
  | { status: 'loading' }
  | { status: 'anonymous' }
  | { status: 'error' }
  | { status: 'authenticated'; token: string; user: { id: string } };

const mocks = vi.hoisted(() => ({
  apiClient: {
    getPublicAuthor: vi.fn(),
    listAuthorNotes: vi.fn(),
    listCatalogs: vi.fn(),
    markNoteUseful: vi.fn(),
    unmarkNoteUseful: vi.fn(),
  },
  authState: { status: 'loading' } as AuthStateMock,
  authorProfileContent: vi.fn(),
  back: vi.fn(),
  localParams: { id: 'author-id' },
  logout: vi.fn(async () => undefined),
  push: vi.fn(),
}));

vi.mock('react-native', () => {
  function NativeView({ children, ...props }: NativeProps) {
    const content = typeof children === 'function' ? null : children;
    return createElement('div', props, content);
  }
  return {
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
    Text: NativeView,
    View: NativeView,
  };
});
vi.mock('../../components/foundation-screen', () => ({
  EmptyStateCard: ({ title }: { title: string }) => createElement('div', { title }),
  FoundationButton: ({ label, onPress }: { label: string; onPress?: () => void }) => createElement('button', { onPress }, label),
  FoundationScreen: ({ children }: { children: ReactNode }) => createElement('section', null, children),
}));
vi.mock('../../features/authors/author-profile-content', () => ({
  AuthorProfileContent: (props: unknown) => {
    mocks.authorProfileContent(props);
    return createElement('div', { testID: 'author-profile-content' });
  },
}));
vi.mock('expo-router', () => ({
  useLocalSearchParams: () => mocks.localParams,
  useRouter: () => ({ back: mocks.back, push: mocks.push }),
}));
vi.mock('@/lib/auth/auth-provider', () => ({
  useAuth: () => ({ apiClient: mocks.apiClient, logout: mocks.logout, state: mocks.authState }),
}));
vi.mock('react-native-safe-area-context', () => ({
  SafeAreaView: ({ children }: { children: ReactNode }) => children,
}));
vi.mock('@/ui/icon-button', () => ({
  IconButton: ({
    icon,
    accessibilityLabel,
  }: {
    icon: ReactNode;
    accessibilityLabel: string;
  }) => createElement('button', { accessibilityLabel }, icon),
}));
vi.mock('@/ui/icons', () => ({
  IconChevronLeft: () => createElement('svg', null),
}));

async function settle(): Promise<void> {
  await Promise.resolve();
  await Promise.resolve();
}

describe('AuthorProfileScreen auth gate', () => {
  beforeEach(() => {
    mocks.authState = {
      status: 'authenticated',
      token: 'session-token',
      user: { id: 'user-id' },
    };
    mocks.localParams = { id: 'author-id' };
    mocks.authorProfileContent.mockClear();
    mocks.logout.mockClear();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('does not render product content before authentication', async () => {
    mocks.authState = { status: 'anonymous' };

    await act(async () => {
      create(createElement(AuthorProfileScreen));
      await settle();
    });

    expect(mocks.authorProfileContent).not.toHaveBeenCalled();
  });

  it('passes token and session-expiry handler to AuthorProfileContent', async () => {
    await act(async () => {
      create(createElement(AuthorProfileScreen));
      await settle();
    });

    expect(mocks.authorProfileContent).toHaveBeenCalledWith(
      expect.objectContaining({
        apiClient: mocks.apiClient,
        authorID: 'author-id',
        onSessionExpired: mocks.logout,
      }),
    );
  });
});
