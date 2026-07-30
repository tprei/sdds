import {
  type ReactNode,
  useCallback,
  useEffect,
  useReducer,
  useRef,
  useState,
} from 'react';
import { ScrollView, View } from 'react-native';
import type { TextInput } from 'react-native';
import { useFocusEffect, useLocalSearchParams, useRouter } from 'expo-router';

import { colors, componentMetrics, semanticColors } from '@sdds/tokens';

import { Screen } from '@/ui/screen';
import { AppHeader } from '@/ui/app-header';
import { EmptyState } from '@/ui/empty-state';
import { lightTick } from '@/ui/haptics';
import { IconButton } from '@/ui/icon-button';
import { IconFlag } from '@/ui/icons';
import { Avatar } from '@/ui/avatar';
import { AppText } from '@/ui/text';
import { PressableScale } from '@/ui/pressable-scale';
import { CommentsSection } from '@/features/comments/comments-section';
import {
  canDeleteComment,
  canSubmitComment,
  commentThreadReducer,
  createCommentThreadState,
  displayedCommentCount,
  validateCommentDraft,
} from '@/features/comments/comment-thread';
import { ReportDialog } from '@/features/reports/report-dialog';
import {
  canSubmitReport,
  createReportFormState,
  reportFormReducer,
} from '@/features/reports/report-form';
import { NoteDetailContent } from '@/features/notes/note-detail-content';
import { NoteActionBar } from '@/features/notes/note-action-bar';
import { buildNoteCatalog, labelNote } from '@/features/notes/catalog';
import type { LabelledNote } from '@/features/notes/catalog';
import { APIRequestError } from '@/lib/api/notes';
import type { Note } from '@/lib/api/notes';
import { requestStatus } from '@/lib/api/request-error';
import { unauthorizedStatus } from '@/lib/api/status';
import { useAuth } from '@/lib/auth/auth-provider';
import type { APIClient } from '@/lib/api/client';
import { useProductEvents } from '@/lib/events/product-event-provider';
import { productEventKinds, type UsefulContext } from '@/lib/events/event-types';
import {
  consumePresentedNoteOrigin,
  readPresentedNoteOrigin,
} from '@/features/notes/presented-note-origin';
import {
  useUsefulMutation,
  type UsefulMutationAction,
} from '@/features/notes/use-useful-mutation';
import { ReadAuthGate } from '@/components/read-auth-gate';

import { styles } from './note-detail-screen.styles';

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
  originNonce: string | undefined;
};

const noteDetailUsefulContext: UsefulContext = { source: 'note_detail' };
const notFoundStatus = 404;

export default function NoteDetailScreen() {
  const router = useRouter();
  const { apiClient, logout, state } = useAuth();
  const { id, origin } = useLocalSearchParams<{
    id?: string | string[];
    origin?: string | string[];
  }>();
  const trimmedNoteID = typeof id === 'string' ? id.trim() : undefined;
  const originNonce = typeof origin === 'string' ? origin : undefined;

  if (!trimmedNoteID) {
    return (
      <Screen scroll={false} header={<AppHeader back />}>
        <View style={styles.fallback}>
          <EmptyState
            title="Nota não encontrada"
            body="Essa nota não existe mais ou o link tá incompleto."
          />
        </View>
      </Screen>
    );
  }

  if (state.status === 'authenticated') {
    return (
      <AuthenticatedNoteDetailScreen
        key={`${state.user.id}:${trimmedNoteID}:${originNonce ?? ''}`}
        apiClient={apiClient}
        currentAuthorID={state.user.author.id}
        noteID={trimmedNoteID}
        onSessionExpired={logout}
        originNonce={originNonce}
      />
    );
  }

  return (
    <Screen scroll={false} header={<AppHeader back />}>
      <View style={styles.fallback}>
        <ReadAuthGate
          onLogin={() =>
            router.push({ pathname: '/login', params: { next: `/notes/${trimmedNoteID}` } })
          }
          onSignup={() =>
            router.push({
              pathname: '/signup',
              params: { next: `/notes/${trimmedNoteID}` },
            })
          }
          status={state.status}
        />

      </View>
    </Screen>
  );
}

