import {
  type ReactNode,
  useCallback,
  useReducer,
  useRef,
  useState,
} from 'react';
import { StyleSheet, Text } from 'react-native';
import { semanticColors, typography } from '@sdds/tokens';
import { useFocusEffect, useLocalSearchParams, useRouter } from 'expo-router';

import {
  EmptyStateCard,
  FoundationButton,
  FoundationScreen,
} from '@/components/foundation-screen';
import { CommentsSection } from '@/features/comments/comments-section';
import {
  canDeleteComment,
  canSubmitComment,
  commentThreadReducer,
  createCommentThreadState,
  validateCommentDraft,
} from '@/features/comments/comment-thread';
import { ReportDialog } from '@/features/reports/report-dialog';
import {
  canSubmitReport,
  createReportFormState,
  reportFormReducer,
} from '@/features/reports/report-form';
import { NoteDetailContent } from '@/features/notes/note-detail-content';
import { buildNoteCatalog, labelNote } from '@/features/notes/catalog';
import type { LabelledNote } from '@/features/notes/catalog';
import { APIRequestError } from '@/lib/api/notes';
import { requestStatus } from '@/lib/api/request-error';
import { unauthorizedStatus } from '@/lib/api/status';
import { useAuth } from '@/lib/auth/auth-provider';
import type { APIClient } from '@/lib/api/client';
import { useUsefulMutation } from '@/features/notes/use-useful-mutation';
import { ReadAuthGate } from '@/components/read-auth-gate';

type NoteDetailState =
  | { status: 'loading' }
  | { status: 'ready'; note: LabelledNote }
  | { status: 'notFound' }
  | { status: 'error' };

type AuthenticatedNoteDetailScreenProps = {
  apiClient: APIClient;
  currentAuthorID: string;
  noteID: string;
  onSessionExpired: () => Promise<void>;
};


const notFoundStatus = 404;

export default function NoteDetailScreen() {
  const router = useRouter();
  const { apiClient, logout, state } = useAuth();
  const { id } = useLocalSearchParams<{ id?: string | string[] }>();
  const noteID = typeof id === 'string' ? id : undefined;

  if (!noteID?.trim()) {
    return (
      <FoundationScreen
        description="Leia a nota completa, com lugar, categoria e data."
        eyebrow="Nota"
        title="Nota"
      >
        <EmptyStateCard
          title="Nota não encontrada"
          body="Essa nota não existe mais ou o link tá incompleto."
        />
        <FoundationButton label="Voltar" onPress={() => router.back()} />
      </FoundationScreen>
    );
  }

  if (state.status === 'authenticated') {
    return (
      <AuthenticatedNoteDetailScreen
        key={`${state.user.id}:${noteID}`}
        apiClient={apiClient}
        currentAuthorID={state.user.author.id}
        noteID={noteID}
        onSessionExpired={logout}
      />
    );
  }

  return (
    <FoundationScreen
      description="Leia a nota completa, com lugar, categoria e data."
      eyebrow="Nota"
      title="Nota"
    >
      <ReadAuthGate
        onLogin={() =>
          router.push({ pathname: '/login', params: { next: `/notes/${noteID}` } })
        }
        onSignup={() =>
          router.push({
            pathname: '/signup',
            params: { next: `/notes/${noteID}` },
          })
        }
        status={state.status}
      />
      <FoundationButton label="Voltar" onPress={() => router.back()} />
    </FoundationScreen>
  );
}

