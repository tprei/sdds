import type {
  Comment,
  CommentPage,
  CommentThread,
} from '@/lib/api/comments';

export const commentBodyMaxCodePoints = 1000;
export const replyMaxPerParent = 20;

type InitialLoadStatus = 'loading' | 'ready' | 'error';
type LoadMoreStatus = 'idle' | 'pending' | 'error';
type SubmitStatus = 'idle' | 'pending' | 'error';
export type CommentDeleteStatus = 'pending' | 'error' | 'deleted';


type CommentDraftError = 'required' | 'too_long';

export type CommentDraftValidation = {
  body: string;
  codePointCount: number;
  error: CommentDraftError | null;
};

export type CommentThreadState = {
  threads: CommentThread[];
  deleteStatusByCommentID: ReadonlyMap<string, CommentDeleteStatus>;
  draft: string;
  draftTouched: boolean;
  initialLoadStatus: InitialLoadStatus;
  loadMoreStatus: LoadMoreStatus;
  localTailThreads: CommentThread[];
  nextCursor: string | null;
  replyTarget: { commentID: string; authorDisplayName: string } | null;
  replyDraft: string;
  replyDraftTouched: boolean;
  replySubmitStatus: 'idle' | 'pending' | 'error';
  submitStatus: SubmitStatus;
};

export type CommentThreadAction =
  | { type: 'reset' }
  | { type: 'initial_load_started' }
  | { type: 'initial_load_succeeded'; page: CommentPage }
  | { type: 'initial_load_failed' }
  | { type: 'load_more_started' }
  | { type: 'load_more_succeeded'; page: CommentPage }
  | { type: 'load_more_failed' }
  | { type: 'draft_changed'; draft: string }
  | { type: 'submit_started' }
  | { type: 'submit_succeeded'; comment: Comment }
  | { type: 'submit_failed' }
  | { type: 'reply_started'; commentID: string; authorDisplayName: string }
  | { type: 'reply_cancelled' }
  | { type: 'reply_draft_changed'; draft: string }
  | { type: 'reply_submit_started' }
  | {
      type: 'reply_submit_succeeded';
      parentCommentID: string;
      comment: Comment;
    }
  | { type: 'reply_submit_failed' }
  | { type: 'delete_started'; commentID: string }
  | { type: 'delete_succeeded'; commentID: string }
  | { type: 'delete_not_found'; commentID: string }
  | { type: 'delete_failed'; commentID: string };

export function createCommentThreadState(): CommentThreadState {
  return {
    threads: [],
    deleteStatusByCommentID: new Map(),
    draft: '',
    draftTouched: false,
    initialLoadStatus: 'loading',
    loadMoreStatus: 'idle',
    localTailThreads: [],
    nextCursor: null,
    replyTarget: null,
    replyDraft: '',
    replyDraftTouched: false,
    replySubmitStatus: 'idle',
    submitStatus: 'idle',
  };
}

