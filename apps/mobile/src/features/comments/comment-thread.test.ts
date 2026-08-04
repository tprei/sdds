import { describe, expect, it } from 'vitest';

import type { Comment, CommentPage, CommentThread } from '@/lib/api/comments';

import {
  canDeleteComment,
  canSubmitReply,
  canSubmitComment,
  commentBodyMaxCodePoints,
  commentThreadReducer,
  displayedCommentCount,
  createCommentThreadState,
  validateCommentDraft,
  visibleThreads,
} from './comment-thread';

const firstComment = comment('comment-1', 'Primeiro comentário');
const secondComment = comment('comment-2', 'Segundo comentário');
const thirdComment = comment('comment-3', 'Terceiro comentário');
const submittedComment = comment('comment-new', 'Comentário novo');
const laterSubmittedComment = comment('comment-later', 'Comentário mais novo');

describe('comment thread reducer', () => {
  it('keeps a submitted comment in a local tail until the paginated gap closes', () => {
    let state = reduce(createCommentThreadState(), {
      type: 'initial_load_succeeded',
      page: page([firstComment, secondComment], 'cursor-2'),
    });
    state = reduce(
      state,
      { type: 'draft_changed', draft: submittedComment.body },
      { type: 'submit_started' },
      { type: 'submit_succeeded', comment: submittedComment },
    );

    expect(state.threads).toEqual([thread(firstComment), thread(secondComment)]);
    expect(state.localTailThreads).toEqual([thread(submittedComment)]);
    expect(state.nextCursor).toBe('cursor-2');

    state = reduce(
      state,
      { type: 'load_more_started' },
      {
        type: 'load_more_succeeded',
        page: page([thirdComment, submittedComment]),
      },
    );

    expect(state.threads).toEqual([
      thread(firstComment),
      thread(secondComment),
      thread(thirdComment),
      thread(submittedComment),
    ]);
    expect(state.localTailThreads).toEqual([]);
  });

  it('promotes an unmatched terminal tail before later submitted comments', () => {
    let state = reduce(createCommentThreadState(), {
      type: 'initial_load_succeeded',
      page: page([firstComment], 'cursor-1'),
    });
    state = reduce(
      state,
      { type: 'draft_changed', draft: submittedComment.body },
      { type: 'submit_started' },
      { type: 'submit_succeeded', comment: submittedComment },
      { type: 'load_more_started' },
      {
        type: 'load_more_succeeded',
        page: page([secondComment]),
      },
      { type: 'draft_changed', draft: laterSubmittedComment.body },
      { type: 'submit_started' },
      { type: 'submit_succeeded', comment: laterSubmittedComment },
    );

    expect(state.threads).toEqual([
      thread(firstComment),
      thread(secondComment),
      thread(submittedComment),
      thread(laterSubmittedComment),
    ]);
    expect(state.localTailThreads).toEqual([]);
  });

  it('appends a submitted comment directly after a terminal page', () => {
    let state = reduce(createCommentThreadState(), {
      type: 'initial_load_succeeded',
      page: page([firstComment]),
    });
    state = reduce(
      state,
      { type: 'draft_changed', draft: submittedComment.body },
      { type: 'submit_started' },
      { type: 'submit_succeeded', comment: submittedComment },
    );

    expect(state.threads).toEqual([thread(firstComment), thread(submittedComment)]);
    expect(state.localTailThreads).toEqual([]);
    expect(state.draft).toBe('');
    expect(state.submitStatus).toBe('idle');
  });

  it('deduplicates repeated comments returned by later pages', () => {
    let state = reduce(createCommentThreadState(), {
      type: 'initial_load_succeeded',
      page: page([firstComment, secondComment], 'cursor-2'),
    });
    state = reduce(
      state,
      { type: 'load_more_started' },
      {
        type: 'load_more_succeeded',
        page: page([secondComment, thirdComment]),
      },
    );

    expect(state.threads).toEqual([thread(firstComment), thread(secondComment), thread(thirdComment)]);
  });

  it('does not duplicate a submitted comment already in the loaded page', () => {
    let state = reduce(createCommentThreadState(), {
      type: 'initial_load_succeeded',
      page: page([firstComment], 'cursor-1'),
    });
    state = reduce(
      state,
      { type: 'draft_changed', draft: firstComment.body },
      { type: 'submit_started' },
      { type: 'submit_succeeded', comment: firstComment },
    );

    expect(state.threads).toEqual([thread(firstComment)]);
    expect(state.localTailThreads).toEqual([]);
  });

  it('prevents duplicate submission while pending and preserves the draft after failure', () => {
    let state = reduce(createCommentThreadState(), {
      type: 'initial_load_succeeded',
      page: page([firstComment]),
    });
    state = reduce(
      state,
      { type: 'draft_changed', draft: submittedComment.body },
      { type: 'submit_started' },
    );
    const pending = state;

    state = reduce(state, { type: 'submit_started' });
    expect(state).toBe(pending);

    state = reduce(state, { type: 'submit_failed' });
    expect(state.submitStatus).toBe('error');
    expect(state.draft).toBe(submittedComment.body);
    expect(state.threads).toEqual([thread(firstComment)]);

    state = reduce(state, { type: 'draft_changed', draft: 'Outra tentativa' });
    expect(state.submitStatus).toBe('idle');
    expect(state.draft).toBe('Outra tentativa');
  });

  it('only starts pagination once and only while another page exists', () => {
    let state = reduce(createCommentThreadState(), {
      type: 'initial_load_succeeded',
      page: page([firstComment], 'cursor-1'),
    });
    state = reduce(state, { type: 'load_more_started' });
    const pending = state;

    expect(state.loadMoreStatus).toBe('pending');
    expect(reduce(state, { type: 'load_more_started' })).toBe(pending);

    state = reduce(state, { type: 'load_more_succeeded', page: page([]) });
    expect(state.loadMoreStatus).toBe('idle');
    expect(reduce(state, { type: 'load_more_started' })).toBe(state);
  });

  it('does not replace an already-pending initial load', () => {
    const pending = createCommentThreadState();

    expect(reduce(pending, { type: 'initial_load_started' })).toBe(pending);
  });

  it('ignores a stale page completion when initial loading restarts', () => {
    let state = reduce(createCommentThreadState(), {
      type: 'initial_load_succeeded',
      page: page([firstComment], 'cursor-1'),
    });
    state = reduce(
      state,
      { type: 'load_more_started' },
      { type: 'initial_load_started' },
    );
    const restarting = state;

    state = reduce(state, {
      type: 'load_more_succeeded',
      page: page([secondComment]),
    });

    expect(state).toBe(restarting);
  });

  it('optimistically hides a comment and restores it after a failed deletion', () => {
    let state = reduce(createCommentThreadState(), {
      type: 'initial_load_succeeded',
      page: page([firstComment, secondComment]),
    });
    state = reduce(state, {
      type: 'delete_started',
      commentID: firstComment.id,
    });
    const pending = state;

    expect(
      visibleThreads(state.threads, state.deleteStatusByCommentID).map(
        (entry) => entry.comment,
      ),
    ).toEqual([secondComment]);
    expect(displayedCommentCount(state)).toBe(1);
    expect(canDeleteComment(state, firstComment.id)).toBe(false);
    expect(
      reduce(state, { type: 'delete_started', commentID: firstComment.id }),
    ).toBe(pending);

    state = reduce(state, {
      type: 'delete_failed',
      commentID: firstComment.id,
    });
    expect(
      visibleThreads(state.threads, state.deleteStatusByCommentID).map(
        (entry) => entry.comment,
      ),
    ).toEqual([firstComment, secondComment]);
    expect(state.deleteStatusByCommentID.get(firstComment.id)).toBe('error');
    expect(canDeleteComment(state, firstComment.id)).toBe(true);
  });

  it('counts replies and allows deleting a visible reply', () => {
    const reply = replyComment('reply-1', 'Uma resposta');
    const state = reduce(createCommentThreadState(), {
      type: 'initial_load_succeeded',
      page: {
        threads: [
          {
            comment: firstComment,
            replies: [reply],
            hasMoreReplies: false,
          },
        ],
        nextCursor: null,
      },
    });

    expect(displayedCommentCount(state)).toBe(2);
    expect(canDeleteComment(state, reply.id)).toBe(true);
  });

  it('opens, guards, retries, and settles a reply submission', () => {
    let state = reduce(createCommentThreadState(), {
      type: 'initial_load_succeeded',
      page: page([firstComment]),
    });

    state = reduce(state, {
      type: 'reply_started',
      commentID: firstComment.id,
      authorDisplayName: firstComment.author.displayName,
    });
    expect(state.replyTarget).toEqual({
      commentID: firstComment.id,
      authorDisplayName: firstComment.author.displayName,
    });
    expect(canSubmitReply(state)).toBe(false);

    state = reduce(state, {
      type: 'reply_draft_changed',
      draft: '  Resposta nova  ',
    });
    expect(canSubmitReply(state)).toBe(true);
    state = reduce(state, { type: 'reply_submit_started' });
    const pending = state;
    expect(state.replySubmitStatus).toBe('pending');
    expect(reduce(state, { type: 'reply_started', commentID: secondComment.id, authorDisplayName: 'Lia' })).toBe(pending);
    expect(reduce(state, { type: 'reply_cancelled' })).toBe(pending);

    state = reduce(state, { type: 'reply_submit_failed' });
    expect(state.replySubmitStatus).toBe('error');
    expect(state.replyDraft).toBe('  Resposta nova  ');
    state = reduce(state, { type: 'reply_draft_changed', draft: 'Resposta de novo' });
    expect(state.replySubmitStatus).toBe('idle');

    state = reduce(state, { type: 'reply_submit_started' });
    const reply = replyComment('reply-new', 'Resposta de novo');
    state = reduce(state, {
      type: 'reply_submit_succeeded',
      parentCommentID: firstComment.id,
      comment: reply,
    });
    expect(state.threads).toEqual([
      { comment: firstComment, replies: [reply], hasMoreReplies: false },
    ]);
    expect(state.replyTarget).toBeNull();
    expect(state.replyDraft).toBe('');
    expect(state.replySubmitStatus).toBe('idle');
  });

  it('keeps a deleted tail comment hidden after terminal settlement and later pages', () => {
    let state = reduce(createCommentThreadState(), {
      type: 'initial_load_succeeded',
      page: page([firstComment], 'cursor-1'),
    });
    state = reduce(
      state,
      { type: 'draft_changed', draft: submittedComment.body },
      { type: 'submit_started' },
      { type: 'submit_succeeded', comment: submittedComment },
      { type: 'delete_started', commentID: submittedComment.id },
      { type: 'load_more_started' },
      { type: 'load_more_succeeded', page: page([secondComment]) },
      { type: 'delete_not_found', commentID: submittedComment.id },
      { type: 'initial_load_started' },
      {
        type: 'initial_load_succeeded',
        page: page([firstComment, secondComment, submittedComment]),
      },
    );

    expect(state.threads).toEqual([
      thread(firstComment),
      thread(secondComment),
      thread(submittedComment),
    ]);
    expect(state.localTailThreads).toEqual([]);
    expect(
      visibleThreads(state.threads, state.deleteStatusByCommentID).map(
        (entry) => entry.comment,
      ),
    ).toEqual([firstComment, secondComment]);
    expect(state.deleteStatusByCommentID.get(submittedComment.id)).toBe(
      'deleted',
    );
  });

  it('tracks concurrent delete outcomes independently', () => {
    let state = reduce(createCommentThreadState(), {
      type: 'initial_load_succeeded',
      page: page([firstComment, secondComment]),
    });
    state = reduce(
      state,
      { type: 'delete_started', commentID: firstComment.id },
      { type: 'delete_started', commentID: secondComment.id },
      { type: 'delete_succeeded', commentID: firstComment.id },
      { type: 'delete_failed', commentID: secondComment.id },
    );

    expect(state.deleteStatusByCommentID.get(firstComment.id)).toBe('deleted');
    expect(state.deleteStatusByCommentID.get(secondComment.id)).toBe('error');
    expect(
      visibleThreads(state.threads, state.deleteStatusByCommentID).map(
        (entry) => entry.comment,
      ),
    ).toEqual([secondComment]);
  });

  it('trims drafts and counts Unicode code points for validation', () => {
    expect(validateCommentDraft(' \n\t ')).toEqual({
      body: '',
      codePointCount: 0,
      error: 'required',
    });
    expect(validateCommentDraft(` ${'😀'.repeat(1000)} `)).toEqual({
      body: '😀'.repeat(1000),
      codePointCount: commentBodyMaxCodePoints,
      error: null,
    });
    expect(validateCommentDraft('😀'.repeat(1001))).toEqual({
      body: '😀'.repeat(1001),
      codePointCount: commentBodyMaxCodePoints + 1,
      error: 'too_long',
    });

    const blankState = reduce(createCommentThreadState(), {
      type: 'draft_changed',
      draft: ' \n\t ',
    });

    expect(canSubmitComment(blankState)).toBe(false);
    expect(reduce(blankState, { type: 'submit_started' })).toBe(blankState);

    const state = reduce(createCommentThreadState(), {
      type: 'draft_changed',
      draft: '😀'.repeat(1001),
    });
    expect(canSubmitComment(state)).toBe(false);
  });
});

function comment(id: string, body: string): Comment {
  return {
    author: { displayName: 'Thiago', id: 'author-id' },
    body,
    createdAt: 1782993600000,
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

function page(comments: Comment[], nextCursor: string | null = null): CommentPage {
  return { threads: comments.map(thread), nextCursor };
}

function reduce(
  state: ReturnType<typeof createCommentThreadState>,
  ...actions: Parameters<typeof commentThreadReducer>[1][]
): ReturnType<typeof createCommentThreadState> {
  return actions.reduce(commentThreadReducer, state);
}
