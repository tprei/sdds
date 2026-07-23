import { useCallback, useRef, useState } from 'react';
import { useFocusEffect, useRouter } from 'expo-router';

import {
  EmptyStateCard,
  FoundationScreen,
} from '@/components/foundation-screen';
import { NoteCard } from '@/components/note-card';
import { useAuth } from '@/lib/auth/auth-provider';
import { requestStatus } from '@/lib/api/request-error';
import { unauthorizedStatus } from '@/lib/api/status';
import { CategoryFilterControls } from '@/features/notes/category-filter-controls';
import { resolveCategoryFilterSlug } from '@/features/notes/category-filter';
import { buildNoteCatalog, labelNotes } from '@/features/notes/catalog';
import type { LabelledNote, NoteCatalog } from '@/features/notes/catalog';
import { listCatalogs } from '@/lib/api/catalogs';
import {
  listNotes,
  markNoteUseful,
  unmarkNoteUseful,
} from '@/lib/api/notes';
import type { ListNotesInput, Note } from '@/lib/api/notes';
import { ReadAuthGate } from '@/components/read-auth-gate';

type CatalogState =
  | { status: 'loading' }
  | { status: 'ready'; catalog: NoteCatalog }
  | { status: 'error' };

type FeedState =
  | { status: 'loading' }
  | { status: 'empty' }
  | { status: 'ready'; notes: LabelledNote[] }
  | { status: 'error' };

type AuthenticatedHomeScreenProps = {
  onSessionExpired: () => Promise<void>;
  token: string;
};

type UsefulMutationState = 'error' | 'pending';

export default function HomeScreen() {
  const { logout, state } = useAuth();
  const router = useRouter();

  if (state.status === 'authenticated') {
    return (
      <AuthenticatedHomeScreen
        key={state.user.id}
        onSessionExpired={logout}
        token={state.token}
      />
    );
  }

  return (
    <FoundationScreen
      eyebrow="sdds."
      title="Explorar"
      description="Um feed global de notas úteis pra descobrir dicas, lugares e achados."
    >
      <ReadAuthGate
        onLogin={() =>
          router.push({
            pathname: '/login',
            params: { next: '/' },
          })
        }
        onSignup={() =>
          router.push({
            pathname: '/signup',
            params: { next: '/' },
          })
        }
        status={state.status}
      />
    </FoundationScreen>
  );
}