export function commentThreadReducer(
  state: CommentThreadState,
  action: CommentThreadAction,
): CommentThreadState {
  switch (action.type) {
    case 'reset':
      return createCommentThreadState();

    case 'initial_load_started':
      if (state.initialLoadStatus === 'loading') {
        return state;
      }
      return {
        ...state,
        initialLoadStatus: 'loading',
        loadMoreStatus: 'idle',
      };

    case 'initial_load_succeeded': {
      if (state.initialLoadStatus !== 'loading') {
        return state;
      }
      const threads = uniqueThreads(action.page.threads);
      const settledTail = settleLocalTail(
        threads,
        withoutThreadIDs(state.localTailThreads, threads),
        action.page.nextCursor,
      );
      return {
        ...state,
        threads: settledTail.threads,
        initialLoadStatus: 'ready',
        loadMoreStatus: 'idle',
        localTailThreads: settledTail.localTailThreads,
        nextCursor: action.page.nextCursor,
      };
    }

    case 'initial_load_failed':
      if (state.initialLoadStatus !== 'loading') {
        return state;
      }
      return { ...state, initialLoadStatus: 'error' };

    case 'load_more_started':
      if (state.initialLoadStatus !== 'ready' || state.nextCursor === null) {
        return state;
      }
      if (state.loadMoreStatus === 'pending') {
        return state;
      }
      return { ...state, loadMoreStatus: 'pending' };

    case 'load_more_succeeded': {
      if (
        state.initialLoadStatus !== 'ready' ||
        state.loadMoreStatus !== 'pending'
      ) {
        return state;
      }
      const threads = appendUniqueThreads(state.threads, action.page.threads);
      const settledTail = settleLocalTail(
        threads,
        withoutThreadIDs(state.localTailThreads, threads),
        action.page.nextCursor,
      );
      return {
        ...state,
        threads: settledTail.threads,
        loadMoreStatus: 'idle',
        localTailThreads: settledTail.localTailThreads,
        nextCursor: action.page.nextCursor,
      };
    }

    case 'load_more_failed':
      if (state.loadMoreStatus !== 'pending') {
        return state;
      }
      return { ...state, loadMoreStatus: 'error' };

    case 'draft_changed':
      return {
        ...state,
        draft: action.draft,
        draftTouched: true,
        submitStatus: state.submitStatus === 'error' ? 'idle' : state.submitStatus,
      };

    case 'submit_started':
      if (!canSubmitComment(state)) {
        return state;
      }
      return { ...state, submitStatus: 'pending' };

    case 'submit_succeeded': {
      if (state.submitStatus !== 'pending') {
        return state;
      }
      const appendToLoaded =
        state.initialLoadStatus === 'ready' && state.nextCursor === null;
      const thread: CommentThread = {
        comment: action.comment,
        replies: [],
        hasMoreReplies: false,
      };
      return {
        ...state,
        threads: appendToLoaded
          ? appendUniqueThreads(state.threads, [thread])
          : state.threads,
        draft: '',
        draftTouched: false,
        localTailThreads: appendToLoaded
          ? state.localTailThreads
          : appendUniqueTailThread(
              state.threads,
              state.localTailThreads,
              thread,
            ),
        submitStatus: 'idle',
      };
    }

    case 'submit_failed':
      if (state.submitStatus !== 'pending') {
        return state;
      }
      return { ...state, submitStatus: 'error' };

    case 'reply_started':
      if (state.replySubmitStatus === 'pending') {
        return state;
      }
      return {
        ...state,
        replyTarget: {
          commentID: action.commentID,
          authorDisplayName: action.authorDisplayName,
        },
        replyDraft: '',
        replyDraftTouched: false,
        replySubmitStatus: 'idle',
      };

    case 'reply_cancelled':
      if (state.replySubmitStatus === 'pending') {
        return state;
      }
      return {
        ...state,
        replyTarget: null,
        replyDraft: '',
        replyDraftTouched: false,
        replySubmitStatus: 'idle',
      };

    case 'reply_draft_changed':
      return {
        ...state,
        replyDraft: action.draft,
        replyDraftTouched: true,
        replySubmitStatus:
          state.replySubmitStatus === 'error'
            ? 'idle'
            : state.replySubmitStatus,
      };

    case 'reply_submit_started':
      if (!canSubmitReply(state)) {
        return state;
      }
      return { ...state, replySubmitStatus: 'pending' };

    case 'reply_submit_succeeded': {
      if (state.replySubmitStatus !== 'pending') {
        return state;
      }
      const loaded = appendReplyToThreads(
        state.threads,
        action.parentCommentID,
        action.comment,
      );
      const localTail = appendReplyToThreads(
        state.localTailThreads,
        action.parentCommentID,
        action.comment,
      );
      return {
        ...state,
        threads: loaded.threads,
        localTailThreads: localTail.threads,
        replyTarget: null,
        replyDraft: '',
        replyDraftTouched: false,
        replySubmitStatus: 'idle',
      };
    }

    case 'reply_submit_failed':
      if (state.replySubmitStatus !== 'pending') {
        return state;
      }
      return { ...state, replySubmitStatus: 'error' };
 

    case 'delete_started':
      if (!canDeleteComment(state, action.commentID)) {
        return state;
      }
      return {
        ...state,
        deleteStatusByCommentID: setDeleteStatus(
          state.deleteStatusByCommentID,
          action.commentID,
          'pending',
        ),
      };

    case 'delete_succeeded':
    case 'delete_not_found':
      if (state.deleteStatusByCommentID.get(action.commentID) !== 'pending') {
        return state;
      }
      return {
        ...state,
        deleteStatusByCommentID: setDeleteStatus(
          state.deleteStatusByCommentID,
          action.commentID,
          'deleted',
        ),
      };

    case 'delete_failed':
      if (state.deleteStatusByCommentID.get(action.commentID) !== 'pending') {
        return state;
      }
      return {
        ...state,
        deleteStatusByCommentID: setDeleteStatus(
          state.deleteStatusByCommentID,
          action.commentID,
          'error',
        ),
      };
  }
}

export function validateCommentDraft(draft: string): CommentDraftValidation {
  const body = draft.trim();
  const codePointCount = Array.from(body).length;
  if (codePointCount === 0) {
    return { body, codePointCount, error: 'required' };
  }
  if (codePointCount > commentBodyMaxCodePoints) {
    return { body, codePointCount, error: 'too_long' };
  }
  return { body, codePointCount, error: null };
}

export function canSubmitComment(state: CommentThreadState): boolean {
  return (
    state.submitStatus !== 'pending' &&
    validateCommentDraft(state.draft).error === null
  );
}

export function canSubmitReply(state: CommentThreadState): boolean {
  return (
    state.replyTarget !== null &&
    state.replySubmitStatus !== 'pending' &&
    validateCommentDraft(state.replyDraft).error === null
  );
}

export function displayedCommentCount(state: CommentThreadState): number {
  return (
    visibleThreadCount(state.threads, state.deleteStatusByCommentID) +
    visibleThreadCount(state.localTailThreads, state.deleteStatusByCommentID)
  );
}

export function canDeleteComment(
  state: CommentThreadState,
  commentID: string,
): boolean {
  const status = state.deleteStatusByCommentID.get(commentID);
  return (
    status !== 'pending' &&
    status !== 'deleted' &&
    (hasCommentID(state.threads, commentID) ||
      hasCommentID(state.localTailThreads, commentID))
  );
}

