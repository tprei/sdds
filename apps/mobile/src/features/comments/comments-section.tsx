import type { Ref } from 'react';
import type { TextInput } from 'react-native';
import { View } from 'react-native';

import { colors, semanticColors } from '@sdds/tokens';

import type { Comment } from '@/lib/api/comments';
import { Avatar } from '@/ui/avatar';
import { Badge } from '@/ui/badge';
import { Button } from '@/ui/button';
import { IconButton } from '@/ui/icon-button';
import { IconFlag, IconTrash } from '@/ui/icons';
import { PressableScale } from '@/ui/pressable-scale';
import { relativeTimeLabel } from '@/ui/relative-time';
import { AppText } from '@/ui/text';
import { TextField } from '@/ui/text-field';

import {
  commentBodyMaxCodePoints,
  canSubmitComment,
  displayedCommentCount,
  visibleComments,
  validateCommentDraft,
} from './comment-thread';
import { styles } from './comments-section.styles';
import type {
  CommentDeleteStatus,
  CommentThreadState,
} from './comment-thread';

export type CommentsSectionProps = {
  currentAuthorID: string;
  noteAuthorID: string;
  onDraftChange: (draft: string) => void;
  onLoadMore: () => void;
  onPressAuthor: (authorID: string) => void;
  onDeleteComment: (commentID: string) => void;
  onReportComment?: (commentID: string) => void;
  onRetryInitial: () => void;
  onSubmit: (body: string) => void;
  composerRef?: Ref<TextInput>;
  thread: CommentThreadState;
};

export function CommentsSection({
  composerRef,
  currentAuthorID,
  noteAuthorID,
  onDraftChange,
  onLoadMore,
  onPressAuthor,
  onDeleteComment,
  onReportComment = () => undefined,
  onRetryInitial,
  onSubmit,
  thread,
}: CommentsSectionProps) {
  const validation = validateCommentDraft(thread.draft);
  const submitDisabled = !canSubmitComment(thread);
  const draftError = thread.draftTouched ? validation.error : null;

  return (
    <View style={styles.section}>
      <AppText accessibilityRole="header" variant="title" weight="extraBold">
        {displayedCommentCount(thread)} comentários
      </AppText>
      <CommentThread
        currentAuthorID={currentAuthorID}
        noteAuthorID={noteAuthorID}
        onDeleteComment={onDeleteComment}
        onLoadMore={onLoadMore}
        onPressAuthor={onPressAuthor}
        onReportComment={onReportComment}
        onRetryInitial={onRetryInitial}
        thread={thread}
      />
      <View style={styles.composer}>
        <TextField
          counter={{
            count: validation.codePointCount,
            max: commentBodyMaxCodePoints,
          }}
          label="Escreva um comentário"
          multiline
          onChangeText={onDraftChange}
          placeholder="Escreva um comentário"
          ref={composerRef}
          testID="comment-draft"
          value={thread.draft}
        />
        {draftError === null ? null : (
          <AppText accessibilityRole="alert" color={colors.danger500} variant="sm" weight="semibold">
            {draftErrorMessage(draftError)}
          </AppText>
        )}
        {thread.submitStatus === 'error' ? (
          <AppText accessibilityRole="alert" color={colors.danger500} variant="sm" weight="semibold">
            Não deu pra publicar o comentário. Tenta de novo.
          </AppText>
        ) : null}
        <Button
          disabled={submitDisabled}
          label={thread.submitStatus === 'pending' ? 'Publicando...' : 'Comentar'}
          onPress={() => onSubmit(validation.body)}
          testID="comment-submit"
          variant="primary"
        />
      </View>
    </View>
  );
}

function CommentThread({
  currentAuthorID,
  noteAuthorID,
  onDeleteComment,
  onReportComment = () => undefined,
  onLoadMore,
  onPressAuthor,
  onRetryInitial,
  thread,
}: Pick<
  CommentsSectionProps,
  | 'currentAuthorID'
  | 'noteAuthorID'
  | 'onReportComment'
  | 'onDeleteComment'
  | 'onLoadMore'
  | 'onPressAuthor'
  | 'onRetryInitial'
  | 'thread'
>) {
  const count = displayedCommentCount(thread);

  if (thread.initialLoadStatus === 'loading' && count === 0) {
    return <StatusMessage>Carregando comentários...</StatusMessage>;
  }

  if (thread.initialLoadStatus === 'error' && count === 0) {
    return <InitialLoadError onRetry={onRetryInitial} />;
  }

  if (
    thread.initialLoadStatus === 'ready' &&
    count === 0 &&
    thread.nextCursor === null
  ) {
    return <StatusMessage>Ainda não tem comentário. Quer começar?</StatusMessage>;
  }

  return (
    <View style={styles.commentList}>
      {thread.initialLoadStatus === 'loading' ? (
        <StatusMessage>Carregando comentários...</StatusMessage>
      ) : null}
      {thread.initialLoadStatus === 'error' ? (
        <InitialLoadError onRetry={onRetryInitial} />
      ) : null}
      {thread.initialLoadStatus === 'ready' && count === 0 ? (
        <StatusMessage>Ainda não tem comentário. Quer começar?</StatusMessage>
      ) : null}
      <CommentList
        comments={visibleComments(
          thread.comments,
          thread.deleteStatusByCommentID,
        )}
        currentAuthorID={currentAuthorID}
        deleteStatusByCommentID={thread.deleteStatusByCommentID}
        noteAuthorID={noteAuthorID}
        onDeleteComment={onDeleteComment}
        onReportComment={onReportComment}
        onPressAuthor={onPressAuthor}
      />
      <LoadMoreControl onLoadMore={onLoadMore} thread={thread} />
      <CommentList
        comments={visibleComments(
          thread.localTailComments,
          thread.deleteStatusByCommentID,
        )}
        currentAuthorID={currentAuthorID}
        deleteStatusByCommentID={thread.deleteStatusByCommentID}
        noteAuthorID={noteAuthorID}
        onDeleteComment={onDeleteComment}
        onReportComment={onReportComment}
        onPressAuthor={onPressAuthor}
      />
    </View>
  );
}

