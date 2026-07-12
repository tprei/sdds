import type { ReactNode } from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { PublicAuthor, AuthorNotesPage } from '../../lib/api/authors';
import type { Catalogs } from '../../lib/api/catalogs';
import type { Note } from '../../lib/api/notes';
import { AuthorProfileContent } from './author-profile-content';

vi.mock('react-native', () => {
  type NativeProps = {
    children?: ReactNode | ((state: { pressed: boolean }) => ReactNode);
    [key: string]: unknown;
  };

  function NativeView({ children, ...props }: NativeProps) {
    return <native-view {...props}>{children}</native-view>;
  }

  function NativePressable({ children, ...props }: NativeProps) {
    const content =
      typeof children === 'function' ? children({ pressed: false }) : children;
    return <native-view {...props}>{content}</native-view>;
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
  }) => (
    <button data-action="retry" onClick={onPress}>
      <text>{label}</text>
    </button>
  ),
}));

const mocks = vi.hoisted(() => ({
  getPublicAuthor: vi.fn<() => Promise<PublicAuthor>>(),
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

describe('AuthorProfileContent pagination errors', () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it('waits for explicit retry after a pagination failure', async () => {
    mocks.getPublicAuthor.mockResolvedValue(author);
    mocks.listCatalogs.mockResolvedValue(catalogs);
    mocks.listAuthorNotes
      .mockResolvedValueOnce({ notes: [note('first-note', 'Primeira nota')], nextCursor: 'cursor-1' })
      .mockRejectedValueOnce(new Error('page_failed'))
      .mockResolvedValueOnce({ notes: [note('second-note', 'Segunda nota')], nextCursor: null });

    let renderer: ReactTestRenderer;
    await act(async () => {
      renderer = create(
        <AuthorProfileContent authorID="author-id" onPressNote={() => undefined} />,
      );
      await flushPromises();
    });

    const scrollView = renderer!.root.findByProps({ testID: 'author-profile-scroll' });
    await act(async () => {
      scrollView.props.onScroll(nearEndEvent());
      await flushPromises();
    });

    expect(mocks.listAuthorNotes).toHaveBeenCalledTimes(2);
    expect(mocks.listAuthorNotes).toHaveBeenLastCalledWith({
      authorID: 'author-id',
      cursor: 'cursor-1',
    });
    expect(renderer!.root.findAllByProps({ accessibilityRole: 'alert' })).not.toHaveLength(0);

    await act(async () => {
      scrollView.props.onScroll(nearEndEvent());
      await flushPromises();
    });
    expect(mocks.listAuthorNotes).toHaveBeenCalledTimes(2);

    const retryButton = renderer!.root.findByProps({ 'data-action': 'retry' });
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
    expect(renderer!.root.findAllByProps({ accessibilityRole: 'alert' })).toHaveLength(0);
    expect(renderer!.root.findAll((node) => node.props.children === 'Segunda nota')).not.toHaveLength(0);

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
}
