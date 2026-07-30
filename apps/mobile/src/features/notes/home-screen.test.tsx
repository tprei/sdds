import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import HomeScreen from '@/app/(tabs)/index';

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
    listNotes: vi.fn(),
    listCatalogs: vi.fn(),
    markNoteUseful: vi.fn(),
    unmarkNoteUseful: vi.fn(),
  },
  authState: { status: 'loading' } as AuthStateMock,
  logout: vi.fn(async () => undefined),
  navigate: vi.fn(),
  push: vi.fn(),
  record: vi.fn(),
  productEventsReady: true,
}));
vi.mock('expo-crypto', () => ({
  randomUUID: () => '018ff5b8-0000-7000-8000-000000000001',
}));

vi.mock('react-native', () => {
  const { createElement } = React;
  type NP = {
    children?: ReactNode;
    [key: string]: unknown;
  };
  function NativeView({ children, ...props }: NP) {
    return createElement('div', props, children);
  }
  function NativePressable({ children, ...props }: NativeProps) {
    const content =
      typeof children === 'function' ? children({ pressed: false }) : children;
    return createElement('button', props, content);
  }
  class AnimatedValue {
    value: number;
    constructor(value: number) {
      this.value = value;
    }
  }
  return {
    View: NativeView,
    Text: NativeView,
    Pressable: NativePressable,
    ScrollView: NativeView,
    Image: NativeView,
    RefreshControl: NativeView,
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
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
  };
});

vi.mock('react-native-safe-area-context', () => {
  const { createElement } = React;
  function SafeAreaView({ children, ...props }: { children?: ReactNode; [key: string]: unknown }) {
    return createElement('div', props, children);
  }
  return { SafeAreaView };
});

vi.mock('react-native-svg', () => {
  const { createElement } = React;
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
  NoteCard: (props: NativeProps) =>
    createElement('div', { ...props, testID: 'note-card' }),
}));
vi.mock('@/features/notes/category-filter-controls', () => ({
  CategoryFilterControls: (props: NativeProps) =>
    createElement('div', { ...props, testID: 'filters' }),
}));
vi.mock('@/features/notes/catalog', () => ({
  buildNoteCatalog: () => ({ kind: 'catalog' }),
  labelNotes: (_catalog: unknown, notes: unknown[]) => notes,
}));
vi.mock('expo-router', async () => {
  const react = (await vi.importActual('react')) as typeof React;
  return {
    useFocusEffect(effect: () => void | (() => void)) {
      react.useEffect(effect, [effect]);
    },
    useRouter: () => ({ push: mocks.push, navigate: mocks.navigate }),
  };
});
vi.mock('@/lib/auth/auth-provider', () => ({
  useAuth: () => ({ apiClient: mocks.apiClient, logout: mocks.logout, state: mocks.authState }),
}));
vi.mock('@/lib/events/product-event-provider', () => ({
  useProductEvents: () => ({
    record: mocks.record,
    get ready() {
      return mocks.productEventsReady;
    },
  }),
}));

async function settle(): Promise<void> {
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
}
type Deferred<T> = {
  promise: Promise<T>;
  resolve(value: T): void;
};

function deferred<T>(): Deferred<T> {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((nextResolve) => {
    resolve = nextResolve;
  });
  return { promise, resolve };
}

function exploreNote(id: string) {
  return {
    author: { displayName: 'Ana', id: 'author-id' },
    body: 'Corpo curto do achado.',
    categorySlug: 'food',
    createdAt: 1782993600000,
    id,
    images: [],
    placeSlug: null,
    title: `Nota ${id}`,
    updatedAt: 1782993600000,
    usefulByCurrentUser: false,
    usefulCount: 0,
  };
}

