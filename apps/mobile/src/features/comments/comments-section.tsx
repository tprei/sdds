import { Pressable, Text, View } from 'react-native';

import {
  FoundationButton,
  FoundationTextInput,
} from '@/components/foundation-screen';
import type { Comment } from '@/lib/api/comments';

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

const dateFormatter = new Intl.DateTimeFormat('pt-BR', {
  dateStyle: 'medium',
  timeStyle: 'short',
});

export type CommentsSectionProps = {
  currentAuthorID: string;
  onDraftChange: (draft: string) => void;
  onLoadMore: () => void;
  onPressAuthor: (authorID: string) => void;
  onDeleteComment: (commentID: string) => void;
  onRetryInitial: () => void;
  onSubmit: (body: string) => void;
  thread: CommentThreadState;
};

export function CommentsSection({
  onDraftChange,
  currentAuthorID,
  onLoadMore,
  onPressAuthor,
  onDeleteComment,
  onRetryInitial,
  onSubmit,
  thread,
}: CommentsSectionProps) {
  const validation = validateCommentDraft(thread.draft);
  const submitDisabled = !canSubmitComment(thread);
  const draftError = thread.draftTouched ? validation.error : null;

  return (
    <View style={styles.section}>
      <Text accessibilityRole="header" style={styles.heading}>
        Comentários
      </Text>
      <CommentThread
        onLoadMore={onLoadMore}
        currentAuthorID={currentAuthorID}
        onPressAuthor={onPressAuthor}
        onRetryInitial={onRetryInitial}
        onDeleteComment={onDeleteComment}
        thread={thread}
      />
      <View style={styles.composer}>
        <Text style={styles.composerLabel}>Escreva um comentário</Text>
        <FoundationTextInput
          accessibilityLabel="Escreva um comentário"
          multiline
          onChangeText={onDraftChange}
          placeholder="Escreva um comentário"
          style={styles.input}
          testID="comment-draft"
          value={thread.draft}
        />
        <Text style={styles.counter}>
          {validation.codePointCount}/{commentBodyMaxCodePoints}
        </Text>
        {draftError === null ? null : (
          <Text accessibilityRole="alert" style={styles.draftError}>
            {draftErrorMessage(draftError)}
          </Text>
        )}
        {thread.submitStatus === 'error' ? (
          <Text accessibilityRole="alert" style={styles.draftError}>
            Não deu pra publicar o comentário. Tenta de novo.
          </Text>
        ) : null}
        <FoundationButton
          disabled={submitDisabled}
          label={thread.submitStatus === 'pending' ? 'Publicando...' : 'Comentar'}
          onPress={() => onSubmit(validation.body)}
          testID="comment-submit"
        />
      </View>
    </View>
  );
}

function CommentThread({
  currentAuthorID,
  onDeleteComment,
  onLoadMore,
  onPressAuthor,
  onRetryInitial,
  thread,
}: Pick<
  CommentsSectionProps,
  | 'currentAuthorID'
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
        onDeleteComment={onDeleteComment}
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
        onDeleteComment={onDeleteComment}
        onPressAuthor={onPressAuthor}
      />
    </View>
  );
}

function LoadMoreControl({
  onLoadMore,
  thread,
}: Pick<CommentsSectionProps, 'onLoadMore' | 'thread'>) {
  if (
    thread.initialLoadStatus !== 'ready' ||
    thread.nextCursor === null
  ) {
    return null;
  }

  if (thread.loadMoreStatus === 'pending') {
    return <StatusMessage>Carregando comentários...</StatusMessage>;
  }

  if (thread.loadMoreStatus === 'error') {
    return (
      <View style={styles.statusGroup}>
        <Text accessibilityRole="alert" style={[styles.status, styles.statusError]}>
          Não deu pra carregar mais comentários. Tenta de novo.
        </Text>
        <FoundationButton
          label="Tentar de novo"
          onPress={onLoadMore}
          testID="comments-retry-load-more"
        />
      </View>
    );
  }

  return (
    <FoundationButton
      label="Ver mais comentários"
      onPress={onLoadMore}
      testID="comments-load-more"
    />
  );
}

function InitialLoadError({ onRetry }: { onRetry: () => void }) {
  return (
    <View style={styles.statusGroup}>
      <Text accessibilityRole="alert" style={[styles.status, styles.statusError]}>
        Não deu pra carregar os comentários.
      </Text>
      <FoundationButton
        label="Tentar de novo"
        onPress={onRetry}
        testID="comments-retry-initial"
      />
    </View>
  );
}

function StatusMessage({ children }: { children: string }) {
  return <Text style={styles.status}>{children}</Text>;
}

function CommentList({
  comments,
  currentAuthorID,
  deleteStatusByCommentID,
  onDeleteComment,
  onPressAuthor,
}: {
  comments: Comment[];
  currentAuthorID: string;
  deleteStatusByCommentID: ReadonlyMap<string, CommentDeleteStatus>;
  onDeleteComment: (commentID: string) => void;
  onPressAuthor: (authorID: string) => void;
}) {
  return comments.map((comment) => (
    <View key={comment.id} style={styles.comment}>
      <View style={styles.commentHeader}>
        <Pressable
          accessibilityLabel={`Abrir perfil do autor: ${comment.author.displayName}`}
          accessibilityRole="button"
          onPress={() => onPressAuthor(comment.author.id)}
          style={({ pressed }) => [
            styles.authorControl,
            pressed ? styles.authorPressed : null,
          ]}
        >
          <Text style={styles.author}>{comment.author.displayName}</Text>
        </Pressable>
        <Text style={styles.date}>{formatTimestamp(comment.createdAt)}</Text>
      </View>
      <Text style={styles.commentBody}>{comment.body}</Text>
      {comment.author.id === currentAuthorID ? (
        <Pressable
          accessibilityLabel="Excluir comentário"
          accessibilityRole="button"
          onPress={() => onDeleteComment(comment.id)}
          style={({ pressed }) => [
            styles.deleteControl,
            pressed ? styles.deletePressed : null,
          ]}
          testID={`comment-delete-${comment.id}`}
        >
          <Text style={styles.deleteText}>Excluir comentário</Text>
        </Pressable>
      ) : null}
      {deleteStatusByCommentID.get(comment.id) === 'error' ? (
        <Text accessibilityRole="alert" style={styles.deleteError}>
          Não deu pra excluir o comentário. Tenta de novo.
        </Text>
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

function formatTimestamp(timestamp: number): string {
  return dateFormatter.format(new Date(timestamp));
}
