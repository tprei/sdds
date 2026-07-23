import { useCallback, useRef, useState } from 'react';
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
import { requestStatus } from '@/lib/api/request-error';
import { unauthorizedStatus } from '@/lib/api/status';
import { CategoryFilterControls } from '@/features/notes/category-filter-controls';
import { resolveCategoryFilterSlug } from '@/features/notes/category-filter';
import { buildNoteCatalog, labelNotes } from '@/features/notes/catalog';
import type { LabelledNote, NoteCatalog } from '@/features/notes/catalog';
import {
  appendRecentSearchQuery,
  createSearchRequest,
  isCurrentSearchRequest,
  searchResultContext,
  searchResultCountLabel,
} from '@/features/notes/search-screen';
import type {
  SearchRequest,
  SearchResultContext,
} from '@/features/notes/search-screen';
import { listCatalogs } from '@/lib/api/catalogs';
import { searchNotes } from '@/lib/api/notes';
import type { Note } from '@/lib/api/notes';

import { styles } from '@/features/notes/search-screen.styles';

type CatalogState =
  | { status: 'loading' }
  | { status: 'ready'; catalog: NoteCatalog }
  | { status: 'error' };

type SearchScreenState =
  | { status: 'idle' }
  | { request: SearchRequest; status: 'loading' }
  | {
      context: SearchResultContext;
      notes: LabelledNote[];
      request: SearchRequest;
      status: 'ready';
    }
  | {
      context: SearchResultContext;
      request: SearchRequest;
      status: 'empty';
    }
  | { request: SearchRequest; status: 'error' };

const idleSearchState: SearchScreenState = { status: 'idle' };

type AuthenticatedSearchScreenProps = {
  onSessionExpired: () => Promise<void>;
  token: string;
};

export default function SearchScreen() {
  const { logout, state } = useAuth();
  const router = useRouter();

  if (state.status === 'authenticated') {
    return (
      <AuthenticatedSearchScreen
        key={state.user.id}
        onSessionExpired={logout}
        token={state.token}
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
  onSessionExpired,
  token,
}: AuthenticatedSearchScreenProps) {
  const router = useRouter();
  const catalogRequestIDRef = useRef(0);
  const searchRequestIDRef = useRef(0);
  const catalogRef = useRef<NoteCatalog | null>(null);
  const selectedCategorySlugRef = useRef<string | null>(null);
  const stateRef = useRef<SearchScreenState>(idleSearchState);
  const submittedQueryRef = useRef<string | null>(null);
  const [query, setQuery] = useState('');
  const [selectedCategorySlug, setSelectedCategorySlug] = useState<
    string | null
  >(null);
  const [recentQueries, setRecentQueries] = useState<string[]>([]);
  const [catalogState, setCatalogState] = useState<CatalogState>({
    status: 'loading',
  });
  const [state, setState] = useState<SearchScreenState>(idleSearchState);
  const openAuthor = useCallback(
    (authorID: string) => {
      router.push({ pathname: '/authors/[id]', params: { id: authorID } });
    },
    [router],
  );
  const openNote = useCallback(
    (note: Note) => {
      router.push({
        pathname: '/notes/[id]',
        params: { id: note.id },
      });
    },
    [router],
  );
  const setSearchState = useCallback((nextState: SearchScreenState) => {
    stateRef.current = nextState;
    setState(nextState);
  }, []);

  const runSearch = useCallback(
    (queryValue: string, categorySlug: string | null) => {
      const request = createSearchRequest({
        categorySlug,
        nextRequestID: searchRequestIDRef.current + 1,
        query: queryValue,
      });
      searchRequestIDRef.current += 1;

      if (request === null) {
        submittedQueryRef.current = null;
        setSearchState(idleSearchState);
        return;
      }

      const catalog = catalogRef.current;
      if (catalog === null) {
        setSearchState({ request, status: 'error' });
        return;
      }

      submittedQueryRef.current = request.query;
      setQuery(request.query);
      setRecentQueries((current) =>
        appendRecentSearchQuery(current, request.query),
      );
      setSearchState({ request, status: 'loading' });

      searchNotes(request.input, token)
        .then((notes) => {
          if (
            !isCurrentSearchRequest({
              activeRequestID: searchRequestIDRef.current,
              responseRequestID: request.id,
            })
          ) {
            return;
          }

          const labelledNotes = labelNotes(catalog, notes);
          if (labelledNotes === null) {
            setSearchState({ request, status: 'error' });
            return;
          }

          const context = searchResultContext({
            catalog,
            categorySlug: request.categorySlug,
            query: request.query,
            resultCount: labelledNotes.length,
          });
          setSearchState(
            labelledNotes.length > 0
              ? { context, notes: labelledNotes, request, status: 'ready' }
              : { context, request, status: 'empty' },
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
    [onSessionExpired, setSearchState, token],
  );

  const loadCatalogs = useCallback(() => {
    catalogRequestIDRef.current += 1;
    const requestID = catalogRequestIDRef.current;
    setCatalogState({ status: 'loading' });

    listCatalogs(token)
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
  }, [onSessionExpired, runSearch, token]);

  useFocusEffect(
    useCallback(() => {
      loadCatalogs();

      return () => {
        catalogRequestIDRef.current += 1;
        searchRequestIDRef.current += 1;
      };
    }, [loadCatalogs]),
  );

  function handleQueryChange(value: string) {
    searchRequestIDRef.current += 1;
    submittedQueryRef.current = null;
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
    submittedQueryRef.current = null;
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
          recentQueries={recentQueries}
          state={state}
        />
      )}
    </FoundationScreen>
  );
}

function ReadAuthGate({
  onLogin,
  onSignup,
  status,
}: {
  onLogin: () => void;
  onSignup: () => void;
  status: 'anonymous' | 'error' | 'loading';
}) {
  if (status === 'loading') {
    return (
      <EmptyStateCard
        title="Conferindo sua sessão"
        body="A gente já libera o formulário se você estiver com uma conta ativa."
      />
    );
  }

  if (status === 'error') {
    return (
      <>
        <EmptyStateCard
          title="Não deu pra confirmar sua sessão"
          body="Verifique sua conexão e entre de novo para publicar."
        />
        <FoundationButton label="Entrar" onPress={onLogin} />
      </>
    );
  }

  return (
    <>
      <EmptyStateCard
        title="Entre para continuar"
        body="Entre ou crie uma conta para acessar as notas."
      />
      <FoundationButton label="Criar conta" onPress={onSignup} />
      <FoundationButton label="Entrar" onPress={onLogin} />
    </>
  );
}

function SearchStateContent({
  onOpenAuthor,
  onOpenNote,
  onSelectRecentQuery,
  recentQueries,
  state,
}: {
  onOpenAuthor: (authorID: string) => void;
  onOpenNote: (note: Note) => void;
  onSelectRecentQuery: (query: string) => void;
  recentQueries: string[];
  state: SearchScreenState;
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
      {state.notes.map((labelledNote) => (
        <NoteCard
          categoryLabel={labelledNote.categoryLabel}
          key={labelledNote.id}
          note={labelledNote}
          onPress={() => onOpenNote(labelledNote)}
          onPressAuthor={onOpenAuthor}
          placeLabel={labelledNote.placeLabel}
        />
      ))}
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
