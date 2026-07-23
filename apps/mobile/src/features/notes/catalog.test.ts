import { describe, expect, it } from 'vitest';

import {
  buildNoteCatalog,
  labelNote,
  resolveSelectedCategorySlug,
  resolveSelectedPlaceSlug,
} from './catalog';
import type { Catalogs } from '@/lib/api/catalogs';
import type { Note } from '@/lib/api/notes';

describe('note catalog helpers', () => {
  it('labels notes without nesting the note payload', () => {
    const catalog = buildNoteCatalog(catalogs());
    const note = apiNote();

    expect(labelNote(catalog, note)).toEqual({
      ...note,
      categoryLabel: 'Comida',
      placeLabel: 'São Paulo',
    });
  });

  it('preserves active compose selections across catalog refreshes', () => {
    const catalog = buildNoteCatalog(catalogs());

    expect(resolveSelectedCategorySlug(catalog, 'travel')).toBe('travel');
    expect(resolveSelectedPlaceSlug(catalog, 'sao-paulo')).toBe('sao-paulo');
  });

  it('falls back when compose selections are no longer active', () => {
    const catalog = buildNoteCatalog(catalogs());

    expect(resolveSelectedCategorySlug(catalog, 'beauty')).toBe('food');
    expect(resolveSelectedPlaceSlug(catalog, 'rio-de-janeiro')).toBeNull();
  });
});

function catalogs(): Catalogs {
  return {
    categories: [
      {
        active: false,
        displayOrder: 10,
        label: 'Beleza',
        slug: 'beauty',
      },
      {
        active: true,
        displayOrder: 20,
        label: 'Comida',
        slug: 'food',
      },
      {
        active: true,
        displayOrder: 30,
        label: 'Viagem',
        slug: 'travel',
      },
    ],
    places: [
      {
        active: true,
        displayOrder: 10,
        label: 'São Paulo',
        slug: 'sao-paulo',
      },
      {
        active: false,
        displayOrder: 20,
        label: 'Rio de Janeiro',
        slug: 'rio-de-janeiro',
      },
    ],
  };
}

function apiNote(): Note {
  return {
    author: {
      displayName: 'Thiago',
      id: 'author-id',
    },
    body: 'Tem pão de queijo decente.',
    categorySlug: 'food',
    createdAt: 1782993600000,
    id: 'note-1',
    images: [],
    placeSlug: 'sao-paulo',
    title: 'Café bom',
    updatedAt: 1782993600000,
    usefulCount: 0,
    usefulByCurrentUser: false,
  };
}
