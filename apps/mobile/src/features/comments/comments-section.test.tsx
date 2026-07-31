import * as React from 'react';
import {
  act,
  create,
  type ReactTestInstance,
  type ReactTestRenderer,
} from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import type { Comment } from '@/lib/api/comments';
import { avatarInitials } from '@/ui/avatar-palette';

import {
  commentThreadReducer,
  createCommentThreadState,
} from './comment-thread';
import { CommentsSection } from './comments-section';
import type { CommentThreadState } from './comment-thread';

const { createElement, useReducer } = React;
type ReactNode = React.ReactNode;

type NativeProps = {
  children?: ReactNode | ((state: { pressed: boolean }) => ReactNode);
  [key: string]: unknown;
};

vi.mock('react-native', () => {
  function NativeText({ children, ...props }: NativeProps) {
    const content = typeof children === 'function' ? null : children;
    return createElement('span', props, content);
  }

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
    AccessibilityInfo: {
      isReduceMotionEnabled: () => Promise.resolve(false),
      addEventListener: () => ({ remove: () => {} }),
    },
    Animated: {
      View: NativeView,
      Value: AnimatedValue,
      timing: () => ({ start: () => {} }),
    },
    Pressable: NativePressable,
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
    Text: NativeText,
    TextInput: NativeTextInput,
    View: NativeView,
  };
});

const noteAuthorID = 'note-author-id';
const firstComment = comment('comment-1', 'Primeiro comentário');
const localComment = comment('comment-new', 'Comentário recém-publicado');