export function visibleComments(
  comments: Comment[],
  deleteStatusByCommentID: ReadonlyMap<string, CommentDeleteStatus>,
): Comment[] {
  if (deleteStatusByCommentID.size === 0) {
    return comments;
  }
  return comments.filter((comment) =>
    isCommentVisible(deleteStatusByCommentID.get(comment.id)),
  );
}

export function visibleThreads(
  threads: CommentThread[],
  deleteStatusByCommentID: ReadonlyMap<string, CommentDeleteStatus>,
): CommentThread[] {
  if (deleteStatusByCommentID.size === 0) {
    return threads;
  }
  return threads.filter((thread) =>
    isCommentVisible(deleteStatusByCommentID.get(thread.comment.id)),
  );
}

function visibleThreadCount(
  threads: CommentThread[],
  deleteStatusByCommentID: ReadonlyMap<string, CommentDeleteStatus>,
): number {
  return threads.reduce((total, thread) => {
    if (!isCommentVisible(deleteStatusByCommentID.get(thread.comment.id))) {
      return total;
    }
    return (
      total +
      1 +
      visibleCommentCount(thread.replies, deleteStatusByCommentID)
    );
  }, 0);
}

function visibleCommentCount(
  comments: Comment[],
  deleteStatusByCommentID: ReadonlyMap<string, CommentDeleteStatus>,
): number {
  if (deleteStatusByCommentID.size === 0) {
    return comments.length;
  }
  return comments.reduce(
    (count, comment) =>
      isCommentVisible(deleteStatusByCommentID.get(comment.id))
        ? count + 1
        : count,
    0,
  );
}

function isCommentVisible(status: CommentDeleteStatus | undefined): boolean {
  return status !== 'pending' && status !== 'deleted';
}

function hasCommentID(threads: CommentThread[], commentID: string): boolean {
  return threads.some(
    (thread) =>
      thread.comment.id === commentID ||
      thread.replies.some((reply) => reply.id === commentID),
  );
}

function setDeleteStatus(
  deleteStatusByCommentID: ReadonlyMap<string, CommentDeleteStatus>,
  commentID: string,
  status: CommentDeleteStatus,
): ReadonlyMap<string, CommentDeleteStatus> {
  if (deleteStatusByCommentID.get(commentID) === status) {
    return deleteStatusByCommentID;
  }
  const nextDeleteStatusByCommentID = new Map(deleteStatusByCommentID);
  nextDeleteStatusByCommentID.set(commentID, status);
  return nextDeleteStatusByCommentID;
}

function uniqueThreads(threads: CommentThread[]): CommentThread[] {
  return appendUniqueThreads([], threads);
}

function appendUniqueTailThread(
  threads: CommentThread[],
  localTailThreads: CommentThread[],
  thread: CommentThread,
): CommentThread[] {
  if (threads.some((existing) => existing.comment.id === thread.comment.id)) {
    return localTailThreads;
  }
  return appendUniqueThreads(localTailThreads, [thread]);
}

function settleLocalTail(
  threads: CommentThread[],
  localTailThreads: CommentThread[],
  nextCursor: string | null,
): { threads: CommentThread[]; localTailThreads: CommentThread[] } {
  if (nextCursor !== null || localTailThreads.length === 0) {
    return { threads, localTailThreads };
  }
  return {
    threads: appendUniqueThreads(threads, localTailThreads),
    localTailThreads: [],
  };
}

function appendUniqueThreads(
  threads: CommentThread[],
  additions: CommentThread[],
): CommentThread[] {
  const knownIDs = new Set(threads.map((thread) => thread.comment.id));
  const uniqueAdditions = additions.filter((thread) => {
    if (knownIDs.has(thread.comment.id)) {
      return false;
    }
    knownIDs.add(thread.comment.id);
    return true;
  });
  return uniqueAdditions.length === 0 ? threads : [...threads, ...uniqueAdditions];
}

function appendReplyToThreads(
  threads: CommentThread[],
  parentCommentID: string,
  reply: Comment,
): { threads: CommentThread[]; found: boolean } {
  let found = false;
  let changed = false;
  const nextThreads = threads.map((thread) => {
    if (thread.comment.id !== parentCommentID) {
      return thread;
    }
    found = true;
    if (thread.replies.some((existing) => existing.id === reply.id)) {
      return thread;
    }
    changed = true;
    return { ...thread, replies: [...thread.replies, reply] };
  });
  return { threads: changed ? nextThreads : threads, found };
}

function withoutThreadIDs(
  threads: CommentThread[],
  knownThreads: CommentThread[],
): CommentThread[] {
  if (threads.length === 0 || knownThreads.length === 0) {
    return threads;
  }
  const knownIDs = new Set(knownThreads.map((thread) => thread.comment.id));
  const remainingThreads = threads.filter(
    (thread) => !knownIDs.has(thread.comment.id),
  );
  return remainingThreads.length === threads.length ? threads : remainingThreads;
}
