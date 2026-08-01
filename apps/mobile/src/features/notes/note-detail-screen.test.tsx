import * as React from 'react';
import {
  act,
  create,
  type ReactTestInstance,
  type ReactTestRenderer,
} from 'react-test-renderer';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import NoteDetailScreen from '@/app/notes/[id]';
import type { Comment, CommentPage, CommentThread } from '@/lib/api/comments';
import type { CommentThreadState } from '@/features/comments/comment-thread';
import { registerPresentedNoteOrigin } from './presented-note-origin';

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
type LocalParams = {
  id?: string | string[];
  origin?: string | string[];
};

const mocks = vi.hoisted(() => ({
  apiClient: {
    createNoteComment: vi.fn(),
    createCommentReply: vi.fn(),
    deleteNoteComment: vi.fn(),
    getNote: vi.fn(),
    listCatalogs: vi.fn(),
    markNoteUseful: vi.fn(),
    listNoteComments: vi.fn(),
    unmarkNoteUseful: vi.fn(),
    createReport: vi.fn(),
  },
  focusVersion: 0,
  authState: { status: 'loading' } as AuthStateMock,
  back: vi.fn(),
  localParams: { id: 'note-id' } as LocalParams,
  logout: vi.fn(async () => undefined),
  push: vi.fn(),
  record: vi.fn(),
}));
vi.mock('expo-crypto', () => ({
  randomUUID: () => '018ff5b8-0000-7000-8000-000000000001',
}));
vi.mock('@/lib/events/product-event-provider', () => {
  const productEvents = { record: mocks.record };
  return {
    useProductEvents: () => productEvents,
  };
});

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
    Modal: ({ children }: NativeProps) =>
      createElement('div', null, typeof children === 'function' ? null : children),
    Pressable: NativePressable,
    ScrollView: NativeView,
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
    Text: NativeView,
    TextInput: NativeTextInput,
    View: NativeView,
    Animated: {
      View: NativeView,
      Value: AnimatedValue,
      createAnimatedComponent: <T,>(component: T): T => component,
      timing: () => ({ start: () => {} }),
    },
    Easing: {
      out: (fn: unknown) => fn,
      ease: (x: number) => x,
    },
    AccessibilityInfo: {
      isReduceMotionEnabled: () => Promise.resolve(false),
      addEventListener: () => ({ remove: () => {} }),
    },
  };
});
vi.mock('react-native-safe-area-context', () => ({
  SafeAreaView: ({ children }: { children: ReactNode }) => children,
}));
vi.mock('@/ui/icon-button', () => ({
  IconButton: ({
    icon,
    accessibilityLabel,
    onPress,
    testID,
  }: {
    icon: ReactNode;
    accessibilityLabel: string;
    onPress?: () => void;
    testID?: string;
  }) => createElement('button', { accessibilityLabel, onPress, testID }, icon),
}));
vi.mock('@/ui/icons', () => ({
  IconChevronLeft: () => createElement('svg', null),
  IconFlag: () => createElement('svg', null),
}));
vi.mock('@/ui/haptics', () => ({
  lightTick: () => {},
  success: () => {},
}));

vi.mock('expo-router', async () => {
  const react = (await vi.importActual('react')) as typeof React;
  return {
    useFocusEffect(effect: () => void | (() => void)) {
      react.useEffect(effect, [effect, mocks.focusVersion]);
    },
    useLocalSearchParams: () => mocks.localParams,
    useRouter: () => ({ back: mocks.back, push: mocks.push }),
  };
});

vi.mock('@/lib/auth/auth-provider', () => ({
  useAuth: () => ({
    apiClient: mocks.apiClient,
    logout: mocks.logout,
    state: mocks.authState,
  }),
}));

vi.mock('@/lib/api/notes', () => ({
  APIRequestError: class APIRequestError extends Error {
    constructor(readonly status: number) {
      super('api_request_failed');
    }
  },
}));

vi.mock('@/features/notes/catalog', () => ({
  buildNoteCatalog: () => ({ kind: 'catalog' }),
  labelNote: (_catalog: unknown, note: Record<string, unknown>) => ({
    ...note,
    categoryLabel: 'Comida',
  }),
}));