describe('CommentsSection', () => {
  it('shows the initial load state and retries an initial error', () => {
    const onRetryInitial = vi.fn();
    const loading = renderSection({
      thread: createCommentThreadState(),
    });
    expect(textNodes(loading, 'Carregando comentários...')).toHaveLength(1);

    const failed = renderSection({
      onRetryInitial,
      thread: readyThread({ initialLoadStatus: 'error' }),
    });
    expect(
      textNodes(failed, 'Não deu pra carregar os comentários.'),
    ).toHaveLength(1);

    act(() => {
      failed.root.findByProps({ testID: 'comments-retry-initial' }).props.onPress();
    });
    expect(onRetryInitial).toHaveBeenCalledOnce();
  });

  it('renders an empty thread and navigates from a public author summary', () => {
    const onLoadMore = vi.fn();
    const onPressAuthor = vi.fn();
    const empty = renderSection({ thread: readyThread() });
    expect(textNodes(empty, 'Ainda não tem comentário. Quer começar?')).toHaveLength(1);
    expect(textNodes(empty, '0 comentários')).toHaveLength(1);

    const nonterminalEmpty = renderSection({
      onLoadMore,
      thread: readyThread({ nextCursor: 'cursor-1' }),
    });
    expect(
      textNodes(nonterminalEmpty, 'Ainda não tem comentário. Quer começar?'),
    ).toHaveLength(1);
    expect(
      nonterminalEmpty.root.findByProps({ testID: 'comments-load-more' }).props
        .label,
    ).toBe('Ver mais comentários');
    act(() => {
      nonterminalEmpty.root
        .findByProps({ testID: 'comments-load-more' })
        .props.onPress();
    });
    expect(onLoadMore).toHaveBeenCalledOnce();

    const populated = renderSection({
      onPressAuthor,
      thread: readyThread({ comments: [firstComment] }),
    });
    expect(textNodes(populated, firstComment.body)).toHaveLength(1);
    expect(textNodes(populated, '1 comentários')).toHaveLength(1);

    act(() => {
      populated.root
        .findByProps({
          accessibilityLabel: 'Abrir perfil do autor: Thiago',
        })
        .props.onPress();
    });
    expect(onPressAuthor).toHaveBeenCalledWith(firstComment.author.id);
  });

  it('shows an Autor badge only for the comment matching the note author', () => {
    const matching = renderSection({
      noteAuthorID: firstComment.author.id,
      thread: readyThread({ comments: [firstComment] }),
    });
    expect(textNodes(matching, 'Autor')).toHaveLength(1);

    const nonMatching = renderSection({
      thread: readyThread({ comments: [firstComment] }),
    });
    expect(textNodes(nonMatching, 'Autor')).toHaveLength(0);
  });

  it('keeps a local tail visible after the load-more control and retries a failed page', () => {
    const onLoadMore = vi.fn();
    const renderer = renderSection({
      onLoadMore,
      thread: readyThread({
        comments: [firstComment],
        loadMoreStatus: 'error',
        localTailComments: [localComment],
        nextCursor: 'cursor-2',
      }),
    });

    expect(textNodes(renderer, firstComment.body)).toHaveLength(1);
    expect(textNodes(renderer, localComment.body)).toHaveLength(1);
    expect(
      textNodes(
        renderer,
        'Não deu pra carregar mais comentários. Tenta de novo.',
      ),
    ).toHaveLength(1);

    const idleRenderer = renderSection({
      thread: readyThread({
        comments: [firstComment],
        localTailComments: [localComment],
        nextCursor: 'cursor-2',
      }),
    });
    const authorLabel = `${avatarInitials('Thiago')}Thiago`;
    expect(buttonLabels(idleRenderer)).toEqual([
      authorLabel,
      'Excluir comentário',
      'Denunciar comentário',
      'Ver mais comentários',
      authorLabel,
      'Excluir comentário',
      'Denunciar comentário',
      'Comentar',
    ]);

    act(() => {
      renderer.root
        .findByProps({ testID: 'comments-retry-load-more' })
        .props.onPress();
    });
    expect(onLoadMore).toHaveBeenCalledOnce();
  });

  it('shows deletion only for the current author and surfaces a failed deletion', () => {
    const onDeleteComment = vi.fn();
    const otherComment: Comment = {
      ...comment('comment-2', 'Comentário de outra pessoa'),
      author: { displayName: 'Lia', id: 'other-author-id' },
    };
    const renderer = renderSection({
      onDeleteComment,
      thread: readyThread({
        comments: [firstComment, otherComment],
        deleteStatusByCommentID: new Map([[firstComment.id, 'error' as const]]),
      }),
    });

    act(() => {
      renderer.root
        .findByProps({ testID: `comment-delete-${firstComment.id}` })
        .props.onPress();
    });
    expect(onDeleteComment).toHaveBeenCalledWith(firstComment.id);
    expect(
      renderer.root.findAllByProps({
        testID: `comment-delete-${otherComment.id}`,
      }),
    ).toHaveLength(0);
    expect(
      textNodes(
        renderer,
        'Não deu pra excluir o comentário. Tenta de novo.',
      ),
    ).toHaveLength(1);

    const pending = renderSection({
      thread: readyThread({
        comments: [firstComment],
        deleteStatusByCommentID: new Map([
          [firstComment.id, 'pending' as const],
        ]),
      }),
    });
    expect(textNodes(pending, firstComment.body)).toHaveLength(0);
  });

  it('renders the report control on every comment and forwards the comment id', () => {
    const onReportComment = vi.fn();
    const otherComment: Comment = {
      ...comment('comment-2', 'Comentário de outra pessoa'),
      author: { displayName: 'Lia', id: 'other-author-id' },
    };
    const renderer = renderSection({
      onReportComment,
      thread: readyThread({ comments: [firstComment, otherComment] }),
    });

    expect(
      renderer.root.findByProps({ testID: `comment-report-${firstComment.id}` })
        .props.accessibilityLabel,
    ).toBe('Denunciar comentário');
    expect(
      renderer.root.findByProps({
        testID: `comment-report-${otherComment.id}`,
      }),
    ).toBeDefined();

    act(() => {
      renderer.root
        .findByProps({ testID: `comment-report-${otherComment.id}` })
        .props.onPress();
    });
    expect(onReportComment).toHaveBeenCalledWith(otherComment.id);

    act(() => {
      renderer.root
        .findByProps({ testID: `comment-report-${firstComment.id}` })
        .props.onPress();
    });
    expect(onReportComment).toHaveBeenCalledWith(firstComment.id);
  });

  it('trims a valid draft before submitting and disables duplicate presses while pending', () => {
    const onSubmit = vi.fn();
    const renderer = render(<ComposerHarness onSubmit={onSubmit} />);
    const input = renderer.root.findByProps({ testID: 'comment-draft' });

    act(() => {
      input.props.onChangeText('  Comentário novo  ');
    });
    expect(textNodes(renderer, '15/1000')).toHaveLength(1);

    const submit = renderer.root.findByProps({ testID: 'comment-submit' });
    expect(submit.props.disabled).toBe(false);
    act(() => {
      submit.props.onPress();
    });

    expect(onSubmit).toHaveBeenCalledWith('Comentário novo');
    const pendingSubmit = renderer.root.findByProps({ testID: 'comment-submit' });
    expect(pendingSubmit.props.disabled).toBe(true);
    expect(pendingSubmit.props.label).toBe('Publicando...');
  });

  it('counts Unicode code points and blocks an overlong draft', () => {
    const renderer = render(<ComposerHarness onSubmit={vi.fn()} />);
    const input = renderer.root.findByProps({ testID: 'comment-draft' });

    act(() => {
      input.props.onChangeText('😀'.repeat(1001));
    });

    expect(textNodes(renderer, '1001/1000')).toHaveLength(1);
    expect(
      textNodes(renderer, 'Seu comentário pode ter até 1.000 caracteres.'),
    ).toHaveLength(1);
    expect(
      renderer.root.findByProps({ testID: 'comment-submit' }).props.disabled,
    ).toBe(true);

    act(() => {
      input.props.onChangeText(' \n ');
    });
    expect(
      textNodes(renderer, 'Escreva alguma coisa antes de comentar.'),
    ).toHaveLength(1);
  });
});

