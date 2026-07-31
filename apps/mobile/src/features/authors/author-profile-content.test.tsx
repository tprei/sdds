import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { PublicAuthor, AuthorNotesPage } from '../../lib/api/authors';
import type { Catalogs } from '../../lib/api/catalogs';
import type { Note } from '../../lib/api/notes';
import type { APIClient } from '../../lib/api/client';
import { AuthorProfileContent } from './author-profile-content';
import { assertLoadingFirstCommit } from '@/test-support/assert-loading-first-commit';

const { createElement } = React;
type ReactNode = React.ReactNode;

vi.mock('react-native', () => {
  type NativeProps = {
    children?: ReactNode | ((state: { pressed: boolean }) => ReactNode);
    [key: string]: unknown;
  };

  function NativeView({ children, ...props }: NativeProps) {
    const content = typeof children === 'function' ? null : children;
    return createElement('div', props, content);
  }

  function NativePressable({ children, ...props }: NativeProps) {
    const content =
      typeof children === 'function' ? children({ pressed: false }) : children;
    return createElement('div', props, content);
  }

  class AnimatedValue {
    value: number;
    constructor(value: number) {
      this.value = value;
    }
  }

  return {
    Image: NativeView,
    Platform: { OS: 'ios' },
    Pressable: NativePressable,
    ScrollView: NativeView,
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
    Text: NativeView,
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
vi.mock('@/ui/haptics', () => ({
  lightTick: () => {},
  success: () => {},
}));

vi.mock('../../components/note-card', () => ({
  NOTE_USEFUL_ERROR_MESSAGE: 'Não deu pra atualizar o Útil. Tenta de novo.',
  NoteCard: (props: { note: Note; [key: string]: unknown }) =>
    createElement(
      'note-card',
      { ...props, testID: 'author-note-card' },
      props.note.title,
    ),
}));

type FocusEffect = () => void | (() => void);

vi.mock('expo-router', async () => {
  const react = (await vi.importActual('react')) as typeof React;
  return {
    useFocusEffect(effect: FocusEffect) {
      mocks.focusEffect = effect;
      react.useEffect(effect, [effect]);
    },
  };
});
vi.mock('../../lib/events/product-event-provider', () => {
  const productEvents = { record: mocks.record };
  return {
    useProductEvents: () => productEvents,
  };
});

const onSessionExpired = vi.fn(async () => undefined);

const mocks = vi.hoisted(() => ({
  getPublicAuthor: vi.fn<(authorID: string) => Promise<PublicAuthor>>(),
  listAuthorNotes:
    vi.fn<(input: { authorID: string; cursor?: string }) => Promise<AuthorNotesPage>>(),
  listCatalogs: vi.fn<() => Promise<Catalogs>>(),
  markNoteUseful: vi.fn<(noteID: string) => Promise<void>>(),
  record: vi.fn(),
  unmarkNoteUseful: vi.fn<(noteID: string) => Promise<void>>(),
  focusEffect: null as FocusEffect | null,
}));

const mockClient = mocks as unknown as APIClient;

const author: PublicAuthor = {
  displayName: 'Marina Alves',
  id: 'author-id',
  noteCount: 2,
};

const catalogs: Catalogs = {
  categories: [
    { active: true, displayOrder: 10, label: 'Comida', slug: 'food' },
  ],
  places: [],
};

describe('AuthorProfileContent', () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it('waits for explicit retry after a pagination failure', async () => {
    mocks.getPublicAuthor.mockResolvedValue(author);
    mocks.listCatalogs.mockResolvedValue(catalogs);
    mocks.listAuthorNotes
      .mockResolvedValueOnce({
        notes: [note('first-note', 'Primeira nota')],
        nextCursor: 'cursor-1',
      })
      .mockRejectedValueOnce(new Error('page_failed'))
      .mockResolvedValueOnce({
        notes: [note('second-note', 'Segunda nota')],
        nextCursor: null,
      });

    let renderer: ReactTestRenderer;
    await act(async () => {
      renderer = create(
        <AuthorProfileContent
          authorID="author-id"
          onPressNote={() => undefined}
          onSessionExpired={onSessionExpired}
          apiClient={mockClient}
        />,
      );
      await flushPromises();
    });

    const scrollView = renderer!.root.findByProps({
      testID: 'author-profile-scroll',
    });
    await act(async () => {
      scrollView.props.onScroll(nearEndEvent());
      await flushPromises();
    });

    expect(mocks.listAuthorNotes).toHaveBeenCalledTimes(2);
    expect(mocks.listAuthorNotes).toHaveBeenLastCalledWith(
      {
        authorID: 'author-id',
        cursor: 'cursor-1',
      },
    );
    expect(
      renderer!.root.findAllByProps({ accessibilityRole: 'alert' }),
    ).not.toHaveLength(0);
    expect(textNodes(renderer!, 'Primeira nota')).not.toHaveLength(0);

    await act(async () => {
      scrollView.props.onScroll(nearEndEvent());
      await flushPromises();
    });
    expect(mocks.listAuthorNotes).toHaveBeenCalledTimes(2);

    const retryButton = renderer!.root.findByProps({ label: 'Tentar de novo' });
    expect(retryButton).toBeDefined();

    await act(async () => {
      retryButton.props.onPress();
      await flushPromises();
    });

    expect(mocks.listAuthorNotes).toHaveBeenCalledTimes(3);
    expect(mocks.listAuthorNotes).toHaveBeenLastCalledWith(
      {
        authorID: 'author-id',
        cursor: 'cursor-1',
      },
    );
    expect(
      renderer!.root.findAllByProps({ accessibilityRole: 'alert' }),
    ).toHaveLength(0);
    expect(textNodes(renderer!, 'Segunda nota')).not.toHaveLength(0);

    renderer!.unmount();
  });

  it('hides loaded author data immediately when the author changes', async () => {
    const nextAuthor: PublicAuthor = {
      displayName: 'João Silva',
      id: 'next-author',
      noteCount: 1,
    };
    const nextProfile = deferred<PublicAuthor>();
    const nextPage = deferred<AuthorNotesPage>();
    mocks.getPublicAuthor
      .mockResolvedValueOnce(author)
      .mockReturnValueOnce(nextProfile.promise);
    mocks.listCatalogs.mockResolvedValue(catalogs);
    mocks.listAuthorNotes
      .mockResolvedValueOnce({
        notes: [note('first-note', 'Primeira nota')],
        nextCursor: null,
      })
      .mockReturnValueOnce(nextPage.promise);

    let renderer: ReactTestRenderer;
    await act(async () => {
      renderer = create(
        <AuthorProfileContent
          authorID="author-id"
          onPressNote={() => undefined}
          onSessionExpired={onSessionExpired}
          apiClient={mockClient}
        />,
      );
      await flushPromises();
    });
    await act(async () => {
      await flushPromises();
    });
    expect(textNodes(renderer!, 'Marina Alves')).not.toHaveLength(0);
    expect(textNodes(renderer!, 'Primeira nota')).not.toHaveLength(0);

    await act(async () => {
      renderer!.update(
        <AuthorProfileContent
          authorID="next-author"
          onPressNote={() => undefined}
          onSessionExpired={onSessionExpired}
          apiClient={mockClient}
        />,
      );
      await flushPromises();
    });

    expect(textNodes(renderer!, 'Marina Alves')).toHaveLength(0);
    expect(textNodes(renderer!, 'Primeira nota')).toHaveLength(0);
    expect(textNodes(renderer!, 'Carregando perfil…')).not.toHaveLength(0);

    await act(async () => {
      nextProfile.resolve(nextAuthor);
      nextPage.resolve({
        notes: [note('next-note', 'Segunda nota')],
        nextCursor: null,
      });
      await flushPromises();
    });

    expect(textNodes(renderer!, 'João Silva')).not.toHaveLength(0);
    expect(textNodes(renderer!, 'Segunda nota')).not.toHaveLength(0);

    renderer!.unmount();
  });

  it('ignores stale author responses after the author changes', async () => {
    const firstProfile = deferred<PublicAuthor>();
    const firstPage = deferred<AuthorNotesPage>();
    const nextAuthor: PublicAuthor = {
      displayName: 'João Silva',
      id: 'next-author',
      noteCount: 1,
    };
    const nextProfile = deferred<PublicAuthor>();
    const nextPage = deferred<AuthorNotesPage>();
    mocks.getPublicAuthor
      .mockReturnValueOnce(firstProfile.promise)
      .mockReturnValueOnce(nextProfile.promise);
    mocks.listCatalogs.mockResolvedValue(catalogs);
    mocks.listAuthorNotes
      .mockReturnValueOnce(firstPage.promise)
      .mockReturnValueOnce(nextPage.promise);

    let renderer: ReactTestRenderer;
    await act(async () => {
      renderer = create(
        <AuthorProfileContent
          authorID="author-id"
          onPressNote={() => undefined}
          onSessionExpired={onSessionExpired}
          apiClient={mockClient}
        />,
      );
      await flushPromises();
    });
    await act(async () => {
      await flushPromises();
    });

    await act(async () => {
      renderer!.update(
        <AuthorProfileContent
          authorID="next-author"
          onPressNote={() => undefined}
          onSessionExpired={onSessionExpired}
          apiClient={mockClient}
        />,
      );
      await flushPromises();
    });

    await act(async () => {
      firstProfile.resolve(author);
      firstPage.resolve({
        notes: [note('first-note', 'Primeira nota')],
        nextCursor: null,
      });
      await flushPromises();
    });

    expect(textNodes(renderer!, 'Marina Alves')).toHaveLength(0);
    expect(textNodes(renderer!, 'Primeira nota')).toHaveLength(0);

    await act(async () => {
      nextProfile.resolve(nextAuthor);
      nextPage.resolve({
        notes: [note('next-note', 'Segunda nota')],
        nextCursor: null,
      });
      await flushPromises();
    });

    expect(textNodes(renderer!, 'João Silva')).not.toHaveLength(0);
    expect(textNodes(renderer!, 'Segunda nota')).not.toHaveLength(0);

    renderer!.unmount();
  });
  it('records useful changes with author-profile provenance', async () => {
    mocks.getPublicAuthor.mockResolvedValue(author);
    mocks.listCatalogs.mockResolvedValue(catalogs);
    mocks.listAuthorNotes.mockResolvedValue({
      notes: [note('author-note', 'Nota do perfil')],
      nextCursor: null,
    });
    mocks.markNoteUseful.mockResolvedValue(undefined);

    let renderer!: ReactTestRenderer;
    await act(async () => {
      renderer = create(
        <AuthorProfileContent
          authorID="author-id"
          onPressNote={() => undefined}
          onSessionExpired={onSessionExpired}
          apiClient={mockClient}
        />,
      );
      await flushPromises();
    });

    const card = renderer.root.findByProps({ testID: 'author-note-card' });
    await act(async () => {
      await card.props.onPressUseful();
      await flushPromises();
    });

    expect(mocks.markNoteUseful).toHaveBeenCalledWith('author-note');
    expect(mocks.record).toHaveBeenCalledWith('note_marked_useful', {
      context: { source: 'author_profile' },
      noteID: 'author-note',
    });
    renderer.unmount();
  });
  it('does not record a failed author-profile useful request', async () => {
    mocks.getPublicAuthor.mockResolvedValue(author);
    mocks.listCatalogs.mockResolvedValue(catalogs);
    mocks.listAuthorNotes.mockResolvedValue({
      notes: [note('failed-note', 'Falha')],
      nextCursor: null,
    });
    mocks.markNoteUseful.mockRejectedValueOnce(new Error('useful_failed'));

    let renderer!: ReactTestRenderer;
    await act(async () => {
      renderer = create(
        <AuthorProfileContent
          authorID="author-id"
          onPressNote={() => undefined}
          onSessionExpired={onSessionExpired}
          apiClient={mockClient}
        />,
      );
      await flushPromises();
    });
    const card = renderer.root.findByProps({ testID: 'author-note-card' });
    await act(async () => {
      await card.props.onPressUseful();
      await flushPromises();
    });

    expect(mocks.record).not.toHaveBeenCalled();
    renderer.unmount();
  });

  it('keeps the profile ready on a second focus for the same author, refreshing without a loading flash', async () => {
    mocks.getPublicAuthor.mockResolvedValue(author);
    mocks.listCatalogs.mockResolvedValue(catalogs);
    mocks.listAuthorNotes.mockResolvedValue({
      notes: [note('first-note', 'Primeira nota')],
      nextCursor: null,
    });

    let renderer!: ReactTestRenderer;
    await act(async () => {
      renderer = create(
        <AuthorProfileContent
          authorID="author-id"
          onPressNote={() => undefined}
          onSessionExpired={onSessionExpired}
          apiClient={mockClient}
        />,
      );
      await flushPromises();
    });

    expect(textNodes(renderer, 'Primeira nota')).not.toHaveLength(0);
    expect(mocks.listAuthorNotes).toHaveBeenCalledTimes(1);
    expect(mocks.focusEffect).not.toBeNull();

    assertLoadingFirstCommit(
      () => {
        mocks.focusEffect?.();
        return renderer;
      },
      ['Carregando perfil…'],
      (r) => {
        expect(textNodes(r, 'Primeira nota')).not.toHaveLength(0);
      },
    );

    await act(async () => {
      await flushPromises();
    });
    expect(mocks.listAuthorNotes).toHaveBeenCalledTimes(2);
    expect(textNodes(renderer, 'Primeira nota')).not.toHaveLength(0);

    renderer.unmount();
  });
});

function note(id: string, title: string): Note {
  return {
    author: { displayName: author.displayName, id: author.id },
    body: `${title} body`,
    categorySlug: 'food',
    createdAt: 1782993600000,
    id,
    images: [],
    placeSlug: null,
    title,
    updatedAt: 1782993600000,
    usefulCount: 0,
    usefulByCurrentUser: false,
  };
}

type Deferred<T> = {
  promise: Promise<T>;
  resolve: (value: T) => void;
};

function deferred<T>(): Deferred<T> {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((nextResolve) => {
    resolve = nextResolve;
  });
  return { promise, resolve };
}

function textNodes(renderer: ReactTestRenderer, text: string) {
  return renderer.root.findAll((node) => node.props.children === text);
}

function nearEndEvent() {
  return {
    nativeEvent: {
      contentOffset: { y: 900 },
      contentSize: { height: 1000 },
      layoutMeasurement: { height: 200 },
    },
  };
}

async function flushPromises(): Promise<void> {
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
}
