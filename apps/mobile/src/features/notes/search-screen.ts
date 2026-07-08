import type { NoteCatalog } from './catalog';
import type { CatalogCategory } from '@/lib/api/catalogs';
import type { SearchNotesInput } from '@/lib/api/notes';

export const searchScopeLabel = 'Mundo todo';
export const searchRecentQueryLimit = 5;

export type SearchRequest = {
  categorySlug: string | null;
  id: number;
  input: SearchNotesInput;
  query: string;
};

export type SearchResultContext = {
  categoryLabel: string | null;
  query: string;
  resultCount: number;
  scopeLabel: typeof searchScopeLabel;
};

export function createSearchRequest({
  categorySlug,
  nextRequestID,
  query,
}: {
  categorySlug: string | null;
  nextRequestID: number;
  query: string;
}): SearchRequest | null {
  const submittedQuery = submittedSearchQuery(query);
  if (submittedQuery === null) {
    return null;
  }

  return {
    categorySlug,
    id: nextRequestID,
    input: searchNotesInput(submittedQuery, categorySlug),
    query: submittedQuery,
  };
}

export function submittedSearchQuery(query: string): string | null {
  const submittedQuery = query.trim();
  return submittedQuery.length > 0 ? submittedQuery : null;
}

export function searchNotesInput(
  query: string,
  categorySlug: string | null,
): SearchNotesInput {
  if (categorySlug === null) {
    return { query };
  }

  return { categorySlug, query };
}

export function isCurrentSearchRequest({
  activeRequestID,
  responseRequestID,
}: {
  activeRequestID: number;
  responseRequestID: number;
}): boolean {
  return activeRequestID === responseRequestID;
}

export function appendRecentSearchQuery(
  recentQueries: readonly string[],
  query: string,
): string[] {
  const submittedQuery = submittedSearchQuery(query);
  if (submittedQuery === null) {
    return [...recentQueries];
  }

  const submittedKey = searchQueryKey(submittedQuery);
  const dedupedQueries = recentQueries.filter(
    (recentQuery) => searchQueryKey(recentQuery) !== submittedKey,
  );

  return [submittedQuery, ...dedupedQueries].slice(0, searchRecentQueryLimit);
}

export function resolveSearchCategorySlug(
  catalog: NoteCatalog,
  currentSlug: string | null,
): string | null {
  if (currentSlug === null) {
    return null;
  }

  return catalog.activeCategories.some(
    (category) => category.slug === currentSlug,
  )
    ? currentSlug
    : null;
}

export function selectedSearchCategory(
  catalog: NoteCatalog,
  categorySlug: string | null,
): CatalogCategory | null {
  if (categorySlug === null) {
    return null;
  }

  return (
    catalog.activeCategories.find(
      (category) => category.slug === categorySlug,
    ) ?? null
  );
}

export function searchResultContext({
  catalog,
  categorySlug,
  query,
  resultCount,
}: {
  catalog: NoteCatalog;
  categorySlug: string | null;
  query: string;
  resultCount: number;
}): SearchResultContext {
  return {
    categoryLabel: selectedSearchCategory(catalog, categorySlug)?.label ?? null,
    query,
    resultCount,
    scopeLabel: searchScopeLabel,
  };
}

export function searchResultCountLabel(resultCount: number): string {
  const noun = resultCount === 1 ? 'nota' : 'notas';
  return `${resultCount} ${noun}`;
}

function searchQueryKey(query: string): string {
  return query.trim().toLocaleLowerCase('pt-BR');
}
