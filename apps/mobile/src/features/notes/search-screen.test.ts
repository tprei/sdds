import { describe, expect, it } from 'vitest';

import { buildNoteCatalog } from './catalog';
import {
  appendRecentSearchQuery,
  createSearchRequest,
  isCurrentSearchRequest,
  labelSearchResults,
  presentSearchResults,
  resolveSearchCategorySlug,
  searchNotesInput,
  searchRecentQueryLimit,
  searchResultContext,
  searchResultCountLabel,
  selectedSearchCategory,
  submittedSearchQuery,
} from './search-screen';
import type { Catalogs } from '@/lib/api/catalogs';
import type { Note } from '@/lib/api/notes';

describe('search screen helpers', () => {
  it('normalizes submitted search text', () => {
    expect(submittedSearchQuery('  café brasileiro  ')).toBe(
      'café brasileiro',
    );
    expect(submittedSearchQuery('   ')).toBeNull();
  });

  it('creates search requests without category filters by default', () => {
    expect(
      createSearchRequest({
        categorySlug: null,
        nextRequestID: 3,
        occurredAt: 1782993600000,
        previousSearch: null,
        query: '  café brasileiro  ',
        searchID: '018ff5b8-0000-7000-8000-000000000001',
      }),
    ).toEqual({
      categorySlug: null,
      id: 3,
      input: { query: 'café brasileiro' },
      occurredAt: 1782993600000,
      previousSearch: null,
      query: 'café brasileiro',
      searchID: '018ff5b8-0000-7000-8000-000000000001',
    });
  });

  it('creates search requests with selected category filters', () => {
    expect(searchNotesInput('café brasileiro', 'food')).toEqual({
      categorySlug: 'food',
      query: 'café brasileiro',
    });
  });

  it('ignores blank search submissions', () => {
    expect(
      createSearchRequest({
        categorySlug: 'food',
        nextRequestID: 4,
        occurredAt: 1782993600000,
        previousSearch: null,
        query: '   ',
        searchID: '018ff5b8-0000-7000-8000-000000000002',
      }),
    ).toBeNull();
  });

  it('rejects stale search responses', () => {
    expect(
      isCurrentSearchRequest({
        activeRequestID: 7,
        responseRequestID: 6,
      }),
    ).toBe(false);
    expect(
      isCurrentSearchRequest({
        activeRequestID: 7,
        responseRequestID: 7,
      }),
    ).toBe(true);
  });

  it('tracks recent session searches newest first', () => {
    const recentQueries = appendRecentSearchQuery(
      ['marmita barata', 'café brasileiro'],
      '  cabelo cacheado  ',
    );

    expect(recentQueries).toEqual([
      'cabelo cacheado',
      'marmita barata',
      'café brasileiro',
    ]);
  });

  it('deduplicates recent session searches by query text', () => {
    expect(
      appendRecentSearchQuery(
        ['Café brasileiro', 'marmita barata'],
        'café brasileiro',
      ),
    ).toEqual(['café brasileiro', 'marmita barata']);
  });

  it('limits recent session searches', () => {
    const recentQueries = appendRecentSearchQuery(
      ['one', 'two', 'three', 'four', 'five'],
      'six',
    );

    expect(recentQueries).toHaveLength(searchRecentQueryLimit);
    expect(recentQueries).toEqual(['six', 'one', 'two', 'three', 'four']);
  });

  it('keeps active selected categories after catalog refreshes', () => {
    expect(
      resolveSearchCategorySlug(buildNoteCatalog(catalogs()), 'food'),
    ).toBe('food');
  });

  it('clears inactive selected categories after catalog refreshes', () => {
    expect(
      resolveSearchCategorySlug(buildNoteCatalog(catalogs()), 'travel'),
    ).toBeNull();
  });

  it('resolves the active category for result context', () => {
    const catalog = buildNoteCatalog(catalogs());

    expect(selectedSearchCategory(catalog, 'food')).toMatchObject({
      label: 'Comida',
      slug: 'food',
    });
    expect(selectedSearchCategory(catalog, null)).toBeNull();
  });

  it('builds result context with count, category, and global scope', () => {
    expect(
      searchResultContext({
        catalog: buildNoteCatalog(catalogs()),
        categorySlug: 'food',
        query: 'café brasileiro',
        resultCount: 2,
      }),
    ).toEqual({
      categoryLabel: 'Comida',
      query: 'café brasileiro',
      resultCount: 2,
      scopeLabel: 'Mundo todo',
    });
  });

  it('labels ordered search results without losing retrieval provenance', () => {
    const labelledResults = labelSearchResults(
      buildNoteCatalog(catalogs()),
      [
        {
          note: searchNote('first'),
          retrievalSource: 'semantic',
        },
        {
          note: searchNote('second'),
          retrievalSource: 'lexical',
        },
      ],
    );

    expect(
      labelledResults?.map((result) => ({
        id: result.note.id,
        retrievalSource: result.retrievalSource,
      })),
    ).toEqual([
      { id: 'first', retrievalSource: 'semantic' },
      { id: 'second', retrievalSource: 'lexical' },
    ]);
  });
  it('keeps immutable search lineage on rendered results', () => {
    const request = createSearchRequest({
      categorySlug: 'food',
      nextRequestID: 2,
      occurredAt: 1782993600000,
      previousSearch: {
        categorySlug: null,
        query: 'café',
        searchID: '018ff5b8-0000-7000-8000-000000000001',
      },
      query: 'pão',
      searchID: '018ff5b8-0000-7000-8000-000000000002',
    });
    if (request === null) {
      throw new Error('request_missing');
    }

    const labelledResults = labelSearchResults(
      buildNoteCatalog(catalogs()),
      [
        { note: searchNote('first'), retrievalSource: 'lexical' },
        { note: searchNote('second'), retrievalSource: 'semantic' },
      ],
    );
    if (labelledResults === null) {
      throw new Error('results_missing');
    }

    const presented = presentSearchResults({
      request,
      results: labelledResults,
      searchVersion: 'fts5-v1',
    });

    expect(
      presented.map((result) => ({
        id: result.note.id,
        rank: result.rank,
        searchID: result.searchID,
        searchVersion: result.searchVersion,
        retrievalSource: result.retrievalSource,
      })),
    ).toEqual([
      {
        id: 'first',
        rank: 1,
        searchID: request.searchID,
        searchVersion: 'fts5-v1',
        retrievalSource: 'lexical',
      },
      {
        id: 'second',
        rank: 2,
        searchID: request.searchID,
        searchVersion: 'fts5-v1',
        retrievalSource: 'semantic',
      },
    ]);
  });
  it('labels singular and plural result counts', () => {
    expect(searchResultCountLabel(1)).toBe('1 nota');
    expect(searchResultCountLabel(2)).toBe('2 notas');
  });
});

function searchNote(id: string): Note {
  return {
    author: { displayName: 'Thiago', id: 'author-id' },
    body: 'Tem pao.',
    categorySlug: 'food',
    createdAt: 1782993600000,
    id,
    images: [],
    placeSlug: null,
    title: 'Cafe bom',
    updatedAt: 1782993600000,
    usefulCount: 0,
    usefulByCurrentUser: false,
  };
}
function catalogs(): Catalogs {
  return {
    categories: [
      {
        active: true,
        displayOrder: 10,
        label: 'Comida',
        slug: 'food',
      },
      {
        active: false,
        displayOrder: 20,
        label: 'Viagem',
        slug: 'travel',
      },
    ],
    places: [],
  };
}
