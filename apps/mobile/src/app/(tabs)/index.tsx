import { useCallback, useEffect, useRef, useState } from 'react';
import { RefreshControl, ScrollView, View, useWindowDimensions } from 'react-native';
import { useFocusEffect, useRouter } from 'expo-router';

import { ReadAuthGate } from '@/components/read-auth-gate';
import { NoteCard, NOTE_USEFUL_ERROR_MESSAGE } from '@/components/note-card';
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
import { productEventKinds } from '@/lib/events/event-types';
import { registerPresentedNoteOrigin } from '@/features/notes/presented-note-origin';
import { estimateNoteCardHeight } from '@/features/notes/note-card-estimate';
import { HomeHeader } from '@/features/notes/home-header';
import { BrandHeader } from '@/features/auth/brand-header';
import { Screen } from '@/ui/screen';
import { type GridLayout, resolveGridLayout } from '@/ui/grid-layout';
import { MasonryGrid } from '@/ui/masonry-grid';
import { NoteCardSkeleton } from '@/ui/skeleton';
import { Button } from '@/ui/button';
import { EmptyState } from '@/ui/empty-state';
import { lightTick } from '@/ui/haptics';
import { semanticColors } from '@sdds/tokens';

import { styles } from './index.styles';

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
    <Screen>
      <BrandHeader compact />
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
    </Screen>
  );
}

