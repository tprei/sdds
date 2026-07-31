import * as Crypto from 'expo-crypto';
import { useCallback, useEffect, useRef, useState } from 'react';
import { ScrollView, View, useWindowDimensions } from 'react-native';
import { useFocusEffect, useRouter } from 'expo-router';

import { ReadAuthGate } from '@/components/read-auth-gate';
import { NoteCard, NOTE_USEFUL_ERROR_MESSAGE } from '@/components/note-card';
import { useAuth } from '@/lib/auth/auth-provider';
import type { APIClient } from '@/lib/api/client';
import { requestStatus } from '@/lib/api/request-error';
import { unauthorizedStatus } from '@/lib/api/status';
import { CategoryFilterControls } from '@/features/notes/category-filter-controls';
import { resolveCategoryFilterSlug } from '@/features/notes/category-filter';
import { buildNoteCatalog } from '@/features/notes/catalog';
import {
  appendRecentSearchQuery,
  createSearchRequest,
  isCurrentSearchRequest,
  labelSearchResults,
  presentSearchResults,
  searchResultContext,
  searchResultCountLabel,
  selectedSearchCategory,
} from '@/features/notes/search-screen';
import type {
  PresentedSearchResult,
  SearchDispatchedContext,
  SearchRequest,
  SearchResultContext,
} from '@/features/notes/search-screen';
import type { NoteCatalog } from '@/features/notes/catalog';
import type { SearchVersion } from '@/lib/api/notes';
import { useProductEvents } from '@/lib/events/product-event-provider';
import { productEventKinds } from '@/lib/events/event-types';
import { registerPresentedNoteOrigin } from '@/features/notes/presented-note-origin';
import { estimateNoteCardHeight } from '@/features/notes/note-card-estimate';
import { SearchIdle } from '@/features/notes/search-idle';
import { BrandHeader } from '@/features/auth/brand-header';
import { Screen } from '@/ui/screen';
import { SearchField } from '@/ui/search-field';
import { Button } from '@/ui/button';
import { MasonryGrid } from '@/ui/masonry-grid';
import { NoteCardSkeleton } from '@/ui/skeleton';
import { EmptyState } from '@/ui/empty-state';
import { lightTick } from '@/ui/haptics';
import { AppText } from '@/ui/text';
import { semanticColors, spacing } from '@sdds/tokens';

import { styles } from './search.styles';

type CatalogState =
  | { status: 'loading' }
  | { status: 'ready'; catalog: NoteCatalog }
  | { status: 'error' };

type SearchScreenState =
  | { status: 'idle' }
  | { request: SearchRequest; status: 'loading' }
  | {
      context: SearchResultContext;
      request: SearchRequest;
      results: PresentedSearchResult[];
      searchVersion: SearchVersion;
      status: 'ready';
    }
  | {
      context: SearchResultContext;
      request: SearchRequest;
      searchVersion: SearchVersion;
      status: 'empty';
    }
  | { request: SearchRequest; status: 'error' };

type AuthenticatedSearchScreenProps = {
  apiClient: APIClient;
  onSessionExpired: () => Promise<void>;
};

type UsefulMutationState = 'error' | 'pending';
type SearchExecution = {
  request: SearchRequest;
  searchVersion?: SearchVersion;
  reformulationEmitted: boolean;
};

const idleSearchState: SearchScreenState = { status: 'idle' };

export default function SearchScreen() {
  const router = useRouter();
  const { apiClient, logout, state } = useAuth();

  if (state.status === 'authenticated') {
    return (
      <AuthenticatedSearchScreen
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
          router.push({ pathname: '/login', params: { next: '/search' } })
        }
        onSignup={() =>
          router.push({ pathname: '/signup', params: { next: '/search' } })
        }
        status={state.status}
      />
    </Screen>
  );
}

