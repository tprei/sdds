import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import SearchScreen from '@/app/(tabs)/search';
import { markNoteDeleted } from '@/features/notes/deleted-notes';

const { createElement } = React;
type ReactNode = React.ReactNode;
type NativeProps = {
  children?: ReactNode | ((state: { pressed: boolean }) => ReactNode);
  [key: string]: unknown;
};

type AuthStateMock =
  | { status: 'authenticated'; user: { id: string } }
  | { status: 'anonymous' };

const fixedNow = Date.UTC(2026, 6, 1, 12, 0, 0);

const mocks = vi.hoisted(() => ({
  apiClient: {
    listCatalogs: vi.fn(),
    searchNotes: vi.fn(),
  },
  authState: { status: 'authenticated', user: { id: 'user-id' } } as AuthStateMock,
  events: { record: vi.fn(), ready: true },
  focusVersion: 0,
  logout: vi.fn(async () => undefined),
  push: vi.fn(),
}));

vi.mock('expo-crypto', () => ({
  randomUUID: () => '018ff5b8-0000-7000-8000-000000000001',
}));
vi.mock('react-native', () => {
  function NativeView({ children, ...props }: NativeProps) {
    return createElement('div', props, typeof children === 'function' ? null : children);
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
    stopAnimation() {}
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
vi.mock('@/ui/haptics', () => ({ lightTick: () => {}, success: () => {} }));
vi.mock('@/components/note-card', () => ({
  NOTE_USEFUL_ERROR_MESSAGE: 'Não deu pra atualizar o Útil. Tenta de novo.',
  NoteCard: ({ note, ...props }: { note: { id: string } } & NativeProps) =>
    createElement('article', { ...props, note, testID: `note-card-${note.id}` }),
}));
vi.mock('@/features/notes/category-filter-controls', () => ({
  CategoryFilterControls: () => createElement('div'),
}));
vi.mock('@/components/read-auth-gate', () => ({
  ReadAuthGate: () => createElement('div'),
}));
vi.mock('@/lib/api/api-client-provider', () => ({
  useAPIClient: () => mocks.apiClient,
}));
vi.mock('@/lib/auth/auth-provider', () => ({
  useAuth: () => ({
    logout: mocks.logout,
    state: mocks.authState,
  }),
}));
vi.mock('@/lib/events/product-event-provider', () => ({
  useProductEvents: () => mocks.events,
}));
vi.mock('expo-router', async () => {
  const react = (await vi.importActual('react')) as typeof React;
  return {
    // Real useFocusEffect fires on focus changes only; this mock re-runs the
    // committed effect when focusVersion changes, so a refocus is testable
    // without re-firing on every state-driven re-render.
    useFocusEffect(effect: () => void | (() => void)) {
      react.useEffect(effect, [mocks.focusVersion]);
    },
    useRouter: () => ({ push: mocks.push }),
  };
});

async function settle(): Promise<void> {
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
}

function note(id: string, title: string) {
  return {
    author: { displayName: 'Thiago', id: 'author-id' },
    body: 'Tem pão.',
    categorySlug: 'food',
    createdAt: fixedNow,
    id,
    images: [],
    title,
    updatedAt: fixedNow,
    usefulCount: 0,
    usefulByCurrentUser: false,
  };
}

let renderer!: ReactTestRenderer;

describe('SearchScreen note mutations', () => {
  beforeEach(() => {
    mocks.authState = { status: 'authenticated', user: { id: 'user-id' } };
    mocks.focusVersion = 0;
    mocks.apiClient.listCatalogs.mockResolvedValue({
      categories: [{ active: true, displayOrder: 1, label: 'Comida', slug: 'food' }],
    });
    mocks.apiClient.searchNotes.mockReset();
    mocks.push.mockReset();
  });

  afterEach(() => {
    renderer?.unmount();
    vi.clearAllMocks();
  });

  it('drops a deleted result and shows the empty state instead of a stale count', async () => {
    const result = [{ note: note('note-x', 'Achado raro'), retrievalSource: 'lexical' as const }];
    mocks.apiClient.searchNotes.mockResolvedValue({
      results: result,
      searchVersion: 'fts5-v1',
    });

    await renderSearch();
    await submit('achado');

    expect(renderer.root.findAllByProps({ testID: 'note-card-note-x' })).toHaveLength(1);
    expect(renderer.root.findByProps({ countLabel: '1 achado' })).toBeTruthy();

    markNoteDeleted('note-x');
    await act(async () => {
      renderer.update(createElement(SearchScreen));
      await settle();
    });

    expect(renderer.root.findAllByProps({ testID: 'note-card-note-x' })).toHaveLength(0);
    expect(renderer.root.findByProps({ countLabel: '0 achados' })).toBeTruthy();
    expect(renderer.root.findAllByProps({ title: 'Nada por aqui ainda' })).toHaveLength(1);
  });

  it('re-issues the submitted query when the screen regains focus', async () => {
    mocks.apiClient.searchNotes.mockResolvedValue({ results: [], searchVersion: 'fts5-v1' });

    await renderSearch();
    await submit('café');
    // Mount must not search (submittedQueryRef starts null); only the submit fires.
    expect(mocks.apiClient.searchNotes).toHaveBeenCalledTimes(1);

    mocks.focusVersion += 1;
    await act(async () => {
      renderer.update(createElement(SearchScreen));
      await settle();
    });

    // Refocus re-issues the submitted query exactly once.
    expect(mocks.apiClient.searchNotes).toHaveBeenCalledTimes(2);
    expect(mocks.apiClient.searchNotes.mock.lastCall?.[0]).toMatchObject({ query: 'café' });
  });
});

async function renderSearch(): Promise<void> {
  await act(async () => {
    renderer = create(createElement(SearchScreen));
    await settle();
  });
}

async function submit(query: string): Promise<void> {
  await act(async () => {
    const matches = renderer.root.findAllByProps({ testID: 'search-field-input' });
    const input = matches[matches.length - 1];
    input.props.onChangeText(query);
    await settle();
  });
  await act(async () => {
    const matches = renderer.root.findAllByProps({ testID: 'search-field-input' });
    const input = matches[matches.length - 1];
    input.props.onSubmitEditing();
    await settle();
  });
}