function AuthenticatedHomeScreen({
  apiClient,
  onSessionExpired,
}: AuthenticatedHomeScreenProps) {
  const router = useRouter();
  const { width } = useWindowDimensions();
  const gridLayout = resolveGridLayout(width);
  const scrollRef = useRef<ScrollView>(null);
  const requestIDRef = useRef(0);
  const selectedCategorySlugRef = useRef<string | null>(null);
  const catalogRef = useRef<NoteCatalog | null>(null);
  const hasLoadedRef = useRef(false);
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
      productEvents.ready === false ||
      (feedState.status !== 'ready' && feedState.status !== 'empty') ||
      impressionGenerationRef.current === feedState.generation
    ) {
      return;
    }
    impressionGenerationRef.current = feedState.generation;
    const notes = feedState.status === 'ready' ? feedState.notes : [];
    try {
      productEvents.record(productEventKinds.exploreNotesImpression, {
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

  const loadCatalogAndFeed = useCallback((options?: { showLoading?: boolean }) => {
    const showLoading = options?.showLoading ?? true;
    requestIDRef.current += 1;
    const requestID = requestIDRef.current;
    if (showLoading) {
      setCatalogState({ status: 'loading' });
      setFeedState({ status: 'loading' });
    }
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
        lightTick();
        try {
          productEvents.record(
            action === 'marked'
              ? productEventKinds.noteMarkedUseful
              : productEventKinds.noteUnmarkedUseful,
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
      loadCatalogAndFeed({ showLoading: !hasLoadedRef.current });
      hasLoadedRef.current = true;

      return () => {
        requestIDRef.current += 1;
      };
    }, [loadCatalogAndFeed]),
  );

  return (
    <Screen
      scroll={false}
      header={
        <HomeHeader
          onScrollToTop={() => {
            scrollRef.current?.scrollTo({ y: 0, animated: true });
          }}
          filterRail={
            <CategoryFilterControls
              catalog={catalogState.status === 'ready' ? catalogState.catalog : null}
              onSelectCategorySlug={selectCategorySlug}
              selectedCategorySlug={selectedCategorySlug}
            />
          }
        />
      }
    >
      <ScrollView
        ref={scrollRef}
        style={styles.feedScroll}
        contentContainerStyle={styles.feedContent}
        refreshControl={
          <RefreshControl
            tintColor={semanticColors.accent}
            refreshing={catalogState.status === 'loading'}
            onRefresh={loadCatalogAndFeed}
          />
        }
      >
        {catalogState.status === 'error' ? (
          <CatalogError />
        ) : (
          <FeedContent
            catalogState={catalogState}
            gridLayout={gridLayout}
            onCompose={() => router.push('/compose')}
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
                productEvents.record(productEventKinds.exploreNoteOpened, {
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
            onRetry={loadCatalogAndFeed}
            onToggleUseful={toggleUseful}
            selectedCategorySlug={selectedCategorySlug}
            state={feedState}
            usefulMutations={usefulMutations}
          />
        )}
      </ScrollView>
    </Screen>
  );
}
function CatalogError() {
  return (
    <EmptyState
      title="Não deu pra carregar as categorias"
      body="A gente precisa delas pra mostrar as notas sem inventar rótulo. Fecha e abre de novo em instantes."
    />
  );
}

function FeedContent({
  catalogState,
  gridLayout,
  onCompose,
  onOpenAuthor,
  onOpenNote,
  onRetry,
  onToggleUseful,
  selectedCategorySlug,
  state,
  usefulMutations,
}: {
  catalogState: CatalogState;
  gridLayout: GridLayout;
  onCompose: () => void;
  onOpenAuthor: (authorID: string) => void;
  onOpenNote: (presented: PresentedExploreNote) => void;
  onRetry: () => void;
  onToggleUseful: (presented: PresentedExploreNote) => Promise<void>;
  selectedCategorySlug: string | null;
  state: FeedState;
  usefulMutations: Partial<Record<string, UsefulMutationState>>;
}) {
  if (state.status === 'loading') {
    return (
      <View style={styles.skeletonRow}>
        <View style={styles.skeletonColumn}>
          <NoteCardSkeleton tall />
          <NoteCardSkeleton />
          <NoteCardSkeleton />
        </View>
        <View style={styles.skeletonColumn}>
          <NoteCardSkeleton />
          <NoteCardSkeleton tall />
          <NoteCardSkeleton />
        </View>
      </View>
    );
  }

  if (state.status === 'error') {
    return (
      <View style={styles.errorWrap}>
        <EmptyState title="Não deu pra carregar agora." />
        <Button variant="secondary" label="Tentar de novo" onPress={onRetry} />
      </View>
    );
  }

  if (state.status === 'empty') {
    return (
      <EmptyState
        title="Nada por aqui ainda"
        body={emptyBody(catalogState, selectedCategorySlug)}
        action={{ label: 'Escrever', onPress: onCompose }}
      />
    );
  }

  return (
    <MasonryGrid
      columnCount={gridLayout.columnCount}
      items={state.notes}
      keyFor={(presented) => presented.note.id}
      estimateHeight={(presented) =>
        estimateNoteCardHeight(presented.note, gridLayout.columnWidth)
      }
      renderItem={(presented) => (
        <NoteCard
          categoryLabel={presented.note.categoryLabel}
          note={presented.note}
          onPress={() => onOpenNote(presented)}
          onPressAuthor={() => onOpenAuthor(presented.note.author.id)}
          onPressUseful={() => {
            void onToggleUseful(presented);
          }}
          usefulError={
            usefulMutations[presented.note.id] === 'error'
              ? NOTE_USEFUL_ERROR_MESSAGE
              : null
          }
          usefulPending={usefulMutations[presented.note.id] === 'pending'}
        />
      )}
    />
  );
}

function emptyBody(
  catalogState: CatalogState,
  selectedCategorySlug: string | null,
): string {
  if (selectedCategorySlug === null || catalogState.status !== 'ready') {
    return 'Que tal escrever o primeiro achado?';
  }

  const category = catalogState.catalog.activeCategories.find(
    (option) => option.slug === selectedCategorySlug,
  );
  if (category === undefined) {
    return 'Que tal escrever o primeiro achado?';
  }

  return `Que tal escrever o primeiro achado em ${category.label}?`;
}

function noteListInput(categorySlug: string | null): ListNotesInput {
  if (categorySlug === null) {
    return {};
  }

  return { categorySlug };
}
