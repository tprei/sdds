import { useCallback, useLayoutEffect, useRef, useState } from 'react';
import {
  ScrollView,
  View,
  useWindowDimensions,
  type NativeScrollEvent,
  type NativeSyntheticEvent,
} from 'react-native';
import { useFocusEffect } from 'expo-router';

import { colors, componentMetrics, semanticColors } from '@sdds/tokens';

import {
  NoteCard,
  NOTE_USEFUL_ERROR_MESSAGE,
} from '../../components/note-card';
import { estimateNoteCardHeight } from '../notes/note-card-estimate';
import { APIRequestError } from '../../lib/api/notes';
import { requestStatus } from '../../lib/api/request-error';
import { unauthorizedStatus } from '../../lib/api/status';
import type { APIClient } from '../../lib/api/client';
import type { PublicAuthor } from '../../lib/api/authors';
import {
  buildNoteCatalog,
  labelNotes,
  type LabelledNote,
  type NoteCatalog,
} from '../notes/catalog';
import { useProductEvents } from '../../lib/events/product-event-provider';
import { productEventKinds } from '../../lib/events/event-types';
import { AppText } from '@/ui/text';
import { Avatar } from '@/ui/avatar';
import { Button } from '@/ui/button';
import { EmptyState } from '@/ui/empty-state';
import { lightTick } from '@/ui/haptics';
import { type GridLayout, resolveGridLayout } from '@/ui/grid-layout';
import { MasonryGrid } from '@/ui/masonry-grid';
import { styles } from './author-profile-content.styles';

type Props = {
  apiClient: APIClient;
  authorID: string;
  isOwnProfile?: boolean;
  onCompose?: () => void;
  onPressNote: (noteID: string) => void;
  onSessionExpired: () => Promise<void>;
};
type ProfileError = 'not_found' | 'error' | null;
type UsefulMutationState = 'error' | 'pending';

function noteCountLabel(count: number): string {
  return count === 1 ? 'achado' : 'achados';
}

function ProfileHeader({
  author,
  isOwnProfile,
}: {
  author: PublicAuthor;
  isOwnProfile: boolean;
}) {
  return (
    <View style={styles.header} testID="author-profile-header">
      <Avatar name={author.displayName} ring={isOwnProfile} size={componentMetrics.avatar.lg} />
      <AppText
        accessibilityRole="header"
        color={semanticColors.textStrong}
        variant="h2"
      >
        {author.displayName}
      </AppText>
      <View style={styles.stat} testID="author-profile-note-count">
        <AppText
          color={semanticColors.textStrong}
          variant="h3"
          weight="extraBold"
        >
          {author.noteCount}
        </AppText>
        <AppText color={semanticColors.textMeta} variant="xs">
          {noteCountLabel(author.noteCount)}
        </AppText>
      </View>
    </View>
  );
}

function InitialLoading() {
  return <EmptyState title="Carregando perfil…" />;
}

function InitialError({
  notFound,
  onRetry,
}: {
  notFound: boolean;
  onRetry: () => void;
}) {
  return (
    <View style={styles.statusGroup}>
      <EmptyState
        title={
          notFound
            ? 'Perfil não encontrado.'
            : 'Não foi possível carregar este perfil.'
        }
      />
      <Button label="Tentar de novo" onPress={onRetry} variant="secondary" />
    </View>
  );
}

function ProfileNotes({
  gridLayout,
  isOwnProfile,
  notes,
  onCompose,
  onPressNote,
  onToggleUseful,
  usefulMutations,
}: {
  gridLayout: GridLayout;
  isOwnProfile: boolean;
  notes: LabelledNote[];
  onCompose?: () => void;
  onPressNote: (noteID: string) => void;
  onToggleUseful: (note: LabelledNote) => Promise<void>;
  usefulMutations: Partial<Record<string, UsefulMutationState>>;
}) {
  if (notes.length === 0) {
    return isOwnProfile ? (
      <EmptyState
        action={onCompose ? { label: 'Escrever', onPress: onCompose } : undefined}
        title="Seus achados vão aparecer aqui. Bora escrever o primeiro?"
      />
    ) : (
      <EmptyState title="Nenhuma nota ainda." />
    );
  }

  return (
    <MasonryGrid
      columnCount={gridLayout.columnCount}
      items={notes}
      keyFor={(note) => note.id}
      estimateHeight={(note) => estimateNoteCardHeight(note, gridLayout.columnWidth)}
      renderItem={(note) => (
        <NoteCard
          categoryLabel={note.categoryLabel}
          note={note}
          onPress={() => onPressNote(note.id)}
          onPressUseful={() => {
            void onToggleUseful(note);
          }}
          usefulError={
            usefulMutations[note.id] === 'error'
              ? NOTE_USEFUL_ERROR_MESSAGE
              : null
          }
          usefulPending={usefulMutations[note.id] === 'pending'}
        />
      )}
    />
  );
}