function AuthenticatedNoteDetailScreen({
  apiClient,
  currentAuthorID,
  noteID,
  onSessionExpired,
  originNonce,
}: AuthenticatedNoteDetailScreenProps) {
  const router = useRouter();
  const productEvents = useProductEvents();
  const composerRef = useRef<TextInput>(null);
  const [presentedUsefulContext] = useState<UsefulContext | null>(() =>
    originNonce === undefined
      ? null
      : (readPresentedNoteOrigin(originNonce, noteID) ?? null),
  );
  useEffect(() => {
    if (originNonce !== undefined && presentedUsefulContext !== null) {
      consumePresentedNoteOrigin(originNonce, noteID);
    }
  }, [noteID, originNonce, presentedUsefulContext]);
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
  const reportPendingTargetRef = useRef<string | null>(null);
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
        try {
          productEvents.record(productEventKinds.commentCreated, {
            commentID: comment.id,
            noteID,
          });
        } catch {}
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
  }, [
    apiClient,
    commentThread,
    handleCommentSessionExpired,
    noteID,
    productEvents,
  ]);

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
    const targetKey = `${reportForm.target.type}:${reportForm.target.id}`;
    if (reportPendingTargetRef.current === targetKey) {
      return;
    }
    reportPendingTargetRef.current = targetKey;

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
      .then((receipt) => {
        try {
          productEvents.record(productEventKinds.reportCreated, {
            reportID: receipt.id,
            targetID: receipt.targetID,
            targetType: receipt.targetType,
          });
        } catch {}
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
      })
      .finally(() => {
        if (
          reportRequestIDRef.current === requestID &&
          reportPendingTargetRef.current === targetKey
        ) {
          reportPendingTargetRef.current = null;
        }
      });
  }, [apiClient, handleCommentSessionExpired, productEvents, reportForm]);

  const recordUsefulSuccess = useCallback(
    (note: Note, action: UsefulMutationAction) => {
      lightTick();
      try {
        productEvents.record(
          action === 'marked'
            ? productEventKinds.noteMarkedUseful
            : productEventKinds.noteUnmarkedUseful,
          {
            context: presentedUsefulContext ?? noteDetailUsefulContext,
            noteID: note.id,
          },
        );
      } catch {}
    },
    [presentedUsefulContext, productEvents],
  );


  const { getMutationState, toggleUseful: handleToggleUseful } = useUsefulMutation({
    apiClient,
    onSuccess: recordUsefulSuccess,
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
        reportRequestIDRef.current += 1;
        reportPendingTargetRef.current = null;
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
      <View style={styles.fallback}>
        <EmptyState
          title="Carregando a nota"
          body="Buscando essa nota completa."
        />
      </View>
    );
  } else if (state.status === 'notFound') {
    content = (
      <View style={styles.fallback}>
        <EmptyState
          title="Nota não encontrada"
          body="Essa nota não existe mais ou o link tá incompleto."
        />
      </View>
    );
  } else if (state.status === 'error') {
    content = (
      <View style={styles.fallback}>
        <EmptyState
          title="Não deu pra abrir"
          body="Confira sua conexão e tente novamente em instantes."
        />
      </View>
    );
  } else {
    content = (
      <>
        <NoteDetailContent note={state.note} />
        <View style={styles.commentsWrap}>
          <CommentsSection
            composerRef={composerRef}
            currentAuthorID={currentAuthorID}
            noteAuthorID={state.note.author.id}
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
        </View>
      </>
    );
  }

  const readyNote = state.status === 'ready' ? state.note : undefined;

  return (
    <Screen
      scroll={false}
      header={
        <AppHeader
          back
          center={
            readyNote ? (
              <PressableScale
                accessibilityLabel={`Abrir perfil do autor: ${readyNote.author.displayName}`}
                accessibilityRole="button"
                onPress={() => {
                  router.push({
                    pathname: '/authors/[id]',
                    params: { id: readyNote.author.id },
                  });
                }}
                style={styles.authorControl}
              >
                <Avatar name={readyNote.author.displayName} size={componentMetrics.avatar.md} />
                <AppText color={semanticColors.textStrong} variant="body" weight="bold">
                  {readyNote.author.displayName}
                </AppText>
              </PressableScale>
            ) : undefined
          }
          right={
            readyNote ? (
              <IconButton
                accessibilityLabel="Denunciar nota"
                icon={<IconFlag />}
                onPress={handleReportNote}
                testID="note-report"
              />
            ) : undefined
          }
        />
      }
    >
      <ScrollView
        style={styles.scroll}
        contentContainerStyle={styles.scrollContent}
        keyboardShouldPersistTaps="handled"
      >
        {content}
        {reportForm.status === 'success' ? (
          <AppText
            accessibilityRole="alert"
            color={semanticColors.accent}
            style={styles.notice}
            variant="sm"
          >
            Valeu por avisar! A gente cuida pra rede seguir feita pra humanos.
          </AppText>
        ) : null}
        {reportForm.status === 'missing' ? (
          <AppText
            accessibilityRole="alert"
            color={semanticColors.textMuted}
            style={styles.notice}
            variant="sm"
          >
            Esse conteúdo não está mais disponível.
          </AppText>
        ) : null}
      </ScrollView>
      {state.status === 'ready' ? (
        <>
          {getMutationState(state.note.id) === 'error' ? (
            <AppText
              accessibilityRole="alert"
              color={colors.danger500}
              style={styles.usefulError}
              testID="useful-error"
              variant="sm"
            >
              Não deu pra atualizar o Útil. Tenta de novo.
            </AppText>
          ) : null}
          <NoteActionBar
            commentCount={displayedCommentCount(commentThread)}
            onFocusComposer={() => composerRef.current?.focus()}
            useful={{
              count: state.note.usefulCount,
              marked: state.note.usefulByCurrentUser,
              onToggle: () => {
                void handleToggleUseful(state.note);
              },
              pending: getMutationState(state.note.id) === 'pending',
            }}
          />
        </>
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
    </Screen>
  );
}

