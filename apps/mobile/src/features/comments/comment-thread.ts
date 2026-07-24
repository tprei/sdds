import type { Comment, CommentPage } from '@/lib/api/comments';

export const commentBodyMaxCodePoints = 1000;

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
  comments: Comment[];
  deleteStatusByCommentID: ReadonlyMap<string, CommentDeleteStatus>;
  draft: string;
  draftTouched: boolean;
  initialLoadStatus: InitialLoadStatus;
  loadMoreStatus: LoadMoreStatus;
  localTailComments: Comment[];
  nextCursor: string | null;
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
  | { type: 'delete_started'; commentID: string }
  | { type: 'delete_succeeded'; commentID: string }
  | { type: 'delete_not_found'; commentID: string }
  | { type: 'delete_failed'; commentID: string };

export function createCommentThreadState(): CommentThreadState {
  return {
    comments: [],
    deleteStatusByCommentID: new Map(),
    draft: '',
    draftTouched: false,
    initialLoadStatus: 'loading',
    loadMoreStatus: 'idle',
    localTailComments: [],
    nextCursor: null,
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
      const comments = uniqueComments(action.page.comments);
      const settledTail = settleLocalTail(
        comments,
        withoutCommentIDs(state.localTailComments, comments),
        action.page.nextCursor,
      );
      return {
        ...state,
        comments: settledTail.comments,
        initialLoadStatus: 'ready',
        loadMoreStatus: 'idle',
        localTailComments: settledTail.localTailComments,
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
      const comments = appendUniqueComments(state.comments, action.page.comments);
      const settledTail = settleLocalTail(
        comments,
        withoutCommentIDs(state.localTailComments, comments),
        action.page.nextCursor,
      );
      return {
        ...state,
        comments: settledTail.comments,
        loadMoreStatus: 'idle',
        localTailComments: settledTail.localTailComments,
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
      const appendToLoadedComments =
        state.initialLoadStatus === 'ready' && state.nextCursor === null;
      return {
        ...state,
        comments: appendToLoadedComments
          ? appendUniqueComments(state.comments, [action.comment])
          : state.comments,
        draft: '',
        draftTouched: false,
        localTailComments: appendToLoadedComments
          ? state.localTailComments
          : appendUniqueTailComment(
              state.comments,
              state.localTailComments,
              action.comment,
            ),
        submitStatus: 'idle',
      };
    }

    case 'submit_failed':
      if (state.submitStatus !== 'pending') {
        return state;
      }
      return { ...state, submitStatus: 'error' };

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

export function displayedCommentCount(state: CommentThreadState): number {
  return (
    visibleCommentCount(state.comments, state.deleteStatusByCommentID) +
    visibleCommentCount(state.localTailComments, state.deleteStatusByCommentID)
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
    (state.comments.some((comment) => comment.id === commentID) ||
      state.localTailComments.some((comment) => comment.id === commentID))
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

function uniqueComments(comments: Comment[]): Comment[] {
  return appendUniqueComments([], comments);
}

function appendUniqueTailComment(
  comments: Comment[],
  localTailComments: Comment[],
  comment: Comment,
): Comment[] {
  if (comments.some((existing) => existing.id === comment.id)) {
    return localTailComments;
  }
  return appendUniqueComments(localTailComments, [comment]);
}

function settleLocalTail(
  comments: Comment[],
  localTailComments: Comment[],
  nextCursor: string | null,
): { comments: Comment[]; localTailComments: Comment[] } {
  if (nextCursor !== null || localTailComments.length === 0) {
    return { comments, localTailComments };
  }
  return {
    comments: appendUniqueComments(comments, localTailComments),
    localTailComments: [],
  };
}

function appendUniqueComments(
  comments: Comment[],
  additions: Comment[],
): Comment[] {
  const knownIDs = new Set(comments.map((comment) => comment.id));
  const uniqueAdditions = additions.filter((comment) => {
    if (knownIDs.has(comment.id)) {
      return false;
    }
    knownIDs.add(comment.id);
    return true;
  });
  return uniqueAdditions.length === 0 ? comments : [...comments, ...uniqueAdditions];
}

function withoutCommentIDs(comments: Comment[], knownComments: Comment[]): Comment[] {
  if (comments.length === 0 || knownComments.length === 0) {
    return comments;
  }
  const knownIDs = new Set(knownComments.map((comment) => comment.id));
  const remainingComments = comments.filter(
    (comment) => !knownIDs.has(comment.id),
  );
  return remainingComments.length === comments.length ? comments : remainingComments;
}
