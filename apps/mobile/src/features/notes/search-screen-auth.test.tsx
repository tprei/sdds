import * as React from 'react';
import { act, create } from 'react-test-renderer';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import SearchScreen from '@/app/search';

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
  authState: { status: 'loading' } as AuthStateMock,
  listCatalogs: vi.fn(),
  logout: vi.fn(async () => undefined),
  push: vi.fn(),
}));

vi.mock('react-native', () => {
  function NativeView({ children, ...props }: NativeProps) {
    const content = typeof children === 'function' ? null : children;
    return createElement('div', props, content);
  }
  function NativePressable({ children, ...props }: NativeProps) {
    const content = typeof children === 'function' ? children({ pressed: false }) : children;
    return createElement('button', props, content);
  }
  return {
    Pressable: NativePressable,
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
    Text: NativeView,
    View: NativeView,
  };
});

vi.mock('@/components/foundation-screen', () => ({
  EmptyStateCard: ({ title }: { title: string }) => createElement('div', { title }),
  FoundationButton: ({ label, onPress }: { label: string; onPress?: () => void }) =>
    createElement('button', { onPress }, label),
  FoundationScreen: ({ children }: { children: ReactNode }) => createElement('section', null, children),
  FoundationTextInput: () => createElement('input'),
}));
vi.mock('@/components/note-card', () => ({ NoteCard: () => createElement('div') }));
vi.mock('@/features/notes/category-filter-controls', () => ({ CategoryFilterControls: () => createElement('div') }));
vi.mock('@/features/notes/catalog', () => ({ buildNoteCatalog: () => ({ kind: 'catalog' }), labelNotes: (_catalog: unknown, notes: unknown[]) => notes }));
vi.mock('@/features/notes/search-screen', () => ({
  appendRecentSearchQuery: (current: string[], query: string) => [...current, query],
  createSearchRequest: () => null,
  isCurrentSearchRequest: () => true,
  searchResultContext: () => ({ categoryLabel: null, query: 'q', resultCount: 0, scopeLabel: 'Mundo todo' }),
  searchResultCountLabel: () => '0 notas',
}));
vi.mock('expo-router', async () => {
  const react = (await vi.importActual('react')) as typeof React;
  return {
    useFocusEffect(effect: () => void | (() => void)) {
      react.useEffect(effect, [effect]);
    },
    useRouter: () => ({ push: mocks.push }),
  };
});
vi.mock('@/lib/auth/auth-provider', () => ({ useAuth: () => ({ logout: mocks.logout, state: mocks.authState }) }));
vi.mock('@/lib/api/catalogs', () => ({ listCatalogs: mocks.listCatalogs }));
vi.mock('@/lib/api/notes', () => ({ searchNotes: vi.fn() }));

async function settle(): Promise<void> {
  await Promise.resolve();
  await Promise.resolve();
}

describe('SearchScreen auth gate', () => {
  beforeEach(() => {
    mocks.authState = { status: 'authenticated', token: 'session-token', user: { id: 'user-id' } };
    mocks.listCatalogs.mockResolvedValue({ categories: [], places: [] });
    mocks.logout.mockClear();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('does not start product reads before authentication', async () => {
    mocks.authState = { status: 'anonymous' };

    await act(async () => {
      create(createElement(SearchScreen));
      await settle();
    });

    expect(mocks.listCatalogs).not.toHaveBeenCalled();
  });

  it('passes the bearer token to catalog reads', async () => {
    await act(async () => {
      create(createElement(SearchScreen));
      await settle();
    });

    expect(mocks.listCatalogs).toHaveBeenCalledWith('session-token');
  });

  it('logs out on a catalog 401', async () => {
    mocks.listCatalogs.mockRejectedValueOnce({ status: 401 });

    await act(async () => {
      create(createElement(SearchScreen));
      await settle();
    });

    expect(mocks.logout).toHaveBeenCalledOnce();
  });
});
