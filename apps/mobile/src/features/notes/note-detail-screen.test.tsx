import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import NoteDetailScreen from '@/app/notes/[id]';

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
  | { status: 'loading' }
  | { status: 'anonymous' }
  | { status: 'error' }
  | {
      status: 'authenticated';
      token: string;
      user: { author: { displayName: string; id: string }; id: string };
    };

const mocks = vi.hoisted(() => ({
  authState: { status: 'loading' } as AuthStateMock,
  back: vi.fn(),
  getNote: vi.fn(),
  listCatalogs: vi.fn(),
  localParams: { id: 'note-id' },
  logout: vi.fn(async () => undefined),
  markNoteUseful: vi.fn(),
  push: vi.fn(),
  unmarkNoteUseful: vi.fn(),
}));

vi.mock('react-native', () => {
  function NativeView({ children, ...props }: NativeProps) {
    const content = typeof children === 'function' ? null : children;
    return createElement('div', props, content);
  }

  function NativePressable({ children, ...props }: NativeProps) {
    const content =
      typeof children === 'function' ? children({ pressed: false }) : children;
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
  EmptyStateCard: ({ body, title }: { body: string; title: string }) =>
    createElement('div', { body, title }),
  FoundationButton: ({
    label,
    onPress,
  }: {
    label: string;
    onPress?: () => void;
  }) => createElement('button', { onPress, testID: label }, label),
  FoundationScreen: ({ children }: { children: ReactNode }) =>
    createElement('section', null, children),
}));

vi.mock('expo-router', async () => {
  const react = (await vi.importActual('react')) as typeof React;
  return {
    useFocusEffect(effect: () => void | (() => void)) {
      react.useEffect(effect, [effect]);
    },
    useLocalSearchParams: () => mocks.localParams,
    useRouter: () => ({ back: mocks.back, push: mocks.push }),
  };
});

vi.mock('@/lib/auth/auth-provider', () => ({
  useAuth: () => ({ logout: mocks.logout, state: mocks.authState }),
}));

vi.mock('@/lib/api/catalogs', () => ({ listCatalogs: mocks.listCatalogs }));
vi.mock('@/lib/api/notes', () => ({
  APIRequestError: class APIRequestError extends Error {
    constructor(readonly status: number) {
      super('api_request_failed');
    }
  },
  getNote: mocks.getNote,
  markNoteUseful: mocks.markNoteUseful,
  unmarkNoteUseful: mocks.unmarkNoteUseful,
}));

vi.mock('@/features/notes/catalog', () => ({
  buildNoteCatalog: () => ({ kind: 'catalog' }),
  labelNote: (_catalog: unknown, note: Record<string, unknown>) => ({
    ...note,
    categoryLabel: 'Comida',
    placeLabel: null,
  }),
}));

vi.mock('@/features/notes/note-detail-content', () => ({
  NoteDetailContent: ({
    note,
    onPressUseful,
    usefulError,
    usefulPending,
  }: {
    note: { usefulByCurrentUser: boolean; usefulCount: number };
    onPressUseful: () => void;
    usefulError?: boolean;
    usefulPending?: boolean;
  }) =>
    createElement(
      'div',
      null,
      createElement(
        'div',
        { testID: 'useful-state' },
        `${note.usefulCount}:${note.usefulByCurrentUser}`,
      ),
      createElement(
        'button',
        { disabled: usefulPending, onPress: onPressUseful, testID: 'useful-button' },
        'Útil',
      ),
      usefulError
        ? createElement(
            'div',
            { testID: 'useful-error' },
            'Não deu pra atualizar o Útil. Tenta de novo.',
          )
        : null,
    ),
}));

async function settle(): Promise<void> {
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
}

function deferred<T>(): Deferred<T> {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((nextResolve, nextReject) => {
    resolve = nextResolve;
    reject = nextReject;
  });
  return { promise, reject, resolve };
}

const note = {
  author: { displayName: 'Thiago', id: 'author-id' },
  body: 'Tem pão de queijo decente.',
  categorySlug: 'food',
  createdAt: 1782993600000,
  id: 'note-id',
  images: [],
  placeSlug: null,
  title: 'Café bom',
  updatedAt: 1782993600000,
  usefulCount: 0,
  usefulByCurrentUser: false,
};

describe('NoteDetailScreen route', () => {
  beforeEach(() => {
    mocks.authState = {
      status: 'authenticated',
      token: 'session-token',
      user: { author: { displayName: 'Thiago', id: 'author-id' }, id: 'user-id' },
    };
    mocks.localParams = { id: 'note-id' };
    mocks.listCatalogs.mockResolvedValue({ categories: [], places: [] });
    mocks.getNote.mockResolvedValue(note);
    mocks.markNoteUseful.mockReset();
    mocks.unmarkNoteUseful.mockReset();
    mocks.logout.mockClear();
    mocks.back.mockClear();
    mocks.push.mockClear();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('does not start product reads before authentication', async () => {
    mocks.authState = { status: 'anonymous' };

    let renderer!: ReactTestRenderer;
    await act(async () => {
      renderer = create(createElement(NoteDetailScreen));
      await settle();
    });

    expect(mocks.listCatalogs).not.toHaveBeenCalled();
    expect(mocks.getNote).not.toHaveBeenCalled();
    expect(
      renderer.root.findByProps({ title: 'Entre para continuar' }),
    ).toBeDefined();
  });

  it('disables the useful button while pending and flips state after 204', async () => {
    const pending = deferred<void>();
    mocks.markNoteUseful.mockReturnValueOnce(pending.promise);

    let renderer!: ReactTestRenderer;
    await act(async () => {
      renderer = create(createElement(NoteDetailScreen));
      await settle();
    });

    const usefulButton = renderer.root.findByProps({ testID: 'useful-button' });
    await act(async () => {
      usefulButton.props.onPress();
      await settle();
    });
    expect(
      renderer.root.findByProps({ testID: 'useful-button' }).props.disabled,
    ).toBe(true);

    await act(async () => {
      pending.resolve();
      await settle();
    });

    expect(renderer.root.findByProps({ testID: 'useful-state' }).props.children).toBe(
      '1:true',
    );
    expect(mocks.markNoteUseful).toHaveBeenCalledWith('note-id', 'session-token');
  });

  it('keeps prior state and shows inline error on non-401 useful failure', async () => {
    mocks.markNoteUseful.mockRejectedValueOnce({ status: 500 });

    let renderer!: ReactTestRenderer;
    await act(async () => {
      renderer = create(createElement(NoteDetailScreen));
      await settle();
    });

    await act(async () => {
      renderer.root.findByProps({ testID: 'useful-button' }).props.onPress();
      await settle();
    });

    expect(renderer.root.findByProps({ testID: 'useful-state' }).props.children).toBe(
      '0:false',
    );
    expect(renderer.root.findByProps({ testID: 'useful-error' }).props.children).toBe(
      'Não deu pra atualizar o Útil. Tenta de novo.',
    );
    expect(mocks.logout).not.toHaveBeenCalled();
  });

  it('logs out on useful 401', async () => {
    mocks.markNoteUseful.mockRejectedValueOnce({ status: 401 });

    let renderer!: ReactTestRenderer;
    await act(async () => {
      renderer = create(createElement(NoteDetailScreen));
      await settle();
    });

    await act(async () => {
      renderer.root.findByProps({ testID: 'useful-button' }).props.onPress();
      await settle();
    });

    expect(mocks.logout).toHaveBeenCalledOnce();
  });
});
