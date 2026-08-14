import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import SearchScreen from '@/app/(tabs)/search';
import { assertLoadingFirstCommit } from '@/test-support/assert-loading-first-commit';
import { Button } from '@/ui/button';

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
    listCatalogs: vi.fn(),
    searchNotes: vi.fn(),
    markNoteUseful: vi.fn(),
    unmarkNoteUseful: vi.fn(),
  },
  authState: { status: 'loading' } as AuthStateMock,
  focusEffect: null as (() => void | (() => void)) | null,
  logout: vi.fn(async () => undefined),
  productEvents: { record: vi.fn() },
  push: vi.fn(),
}));
vi.mock('expo-crypto', () => ({
  randomUUID: () => '018ff5b8-0000-7000-8000-000000000001',
}));
vi.mock('@/lib/events/product-event-provider', () => ({
  useProductEvents: () => mocks.productEvents,
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
  function NativeTextInput(props: NativeProps) {
    return createElement('input', props);
  }
  class AnimatedValue {
    value: number;
    constructor(value: number) {
      this.value = value;
    }
  }
  return {
    Pressable: NativePressable,
    ScrollView: NativeView,
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
    Text: NativeView,
    TextInput: NativeTextInput,
    View: NativeView,
    useWindowDimensions: () => ({ width: 390, height: 844, scale: 1, fontScale: 1 }),
    Animated: {
      View: NativeView,
      Value: AnimatedValue,
      createAnimatedComponent: <T,>(component: T): T => component,
      timing: () => ({ start: () => {} }),
    },
    AccessibilityInfo: {
      isReduceMotionEnabled: () => Promise.resolve(false),
      addEventListener: () => ({ remove: () => {} }),
    },
  };
});

vi.mock('react-native-safe-area-context', () => {
  function SafeAreaView({ children, ...props }: { children?: ReactNode; [key: string]: unknown }) {
    return createElement('div', props, children);
  }
  return { SafeAreaView };
});

vi.mock('react-native-svg', () => {
  function Node({ children, ...props }: { children?: ReactNode; [key: string]: unknown }) {
    return createElement('div', props, children);
  }
  return { Svg: Node, Path: Node, Circle: Node, Rect: Node };
});
vi.mock('@/ui/haptics', () => ({
  lightTick: () => {},
  success: () => {},
}));

vi.mock('@/components/note-card', () => ({
  NOTE_USEFUL_ERROR_MESSAGE: 'Não deu pra atualizar o Útil. Tenta de novo.',
  NoteCard: () => createElement('div'),
}));
vi.mock('@/features/notes/category-filter-controls', () => ({ CategoryFilterControls: () => createElement('div') }));
vi.mock('@/features/notes/catalog', () => ({ buildNoteCatalog: () => ({ kind: 'catalog', activeCategories: [] }) }));
vi.mock('@/features/notes/search-screen', () => ({
  appendRecentSearchQuery: (current: string[], query: string) => [...current, query],
  createSearchRequest: () => null,
  isCurrentSearchRequest: () => true,
  labelSearchResults: (_catalog: unknown, results: unknown[]) => results,
  searchResultContext: () => ({ categoryLabel: null, query: 'q', resultCount: 0 }),
  searchResultCountLabel: () => '0 achados',
  selectedSearchCategory: () => null,
}));
vi.mock('expo-router', async () => {
  const react = (await vi.importActual('react')) as typeof React;
  return {
    useFocusEffect(effect: () => void | (() => void)) {
      mocks.focusEffect = effect;
      react.useEffect(effect, [effect]);
    },
    useRouter: () => ({ push: mocks.push }),
  };
});
vi.mock('@/lib/api/api-client-provider', () => ({ useAPIClient: () => mocks.apiClient }));
vi.mock('@/lib/auth/auth-provider', () => ({ useAuth: () => ({ logout: mocks.logout, state: mocks.authState }) }));

async function settle(): Promise<void> {
  await Promise.resolve();
  await Promise.resolve();
}

describe('SearchScreen auth gate', () => {
  beforeEach(() => {
    mocks.authState = { status: 'authenticated', token: 'session-token', user: { id: 'user-id' } };
    mocks.apiClient.listCatalogs.mockResolvedValue({ categories: [] });
    mocks.logout.mockClear();
    mocks.productEvents.record.mockClear();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('renders the search screen for an anonymous visitor', async () => {
    mocks.authState = { status: 'anonymous' };

    await act(async () => {
      create(createElement(SearchScreen));
      await settle();
    });

    // Anonymous visitors load the catalog rather than hitting an auth wall.
    expect(mocks.apiClient.listCatalogs).toHaveBeenCalledWith();
  });

  it('passes the bearer token to catalog reads', async () => {
    await act(async () => {
      create(createElement(SearchScreen));
      await settle();
    });

    expect(mocks.apiClient.listCatalogs).toHaveBeenCalledWith();
  });

  it('logs out on a catalog 401', async () => {
    mocks.apiClient.listCatalogs.mockRejectedValueOnce({ status: 401 });

    await act(async () => {
      create(createElement(SearchScreen));
      await settle();
    });

    expect(mocks.logout).toHaveBeenCalledOnce();
  });

  it('keeps the catalog ready on a second focus, refreshing without a loading flash', async () => {
    let renderer!: ReactTestRenderer;
    await act(async () => {
      renderer = create(createElement(SearchScreen));
      await settle();
    });

    expect(mocks.apiClient.listCatalogs).toHaveBeenCalledTimes(1);
    expect(renderer.root.findByType(Button).props.disabled).toBe(false);
    expect(mocks.focusEffect).not.toBeNull();

    assertLoadingFirstCommit(
      () => {
        mocks.focusEffect?.();
        return renderer;
      },
      [],
      (r) => {
        expect(r.root.findByType(Button).props.disabled).toBe(false);
      },
    );

    expect(mocks.apiClient.listCatalogs).toHaveBeenCalledTimes(2);
  });
});