function ComposerHarness({ onSubmit }: { onSubmit: (body: string) => void }) {
  const [thread, dispatch] = useReducer(
    commentThreadReducer,
    undefined,
    createCommentThreadState,
  );

  return (
    <CommentsSection
      currentAuthorID="author-id"
      noteAuthorID={noteAuthorID}
      onDraftChange={(draft) => dispatch({ type: 'draft_changed', draft })}
      onLoadMore={() => undefined}
      onPressAuthor={() => undefined}
      onDeleteComment={() => undefined}
      onRetryInitial={() => undefined}
      onSubmit={(body) => {
        dispatch({ type: 'submit_started' });
        onSubmit(body);
      }}
      thread={{ ...thread, initialLoadStatus: 'ready' }}
    />
  );
}

function renderSection(
  overrides: Partial<React.ComponentProps<typeof CommentsSection>> = {},
): ReactTestRenderer {
  return render(
    <CommentsSection
      currentAuthorID="author-id"
      noteAuthorID={noteAuthorID}
      onDraftChange={() => undefined}
      onLoadMore={() => undefined}
      onPressAuthor={() => undefined}
      onDeleteComment={() => undefined}
      onRetryInitial={() => undefined}
      onSubmit={() => undefined}
      thread={readyThread()}
      {...overrides}
    />,
  );
}

function readyThread(
  overrides: Partial<CommentThreadState> = {},
): CommentThreadState {
  return {
    ...createCommentThreadState(),
    initialLoadStatus: 'ready',
    ...overrides,
  };
}

function comment(id: string, body: string): Comment {
  return {
    author: { displayName: 'Thiago', id: 'author-id' },
    body,
    createdAt: 1782993600000,
    id,
  };
}

function render(element: React.ReactElement): ReactTestRenderer {
  let renderer!: ReactTestRenderer;
  act(() => {
    renderer = create(element);
  });
  return renderer;
}

function textNodes(
  renderer: ReactTestRenderer,
  text: string,
): ReactTestInstance[] {
  return renderer.root.findAll(
    (node) => node.type === 'span' && textContent(node) === text,
  );
}

function textContent(node: ReactTestInstance): string {
  return node.children
    .map((child) =>
      typeof child === 'string' || typeof child === 'number'
        ? String(child)
        : textContent(child),
    )
    .join('');
}

function buttonLabels(renderer: ReactTestRenderer): string[] {
  return renderer.root
    .findAll((node) => node.type === 'button')
    .map(textContent);
}