function LoadMoreControl({
  onLoadMore,
  thread,
}: Pick<CommentsSectionProps, 'onLoadMore' | 'thread'>) {
  if (thread.initialLoadStatus !== 'ready' || thread.nextCursor === null) {
    return null;
  }

  if (thread.loadMoreStatus === 'pending') {
    return <StatusMessage>Carregando comentários...</StatusMessage>;
  }

  if (thread.loadMoreStatus === 'error') {
    return (
      <View style={styles.statusGroup}>
        <AppText accessibilityRole="alert" color={colors.danger500} variant="body" weight="semibold">
          Não deu pra carregar mais comentários. Tenta de novo.
        </AppText>
        <Button
          label="Tentar de novo"
          onPress={onLoadMore}
          testID="comments-retry-load-more"
          variant="secondary"
        />
      </View>
    );
  }

  return (
    <Button
      label="Ver mais comentários"
      onPress={onLoadMore}
      testID="comments-load-more"
      variant="secondary"
    />
  );
}

function InitialLoadError({ onRetry }: { onRetry: () => void }) {
  return (
    <View style={styles.statusGroup}>
      <AppText accessibilityRole="alert" color={colors.danger500} variant="body" weight="semibold">
        Não deu pra carregar os comentários.
      </AppText>
      <Button
        label="Tentar de novo"
        onPress={onRetry}
        testID="comments-retry-initial"
        variant="secondary"
      />
    </View>
  );
}

function StatusMessage({ children }: { children: string }) {
  return (
    <AppText color={semanticColors.textMuted} variant="body">
      {children}
    </AppText>
  );
}

function CommentList({
  comments,
  currentAuthorID,
  deleteStatusByCommentID,
  noteAuthorID,
  onDeleteComment,
  onPressAuthor,
  onReportComment,
}: {
  comments: Comment[];
  currentAuthorID: string;
  deleteStatusByCommentID: ReadonlyMap<string, CommentDeleteStatus>;
  noteAuthorID: string;
  onDeleteComment: (commentID: string) => void;
  onReportComment: (commentID: string) => void;
  onPressAuthor: (authorID: string) => void;
}) {
  return comments.map((comment) => (
    <View key={comment.id} style={styles.comment}>
      <View style={styles.commentHeader}>
        <PressableScale
          accessibilityLabel={`Abrir perfil do autor: ${comment.author.displayName}`}
          accessibilityRole="button"
          onPress={() => onPressAuthor(comment.author.id)}
          style={styles.authorControl}
        >
          <Avatar name={comment.author.displayName} size={32} />
          <AppText color={semanticColors.textMuted} variant="sm" weight="bold">
            {comment.author.displayName}
          </AppText>
        </PressableScale>
        {comment.author.id === noteAuthorID ? <Badge label="Autor" /> : null}
      </View>
      <AppText color={semanticColors.textMeta} variant="xs">
        {relativeTimeLabel(new Date(comment.createdAt).toISOString(), new Date())}
      </AppText>
      <AppText color={semanticColors.textStrong} variant="body">
        {comment.body}
      </AppText>
      <View style={styles.commentActions}>
        {comment.author.id === currentAuthorID ? (
          <IconButton
            accessibilityLabel="Excluir comentário"
            icon={<IconTrash color={semanticColors.textMeta} size={20} />}
            onPress={() => onDeleteComment(comment.id)}
            testID={`comment-delete-${comment.id}`}
          />
        ) : null}
        <IconButton
          accessibilityLabel="Denunciar comentário"
          icon={<IconFlag color={semanticColors.textMeta} size={20} />}
          onPress={() => onReportComment(comment.id)}
          testID={`comment-report-${comment.id}`}
        />
      </View>
      {deleteStatusByCommentID.get(comment.id) === 'error' ? (
        <AppText accessibilityRole="alert" color={colors.danger500} variant="sm" weight="semibold">
          Não deu pra excluir o comentário. Tenta de novo.
        </AppText>
      ) : null}
    </View>
  ));
}

function draftErrorMessage(error: 'required' | 'too_long'): string {
  if (error === 'required') {
    return 'Escreva alguma coisa antes de comentar.';
  }
  return 'Seu comentário pode ter até 1.000 caracteres.';
}