vi.mock('@/features/notes/note-detail-content', () => ({
  NoteDetailContent: ({ note }: { note: { id: string } }) =>
    createElement('div', { testID: 'note-detail-content' }, note.id),
}));

vi.mock('@/features/notes/note-action-bar', () => ({
  NoteActionBar: ({
    commentCount,
    onFocusComposer,
    useful,
  }: {
    commentCount: number;
    onFocusComposer: () => void;
    useful: {
      count: number;
      marked: boolean;
      pending: boolean;
      onToggle: () => void;
    };
  }) =>
    createElement(
      'div',
      null,
      createElement(
        'div',
        { testID: 'useful-state' },
        `${useful.count}:${useful.marked}`,
      ),
      createElement(
        'button',
        { disabled: useful.pending, onPress: useful.onToggle, testID: 'useful-button' },
        'Útil',
      ),
      createElement(
        'button',
        { onPress: onFocusComposer, testID: 'focus-composer' },
        'Comentar',
      ),
      createElement('div', { testID: 'comment-count' }, String(commentCount)),
    ),
}));

vi.mock('@/features/comments/comments-section', () => ({
  CommentsSection: (props: NativeProps) =>
    createElement('div', { ...props, testID: 'comments-section' }),
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
  createdAt: testTimestamp(),
  id: 'note-id',
  images: [],
  title: 'Café bom',
  updatedAt: testTimestamp(),
  usefulCount: 0,
  usefulByCurrentUser: false,
};

const firstComment = comment('comment-1', 'Primeiro comentário');
const secondComment = comment('comment-2', 'Segundo comentário');
const submittedComment = comment('comment-new', 'Comentário novo');

describe('NoteDetailScreen route', () => {
  beforeEach(() => {
    mocks.authState = {
      status: 'authenticated',
      token: 'session-token',
      user: { author: { displayName: 'Thiago', id: 'author-id' }, id: 'user-id' },
    };
    mocks.localParams = { id: 'note-id' };
    mocks.focusVersion = 0;
    mocks.apiClient.listCatalogs.mockResolvedValue({ categories: [] });
    mocks.apiClient.getNote.mockResolvedValue(note);
    mocks.apiClient.listNoteComments.mockResolvedValue({
      threads: [],
      nextCursor: null,
    });
    mocks.apiClient.createNoteComment.mockReset();
    mocks.apiClient.createCommentReply.mockReset();
    mocks.apiClient.deleteNoteComment.mockReset();
    mocks.apiClient.markNoteUseful.mockReset();
    mocks.apiClient.unmarkNoteUseful.mockReset();
    mocks.apiClient.createReport.mockReset();
    mocks.logout.mockClear();
    mocks.back.mockClear();
    mocks.push.mockClear();
    mocks.record.mockClear();
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

    expect(mocks.apiClient.listCatalogs).not.toHaveBeenCalled();
    expect(mocks.apiClient.getNote).not.toHaveBeenCalled();
    expect(mocks.apiClient.listNoteComments).not.toHaveBeenCalled();
    expect(
      renderer.root.findByProps({ title: 'Entre para continuar' }),
    ).toBeDefined();
  });

  it('disables the useful button while pending and flips state after 204', async () => {
    const pending = deferred<void>();
    mocks.apiClient.markNoteUseful.mockReturnValueOnce(pending.promise);

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

    expect(
      renderer.root.findByProps({ testID: 'useful-state' }).props.children,
    ).toBe('1:true');
    expect(mocks.record).toHaveBeenCalledWith('note_marked_useful', {
      context: { source: 'note_detail' },
      noteID: 'note-id',
    });
    expect(mocks.apiClient.markNoteUseful).toHaveBeenCalledWith('note-id');
  });
  it('uses a one-shot search origin for useful provenance', async () => {
    const searchContext = {
      retrievalSource: 'lexical' as const,
      rank: 1,
      searchID: '018ff5b8-0000-7000-8000-000000000001',
      searchVersion: 'fts5-v1' as const,
      source: 'search' as const,
    };
    const origin = registerPresentedNoteOrigin('note-id', searchContext);
    mocks.localParams = { id: 'note-id', origin };

    const renderer = await renderScreen();
    await act(async () => {
      renderer.root.findByProps({ testID: 'useful-button' }).props.onPress();
      await settle();
    });

    expect(mocks.record).toHaveBeenCalledWith('note_marked_useful', {
      context: searchContext,
      noteID: 'note-id',
    });
  });

  it('does not refresh an unmounted note after a useful request settles', async () => {
    const pending = deferred<void>();
    const nextNote = {
      ...note,
      id: 'next-note-id',
      usefulCount: 7,
    };
    mocks.apiClient.markNoteUseful.mockReturnValueOnce(pending.promise);

    const renderer = await renderScreen();
    await act(async () => {
      renderer.root.findByProps({ testID: 'useful-button' }).props.onPress();
      await settle();
    });

    mocks.localParams = { id: nextNote.id };
    mocks.apiClient.getNote.mockResolvedValueOnce(nextNote);
    await act(async () => {
      renderer.update(createElement(NoteDetailScreen));
      await settle();
    });
    await act(async () => {
      pending.resolve();
      await settle();
    });

    expect(renderer.root.findByProps({ testID: 'useful-state' }).props.children).toBe(
      '7:false',
    );
    expect(mocks.apiClient.getNote).toHaveBeenLastCalledWith(nextNote.id);
  });

  it('reloads comments after a useful request settles during a refocus', async () => {
    const pendingUseful = deferred<void>();
    const refocusedNote = deferred<typeof note>();
    const refreshedNote = {
      ...note,
      usefulByCurrentUser: true,
      usefulCount: 1,
    };
    mocks.apiClient.listNoteComments
      .mockResolvedValueOnce(commentPage([firstComment]))
      .mockResolvedValueOnce(commentPage([secondComment]));
    mocks.apiClient.markNoteUseful.mockReturnValueOnce(pendingUseful.promise);

    const renderer = await renderScreen();
    await act(async () => {
      renderer.root.findByProps({ testID: 'useful-button' }).props.onPress();
      await settle();
    });

    mocks.apiClient.getNote
      .mockReturnValueOnce(refocusedNote.promise)
      .mockResolvedValueOnce(refreshedNote);
    mocks.focusVersion += 1;
    await act(async () => {
      renderer.update(createElement(NoteDetailScreen));
      await settle();
    });
    await act(async () => {
      pendingUseful.resolve();
      await settle();
    });

    expect(renderer.root.findByProps({ testID: 'useful-state' }).props.children).toBe(
      '1:true',
    );
    expect(renderedCommentThread(renderer).threads).toEqual([thread(secondComment)]);

    await act(async () => {
      refocusedNote.resolve(note);
      await settle();
    });
    expect(renderedCommentThread(renderer).threads).toEqual([thread(secondComment)]);
  });


  it('keeps prior state and shows inline error on non-401 useful failure', async () => {
    mocks.apiClient.markNoteUseful.mockRejectedValueOnce({ status: 500 });

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
    mocks.apiClient.markNoteUseful.mockRejectedValueOnce({ status: 401 });

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

  it('loads comments after the note and appends an incremental page', async () => {
    mocks.apiClient.listNoteComments
      .mockResolvedValueOnce(commentPage([firstComment], 'cursor-1'))
      .mockResolvedValueOnce(commentPage([secondComment]));

    const renderer = await renderScreen();

    expect(mocks.apiClient.listNoteComments).toHaveBeenCalledWith({
      noteID: 'note-id',
    });
    expect(commentsSection(renderer).props.currentAuthorID).toBe('author-id');
    expect(renderedCommentThread(renderer).threads).toEqual([thread(firstComment)]);

    await act(async () => {
      commentsSection(renderer).props.onLoadMore();
      await settle();
    });

    expect(mocks.apiClient.listNoteComments).toHaveBeenLastCalledWith({
      cursor: 'cursor-1',
      noteID: 'note-id',
    });
    expect(renderedCommentThread(renderer).threads).toEqual([
      thread(firstComment),
      thread(secondComment),
    ]);
  });

  it('hydrates replies beneath a parent and includes them in the comment count', async () => {
    const reply = replyComment('reply-1', 'Uma resposta');
    mocks.apiClient.listNoteComments.mockResolvedValueOnce({
      threads: [
        {
          comment: firstComment,
          replies: [reply],
          hasMoreReplies: false,
        },
      ],
      nextCursor: null,
    });

    const renderer = await renderScreen();

    expect(renderedCommentThread(renderer).threads).toEqual([
      {
        comment: firstComment,
        replies: [reply],
        hasMoreReplies: false,
      },
    ]);
    expect(
      renderer.root.findByProps({ testID: 'comment-count' }).children,
    ).toEqual(['2']);
  });

  it('submits a trimmed reply under the selected parent', async () => {
    const reply = replyComment('reply-new', 'Resposta nova');
    mocks.apiClient.listNoteComments.mockResolvedValueOnce(
      commentPage([firstComment]),
    );
    mocks.apiClient.createCommentReply.mockResolvedValueOnce(reply);

    const renderer = await renderScreen();
    await act(async () => {
      commentsSection(renderer).props.onStartReply(
        firstComment.id,
        firstComment.author.displayName,
      );
      await settle();
    });
    await act(async () => {
      commentsSection(renderer).props.onReplyDraftChange('  Resposta nova  ');
      await settle();
    });
    await act(async () => {
      commentsSection(renderer).props.onSubmitReply();
      await settle();
    });

    expect(mocks.apiClient.createCommentReply).toHaveBeenCalledWith({
      body: 'Resposta nova',
      parentCommentID: firstComment.id,
    });
    expect(mocks.record).toHaveBeenCalledWith('comment_created', {
      commentID: reply.id,
      noteID: 'note-id',
      parentCommentID: firstComment.id,
    });
    expect(renderedCommentThread(renderer).threads).toEqual([
      { comment: firstComment, replies: [reply], hasMoreReplies: false },
    ]);
  });

  it('logs out when a reply submission returns 401', async () => {
    mocks.apiClient.listNoteComments.mockResolvedValueOnce(
      commentPage([firstComment]),
    );
    mocks.apiClient.createCommentReply.mockRejectedValueOnce({ status: 401 });

    const renderer = await renderScreen();
    await act(async () => {
      commentsSection(renderer).props.onStartReply(
        firstComment.id,
        firstComment.author.displayName,
      );
      await settle();
    });
    await act(async () => {
      commentsSection(renderer).props.onReplyDraftChange('Resposta');
      await settle();
    });
    await act(async () => {
      commentsSection(renderer).props.onSubmitReply();
      await settle();
    });

    expect(mocks.logout).toHaveBeenCalledOnce();
  });

  it('fences invalid drafts and keeps a created comment in the local tail', async () => {
    mocks.apiClient.listNoteComments.mockResolvedValueOnce(
      commentPage([firstComment], 'cursor-1'),
    );
    mocks.apiClient.createNoteComment.mockResolvedValueOnce(submittedComment);

    const renderer = await renderScreen();

    await act(async () => {
      commentsSection(renderer).props.onDraftChange(' \n ');
      await settle();
    });
    await act(async () => {
      commentsSection(renderer).props.onSubmit();
      await settle();
    });
    expect(mocks.apiClient.createNoteComment).not.toHaveBeenCalled();

    await act(async () => {
      commentsSection(renderer).props.onDraftChange('  Comentário novo  ');
      await settle();
    });
    await act(async () => {
      commentsSection(renderer).props.onSubmit();
      await settle();
    });

    expect(mocks.apiClient.createNoteComment).toHaveBeenCalledWith({
      body: 'Comentário novo',
      noteID: 'note-id',
    });
    expect(renderedCommentThread(renderer).draft).toBe('');
    expect(renderedCommentThread(renderer).localTailThreads).toEqual([
      thread(submittedComment),
    ]);
    expect(mocks.record).toHaveBeenCalledWith('comment_created', {
      commentID: submittedComment.id,
      noteID: 'note-id',
      parentCommentID: null,
    });
  });

  it('keeps comment errors local and retries the initial list', async () => {
    mocks.apiClient.listNoteComments
      .mockRejectedValueOnce({ status: 500 })
      .mockResolvedValueOnce(commentPage([firstComment]));

    const renderer = await renderScreen();

    expect(renderedCommentThread(renderer).initialLoadStatus).toBe('error');
    await act(async () => {
      commentsSection(renderer).props.onRetryInitial();
      await settle();
    });
    expect(renderedCommentThread(renderer).initialLoadStatus).toBe('ready');
    expect(renderedCommentThread(renderer).threads).toEqual([thread(firstComment)]);

    mocks.apiClient.createNoteComment.mockRejectedValueOnce({ status: 500 });
    await act(async () => {
      commentsSection(renderer).props.onDraftChange('Comentário com falha');
      await settle();
    });
    await act(async () => {
      commentsSection(renderer).props.onSubmit();
      await settle();
    });
    expect(renderedCommentThread(renderer).submitStatus).toBe('error');
    expect(renderedCommentThread(renderer).draft).toBe('Comentário com falha');
    expect(mocks.record).not.toHaveBeenCalled();
  });

  it('logs out on a comment-list 401 and replaces the detail for a create 404', async () => {
    mocks.apiClient.listNoteComments.mockRejectedValueOnce({ status: 401 });

    await renderScreen();

    expect(mocks.logout).toHaveBeenCalledOnce();

    mocks.logout.mockClear();
    mocks.apiClient.listNoteComments.mockResolvedValueOnce(commentPage([]));
    mocks.apiClient.createNoteComment.mockRejectedValueOnce({ status: 404 });
    const renderer = await renderScreen();
    await act(async () => {
      commentsSection(renderer).props.onDraftChange('Comentário');
      await settle();
    });
    await act(async () => {
      commentsSection(renderer).props.onSubmit();
      await settle();
    });

    expect(
      renderer.root.findByProps({ title: 'Nota não encontrada' }),
    ).toBeDefined();
  });

  it('removes an owner comment on 204, restores a forbidden comment, and keeps a 404 hidden', async () => {
    const pendingDelete = deferred<void>();
    mocks.apiClient.listNoteComments.mockResolvedValueOnce(
      commentPage([firstComment, secondComment, submittedComment]),
    );
    mocks.apiClient.deleteNoteComment
      .mockReturnValueOnce(pendingDelete.promise)
      .mockRejectedValueOnce({ status: 403 })
      .mockRejectedValueOnce({ status: 404 });

    const renderer = await renderScreen();

    await act(async () => {
      commentsSection(renderer).props.onDeleteComment(firstComment.id);
      await settle();
    });
    expect(mocks.apiClient.deleteNoteComment).toHaveBeenCalledWith({
      commentID: firstComment.id,
      noteID: 'note-id',
    });
    expect(
      renderedCommentThread(renderer).deleteStatusByCommentID.get(firstComment.id),
    ).toBe('pending');

    await act(async () => {
      commentsSection(renderer).props.onDeleteComment(firstComment.id);
      await settle();
    });
    expect(mocks.apiClient.deleteNoteComment).toHaveBeenCalledOnce();

    await act(async () => {
      pendingDelete.resolve();
      await settle();
    });
    expect(
      renderedCommentThread(renderer).deleteStatusByCommentID.get(firstComment.id),
    ).toBe('deleted');

    await act(async () => {
      commentsSection(renderer).props.onDeleteComment(secondComment.id);
      await settle();
    });
    expect(
      renderedCommentThread(renderer).deleteStatusByCommentID.get(secondComment.id),
    ).toBe('error');

    await act(async () => {
      commentsSection(renderer).props.onDeleteComment(submittedComment.id);
      await settle();
    });
    expect(
      renderedCommentThread(renderer).deleteStatusByCommentID.get(
        submittedComment.id,
      ),
    ).toBe('deleted');
    expect(
      renderer.root.findAllByProps({ title: 'Nota não encontrada' }),
    ).toHaveLength(0);
  });

  it('ignores a comment page that settles after the note route changes', async () => {
    const stalePage = deferred<CommentPage>();
    mocks.apiClient.listNoteComments
      .mockReturnValueOnce(stalePage.promise)
      .mockResolvedValueOnce(commentPage([secondComment]));

    const renderer = await renderScreen();

    mocks.localParams = { id: 'next-note-id' };
    mocks.apiClient.getNote.mockResolvedValueOnce({
      ...note,
      id: 'next-note-id',
    });
    await act(async () => {
      renderer.update(createElement(NoteDetailScreen));
      await settle();
    });
    expect(renderedCommentThread(renderer).threads).toEqual([thread(secondComment)]);

    await act(async () => {
      stalePage.resolve(commentPage([firstComment]));
      await settle();
    });
    expect(renderedCommentThread(renderer).threads).toEqual([thread(secondComment)]);
  });

  it('ignores create and delete completions after the note route changes', async () => {
    const staleCreate = deferred<Comment>();
    const staleDelete = deferred<void>();
    const nextRouteComment = comment(
      'next-route-comment',
      'Comentário da próxima nota',
    );
    mocks.apiClient.listNoteComments
      .mockResolvedValueOnce(commentPage([firstComment, secondComment], 'cursor-2'))
      .mockResolvedValueOnce(commentPage([nextRouteComment]));
    mocks.apiClient.createNoteComment.mockReturnValueOnce(staleCreate.promise);
    mocks.apiClient.deleteNoteComment.mockReturnValueOnce(staleDelete.promise);

    const renderer = await renderScreen();
    await act(async () => {
      commentsSection(renderer).props.onDraftChange('Comentário pendente');
      await settle();
    });
    await act(async () => {
      commentsSection(renderer).props.onSubmit();
      commentsSection(renderer).props.onDeleteComment(firstComment.id);
      await settle();
    });

    mocks.localParams = { id: 'next-note-id' };
    mocks.apiClient.getNote.mockResolvedValueOnce({
      ...note,
      id: 'next-note-id',
    });
    await act(async () => {
      renderer.update(createElement(NoteDetailScreen));
      await settle();
    });
    expect(renderedCommentThread(renderer).threads).toEqual([thread(nextRouteComment)]);

    await act(async () => {
      staleCreate.resolve(submittedComment);
      staleDelete.resolve();
      await settle();
    });
    expect(renderedCommentThread(renderer).threads).toEqual([thread(nextRouteComment)]);
    expect(mocks.record).toHaveBeenCalledWith('comment_created', {
      commentID: submittedComment.id,
      noteID: 'note-id',
      parentCommentID: null,
    });
    expect(renderedCommentThread(renderer).localTailThreads).toEqual([]);
    expect(renderedCommentThread(renderer).deleteStatusByCommentID).toEqual(
      new Map(),
    );
  });

  it('opens the note report dialog targeting the loaded note', async () => {
    const renderer = await renderScreen();

    await act(async () => {
      renderer.root.findByProps({ testID: 'note-report' }).props.onPress();
      await settle();
    });

    expect(hostCount(renderer, 'report-sheet')).toBe(1);
    expect(
      renderer.root.findAll(
        (node) =>
          typeof node.type === 'string' &&
          node.props.accessibilityRole === 'header' &&
          node.props.children === 'Denunciar esta nota?',
      ),
    ).toHaveLength(1);
    expect(mocks.apiClient.createReport).not.toHaveBeenCalled();
  });

  it('opens a comment report dialog targeting the selected comment', async () => {
    mocks.apiClient.listNoteComments.mockResolvedValueOnce(
      commentPage([firstComment]),
    );
    const renderer = await renderScreen();

    await act(async () => {
      commentsSection(renderer).props.onReportComment(firstComment.id);
      await settle();
    });

    expect(hostCount(renderer, 'report-sheet')).toBe(1);
    expect(hostTextCount(renderer, 'Denunciar este comentário?')).toBe(1);
  });

  it('submits a note report, shows the success notice, and closes the dialog', async () => {
    mocks.apiClient.createReport.mockResolvedValueOnce({
      createdAt: testTimestamp(),
      details: null,
      id: 'report-1',
      reason: 'spam',
      targetID: 'note-id',
      targetType: 'note',
    });
    const renderer = await renderScreen();

    await openNoteReport(renderer);

    expect(mocks.apiClient.createReport).toHaveBeenCalledWith({
      targetType: 'note',
      targetID: 'note-id',
      reason: 'spam',
      details: '',
    });
    expect(mocks.record).toHaveBeenCalledWith('report_created', {
      reportID: 'report-1',
      targetID: 'note-id',
      targetType: 'note',
    });
    expect(
      hostTextCount(renderer, 'Valeu por avisar! A gente cuida pra rede seguir feita pra humanos.'),
    ).toBe(1);
    expect(hostCount(renderer, 'report-sheet')).toBe(0);
  });

  it('logs out on a report 401', async () => {
    mocks.apiClient.createReport.mockRejectedValueOnce({ status: 401 });
    const renderer = await renderScreen();

    await openNoteReport(renderer);

    expect(mocks.logout).toHaveBeenCalledOnce();
    expect(mocks.record).not.toHaveBeenCalled();
  });

  it('releases the report dialog when logout fails after a report 401', async () => {
    mocks.apiClient.createReport.mockRejectedValueOnce({ status: 401 });
    mocks.logout.mockRejectedValueOnce(new Error('logout failed'));
    const renderer = await renderScreen();

    await openNoteReport(renderer);

    expect(mocks.logout).toHaveBeenCalledOnce();
    expect(hostByTestID(renderer, 'report-submit').props.disabled).toBe(false);
  });

  it('shows the missing notice and closes the dialog on a report 404', async () => {
    mocks.apiClient.createReport.mockRejectedValueOnce({ status: 404 });
    const renderer = await renderScreen();

    await openNoteReport(renderer);

    expect(
      hostTextCount(renderer, 'Esse conteúdo não está mais disponível.'),
    ).toBe(1);
    expect(hostCount(renderer, 'report-sheet')).toBe(0);
  });

  it('preserves the reason and details and shows the retry notice on a retryable failure', async () => {
    mocks.apiClient.createReport.mockRejectedValueOnce({ status: 500 });
    const renderer = await renderScreen();

    await act(async () => {
      renderer.root.findByProps({ testID: 'note-report' }).props.onPress();
      await settle();
    });
    await act(async () => {
      hostByTestID(renderer, 'report-details').props.onChangeText(
        'contexto extra',
      );
      await settle();
    });
    await act(async () => {
      hostByTestID(renderer, 'report-reason-spam').props.onPress();
      await settle();
    });
    await act(async () => {
      hostByTestID(renderer, 'report-submit').props.onPress();
      await settle();
    });

    expect(hostCount(renderer, 'report-sheet')).toBe(1);
    expect(
      hostTextCount(renderer, 'Não deu pra enviar a denúncia. Tenta de novo.'),
    ).toBe(1);
    expect(hostByTestID(renderer, 'report-reason-spam').props.accessibilityState).toEqual(
      { checked: true },
    );
    expect(hostByTestID(renderer, 'report-details').props.value).toBe(
      'contexto extra',
    );
  });

  it('prevents a duplicate report submission while pending', async () => {
    const pending = deferred<{ id: string }>();
    mocks.apiClient.createReport.mockReturnValueOnce(pending.promise);
    const renderer = await renderScreen();

    await act(async () => {
      renderer.root.findByProps({ testID: 'note-report' }).props.onPress();
      await settle();
    });
    await act(async () => {
      hostByTestID(renderer, 'report-reason-spam').props.onPress();
      await settle();
    });
    await act(async () => {
      const submit = hostByTestID(renderer, 'report-submit');
      submit.props.onPress();
      submit.props.onPress();
    });
    expect(mocks.apiClient.createReport).toHaveBeenCalledTimes(1);

    await act(async () => {
      pending.resolve({ id: 'report-1' });
      await settle();
    });
    expect(
      hostTextCount(renderer, 'Valeu por avisar! A gente cuida pra rede seguir feita pra humanos.'),
    ).toBe(1);
  });

  it('ignores a report completion that settles after the note route changes', async () => {
    const receipt = {
      createdAt: 1782993600000,
      details: null,
      id: 'report-1',
      reason: 'spam',
      targetID: 'note-id',
      targetType: 'note',
    } as const;
    const pending = deferred<typeof receipt>();
    mocks.apiClient.createReport.mockReturnValueOnce(pending.promise);
    const renderer = await renderScreen();

    await act(async () => {
      renderer.root.findByProps({ testID: 'note-report' }).props.onPress();
      await settle();
    });
    await act(async () => {
      hostByTestID(renderer, 'report-reason-spam').props.onPress();
      await settle();
    });
    await act(async () => {
      hostByTestID(renderer, 'report-submit').props.onPress();
    });

    mocks.localParams = { id: 'next-note-id' };
    mocks.apiClient.getNote.mockResolvedValueOnce({ ...note, id: 'next-note-id' });
    await act(async () => {
      renderer.update(createElement(NoteDetailScreen));
      await settle();
    });

    await act(async () => {
      pending.resolve(receipt);
      await settle();
    });
    expect(mocks.record).toHaveBeenCalledWith('report_created', {
      reportID: 'report-1',
      targetID: 'note-id',
      targetType: 'note',
    });
    expect(
      hostTextCount(renderer, 'Valeu por avisar! A gente cuida pra rede seguir feita pra humanos.'),
    ).toBe(0);
  });
});

async function renderScreen(): Promise<ReactTestRenderer> {
  let renderer!: ReactTestRenderer;
  await act(async () => {
    renderer = create(createElement(NoteDetailScreen));
    await settle();
  });
  return renderer;
}

function commentsSection(renderer: ReactTestRenderer): ReactTestInstance {
  return renderer.root.findByProps({ testID: 'comments-section' });
}

function renderedCommentThread(
  renderer: ReactTestRenderer,
): CommentThreadState {
  return commentsSection(renderer).props.thread as CommentThreadState;
}

function comment(id: string, body: string): Comment {
  return {
    author: { displayName: 'Thiago', id: 'author-id' },
    body,
    createdAt: testTimestamp(),
    id,
    parentCommentID: null,
  };
}

function replyComment(id: string, body: string): Comment {
  return {
    ...comment(id, body),
    author: { displayName: 'Lia', id: 'reply-author-id' },
    parentCommentID: firstComment.id,
  };
}

function thread(comment: Comment): CommentThread {
  return { comment, replies: [], hasMoreReplies: false };
}

function commentPage(
  comments: Comment[],
  nextCursor: string | null = null,
): CommentPage {
  return { threads: comments.map(thread), nextCursor };
}

function testTimestamp(): number {
  return Date.UTC(2026, 6, 2, 12, 0, 0);
}

async function openNoteReport(renderer: ReactTestRenderer): Promise<void> {
  await act(async () => {
    renderer.root.findByProps({ testID: 'note-report' }).props.onPress();
    await settle();
  });
  await act(async () => {
    hostByTestID(renderer, 'report-reason-spam').props.onPress();
    await settle();
  });
  await act(async () => {
    hostByTestID(renderer, 'report-submit').props.onPress();
    await settle();
  });
}

function hostByTestID(
  renderer: ReactTestRenderer,
  testID: string,
): ReactTestInstance {
  return renderer.root.findAll(
    (node) => typeof node.type === 'string' && node.props.testID === testID,
  )[0];
}

function hostCount(renderer: ReactTestRenderer, testID: string): number {
  return renderer.root.findAll(
    (node) => typeof node.type === 'string' && node.props.testID === testID,
  ).length;
}

function hostTextCount(renderer: ReactTestRenderer, text: string): number {
  return renderer.root.findAll(
    (node) => typeof node.type === 'string' && node.props.children === text,
  ).length;
}
