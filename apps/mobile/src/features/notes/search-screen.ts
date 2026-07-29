import { labelNote } from './catalog';
import type {
  LabelledNote,
  NoteCatalog,
} from './catalog';
import type { CatalogCategory } from '@/lib/api/catalogs';
import type {
  RetrievalSource,
  SearchNoteResult,
  SearchNotesInput,
  SearchVersion,
} from '@/lib/api/notes';

export const searchRecentQueryLimit = 5;

export type SearchDispatchedContext = {
  categorySlug: string | null;
  query: string;
  searchID: string;
};
export type SearchRequest = {
  categorySlug: string | null;
  id: number;
  input: SearchNotesInput;
  occurredAt: number;
  previousSearch: SearchDispatchedContext | null;
  query: string;
  searchID: string;
};

export type SearchResultContext = {
  categoryLabel: string | null;
  query: string;
  resultCount: number;
};

export type LabelledSearchResult = {
  note: LabelledNote;
  retrievalSource: RetrievalSource;
};
export type PresentedSearchResult = LabelledSearchResult & {
  rank: number;
  searchID: string;
  searchVersion: SearchVersion;
};

export function labelSearchResults(
  catalog: NoteCatalog,
  results: readonly SearchNoteResult[],
): LabelledSearchResult[] | null {
  const labelledResults: LabelledSearchResult[] = [];
  for (const result of results) {
    const labelledNote = labelNote(catalog, result.note);
    if (labelledNote === null) {
      return null;
    }
    labelledResults.push({
      note: labelledNote,
      retrievalSource: result.retrievalSource,
    });
  }
  return labelledResults;
}

export function presentSearchResults({
  request,
  results,
  searchVersion,
}: {
  request: SearchRequest;
  results: readonly LabelledSearchResult[];
  searchVersion: SearchVersion;
}): PresentedSearchResult[] {
  return results.map((result, index) => ({
    ...result,
    rank: index + 1,
    searchID: request.searchID,
    searchVersion,
  }));
}

export function createSearchRequest({
  categorySlug,
  nextRequestID,
  occurredAt,
  previousSearch,
  query,
  searchID,
}: {
  categorySlug: string | null;
  nextRequestID: number;
  occurredAt: number;
  previousSearch: SearchDispatchedContext | null;
  query: string;
  searchID: string;
}): SearchRequest | null {
  const submittedQuery = submittedSearchQuery(query);
  if (submittedQuery === null) {
    return null;
  }

  return {
    categorySlug,
    id: nextRequestID,
    input: searchNotesInput(submittedQuery, categorySlug),
    occurredAt,
    previousSearch,
    query: submittedQuery,
    searchID,
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
  };
}

export function searchResultCountLabel(resultCount: number): string {
  const noun = resultCount === 1 ? 'achado' : 'achados';
  return `${resultCount} ${noun}`;
}

function searchQueryKey(query: string): string {
  return query.trim().toLocaleLowerCase('pt-BR');
}
