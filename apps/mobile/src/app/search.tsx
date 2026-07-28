import * as Crypto from 'expo-crypto';
import { useCallback, useEffect, useRef, useState } from 'react';
import { Pressable, Text, View } from 'react-native';
import { useFocusEffect, useRouter } from 'expo-router';

import {
  EmptyStateCard,
  FoundationButton,
  FoundationScreen,
  FoundationTextInput,
} from '@/components/foundation-screen';
import { NoteCard } from '@/components/note-card';
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
import { registerPresentedNoteOrigin } from '@/features/notes/presented-note-origin';

import { styles } from '@/features/notes/search-screen.styles';
import { ReadAuthGate } from '@/components/read-auth-gate';

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
    <FoundationScreen
      eyebrow="Buscar"
      title="Buscar"
      description="Ache notas, produtos, lugares e dicas."
    >
      <ReadAuthGate
        onLogin={() =>
          router.push({ pathname: '/login', params: { next: '/search' } })
        }
        onSignup={() =>
          router.push({ pathname: '/signup', params: { next: '/search' } })
        }
        status={state.status}
      />
    </FoundationScreen>
  );
}

function AuthenticatedSearchScreen({
  apiClient,
  onSessionExpired,
}: AuthenticatedSearchScreenProps) {
  const router = useRouter();
  const productEvents = useProductEvents();
  const catalogRequestIDRef = useRef(0);
  const searchRequestIDRef = useRef(0);
  const catalogRef = useRef<NoteCatalog | null>(null);
  const selectedCategorySlugRef = useRef<string | null>(null);
  const stateRef = useRef<SearchScreenState>(idleSearchState);
  const submittedQueryRef = useRef<string | null>(null);
  const previousSearchRef = useRef<SearchDispatchedContext | null>(null);
  const searchExecutionsRef = useRef(new Map<string, SearchExecution>());
  const impressionSearchIDRef = useRef<string | null>(null);
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
      try {
        productEvents.record('search_result_opened', {
          noteID: result.note.id,
          rank: result.rank,
          retrievalSource: result.retrievalSource,
          searchID: result.searchID,
          searchVersion: result.searchVersion,
        });
      } catch {}
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
      try {
        productEvents.record(
          'search_submitted',
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
            'search_reformulated',
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
    impressionSearchIDRef.current = searchID;

    try {
      if (state.status === 'empty') {
        productEvents.record('search_no_results', {
          categorySlug: state.request.categorySlug,
          query: state.request.query,
          resultCount: 0,
          searchID,
          searchVersion: state.searchVersion,
        });
      } else {
        productEvents.record('search_results_impression', {
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
    } catch {}
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
    setCatalogState({ status: 'loading' });
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
        try {
          productEvents.record(
            targetNote.usefulByCurrentUser
              ? 'note_unmarked_useful'
              : 'note_marked_useful',
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
    <FoundationScreen
      eyebrow="Buscar"
      title="Buscar"
      description="Ache notas, produtos, lugares e dicas."
    >
      <FoundationTextInput
        accessibilityLabel="Buscar"
        onChangeText={handleQueryChange}
        onSubmitEditing={handleSubmit}
        placeholder="Buscar uma dica"
        returnKeyType="search"
        value={query}
      />
      <View style={styles.actionRow}>
        <FoundationButton
          disabled={
            state.status === 'loading' || catalogState.status === 'loading'
          }
          label={state.status === 'loading' ? 'Buscando...' : 'Buscar'}
          onPress={handleSubmit}
        />
        {query.length === 0 ? null : (
          <FoundationButton
            label="Limpar"
            onPress={handleClear}
            style={styles.secondaryButton}
          />
        )}
      </View>
      <CategoryFilterControls
        catalog={catalogState.status === 'ready' ? catalogState.catalog : null}
        onSelectCategorySlug={handleSelectCategorySlug}
        selectedCategorySlug={selectedCategorySlug}
      />
      {catalogState.status === 'error' ? (
        <CatalogError />
      ) : (
        <SearchStateContent
          onOpenAuthor={openAuthor}
          onOpenNote={openNote}
          onSelectRecentQuery={handleSelectRecentQuery}
          onToggleUseful={toggleUseful}
          recentQueries={recentQueries}
          state={state}
          usefulMutations={usefulMutations}
        />
      )}
    </FoundationScreen>
  );
}
function SearchStateContent({
  onOpenAuthor,
  onOpenNote,
  onSelectRecentQuery,
  onToggleUseful,
  recentQueries,
  state,
  usefulMutations,
}: {
  onOpenAuthor: (authorID: string) => void;
  onOpenNote: (result: PresentedSearchResult) => void;
  onSelectRecentQuery: (query: string) => void;
  onToggleUseful: (result: PresentedSearchResult) => Promise<void>;
  recentQueries: string[];
  state: SearchScreenState;
  usefulMutations: Partial<Record<string, UsefulMutationState>>;
}) {
  if (state.status === 'idle') {
    return (
      <>
        <RecentSearches
          onSelectRecentQuery={onSelectRecentQuery}
          recentQueries={recentQueries}
        />
        <EmptyStateCard
          title="Nada pesquisado ainda"
          body="Comece por uma dica, produto, bairro ou dúvida."
        />
      </>
    );
  }

  if (state.status === 'loading') {
    return (
      <EmptyStateCard
        title="Buscando notas"
        body={`Procurando achados para "${state.request.query}" no Mundo todo.`}
      />
    );
  }

  if (state.status === 'empty') {
    return (
      <>
        <ResultHeader context={state.context} />
        <EmptyStateCard
          title="Nada por aqui ainda"
          body={`Que tal escrever a primeira nota útil sobre "${state.request.query}"?`}
        />
      </>
    );
  }

  if (state.status === 'error') {
    return (
      <EmptyStateCard
        title="Não deu pra buscar"
        body={`Mantive "${state.request.query}" aqui. Confere sua conexão e tenta de novo.`}
      />
    );
  }

  return (
    <>
      <ResultHeader context={state.context} />
      {state.results.map((result) => {
        const labelledNote = result.note;
        return (
          <NoteCard
            categoryLabel={labelledNote.categoryLabel}
            key={labelledNote.id}
            note={labelledNote}
            onPress={() => onOpenNote(result)}
            onPressAuthor={onOpenAuthor}
            onPressUseful={() => {
              void onToggleUseful(result);
            }}
            placeLabel={labelledNote.placeLabel}
            usefulError={usefulMutations[labelledNote.id] === 'error'}
            usefulPending={usefulMutations[labelledNote.id] === 'pending'}
          />
        );
      })}
    </>
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

function RecentSearches({
  onSelectRecentQuery,
  recentQueries,
}: {
  onSelectRecentQuery: (query: string) => void;
  recentQueries: string[];
}) {
  if (recentQueries.length === 0) {
    return null;
  }

  return (
    <View style={styles.recentSection}>
      <Text style={styles.sectionLabel}>Pesquisas desta sessão</Text>
      <View style={styles.recentRow}>
        {recentQueries.map((recentQuery) => (
          <Pressable
            accessibilityRole="button"
            key={recentQuery}
            onPress={() => onSelectRecentQuery(recentQuery)}
            style={({ pressed }) => [
              styles.recentButton,
              pressed ? styles.recentButtonPressed : null,
            ]}
          >
            <Text style={styles.recentButtonText}>{recentQuery}</Text>
          </Pressable>
        ))}
      </View>
    </View>
  );
}

function ResultHeader({ context }: { context: SearchResultContext }) {
  const contextParts =
    context.categoryLabel === null
      ? [context.scopeLabel]
      : [`Categoria ${context.categoryLabel}`, context.scopeLabel];

  return (
    <View
      accessible
      accessibilityLabel={`Resultado da busca: ${searchResultCountLabel(context.resultCount)} para ${context.query}. ${contextParts.join(', ')}.`}
      style={styles.resultHeader}
    >
      <Text style={styles.resultTitle}>
        {searchResultCountLabel(context.resultCount)} para {context.query}
      </Text>
      <Text style={styles.resultMeta}>{contextParts.join(' · ')}</Text>
    </View>
  );
}
