import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import SearchScreen from '@/app/(tabs)/search';
import { assertLoadingFirstCommit } from '@/test-support/assert-loading-first-commit';
import { NoteCardSkeleton } from '@/ui/skeleton';

const { createElement } = React;
type ReactNode = React.ReactNode;
type NativeProps = {
  children?: ReactNode | ((state: { pressed: boolean }) => ReactNode);
  [key: string]: unknown;
};
type Deferred<T> = {
  promise: Promise<T>;
  reject: (reason?: unknown) => void;
  resolve: (value: T) => void;
};

type AuthStateMock =
  | { status: 'authenticated'; user: { id: string } }
  | { status: 'anonymous' };

const mocks = vi.hoisted(() => ({
  apiClient: {
    listCatalogs: vi.fn(),
    searchNotes: vi.fn(),
    markNoteUseful: vi.fn(),
    unmarkNoteUseful: vi.fn(),
  },
  authState: { status: 'authenticated', user: { id: 'user-id' } } as AuthStateMock,
  events: { record: vi.fn() },
  logout: vi.fn(async () => undefined),
  push: vi.fn(),
  uuid: 0,
}));

vi.mock('expo-crypto', () => ({
  randomUUID: () => {
    mocks.uuid += 1;
    return `018ff5b8-0000-7000-8000-${String(mocks.uuid).padStart(12, '0')}`;
  },
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
vi.mock('@/ui/haptics', () => ({
  lightTick: () => {},
  success: () => {},
}));
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
vi.mock('@/lib/auth/auth-provider', () => ({
  useAuth: () => ({
    apiClient: mocks.apiClient,
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
    useFocusEffect(effect: () => void | (() => void)) {
      react.useEffect(effect, [effect]);
    },
    useRouter: () => ({ push: mocks.push }),
  };
});

function deferred<T>(): Deferred<T> {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((nextResolve, nextReject) => {
    resolve = nextResolve;
    reject = nextReject;
  });
  return { promise, reject, resolve };
}

async function settle(): Promise<void> {
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
}

const noteA = note('note-a', 'A');
const noteB = note('note-b', 'B');

let renderer!: ReactTestRenderer;
describe('SearchScreen product events', () => {
  beforeEach(() => {
    mocks.authState = { status: 'authenticated', user: { id: 'user-id' } };
    mocks.uuid = 0;
    mocks.apiClient.listCatalogs.mockResolvedValue({
      categories: [
        { active: true, displayOrder: 1, label: 'Comida', slug: 'food' },
      ],
    });
    mocks.apiClient.markNoteUseful.mockResolvedValue(undefined);
    mocks.apiClient.unmarkNoteUseful.mockResolvedValue(undefined);
    mocks.events.record.mockReset();
    mocks.push.mockReset();
  });

  afterEach(() => {
    renderer?.unmount();
    vi.clearAllMocks();
  });

  it('records deferred search lineage only for the committed current result set', async () => {
    const searchResultA = [{ note: noteA, retrievalSource: 'lexical' as const }];
    const searchResultB = [
      { note: noteB, retrievalSource: 'lexical' as const },
    ];
    const responseA = deferred<{
      searchVersion: 'fts5-v1';
      results: typeof searchResultA;
    }>();
    const responseB = deferred<{
      searchVersion: 'fts5-v1';
      results: typeof searchResultB;
    }>();
    mocks.apiClient.searchNotes
      .mockReturnValueOnce(responseA.promise)
      .mockReturnValueOnce(responseB.promise);

    await renderSearch();
    await submit('café');
    await submit('pão');

    await act(async () => {
      responseB.resolve({ results: searchResultB, searchVersion: 'fts5-v1' });
      await settle();
    });
    await act(async () => {
      responseA.resolve({ results: searchResultA, searchVersion: 'fts5-v1' });
      await settle();
    });

    const calls = mocks.events.record.mock.calls as [
      string,
      Record<string, unknown>,
      ...unknown[],
    ][];
    const submitted = calls.filter(([kind]) => kind === 'search_submitted');
    const impressions = calls.filter(([kind]) => kind === 'search_results_impression');
    const reformulations = calls.filter(([kind]) => kind === 'search_reformulated');

    expect(submitted).toHaveLength(2);
    expect(impressions).toHaveLength(1);
    expect(reformulations).toHaveLength(1);
    expect(impressions[0][1]).toMatchObject({
      query: 'pão',
      resultCount: 1,
      results: [{ noteID: 'note-b', rank: 1, retrievalSource: 'lexical' }],
      searchVersion: 'fts5-v1',
    });
    expect(reformulations[0][1]).toMatchObject({
      previousQuery: 'café',
      query: 'pão',
      previousSearchVersion: 'fts5-v1',
      searchVersion: 'fts5-v1',
    });

    const card = renderer.root.findByProps({ testID: 'note-card-note-b' });
    await act(async () => {
      card.props.onPress();
      await settle();
    });
    const opened = mocks.events.record.mock.calls.find(
      ([kind]) => kind === 'search_result_opened',
    ) as [string, Record<string, unknown>] | undefined;
    expect(opened?.[1]).toMatchObject({
      noteID: 'note-b',
      rank: 1,
      retrievalSource: 'lexical',
      searchID: impressions[0][1].searchID,
      searchVersion: 'fts5-v1',
    });
    expect(mocks.push).toHaveBeenCalledWith({
      params: { id: 'note-b', origin: expect.any(String) },
      pathname: '/notes/[id]',
    });

    await act(async () => {
      card.props.onPressUseful();
      await settle();
    });
    expect(mocks.events.record).toHaveBeenCalledWith('note_marked_useful', {
      context: {
        rank: 1,
        retrievalSource: 'lexical',
        searchID: impressions[0][1].searchID,
        searchVersion: 'fts5-v1',
        source: 'search',
      },
      noteID: 'note-b',
    });
  });

  it('drops an in-flight search after logout and login', async () => {
    const response = deferred<{
      searchVersion: 'fts5-v1';
      results: [];
    }>();
    mocks.apiClient.searchNotes.mockReturnValueOnce(response.promise);

    await renderSearch();
    await submit('café');

    mocks.authState = { status: 'anonymous' };
    await act(async () => {
      renderer.update(createElement(SearchScreen));
      await settle();
    });

    mocks.authState = {
      status: 'authenticated',
      user: { id: 'user-b' },
    };
    await act(async () => {
      renderer.update(createElement(SearchScreen));
      await settle();
    });

    await act(async () => {
      response.resolve({ results: [], searchVersion: 'fts5-v1' });
      await settle();
    });

    expect(mocks.events.record).not.toHaveBeenCalled();
  });

  it('suppresses reformulation when the predecessor fails', async () => {
    const failed = deferred<never>();
    const succeeded = deferred<{ searchVersion: 'fts5-v1'; results: [] }>();
    mocks.apiClient.searchNotes
      .mockReturnValueOnce(failed.promise)
      .mockReturnValueOnce(succeeded.promise);

    await renderSearch();
    await submit('café');
    await submit('pão');
    await act(async () => {
      failed.reject(new Error('offline'));
      succeeded.resolve({ results: [], searchVersion: 'fts5-v1' });
      await settle();
    });

    expect(
      mocks.events.record.mock.calls.filter(
        ([kind]) => kind === 'search_reformulated',
      ),
    ).toHaveLength(0);
    expect(
      mocks.events.record.mock.calls.filter(
        ([kind]) => kind === 'search_no_results',
      ),
    ).toHaveLength(1);
  });

  it('shows the skeleton grid the instant a search is submitted, before any promise flushes', async () => {
    const response = deferred<{ searchVersion: 'fts5-v1'; results: [] }>();
    mocks.apiClient.searchNotes.mockReturnValueOnce(response.promise);

    await renderSearch();
    await act(async () => {
      const matches = renderer.root.findAllByProps({ testID: 'search-field-input' });
      const input = matches[matches.length - 1];
      input.props.onChangeText('café');
      await settle();
    });

    assertLoadingFirstCommit(
      () => {
        const matches = renderer.root.findAllByProps({ testID: 'search-field-input' });
        const input = matches[matches.length - 1];
        input.props.onSubmitEditing();
        return renderer;
      },
      ['Nada por aqui ainda', 'Não deu pra buscar'],
      (r) => {
        expect(r.root.findAllByType(NoteCardSkeleton)).toHaveLength(4);
      },
    );
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

function note(id: string, title: string) {
  return {
    author: { displayName: 'Thiago', id: 'author-id' },
    body: 'Tem pão.',
    categorySlug: 'food',
    createdAt: 1782993600000,
    id,
    images: [],
    title,
    updatedAt: 1782993600000,
    usefulCount: 0,
    usefulByCurrentUser: false,
  };
}