function AuthenticatedNoteDetailScreen({
  apiClient,
  currentAuthorID,
  noteID,
  onSessionExpired,
}: AuthenticatedNoteDetailScreenProps) {
  const router = useRouter();
  const detailGenerationRef = useRef(0);
  const detailActiveRef = useRef(false);
  const commentRequestIDRef = useRef(0);
  const commentListRequestRef = useRef<number | null>(null);
  const commentCreateRequestRef = useRef<number | null>(null);
  const commentDeleteRequestRefs = useRef(new Map<string, number>());
  const [state, setState] = useState<NoteDetailState>({ status: 'loading' });
  const [commentThread, dispatchCommentThread] = useReducer(
    commentThreadReducer,
    undefined,
    createCommentThreadState,
  );
  const reportRequestIDRef = useRef(0);
  const [reportForm, dispatchReportForm] = useReducer(
    reportFormReducer,
    createReportFormState(),
  );

  const handleCommentSessionExpired = useCallback(async () => {
    const expiredGeneration = ++detailGenerationRef.current;
    try {
      await onSessionExpired();
    } catch {
      if (detailGenerationRef.current === expiredGeneration) {
        setState({ status: 'error' });
      }
    }
  }, [onSessionExpired]);

  const loadCommentPage = useCallback(
    (cursor?: string) => {
      if (commentListRequestRef.current !== null) {
        return;
      }

      const generation = detailGenerationRef.current;
      const requestID = ++commentRequestIDRef.current;
      commentListRequestRef.current = requestID;
      dispatchCommentThread({
        type:
          cursor === undefined ? 'initial_load_started' : 'load_more_started',
      });

      void apiClient
        .listNoteComments({
          noteID,
          ...(cursor === undefined ? {} : { cursor }),
        })
        .then((page) => {
          if (
            detailGenerationRef.current !== generation ||
            commentListRequestRef.current !== requestID
          ) {
            return;
          }
          dispatchCommentThread(
            cursor === undefined
              ? { type: 'initial_load_succeeded', page }
              : { type: 'load_more_succeeded', page },
          );
        })
        .catch((error: unknown) => {
          if (
            detailGenerationRef.current !== generation ||
            commentListRequestRef.current !== requestID
          ) {
            return;
          }

          const status = requestStatus(error);
          if (status === unauthorizedStatus) {
            void handleCommentSessionExpired();
            return;
          }
          if (status === notFoundStatus) {
            setState({ status: 'notFound' });
            return;
          }
          dispatchCommentThread({
            type:
              cursor === undefined
                ? 'initial_load_failed'
                : 'load_more_failed',
          });
        })
        .finally(() => {
          if (commentListRequestRef.current === requestID) {
            commentListRequestRef.current = null;
          }
        });
    },
    [apiClient, handleCommentSessionExpired, noteID],
  );

  const handleLoadMoreComments = useCallback(() => {
    const cursor = commentThread.nextCursor;
    if (
      commentThread.initialLoadStatus !== 'ready' ||
      commentThread.loadMoreStatus === 'pending' ||
      cursor === null
    ) {
      return;
    }
    loadCommentPage(cursor);
  }, [
    commentThread.initialLoadStatus,
    commentThread.loadMoreStatus,
    commentThread.nextCursor,
    loadCommentPage,
  ]);

  const handleSubmitComment = useCallback(() => {
    if (
      commentCreateRequestRef.current !== null ||
      !canSubmitComment(commentThread)
    ) {
      return;
    }

    const generation = detailGenerationRef.current;
    const requestID = ++commentRequestIDRef.current;
    const body = validateCommentDraft(commentThread.draft).body;
    commentCreateRequestRef.current = requestID;
    dispatchCommentThread({ type: 'submit_started' });

    void apiClient
      .createNoteComment({ body, noteID })
      .then((comment) => {
        if (
          detailGenerationRef.current !== generation ||
          commentCreateRequestRef.current !== requestID
        ) {
          return;
        }
        dispatchCommentThread({ type: 'submit_succeeded', comment });
      })
      .catch((error: unknown) => {
        if (
          detailGenerationRef.current !== generation ||
          commentCreateRequestRef.current !== requestID
        ) {
          return;
        }

        const status = requestStatus(error);
        if (status === unauthorizedStatus) {
          void handleCommentSessionExpired();
          return;
        }
        if (status === notFoundStatus) {
          setState({ status: 'notFound' });
          return;
        }
        dispatchCommentThread({ type: 'submit_failed' });
      })
      .finally(() => {
        if (commentCreateRequestRef.current === requestID) {
          commentCreateRequestRef.current = null;
        }
      });
  }, [apiClient, commentThread, handleCommentSessionExpired, noteID]);

  const handleDeleteComment = useCallback(
    (commentID: string) => {
      if (
        commentDeleteRequestRefs.current.has(commentID) ||
        !canDeleteComment(commentThread, commentID)
      ) {
        return;
      }

      const generation = detailGenerationRef.current;
      const requestID = ++commentRequestIDRef.current;
      commentDeleteRequestRefs.current.set(commentID, requestID);
      dispatchCommentThread({ type: 'delete_started', commentID });

      void apiClient
        .deleteNoteComment({ commentID, noteID })
        .then(() => {
          if (
            detailGenerationRef.current !== generation ||
            commentDeleteRequestRefs.current.get(commentID) !== requestID
          ) {
            return;
          }
          dispatchCommentThread({ type: 'delete_succeeded', commentID });
        })
        .catch((error: unknown) => {
          if (
            detailGenerationRef.current !== generation ||
            commentDeleteRequestRefs.current.get(commentID) !== requestID
          ) {
            return;
          }

          const status = requestStatus(error);
          if (status === unauthorizedStatus) {
            void handleCommentSessionExpired();
            return;
          }
          if (status === notFoundStatus) {
            dispatchCommentThread({ type: 'delete_not_found', commentID });
            return;
          }
          dispatchCommentThread({ type: 'delete_failed', commentID });
        })
        .finally(() => {
          if (commentDeleteRequestRefs.current.get(commentID) === requestID) {
            commentDeleteRequestRefs.current.delete(commentID);
          }
        });
    },
    [apiClient, commentThread, handleCommentSessionExpired, noteID],
  );
  const handleReportNote = useCallback(() => {
    if (state.status !== 'ready') {
      return;
    }
    dispatchReportForm({ type: 'open', target: { type: 'note', id: noteID } });
  }, [noteID, state.status]);

  const handleReportComment = useCallback(
    (commentID: string) => {
      dispatchReportForm({
        type: 'open',
        target: { type: 'comment', id: commentID },
      });
    },
    [],
  );

  const handleSubmitReport = useCallback(() => {
    if (
      !canSubmitReport(reportForm) ||
      reportForm.target === null ||
      reportForm.reason === null
    ) {
      return;
    }

    const generation = detailGenerationRef.current;
    const requestID = ++reportRequestIDRef.current;
    const { target, reason, details } = reportForm;
    dispatchReportForm({ type: 'submit_started' });

    void apiClient
      .createReport({
        targetType: target.type,
        targetID: target.id,
        reason,
        details,
      })
      .then(() => {
        if (
          detailGenerationRef.current !== generation ||
          reportRequestIDRef.current !== requestID
        ) {
          return;
        }
        dispatchReportForm({ type: 'submit_succeeded' });
      })
      .catch((error: unknown) => {
        if (
          detailGenerationRef.current !== generation ||
          reportRequestIDRef.current !== requestID
        ) {
          return;
        }

        const status = requestStatus(error);
        if (status === unauthorizedStatus) {
          dispatchReportForm({ type: 'session_expired' });
          void handleCommentSessionExpired();
          return;
        }
        if (status === notFoundStatus) {
          dispatchReportForm({ type: 'target_missing' });
          return;
        }
        dispatchReportForm({ type: 'submit_failed' });
      });
  }, [apiClient, handleCommentSessionExpired, reportForm]);


  const { getMutationState, toggleUseful: handleToggleUseful } = useUsefulMutation({
    apiClient,
    onSessionExpired,
    getGeneration: () => detailGenerationRef.current,
    isStale: (gen) => gen !== detailGenerationRef.current,
    onStaleWrite: () => {
      if (!detailActiveRef.current) {
        return;
      }

      const generation = ++detailGenerationRef.current;
      commentListRequestRef.current = null;
      commentCreateRequestRef.current = null;
      commentDeleteRequestRefs.current.clear();
      dispatchCommentThread({ type: 'reset' });
      void Promise.all([apiClient.listCatalogs(), apiClient.getNote(noteID)])
        .then(([catalogs, note]) => {
          if (
            !detailActiveRef.current ||
            detailGenerationRef.current !== generation
          ) {
            return;
          }
          const labelled = labelNote(buildNoteCatalog(catalogs), note);
          if (labelled === null) {
            setState({ status: 'error' });
            return;
          }
          setState({ status: 'ready', note: labelled });
          loadCommentPage();
        })
        .catch(() => {
          if (
            !detailActiveRef.current ||
            detailGenerationRef.current !== generation
          ) {
            return;
          }
          setState({ status: 'error' });
        });
    },
    applyResult: (noteId, updater) => {
      setState((current) => {
        if (current.status !== 'ready') return current;
        return {
          status: 'ready',
          note: updater(current.note) as typeof current.note,
        };
      });
    },
  });

  useFocusEffect(
    useCallback(() => {
      const generation = ++detailGenerationRef.current;
      detailActiveRef.current = true;
      let isActive = true;
      commentListRequestRef.current = null;
      commentCreateRequestRef.current = null;
      commentDeleteRequestRefs.current.clear();
      dispatchCommentThread({ type: 'reset' });
      dispatchReportForm({ type: 'reset' });
      setState({ status: 'loading' });

      void Promise.all([apiClient.listCatalogs(), apiClient.getNote(noteID)])
        .then(([catalogs, note]) => {
          if (!isActive || detailGenerationRef.current !== generation) {
            return;
          }
          const catalog = buildNoteCatalog(catalogs);
          const labelledNote = labelNote(catalog, note);
          if (labelledNote === null) {
            setState({ status: 'error' });
            return;
          }
          setState({ status: 'ready', note: labelledNote });
          loadCommentPage();
        })
        .catch((error: unknown) => {
          if (!isActive || detailGenerationRef.current !== generation) {
            return;
          }
          if (requestStatus(error) === unauthorizedStatus) {
            void handleCommentSessionExpired();
            return;
          }
          setState(
            error instanceof APIRequestError && error.status === notFoundStatus
              ? { status: 'notFound' }
              : { status: 'error' },
          );
        });

      return () => {
        isActive = false;
        detailActiveRef.current = false;
        detailGenerationRef.current += 1;
        commentListRequestRef.current = null;
        commentCreateRequestRef.current = null;
        commentDeleteRequestRefs.current.clear();
      };
    }, [
      apiClient,
      handleCommentSessionExpired,
      loadCommentPage,
      noteID,
    ]),
  );

  let content: ReactNode;
  if (state.status === 'loading') {
    content = (
      <EmptyStateCard
        title="Carregando a nota"
        body="Buscando essa nota completa."
      />
    );
  } else if (state.status === 'notFound') {
    content = (
      <EmptyStateCard
        title="Nota não encontrada"
        body="Essa nota não existe mais ou o link tá incompleto."
      />
    );
  } else if (state.status === 'error') {
    content = (
      <EmptyStateCard
        title="Não deu pra abrir"
        body="Confira sua conexão e tente novamente em instantes."
      />
    );
  } else {
    content = (
      <>
        <NoteDetailContent
          note={state.note}
          onPressAuthor={(authorID) =>
            router.push({ pathname: '/authors/[id]', params: { id: authorID } })
          }
          onPressUseful={() => {
            void handleToggleUseful(state.note);
          }}
          usefulError={getMutationState(state.note.id) === 'error'}
          usefulPending={getMutationState(state.note.id) === 'pending'}
          onReportNote={handleReportNote}
        />
        <CommentsSection
          currentAuthorID={currentAuthorID}
          onDeleteComment={handleDeleteComment}
          onReportComment={handleReportComment}
          onDraftChange={(draft) =>
            dispatchCommentThread({ type: 'draft_changed', draft })
          }
          onLoadMore={handleLoadMoreComments}
          onPressAuthor={(authorID) =>
            router.push({ pathname: '/authors/[id]', params: { id: authorID } })
          }
          onRetryInitial={() => loadCommentPage()}
          onSubmit={handleSubmitComment}
          thread={commentThread}
        />
      </>
    );
  }

  return (
    <FoundationScreen
      description="Leia a nota completa, com lugar, categoria e data."
      eyebrow="Nota"
      title="Nota"
    >
      {content}
      {reportForm.status === 'success' ? (
        <Text accessibilityRole="alert" style={noticeStyles.notice}>
          Denúncia recebida. Obrigado por avisar.
        </Text>
      ) : null}
      {reportForm.status === 'missing' ? (
        <Text accessibilityRole="alert" style={noticeStyles.notice}>
          Esse conteúdo não está mais disponível.
        </Text>
      ) : null}
      <ReportDialog
        target={
          reportForm.target !== null &&
          reportForm.status !== 'success' &&
          reportForm.status !== 'missing'
            ? reportForm.target
            : null
        }
        state={reportForm}
        onReasonChange={(reason) =>
          dispatchReportForm({ type: 'reason_changed', reason })
        }
        onDetailsChange={(details) =>
          dispatchReportForm({ type: 'details_changed', details })
        }
        onCancel={() => dispatchReportForm({ type: 'close' })}
        onSubmit={handleSubmitReport}
      />
      <FoundationButton label="Voltar" onPress={() => router.back()} />
    </FoundationScreen>
  );
}

const noticeStyles = StyleSheet.create({
  notice: {
    color: semanticColors.textMuted,
    fontSize: typography.sizeBody,
    lineHeight: 22,
  },
});