function AuthenticatedHomeScreen({
  onSessionExpired,
  token,
}: AuthenticatedHomeScreenProps) {
  const router = useRouter();
  const requestIDRef = useRef(0);
  const selectedCategorySlugRef = useRef<string | null>(null);
  const catalogRef = useRef<NoteCatalog | null>(null);
  const [selectedCategorySlug, setSelectedCategorySlug] = useState<
    string | null
  >(null);
  const [catalogState, setCatalogState] = useState<CatalogState>({
    status: 'loading',
  });
  const [feedState, setFeedState] = useState<FeedState>({ status: 'loading' });
  const [usefulMutations, setUsefulMutations] = useState<
    Partial<Record<string, UsefulMutationState>>
  >({});

  const loadFeed = useCallback(
    (catalog: NoteCatalog, categorySlug: string | null) => {
      requestIDRef.current += 1;
      const requestID = requestIDRef.current;
      setFeedState({ status: 'loading' });
      setUsefulMutations({});

      listNotes(noteListInput(categorySlug), token)
        .then((notes) => {
          if (requestIDRef.current !== requestID) {
            return;
          }
          const labelledNotes = labelNotes(catalog, notes);
          if (labelledNotes === null) {
            setFeedState({ status: 'error' });
            return;
          }
          setFeedState(
            labelledNotes.length > 0
              ? { status: 'ready', notes: labelledNotes }
              : { status: 'empty' },
          );
        })
        .catch(async (error: unknown) => {
          if (requestIDRef.current !== requestID) {
            return;
          }
          if (requestStatus(error) === unauthorizedStatus) {
            try {
              await onSessionExpired();
            } catch {}
            return;
          }
          setFeedState({ status: 'error' });
        });
    },
    [onSessionExpired, token],
  );

  const loadCatalogAndFeed = useCallback(() => {
    requestIDRef.current += 1;
    const requestID = requestIDRef.current;
    setCatalogState({ status: 'loading' });
    setFeedState({ status: 'loading' });
    setUsefulMutations({});

    listCatalogs(token)
      .then((catalogs) => {
        if (requestIDRef.current !== requestID) {
          return;
        }
        const catalog = buildNoteCatalog(catalogs);
        catalogRef.current = catalog;
        const categorySlug = resolveCategoryFilterSlug(
          catalog,
          selectedCategorySlugRef.current,
        );
        selectedCategorySlugRef.current = categorySlug;
        setSelectedCategorySlug(categorySlug);
        setCatalogState({ status: 'ready', catalog });

        listNotes(noteListInput(categorySlug), token)
          .then((notes) => {
            if (requestIDRef.current !== requestID) {
              return;
            }
            const labelledNotes = labelNotes(catalog, notes);
            if (labelledNotes === null) {
              setFeedState({ status: 'error' });
              return;
            }
            setFeedState(
              labelledNotes.length > 0
                ? { status: 'ready', notes: labelledNotes }
                : { status: 'empty' },
            );
          })
          .catch(async (error: unknown) => {
            if (requestIDRef.current !== requestID) {
              return;
            }
            if (requestStatus(error) === unauthorizedStatus) {
              try {
                await onSessionExpired();
              } catch {}
              return;
            }
            setFeedState({ status: 'error' });
          });
      })
      .catch(async (error: unknown) => {
        if (requestIDRef.current !== requestID) {
          return;
        }
        if (requestStatus(error) === unauthorizedStatus) {
          try {
            await onSessionExpired();
          } catch {}
          return;
        }
        catalogRef.current = null;
        setCatalogState({ status: 'error' });
        setFeedState({ status: 'error' });
      });
  }, [onSessionExpired, token]);

  const selectCategorySlug = useCallback(
    (categorySlug: string | null) => {
      if (selectedCategorySlugRef.current === categorySlug) {
        return;
      }

      selectedCategorySlugRef.current = categorySlug;
      setSelectedCategorySlug(categorySlug);

      const catalog = catalogRef.current;
      if (catalog !== null) {
        loadFeed(catalog, categorySlug);
      }
    },
    [loadFeed],
  );

  const toggleUseful = useCallback(
    async (target: LabelledNote) => {
      if (usefulMutations[target.id] === 'pending') {
        return;
      }
      const generation = requestIDRef.current;
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
        if (requestIDRef.current !== generation) {
          return;
        }
        setFeedState((current) => {
          if (current.status !== 'ready') {
            return current;
          }
          return {
            status: 'ready',
            notes: current.notes.map((note) =>
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
          };
        });
        setUsefulMutations((current) => {
          const { [target.id]: _removed, ...rest } = current;
          return rest;
        });
      } catch (error: unknown) {
        if (requestIDRef.current !== generation) {
          return;
        }
        if (requestStatus(error) === unauthorizedStatus) {
          try {
            await onSessionExpired();
          } catch {}
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
      loadCatalogAndFeed();

      return () => {
        requestIDRef.current += 1;
      };
    }, [loadCatalogAndFeed]),
  );

  return (
    <FoundationScreen
      eyebrow="sdds."
      title="Explorar"
      description="Um feed global de notas úteis pra descobrir dicas, lugares e achados."
    >
      <CategoryFilterControls
        catalog={catalogState.status === 'ready' ? catalogState.catalog : null}
        onSelectCategorySlug={selectCategorySlug}
        selectedCategorySlug={selectedCategorySlug}
      />
      {catalogState.status === 'error' ? (
        <CatalogError />
      ) : (
        <FeedContent
          catalogState={catalogState}
          onOpenAuthor={(authorID) => {
            router.push({ pathname: '/authors/[id]', params: { id: authorID } });
          }}
          onOpenNote={(note) => {
            router.push({
              pathname: '/notes/[id]',
              params: { id: note.id },
            });
          }}
          onToggleUseful={toggleUseful}
          selectedCategorySlug={selectedCategorySlug}
          state={feedState}
          usefulMutations={usefulMutations}
        />
      )}
    </FoundationScreen>
  );
}
function CatalogError() {
  return (
    <EmptyStateCard
      title="Não deu pra carregar as categorias"
      body="A gente precisa delas pra mostrar as notas sem inventar rótulo. Fecha e abre de novo em instantes."
    />
  );
}

function FeedContent({
  catalogState,
  onOpenAuthor,
  onOpenNote,
  onToggleUseful,
  selectedCategorySlug,
  state,
  usefulMutations,
}: {
  catalogState: CatalogState;
  onOpenAuthor: (authorID: string) => void;
  onOpenNote: (note: Note) => void;
  onToggleUseful: (note: LabelledNote) => Promise<void>;
  selectedCategorySlug: string | null;
  state: FeedState;
  usefulMutations: Partial<Record<string, UsefulMutationState>>;
}) {
  if (state.status === 'loading') {
    return (
      <EmptyStateCard
        title="Carregando as notas"
        body="Buscando as notas mais recentes do Mundo todo."
      />
    );
  }

  if (state.status === 'error') {
    return (
      <EmptyStateCard
        title="Não deu pra carregar"
        body="Confere sua conexão e tenta abrir o app de novo."
      />
    );
  }

  if (state.status === 'empty') {
    return (
      <EmptyStateCard
        title="Ainda tá quietinho"
        body={emptyBody(catalogState, selectedCategorySlug)}
      />
    );
  }

  return state.notes.map((labelledNote) => (
    <NoteCard
      categoryLabel={labelledNote.categoryLabel}
      key={labelledNote.id}
      note={labelledNote}
      onPress={() => onOpenNote(labelledNote)}
      onPressAuthor={onOpenAuthor}
      onPressUseful={() => {
        void onToggleUseful(labelledNote);
      }}
      placeLabel={labelledNote.placeLabel}
      usefulError={usefulMutations[labelledNote.id] === 'error'}
      usefulPending={usefulMutations[labelledNote.id] === 'pending'}
    />
  ));
}

function emptyBody(
  catalogState: CatalogState,
  selectedCategorySlug: string | null,
): string {
  if (selectedCategorySlug === null || catalogState.status !== 'ready') {
    return 'Seja a primeira pessoa a escrever uma nota útil.';
  }

  const category = catalogState.catalog.activeCategories.find(
    (option) => option.slug === selectedCategorySlug,
  );
  if (category === undefined) {
    return 'Seja a primeira pessoa a escrever uma nota útil.';
  }

  return `Que tal escrever o primeiro achado em ${category.label}?`;
}

function noteListInput(categorySlug: string | null): ListNotesInput {
  if (categorySlug === null) {
    return {};
  }

  return { categorySlug };
}
