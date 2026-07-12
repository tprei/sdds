import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { PublicAuthor, AuthorNotesPage } from '../../lib/api/authors';
import type { Catalogs } from '../../lib/api/catalogs';
import type { Note } from '../../lib/api/notes';
import { AuthorProfileContent } from './author-profile-content';

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

  return {
    Platform: { OS: 'ios' },
    Pressable: NativePressable,
    ScrollView: NativeView,
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
    Text: NativeView,
    View: NativeView,
  };
});

vi.mock('../../components/foundation-screen', () => ({
  FoundationButton: ({
    label,
    onPress,
  }: {
    label: string;
    onPress?: () => void;
  }) =>
    createElement(
      'button',
      { dataAction: 'retry', onClick: onPress },
      createElement('span', null, label),
    ),
}));

type FocusEffect = () => void | (() => void);

vi.mock('expo-router', async () => {
  const react = (await vi.importActual('react')) as typeof React;
  return {
    useFocusEffect(effect: FocusEffect) {
      react.useEffect(effect, [effect]);
    },
  };
});


const mocks = vi.hoisted(() => ({
  getPublicAuthor: vi.fn<(authorID: string) => Promise<PublicAuthor>>(),
  listAuthorNotes: vi.fn<(
    input: { authorID: string; cursor?: string },
  ) => Promise<AuthorNotesPage>>(),
  listCatalogs: vi.fn<() => Promise<Catalogs>>(),
}));

vi.mock('../../lib/api/authors', () => ({
  getPublicAuthor: mocks.getPublicAuthor,
  listAuthorNotes: mocks.listAuthorNotes,
}));

vi.mock('../../lib/api/catalogs', () => ({
  listCatalogs: mocks.listCatalogs,
}));

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
        <AuthorProfileContent authorID="author-id" onPressNote={() => undefined} />,
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
    expect(mocks.listAuthorNotes).toHaveBeenLastCalledWith({
      authorID: 'author-id',
      cursor: 'cursor-1',
    });
    expect(
      renderer!.root.findAllByProps({ accessibilityRole: 'alert' }),
    ).not.toHaveLength(0);
    expect(textNodes(renderer!, 'Primeira nota')).not.toHaveLength(0);

    await act(async () => {
      scrollView.props.onScroll(nearEndEvent());
      await flushPromises();
    });
    expect(mocks.listAuthorNotes).toHaveBeenCalledTimes(2);

    const retryButton = renderer!.root.findByProps({ dataAction: 'retry' });
    expect(retryButton).toBeDefined();

    await act(async () => {
      retryButton.props.onClick();
      await flushPromises();
    });

    expect(mocks.listAuthorNotes).toHaveBeenCalledTimes(3);
    expect(mocks.listAuthorNotes).toHaveBeenLastCalledWith({
      authorID: 'author-id',
      cursor: 'cursor-1',
    });
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
        <AuthorProfileContent authorID="author-id" onPressNote={() => undefined} />,
      );
      await flushPromises();
    });
    expect(textNodes(renderer!, 'Marina Alves')).not.toHaveLength(0);
    expect(textNodes(renderer!, 'Primeira nota')).not.toHaveLength(0);

    await act(async () => {
      renderer!.update(
        <AuthorProfileContent authorID="next-author" onPressNote={() => undefined} />,
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
        <AuthorProfileContent authorID="author-id" onPressNote={() => undefined} />,
      );
      await flushPromises();
    });

    await act(async () => {
      renderer!.update(
        <AuthorProfileContent authorID="next-author" onPressNote={() => undefined} />,
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
});

function note(id: string, title: string): Note {
  return {
    author: { displayName: author.displayName, id: author.id },
    body: `${title} body`,
    categorySlug: 'food',
    createdAt: 1782993600000,
    id,
    placeSlug: null,
    title,
    updatedAt: 1782993600000,
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
