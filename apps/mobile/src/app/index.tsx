import { useCallback, useEffect, useRef, useState } from 'react';
import { useFocusEffect, useRouter } from 'expo-router';

import {
  EmptyStateCard,
  FoundationScreen,
} from '@/components/foundation-screen';
import { NoteCard } from '@/components/note-card';
import { useAuth } from '@/lib/auth/auth-provider';
import type { APIClient } from '@/lib/api/client';
import { requestStatus } from '@/lib/api/request-error';
import { unauthorizedStatus } from '@/lib/api/status';
import { CategoryFilterControls } from '@/features/notes/category-filter-controls';
import { resolveCategoryFilterSlug } from '@/features/notes/category-filter';
import { buildNoteCatalog, labelNotes } from '@/features/notes/catalog';
import type { LabelledNote, NoteCatalog } from '@/features/notes/catalog';
import type { ListNotesInput, Note } from '@/lib/api/notes';
import { useProductEvents } from '@/lib/events/product-event-provider';
import { registerPresentedNoteOrigin } from '@/features/notes/presented-note-origin';
import { ReadAuthGate } from '@/components/read-auth-gate';

type CatalogState =
  | { status: 'loading' }
  | { status: 'ready'; catalog: NoteCatalog }
  | { status: 'error' };

type FeedState =
  | { status: 'loading' }
  | {
      categorySlug: string | null;
      generation: number;
      status: 'empty';
    }
  | {
      categorySlug: string | null;
      generation: number;
      notes: PresentedExploreNote[];
      status: 'ready';
    }
  | { status: 'error' };

type AuthenticatedHomeScreenProps = {
  apiClient: APIClient;
  onSessionExpired: () => Promise<void>;
};

type UsefulMutationState = 'error' | 'pending';
type PresentedExploreNote = {
  categorySlug: string | null;
  note: LabelledNote;
  rank: number;
};
function presentExploreNotes(
  catalog: NoteCatalog,
  notes: Note[],
  categorySlug: string | null,
): PresentedExploreNote[] | null {
  const labelledNotes = labelNotes(catalog, notes);
  if (labelledNotes === null) {
    return null;
  }
  return labelledNotes.map((note, index) => ({
    categorySlug,
    note,
    rank: index + 1,
  }));
}