describe('HomeScreen auth gate', () => {
  beforeEach(() => {
    mocks.authState = { status: 'authenticated', token: 'session-token', user: { id: 'user-id' } };
    mocks.apiClient.listCatalogs.mockResolvedValue({ categories: [], places: [] });
    mocks.apiClient.listNotes.mockResolvedValue([]);
    mocks.logout.mockClear();
    mocks.push.mockClear();
    mocks.productEventsReady = true;
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('does not start product reads before authentication', async () => {
    mocks.authState = { status: 'anonymous' };

    let renderer!: ReactTestRenderer;
    await act(async () => {
      renderer = create(createElement(HomeScreen));
      await settle();
    });

    expect(mocks.apiClient.listCatalogs).not.toHaveBeenCalled();
    expect(mocks.apiClient.listNotes).not.toHaveBeenCalled();
    expect(renderer.root.findByProps({ title: 'Entre para continuar' })).toBeDefined();
  });

  it('passes the bearer token to the initial product reads', async () => {
    await act(async () => {
      create(createElement(HomeScreen));
      await settle();
    });

    expect(mocks.apiClient.listCatalogs).toHaveBeenCalledWith();
    expect(mocks.apiClient.listNotes).toHaveBeenCalledWith({});
  });

  it('logs out on a read 401', async () => {
    mocks.apiClient.listCatalogs.mockRejectedValueOnce({ status: 401 });

    await act(async () => {
      create(createElement(HomeScreen));
      await settle();
    });

    expect(mocks.logout).toHaveBeenCalledOnce();
  });
  it('records an impression for an empty Explore result set', async () => {
    await act(async () => {
      create(createElement(HomeScreen));
      await settle();
    });

    expect(mocks.record).toHaveBeenCalledWith('explore_notes_impression', {
      categorySlug: null,
      resultCount: 0,
      results: [],
    });
  });
  it('retries an Explore impression when event recording becomes ready', async () => {
    mocks.productEventsReady = false;
    let renderer!: ReactTestRenderer;
    await act(async () => {
      renderer = create(createElement(HomeScreen));
      await settle();
    });
    expect(mocks.record).not.toHaveBeenCalled();

    mocks.productEventsReady = true;
    await act(async () => {
      renderer.update(createElement(HomeScreen));
      await settle();
    });
    expect(mocks.record).toHaveBeenCalledWith('explore_notes_impression', {
      categorySlug: null,
      resultCount: 0,
      results: [],
    });
    renderer.unmount();
  });
  it('records the rendered Explore set and provenance before interactions', async () => {
    mocks.apiClient.listNotes.mockResolvedValueOnce([exploreNote('note-id')]);
    mocks.apiClient.markNoteUseful.mockResolvedValueOnce(undefined);

    let renderer!: ReactTestRenderer;
    await act(async () => {
      renderer = create(createElement(HomeScreen));
      await settle();
    });

    const card = renderer.root.findByProps({ testID: 'note-card' });
    expect(mocks.record).toHaveBeenCalledWith('explore_notes_impression', {
      categorySlug: null,
      resultCount: 1,
      results: [{ noteID: 'note-id', rank: 1 }],
    });

    await act(async () => {
      card.props.onPress();
      await settle();
    });
    expect(mocks.record).toHaveBeenCalledWith('explore_note_opened', {
      categorySlug: null,
      noteID: 'note-id',
      rank: 1,
    });
    expect(mocks.push).toHaveBeenCalledWith({
      pathname: '/notes/[id]',
      params: {
        id: 'note-id',
        origin: '018ff5b8-0000-7000-8000-000000000001',
      },
    });

    await act(async () => {
      await card.props.onPressUseful();
      await settle();
    });
    expect(mocks.record).toHaveBeenCalledWith('note_marked_useful', {
      context: { categorySlug: null, rank: 1, source: 'explore' },
      noteID: 'note-id',
    });
  });
  it('ignores stale Explore responses and deduplicates a committed impression', async () => {
    const first = deferred<unknown[]>();
    const second = deferred<unknown[]>();
    mocks.apiClient.listNotes
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise);

    let renderer!: ReactTestRenderer;
    await act(async () => {
      renderer = create(createElement(HomeScreen));
      await settle();
    });
    await act(async () => {
      renderer.root.findByProps({ testID: 'filters' }).props.onSelectCategorySlug('food');
      await settle();
    });

    await act(async () => {
      second.resolve([exploreNote('current-note')]);
      await settle();
    });
    await act(async () => {
      first.resolve([exploreNote('stale-note')]);
      await settle();
      renderer.update(createElement(HomeScreen));
      await settle();
    });

    const impressions = mocks.record.mock.calls.filter(
      ([kind]) => kind === 'explore_notes_impression',
    );
    expect(impressions).toHaveLength(1);
    expect(impressions[0]?.[1]).toEqual({
      categorySlug: 'food',
      resultCount: 1,
      results: [{ noteID: 'current-note', rank: 1 }],
    });
  });
  it('keeps API-order ranks across two masonry columns', async () => {
    mocks.apiClient.listNotes.mockResolvedValueOnce([
      exploreNote('n1'),
      exploreNote('n2'),
      exploreNote('n3'),
      exploreNote('n4'),
      exploreNote('n5'),
    ]);

    let renderer!: ReactTestRenderer;
    await act(async () => {
      renderer = create(createElement(HomeScreen));
      await settle();
    });

    expect(mocks.record).toHaveBeenCalledWith('explore_notes_impression', {
      categorySlug: null,
      resultCount: 5,
      results: [
        { noteID: 'n1', rank: 1 },
        { noteID: 'n2', rank: 2 },
        { noteID: 'n3', rank: 3 },
        { noteID: 'n4', rank: 4 },
        { noteID: 'n5', rank: 5 },
      ],
    });

    const cards = renderer.root.findAllByProps({ testID: 'note-card' });
    expect(cards).toHaveLength(5);

    const columns = new Set(
      cards.map((card) => card.parent?.parent?.parent?.parent),
    );
    expect(columns.size).toBe(2);
  });
});