function PaginationStatus({
  loading,
  error,
  onRetry,
}: {
  loading: boolean;
  error: boolean;
  onRetry: () => void;
}) {
  if (error) {
    return (
      <View style={styles.statusGroup}>
        <AppText accessibilityRole="alert" color={colors.danger500} variant="sm">
          Não foi possível carregar mais notas.
        </AppText>
        <Button label="Tentar de novo" onPress={onRetry} variant="secondary" />
      </View>
    );
  }
  return loading ? (
    <AppText color={semanticColors.textMeta} variant="sm">
      Carregando mais notas…
    </AppText>
  ) : null;
}

function nearScrollEnd(event: {
  nativeEvent: {
    contentOffset: { y: number };
    contentSize: { height: number };
    layoutMeasurement: { height: number };
  };
}): boolean {
  const { contentOffset, contentSize, layoutMeasurement } = event.nativeEvent;
  return contentOffset.y + layoutMeasurement.height >= contentSize.height - 120;
}

export function AuthorProfileContent({
  apiClient,
  authorID,
  isOwnProfile = false,
  onCompose,
  onPressNote,
  onSessionExpired,
}: Props) {
  const productEvents = useProductEvents();
  const { width } = useWindowDimensions();
  const gridLayout = resolveGridLayout(width);
  const [author, setAuthor] = useState<PublicAuthor | null>(null);
  const [catalog, setCatalog] = useState<NoteCatalog | null>(null);
  const [notes, setNotes] = useState<LabelledNote[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadingNext, setLoadingNext] = useState(false);
  const [error, setError] = useState<ProfileError>(null);
  const [nextError, setNextError] = useState(false);
  const [usefulMutations, setUsefulMutations] = useState<
    Partial<Record<string, UsefulMutationState>>
  >({});
  const pendingCursor = useRef<string | null | undefined>(undefined);
  const requestVersion = useRef(0);
  const usefulMutationGenerationRef = useRef(0);
  const usefulPendingRef = useRef(new Set<string>());
  const [activeAuthorID, setActiveAuthorID] = useState<string | null>(null);
  const currentAuthorID = useRef(authorID);
  const hasLoadedAuthorIDRef = useRef<string | null>(null);

  useLayoutEffect(() => {
    currentAuthorID.current = authorID;
    requestVersion.current += 1;
    pendingCursor.current = undefined;
  }, [authorID]);

  const isCurrentRequest = useCallback(
    (version: number, requestedAuthorID: string) =>
      version === requestVersion.current &&
      requestedAuthorID === currentAuthorID.current,
    [],
  );

  const invalidateRequests = useCallback(() => {
    requestVersion.current += 1;
    usefulMutationGenerationRef.current += 1;
    pendingCursor.current = undefined;
    setUsefulMutations({});
  }, []);

  const loadInitial = useCallback(async () => {
    const requestedAuthorID = authorID;
    const version = requestVersion.current + 1;
    requestVersion.current = version;
    pendingCursor.current = undefined;
    const isWarm = hasLoadedAuthorIDRef.current === requestedAuthorID;
    hasLoadedAuthorIDRef.current = requestedAuthorID;
    if (!isWarm) {
      setActiveAuthorID(null);
      setAuthor(null);
      setCatalog(null);
      setNotes([]);
      setCursor(null);
      setLoadingNext(false);
      setNextError(false);
      setUsefulMutations({});
      setLoading(true);
      usefulMutationGenerationRef.current += 1;
      setError(null);
    }
    try {
      const [profile, page, catalogs] = await Promise.all([
        apiClient.getPublicAuthor(requestedAuthorID),
        apiClient.listAuthorNotes({ authorID: requestedAuthorID }),
        apiClient.listCatalogs(),
      ]);
      if (!isCurrentRequest(version, requestedAuthorID)) return;
      const nextCatalog = buildNoteCatalog(catalogs);
      const labelledNotes = labelNotes(nextCatalog, page.notes);
      if (labelledNotes === null) throw new Error('catalog_labels_missing');
      setAuthor(profile);
      setCatalog(nextCatalog);
      setNotes(labelledNotes);
      setCursor(page.nextCursor);
      setActiveAuthorID(requestedAuthorID);
    } catch (caught: unknown) {
      if (!isCurrentRequest(version, requestedAuthorID)) {
        return;
      }
      if (requestStatus(caught) === unauthorizedStatus) {
        await onSessionExpired().catch(() => undefined);
        return;
      }
      setActiveAuthorID(requestedAuthorID);
      setError(
        caught instanceof APIRequestError && caught.status === 404
          ? 'not_found'
          : 'error',
      );
    } finally {
      if (isCurrentRequest(version, requestedAuthorID)) {
        setLoading(false);
      }
    }
  }, [apiClient, authorID, isCurrentRequest, onSessionExpired]);

  const loadNext = useCallback(
    async (nextCursor: string, nextCatalog: NoteCatalog) => {
      if (pendingCursor.current === nextCursor) return;
      const requestedAuthorID = authorID;
      pendingCursor.current = nextCursor;
      const version = requestVersion.current + 1;
      requestVersion.current = version;
      setLoadingNext(true);
      setNextError(false);
      try {
        const page = await apiClient.listAuthorNotes(
          {
            authorID: requestedAuthorID,
            cursor: nextCursor,
          },
        );
        if (!isCurrentRequest(version, requestedAuthorID)) return;
        const labelledNotes = labelNotes(nextCatalog, page.notes);
        if (labelledNotes === null) throw new Error('catalog_labels_missing');
        setNotes((current) => {
          const ids = new Set(current.map((note) => note.id));
          return [
            ...current,
            ...labelledNotes.filter((note) => !ids.has(note.id)),
          ];
        });
        setCursor(page.nextCursor);
      } catch (caught: unknown) {
        if (!isCurrentRequest(version, requestedAuthorID)) {
          return;
        }
        if (requestStatus(caught) === unauthorizedStatus) {
          await onSessionExpired().catch(() => undefined);
          return;
        }
        setNextError(true);
      } finally {
        if (isCurrentRequest(version, requestedAuthorID)) {
          pendingCursor.current = undefined;
          setLoadingNext(false);
        }
      }
    },
    [apiClient, authorID, isCurrentRequest, onSessionExpired],
  );
  const toggleUseful = useCallback(
    async (target: LabelledNote) => {
      if (
        usefulMutations[target.id] === 'pending' ||
        usefulPendingRef.current.has(target.id)
      ) {
        return;
      }
      usefulPendingRef.current.add(target.id);
      const generation = usefulMutationGenerationRef.current;
      const action = target.usefulByCurrentUser ? 'unmarked' : 'marked';
      setUsefulMutations((current) => ({
        ...current,
        [target.id]: 'pending',
      }));
      try {
        if (target.usefulByCurrentUser) {
          await apiClient.unmarkNoteUseful(target.id);
        } else {
          await apiClient.markNoteUseful(target.id);
        }
        lightTick();
        try {
          productEvents.record(
            action === 'marked'
              ? productEventKinds.noteMarkedUseful
              : productEventKinds.noteUnmarkedUseful,
            {
              noteID: target.id,
              context: { source: 'author_profile' },
            },
          );
        } catch {}
        if (usefulMutationGenerationRef.current !== generation) {
          return;
        }
        setNotes((current) =>
          current.map((note) =>
            note.id === target.id
              ? {
                  ...note,
                  usefulByCurrentUser: !note.usefulByCurrentUser,
                  usefulCount: note.usefulByCurrentUser
                    ? note.usefulCount - 1
                    : note.usefulCount + 1,
                }
              : note,
          ),
        );
        setUsefulMutations((current) => {
          const { [target.id]: _removed, ...rest } = current;
          return rest;
        });
      } catch (error: unknown) {
        if (usefulMutationGenerationRef.current !== generation) {
          return;
        }
        if (requestStatus(error) === unauthorizedStatus) {
          await onSessionExpired().catch(() => undefined);
          return;
        }
        setUsefulMutations((current) => ({
          ...current,
          [target.id]: 'error',
        }));
      } finally {
        usefulPendingRef.current.delete(target.id);
      }
    },
    [apiClient, onSessionExpired, productEvents, usefulMutations],
  );

  useFocusEffect(
    useCallback(() => {
      void loadInitial();
      return invalidateRequests;
    }, [invalidateRequests, loadInitial]),
  );

  const handleScroll = useCallback(
    (event: NativeSyntheticEvent<NativeScrollEvent>) => {
      if (
        !loadingNext &&
        !nextError &&
        cursor !== null &&
        catalog !== null &&
        nearScrollEnd(event)
      ) {
        void loadNext(cursor, catalog);
      }
    },
    [catalog, cursor, loadNext, loadingNext, nextError],
  );

  if (activeAuthorID !== authorID || loading) return <InitialLoading />;
  if (error !== null || author === null || catalog === null) {
    return (
      <InitialError
        notFound={error === 'not_found'}
        onRetry={() => void loadInitial()}
      />
    );
  }

  return (
    <ScrollView
      contentContainerStyle={styles.content}
      onScroll={handleScroll}
      scrollEventThrottle={100}
      testID="author-profile-scroll"
    >
      <ProfileHeader author={author} isOwnProfile={isOwnProfile} />
      <ProfileNotes
        gridLayout={gridLayout}
        isOwnProfile={isOwnProfile}
        notes={notes}
        onCompose={onCompose}
        onPressNote={onPressNote}
        onToggleUseful={toggleUseful}
        usefulMutations={usefulMutations}
      />
      <PaginationStatus
        error={nextError}
        loading={loadingNext}
        onRetry={() => cursor !== null && void loadNext(cursor, catalog)}
      />
    </ScrollView>
  );
}