export default function HomeScreen() {
  const { apiClient, logout, state } = useAuth();
  const router = useRouter();

  if (state.status === 'authenticated') {
    return (
      <AuthenticatedHomeScreen
        key={state.user.id}
        apiClient={apiClient}
        onSessionExpired={logout}
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
  apiClient,
  onSessionExpired,
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
  const productEvents = useProductEvents();
  const impressionGenerationRef = useRef<number | null>(null);
  const usefulPendingRef = useRef(new Set<string>());

  useEffect(() => {
    if (
      (feedState.status !== 'ready' && feedState.status !== 'empty') ||
      impressionGenerationRef.current === feedState.generation
    ) {
      return;
    }
    impressionGenerationRef.current = feedState.generation;
    const notes = feedState.status === 'ready' ? feedState.notes : [];
    try {
      productEvents.record('explore_notes_impression', {
        categorySlug: feedState.categorySlug,
        resultCount: notes.length,
        results: notes.map((presented) => ({
          noteID: presented.note.id,
          rank: presented.rank,
        })),
      });
    } catch {}
  }, [feedState, productEvents]);

  const loadFeed = useCallback(
    (catalog: NoteCatalog, categorySlug: string | null) => {
      requestIDRef.current += 1;
      const requestID = requestIDRef.current;
      setFeedState({ status: 'loading' });
      setUsefulMutations({});

      apiClient.listNotes(noteListInput(categorySlug))
        .then((notes) => {
          if (requestIDRef.current !== requestID) {
            return;
          }
          const presentedNotes = presentExploreNotes(catalog, notes, categorySlug);
          if (presentedNotes === null) {
            setFeedState({ status: 'error' });
            return;
          }
          setFeedState(
            presentedNotes.length > 0
              ? {
                  categorySlug,
                  generation: requestID,
                  notes: presentedNotes,
                  status: 'ready',
                }
              : {
                  categorySlug,
                  generation: requestID,
                  status: 'empty',
                },
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
    [apiClient, onSessionExpired],
  );

  const loadCatalogAndFeed = useCallback(() => {
    requestIDRef.current += 1;
    const requestID = requestIDRef.current;
    setCatalogState({ status: 'loading' });
    setFeedState({ status: 'loading' });
    setUsefulMutations({});

    apiClient.listCatalogs()
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

        apiClient.listNotes(noteListInput(categorySlug))
          .then((notes) => {
            if (requestIDRef.current !== requestID) {
              return;
            }
            const presentedNotes = presentExploreNotes(catalog, notes, categorySlug);
            if (presentedNotes === null) {
              setFeedState({ status: 'error' });
              return;
            }
            setFeedState(
              presentedNotes.length > 0
                ? {
                    categorySlug,
                    generation: requestID,
                    notes: presentedNotes,
                    status: 'ready',
                  }
                : {
                    categorySlug,
                    generation: requestID,
                    status: 'empty',
                  },
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
  }, [apiClient, onSessionExpired]);

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
    async (target: PresentedExploreNote) => {
      const note = target.note;
      if (
        usefulMutations[note.id] === 'pending' ||
        usefulPendingRef.current.has(note.id)
      ) {
        return;
      }
      usefulPendingRef.current.add(note.id);
      const generation = requestIDRef.current;
      const action = note.usefulByCurrentUser ? 'unmarked' : 'marked';
      setUsefulMutations((current) => ({
        ...current,
        [note.id]: 'pending',
      }));
      try {
        if (note.usefulByCurrentUser) {
          await apiClient.unmarkNoteUseful(note.id);
        } else {
          await apiClient.markNoteUseful(note.id);
        }
        try {
          productEvents.record(
            action === 'marked'
              ? 'note_marked_useful'
              : 'note_unmarked_useful',
            {
              noteID: note.id,
              context: {
                categorySlug: target.categorySlug,
                rank: target.rank,
                source: 'explore',
              },
            },
          );
        } catch {}
        if (requestIDRef.current !== generation) {
          return;
        }
        setFeedState((current) => {
          if (current.status !== 'ready') {
            return current;
          }
          return {
            ...current,
            notes: current.notes.map((presented) =>
              presented.note.id === note.id
                ? {
                    ...presented,
                    note: {
                      ...presented.note,
                      usefulByCurrentUser: !presented.note.usefulByCurrentUser,
                      usefulCount: presented.note.usefulByCurrentUser
                        ? presented.note.usefulCount - 1
                        : presented.note.usefulCount + 1,
                    },
                  }
                : presented,
            ),
          };
        });
        setUsefulMutations((current) => {
          const { [note.id]: _removed, ...rest } = current;
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
          [note.id]: 'error',
        }));
      } finally {
        usefulPendingRef.current.delete(note.id);
      }
    },
    [apiClient, onSessionExpired, productEvents, usefulMutations],
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
          onOpenNote={(presented) => {
            const origin = registerPresentedNoteOrigin(presented.note.id, {
              categorySlug: presented.categorySlug,
              rank: presented.rank,
              source: 'explore',
            });
            try {
              productEvents.record('explore_note_opened', {
                categorySlug: presented.categorySlug,
                noteID: presented.note.id,
                rank: presented.rank,
              });
            } catch {}
            const params: { id: string; origin?: string } = {
              id: presented.note.id,
            };
            if (origin !== '') {
              params.origin = origin;
            }
            router.push({ pathname: '/notes/[id]', params });
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
  onOpenNote: (presented: PresentedExploreNote) => void;
  onToggleUseful: (presented: PresentedExploreNote) => Promise<void>;
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

  return state.notes.map((presented) => {
    const labelledNote = presented.note;
    return (
      <NoteCard
        categoryLabel={labelledNote.categoryLabel}
        key={labelledNote.id}
        note={labelledNote}
        onPress={() => onOpenNote(presented)}
        onPressAuthor={onOpenAuthor}
        onPressUseful={() => {
          void onToggleUseful(presented);
        }}
        placeLabel={labelledNote.placeLabel}
        usefulError={usefulMutations[labelledNote.id] === 'error'}
        usefulPending={usefulMutations[labelledNote.id] === 'pending'}
      />
    );
  });
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