function AuthenticatedSearchScreen({
  apiClient,
  onSessionExpired,
}: AuthenticatedSearchScreenProps) {
  const router = useRouter();
  const { width } = useWindowDimensions();
  const columnWidth = (width - 2 * spacing.gutter - spacing.masonryGap) / 2;
  const productEvents = useProductEvents();
  const catalogRequestIDRef = useRef(0);
  const searchRequestIDRef = useRef(0);
  const catalogRef = useRef<NoteCatalog | null>(null);
  const hasLoadedCatalogRef = useRef(false);
  const selectedCategorySlugRef = useRef<string | null>(null);
  const stateRef = useRef<SearchScreenState>(idleSearchState);
  const submittedQueryRef = useRef<string | null>(null);
  const previousSearchRef = useRef<SearchDispatchedContext | null>(null);
  const searchExecutionsRef = useRef(new Map<string, SearchExecution>());
  const impressionSearchIDRef = useRef<string | null>(null);
  const isMountedRef = useRef(false);
  const [query, setQuery] = useState('');
  const [selectedCategorySlug, setSelectedCategorySlug] = useState<
    string | null
  >(null);
  const [recentQueries, setRecentQueries] = useState<string[]>([]);
  const [catalogState, setCatalogState] = useState<CatalogState>({
    status: 'loading',
  });
  const [state, setState] = useState<SearchScreenState>(idleSearchState);
  const [usefulMutations, setUsefulMutations] = useState<
    Partial<Record<string, UsefulMutationState>>
  >({});
  const usefulPendingRef = useRef(new Set<string>());

  useEffect(() => {
    isMountedRef.current = true;
    return () => {
      isMountedRef.current = false;
    };
  }, []);
  const openAuthor = useCallback(
    (authorID: string) => {
      router.push({ pathname: '/authors/[id]', params: { id: authorID } });
    },
    [router],
  );
  const openNote = useCallback(
    (result: PresentedSearchResult) => {
      const originNonce = registerPresentedNoteOrigin(result.note.id, {
        retrievalSource: result.retrievalSource,
        rank: result.rank,
        searchID: result.searchID,
        searchVersion: result.searchVersion,
        source: 'search',
      });
      productEvents.record(productEventKinds.searchResultOpened, {
        noteID: result.note.id,
        rank: result.rank,
        retrievalSource: result.retrievalSource,
        searchID: result.searchID,
        searchVersion: result.searchVersion,
      });
      const params: { id: string; origin?: string } = { id: result.note.id };
      if (originNonce !== '') {
        params.origin = originNonce;
      }
      router.push({ pathname: '/notes/[id]', params });
    },
    [productEvents, router],
  );
  const setSearchState = useCallback((nextState: SearchScreenState) => {
    stateRef.current = nextState;
    setState(nextState);
  }, []);
  const resetSearchLineage = useCallback(() => {
    previousSearchRef.current = null;
    searchExecutionsRef.current.clear();
    impressionSearchIDRef.current = null;
  }, []);

  const recordSuccessfulSearch = useCallback(
    (request: SearchRequest, searchVersion: SearchVersion) => {
      // A search resolving after the screen unmounts must not emit; the user
      // has already left the search journey.
      if (!isMountedRef.current) {
        return;
      }
      try {
        productEvents.record(
          productEventKinds.searchSubmitted,
          {
            categorySlug: request.categorySlug,
            query: request.query,
            searchID: request.searchID,
            searchVersion,
          },
          { occurredAt: request.occurredAt },
        );
      } catch {}

      const execution = searchExecutionsRef.current.get(request.searchID);
      if (execution === undefined) {
        return;
      }
      execution.searchVersion = searchVersion;

      for (const successor of searchExecutionsRef.current.values()) {
        const previous = successor.request.previousSearch;
        if (
          successor.reformulationEmitted ||
          previous === null ||
          previous.searchID !== request.searchID ||
          successor.searchVersion === undefined ||
          (previous.query === successor.request.query &&
            previous.categorySlug === successor.request.categorySlug)
        ) {
          continue;
        }

        successor.reformulationEmitted = true;
        try {
          productEvents.record(
            productEventKinds.searchReformulated,
            {
              categorySlug: successor.request.categorySlug,
              previousCategorySlug: previous.categorySlug,
              previousQuery: previous.query,
              previousSearchID: previous.searchID,
              previousSearchVersion: searchVersion,
              query: successor.request.query,
              searchID: successor.request.searchID,
              searchVersion: successor.searchVersion,
            },
            { occurredAt: successor.request.occurredAt },
          );
        } catch {}
      }
    },
    [productEvents],
  );

  useEffect(() => {
    if (state.status !== 'ready' && state.status !== 'empty') {
      return;
    }
    const searchID = state.request.searchID;
    if (impressionSearchIDRef.current === searchID) {
      return;
    }
    if (productEvents.ready === false) {
      return;
    }
    if (state.status === 'empty') {
      productEvents.record(productEventKinds.searchNoResults, {
        categorySlug: state.request.categorySlug,
        query: state.request.query,
        resultCount: 0,
        searchID,
        searchVersion: state.searchVersion,
      });
    } else {
      productEvents.record(productEventKinds.searchResultsImpression, {
        categorySlug: state.request.categorySlug,
        query: state.request.query,
        resultCount: state.results.length,
        results: state.results.map((result) => ({
          noteID: result.note.id,
          rank: result.rank,
          retrievalSource: result.retrievalSource,
        })),
        searchID,
        searchVersion: state.searchVersion,
      });
    }
    impressionSearchIDRef.current = searchID;
  }, [productEvents, state]);


  const runSearch = useCallback(
    (queryValue: string, categorySlug: string | null) => {
      let searchID = '';
      try {
        searchID = Crypto.randomUUID();
      } catch {}
      const request = createSearchRequest({
        categorySlug,
        nextRequestID: searchRequestIDRef.current + 1,
        occurredAt: Date.now(),
        previousSearch: previousSearchRef.current,
        query: queryValue,
        searchID,
      });
      searchRequestIDRef.current += 1;

      if (request === null) {
        submittedQueryRef.current = null;
        resetSearchLineage();
        setSearchState(idleSearchState);
        return;
      }

      const catalog = catalogRef.current;
      if (catalog === null) {
        setSearchState({ request, status: 'error' });
        return;
      }

      previousSearchRef.current = {
        categorySlug: request.categorySlug,
        query: request.query,
        searchID: request.searchID,
      };
      searchExecutionsRef.current.set(request.searchID, {
        request,
        reformulationEmitted: false,
      });
      submittedQueryRef.current = request.query;
      setQuery(request.query);
      setRecentQueries((current) =>
        appendRecentSearchQuery(current, request.query),
      );
      setSearchState({ request, status: 'loading' });
      setUsefulMutations({});

      apiClient.searchNotes(request.input)
        .then((searchResult) => {
          recordSuccessfulSearch(request, searchResult.searchVersion);
          if (
            !isCurrentSearchRequest({
              activeRequestID: searchRequestIDRef.current,
              responseRequestID: request.id,
            })
          ) {
            return;
          }

          const labelledResults = labelSearchResults(
            catalog,
            searchResult.results,
          );
          if (labelledResults === null) {
            setSearchState({ request, status: 'error' });
            return;
          }

          const context = searchResultContext({
            catalog,
            categorySlug: request.categorySlug,
            query: request.query,
            resultCount: labelledResults.length,
          });
          setSearchState(
            labelledResults.length > 0
              ? {
                  context,
                  request,
                  results: presentSearchResults({
                    request,
                    results: labelledResults,
                    searchVersion: searchResult.searchVersion,
                  }),
                  searchVersion: searchResult.searchVersion,
                  status: 'ready',
                }
              : {
                  context,
                  request,
                  searchVersion: searchResult.searchVersion,
                  status: 'empty',
                },
          );
        })
        .catch(async (error: unknown) => {
          if (
            !isCurrentSearchRequest({
              activeRequestID: searchRequestIDRef.current,
              responseRequestID: request.id,
            })
          ) {
            return;
          }
          if (requestStatus(error) === unauthorizedStatus) {
            try {
              await onSessionExpired();
            } catch {}
            return;
          }
          setSearchState({ request, status: 'error' });
        });
    },
    [
      apiClient,
      onSessionExpired,
      recordSuccessfulSearch,
      resetSearchLineage,
      setSearchState,
    ],
  );

  const loadCatalogs = useCallback(() => {
    catalogRequestIDRef.current += 1;
    const requestID = catalogRequestIDRef.current;
    if (!hasLoadedCatalogRef.current) {
      hasLoadedCatalogRef.current = true;
      setCatalogState({ status: 'loading' });
    }
    setUsefulMutations({});

    apiClient.listCatalogs()
      .then((catalogs) => {
        if (
          !isCurrentSearchRequest({
            activeRequestID: catalogRequestIDRef.current,
            responseRequestID: requestID,
          })
        ) {
          return;
        }

        const catalog = buildNoteCatalog(catalogs);
        catalogRef.current = catalog;
        const resolvedCategorySlug = resolveCategoryFilterSlug(
          catalog,
          selectedCategorySlugRef.current,
        );
        const categoryChanged =
          resolvedCategorySlug !== selectedCategorySlugRef.current;
        const shouldRestartSearch =
          submittedQueryRef.current !== null &&
          (categoryChanged || stateRef.current.status === 'loading');

        selectedCategorySlugRef.current = resolvedCategorySlug;
        setSelectedCategorySlug(resolvedCategorySlug);
        setCatalogState({ catalog, status: 'ready' });

        if (shouldRestartSearch && submittedQueryRef.current !== null) {
          runSearch(submittedQueryRef.current, resolvedCategorySlug);
        }
      })
      .catch(async (error: unknown) => {
        if (
          !isCurrentSearchRequest({
            activeRequestID: catalogRequestIDRef.current,
            responseRequestID: requestID,
          })
        ) {
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
      });
  }, [apiClient, onSessionExpired, runSearch]);

  const toggleUseful = useCallback(
    async (target: PresentedSearchResult) => {
      const targetNote = target.note;
      if (
        usefulMutations[targetNote.id] === 'pending' ||
        usefulPendingRef.current.has(targetNote.id)
      ) {
        return;
      }
      usefulPendingRef.current.add(targetNote.id);
      const generation = `${catalogRequestIDRef.current}:${searchRequestIDRef.current}`;
      setUsefulMutations((current) => ({
        ...current,
        [targetNote.id]: 'pending',
      }));
      try {
        if (targetNote.usefulByCurrentUser) {
          await apiClient.unmarkNoteUseful(targetNote.id);
        } else {
          await apiClient.markNoteUseful(targetNote.id);
        }
        lightTick();
        try {
          productEvents.record(
            targetNote.usefulByCurrentUser
              ? productEventKinds.noteUnmarkedUseful
              : productEventKinds.noteMarkedUseful,
            {
              context: {
                rank: target.rank,
                retrievalSource: target.retrievalSource,
                searchID: target.searchID,
                searchVersion: target.searchVersion,
                source: 'search',
              },
              noteID: targetNote.id,
            },
          );
        } catch {}
        if (
          generation !==
          `${catalogRequestIDRef.current}:${searchRequestIDRef.current}`
        ) {
          return;
        }
        setState((current) => {
          if (current.status !== 'ready') {
            return current;
          }
          return {
            ...current,
            results: current.results.map((result) =>
              result.note.id === targetNote.id
                ? {
                    ...result,
                    note: {
                      ...result.note,
                      usefulByCurrentUser: !result.note.usefulByCurrentUser,
                      usefulCount: result.note.usefulByCurrentUser
                        ? result.note.usefulCount - 1
                        : result.note.usefulCount + 1,
                    },
                  }
                : result,
            ),
          };
        });
        setUsefulMutations((current) => {
          const { [targetNote.id]: _removed, ...rest } = current;
          return rest;
        });
      } catch (error: unknown) {
        if (
          generation !==
          `${catalogRequestIDRef.current}:${searchRequestIDRef.current}`
        ) {
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
          [targetNote.id]: 'error',
        }));
      } finally {
        usefulPendingRef.current.delete(targetNote.id);
      }
    },
    [apiClient, onSessionExpired, productEvents, usefulMutations],
  );
  useFocusEffect(
    useCallback(() => {
      resetSearchLineage();
      loadCatalogs();

      return () => {
        catalogRequestIDRef.current += 1;
        searchRequestIDRef.current += 1;
        resetSearchLineage();
      };
    }, [loadCatalogs, resetSearchLineage]),
  );

  function handleQueryChange(value: string) {
    searchRequestIDRef.current += 1;
    submittedQueryRef.current = null;
    setUsefulMutations({});
    setQuery(value);
    setSearchState(idleSearchState);
  }

  function handleSubmit() {
    if (catalogState.status !== 'ready') {
      return;
    }

    runSearch(query, selectedCategorySlugRef.current);
  }

  function handleClear() {
    searchRequestIDRef.current += 1;
    resetSearchLineage();
    submittedQueryRef.current = null;
    setUsefulMutations({});
    setQuery('');
    setSearchState(idleSearchState);
  }

  function handleSelectCategorySlug(categorySlug: string | null) {
    if (selectedCategorySlugRef.current === categorySlug) {
      return;
    }

    selectedCategorySlugRef.current = categorySlug;
    setSelectedCategorySlug(categorySlug);

    if (submittedQueryRef.current !== null) {
      runSearch(submittedQueryRef.current, categorySlug);
    }
  }

  function handleSelectRecentQuery(recentQuery: string) {
    if (catalogState.status !== 'ready') {
      return;
    }

    runSearch(recentQuery, selectedCategorySlug);
  }

  return (
    <Screen scroll={false}>
      <View style={styles.header}>
        <View style={styles.searchRow}>
          <View style={styles.searchFieldSlot}>
            <SearchField
              value={query}
              onChangeText={handleQueryChange}
              onSubmit={handleSubmit}
              onClear={handleClear}
              placeholder="O que você tá procurando?"
              autoFocus={false}
              testID="search-field-input"
            />
          </View>
          <Button
            variant="ghost"
            label="Buscar"
            onPress={handleSubmit}
            disabled={catalogState.status !== 'ready'}
          />
        </View>
        <CategoryFilterControls
          catalog={catalogState.status === 'ready' ? catalogState.catalog : null}
          onSelectCategorySlug={handleSelectCategorySlug}
          selectedCategorySlug={selectedCategorySlug}
        />
      </View>
      <ScrollView
        style={styles.resultsScroll}
        contentContainerStyle={styles.resultsContent}
        keyboardShouldPersistTaps="handled"
      >
        {catalogState.status === 'error' ? (
          <CatalogError />
        ) : (
          <SearchStateContent
            catalogState={catalogState}
            columnWidth={columnWidth}
            onClearRecents={() => setRecentQueries([])}
            onCompose={() => router.push('/compose')}
            onOpenAuthor={openAuthor}
            onOpenNote={openNote}
            onPickCategory={handleSelectCategorySlug}
            onPickQuery={handleSelectRecentQuery}
            onToggleUseful={toggleUseful}
            recentQueries={recentQueries}
            state={state}
            usefulMutations={usefulMutations}
          />
        )}
      </ScrollView>
    </Screen>
  );
}
function SearchStateContent({
  catalogState,
  columnWidth,
  onClearRecents,
  onCompose,
  onOpenAuthor,
  onOpenNote,
  onPickCategory,
  onPickQuery,
  onToggleUseful,
  recentQueries,
  state,
  usefulMutations,
}: {
  catalogState: CatalogState;
  columnWidth: number;
  onClearRecents: () => void;
  onCompose: () => void;
  onOpenAuthor: (authorID: string) => void;
  onOpenNote: (result: PresentedSearchResult) => void;
  onPickCategory: (categorySlug: string) => void;
  onPickQuery: (query: string) => void;
  onToggleUseful: (result: PresentedSearchResult) => Promise<void>;
  recentQueries: string[];
  state: SearchScreenState;
  usefulMutations: Partial<Record<string, UsefulMutationState>>;
}) {
  if (state.status === 'idle') {
    return (
      <SearchIdle
        recentQueries={recentQueries}
        onPickQuery={onPickQuery}
        onClearRecents={onClearRecents}
        categories={
          catalogState.status === 'ready'
            ? catalogState.catalog.activeCategories
            : []
        }
        onPickCategory={onPickCategory}
      />
    );
  }

  if (state.status === 'loading') {
    const categoryLabel =
      catalogState.status === 'ready'
        ? selectedSearchCategory(catalogState.catalog, state.request.categorySlug)
            ?.label ?? null
        : null;
    return (
      <>
        <SearchFeedback categoryLabel={categoryLabel} query={state.request.query} />
        <View style={styles.skeletonRow}>
          <View style={styles.skeletonColumn}>
            <NoteCardSkeleton tall />
            <NoteCardSkeleton />
          </View>
          <View style={styles.skeletonColumn}>
            <NoteCardSkeleton />
            <NoteCardSkeleton tall />
          </View>
        </View>
      </>
    );
  }

  if (state.status === 'error') {
    return (
      <EmptyState
        title="Não deu pra buscar"
        body={`Mantivemos "${state.request.query}" aqui. Confere sua conexão e tenta de novo.`}
      />
    );
  }

  if (state.status === 'empty') {
    return (
      <>
        <SearchFeedback
          categoryLabel={state.context.categoryLabel}
          countLabel={searchResultCountLabel(state.context.resultCount)}
          query={state.context.query}
        />
        <EmptyState
          title="Nada por aqui ainda"
          body="Que tal escrever o primeiro achado sobre isso?"
          action={{ label: 'Escrever', onPress: onCompose }}
        />
      </>
    );
  }

  return (
    <>
      <SearchFeedback
        categoryLabel={state.context.categoryLabel}
        countLabel={searchResultCountLabel(state.context.resultCount)}
        query={state.context.query}
      />
      <MasonryGrid
        items={state.results}
        keyFor={(result) => result.note.id}
        estimateHeight={(result) =>
          estimateNoteCardHeight(result.note, columnWidth)
        }
        renderItem={(result) => (
          <NoteCard
            categoryLabel={result.note.categoryLabel}
            key={result.note.id}
            note={result.note}
            onPress={() => onOpenNote(result)}
            onPressAuthor={() => onOpenAuthor(result.note.author.id)}
            onPressUseful={() => {
              void onToggleUseful(result);
            }}
            usefulError={
              usefulMutations[result.note.id] === 'error'
                ? NOTE_USEFUL_ERROR_MESSAGE
                : null
            }
            usefulPending={usefulMutations[result.note.id] === 'pending'}
          />
        )}
      />
    </>
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

function SearchFeedback({
  categoryLabel,
  countLabel,
  query,
}: {
  categoryLabel: string | null;
  countLabel?: string;
  query: string;
}) {
  const categorySuffix = categoryLabel === null ? '' : ` · ${categoryLabel}`;
  return (
    <View
      accessible
      accessibilityLabel={`${countLabel ?? 'Buscando'} para ${query}${categoryLabel === null ? '' : `, categoria ${categoryLabel}`}.`}
      style={styles.feedback}
    >
      <AppText variant="sm" color={semanticColors.textMuted}>
        {countLabel === undefined ? (
          'Buscando'
        ) : (
          <AppText variant="sm" weight="bold" color={semanticColors.textMuted}>
            {countLabel}
          </AppText>
        )}
        {` para "${query}"`}
        {categorySuffix}
      </AppText>
    </View>
  );
}
