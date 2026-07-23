import { useCallback, useLayoutEffect, useRef, useState } from 'react';
import {
  ScrollView,
  Text,
  View,
  type NativeScrollEvent,
  type NativeSyntheticEvent,
} from 'react-native';
import { useFocusEffect } from 'expo-router';

import { NoteCard } from '../../components/note-card';
import { FoundationButton } from '../../components/foundation-screen';
import { listCatalogs } from '../../lib/api/catalogs';
import {
  APIRequestError,
  markNoteUseful,
  unmarkNoteUseful,
} from '../../lib/api/notes';
import { requestStatus } from '../../lib/api/request-error';
import { unauthorizedStatus } from '../../lib/api/status';
import { getPublicAuthor, listAuthorNotes } from '../../lib/api/authors';
import type { PublicAuthor } from '../../lib/api/authors';
import {
  buildNoteCatalog,
  labelNotes,
  type LabelledNote,
  type NoteCatalog,
} from '../notes/catalog';
import { styles } from './author-profile-content.styles';

type Props = {
  authorID: string;
  onPressNote: (noteID: string) => void;
  token: string;
  onSessionExpired: () => Promise<void>;
};
type ProfileError = 'not_found' | 'error' | null;
type UsefulMutationState = 'error' | 'pending';

function initials(name: string): string {
  return name
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((part) => Array.from(part)[0]?.toLocaleUpperCase('pt-BR') ?? '')
    .join('');
}

function noteCount(count: number): string {
  return `${count} ${count === 1 ? 'Nota' : 'Notas'}`;
}

function ProfileHeader({ author }: { author: PublicAuthor }) {
  return (
    <View style={styles.header} testID="author-profile-header">
      <View style={styles.avatar}>
        <Text style={styles.avatarText}>{initials(author.displayName)}</Text>
      </View>
      <Text
        accessibilityRole="header"
        style={styles.name}
        testID="author-profile-name"
      >
        {author.displayName}
      </Text>
      <Text style={styles.count} testID="author-profile-note-count">
        {noteCount(author.noteCount)}
      </Text>
    </View>
  );
}

function InitialLoading() {
  return <Text style={styles.message}>Carregando perfil…</Text>;
}

function InitialError({
  notFound,
  onRetry,
}: {
  notFound: boolean;
  onRetry: () => void;
}) {
  return (
    <View>
      <Text
        accessibilityRole={notFound ? undefined : 'alert'}
        style={styles.message}
      >
        {notFound
          ? 'Perfil não encontrado.'
          : 'Não foi possível carregar este perfil.'}
      </Text>
      <FoundationButton label="Tentar de novo" onPress={onRetry} />
    </View>
  );
}

function ProfileNotes({
  notes,
  onPressNote,
  onToggleUseful,
  usefulMutations,
}: {
  notes: LabelledNote[];
  onPressNote: (noteID: string) => void;
  onToggleUseful: (note: LabelledNote) => Promise<void>;
  usefulMutations: Partial<Record<string, UsefulMutationState>>;
}) {
  if (notes.length === 0) {
    return <Text style={styles.message}>Nenhuma nota ainda.</Text>;
  }

  return (
    <>
      {notes.map((note) => (
        <NoteCard
          categoryLabel={note.categoryLabel}
          key={note.id}
          note={note}
          onPress={() => onPressNote(note.id)}
          onPressUseful={() => {
            void onToggleUseful(note);
          }}
          placeLabel={note.placeLabel}
          usefulError={usefulMutations[note.id] === 'error'}
          usefulPending={usefulMutations[note.id] === 'pending'}
        />
      ))}
    </>
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
      <View>
        <Text accessibilityRole="alert" style={styles.message}>
          Não foi possível carregar mais notas.
        </Text>
        <FoundationButton label="Tentar de novo" onPress={onRetry} />
      </View>
    );
  }
  return loading ? (
    <Text style={styles.message}>Carregando mais notas…</Text>
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
  authorID,
  onPressNote,
  token,
  onSessionExpired,
}: Props) {
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
  const [activeAuthorID, setActiveAuthorID] = useState<string | null>(null);
  const currentAuthorID = useRef(authorID);

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
    try {
      const [profile, page, catalogs] = await Promise.all([
        getPublicAuthor(requestedAuthorID, token),
        listAuthorNotes({ authorID: requestedAuthorID }, token),
        listCatalogs(token),
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
  }, [authorID, isCurrentRequest, onSessionExpired, token]);

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
        const page = await listAuthorNotes(
          {
            authorID: requestedAuthorID,
            cursor: nextCursor,
          },
          token,
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
    [authorID, isCurrentRequest, onSessionExpired, token],
  );

  const toggleUseful = useCallback(
    async (target: LabelledNote) => {
      if (usefulMutations[target.id] === 'pending') {
        return;
      }
      const generation = usefulMutationGenerationRef.current;
      setUsefulMutations((current) => ({
        ...current,
        [target.id]: 'pending',
      }));
      try {
        if (target.usefulByCurrentUser) {
          await unmarkNoteUseful(target.id, token);
        } else {
          await markNoteUseful(target.id, token);
        }
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
      }
    },
    [onSessionExpired, token, usefulMutations],
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
      <ProfileHeader author={author} />
      <ProfileNotes
        notes={notes}
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
