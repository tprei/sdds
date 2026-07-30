import { describe, expect, it } from 'vitest';

import { categoryColors } from '@sdds/tokens';

import {
  buildNoteCatalog,
  categoryHue,
  categoryLabel,
  labelNote,
  resolveSelectedCategorySlug,
} from './catalog';
import type { NoteCatalog } from './catalog';
import type { Catalogs } from '@/lib/api/catalogs';
import type { Note } from '@/lib/api/notes';

describe('note catalog helpers', () => {
  it('labels notes without nesting the note payload', () => {
    const catalog = builtCatalog();
    const note = apiNote();

    expect(labelNote(catalog, note)).toEqual({
      ...note,
      categoryHue: categoryColors.food,
      categoryLabel: 'Comida',
    });
  });

  it('preserves active compose selections across catalog refreshes', () => {
    const catalog = builtCatalog();

    expect(resolveSelectedCategorySlug(catalog, 'travel')).toBe('travel');
  });

  it('falls back when compose selections are no longer active', () => {
    const catalog = builtCatalog();

    expect(resolveSelectedCategorySlug(catalog, 'beauty')).toBe('food');
  });

  it('fails the catalog when an active category has no configured hue', () => {
    const catalog = buildNoteCatalog({
      categories: [
        {
          active: true,
          displayOrder: 10,
          label: 'Bem-estar',
          slug: 'wellness',
        },
      ],
    });

    expect(catalog).toBeNull();
  });

  it('spares the catalog when an inactive category has no configured hue', () => {
    const catalog = buildNoteCatalog({
      categories: [
        {
          active: false,
          displayOrder: 10,
          label: 'Bem-estar',
          slug: 'wellness',
        },
      ],
    });
    if (catalog === null) {
      throw new Error('inactive hueless category must not fail the catalog');
    }

    expect(catalog.activeCategories).toEqual([]);
    expect(categoryHue(catalog, 'wellness')).toBeNull();
    expect(categoryLabel(catalog, 'wellness')).toBe('Bem-estar');
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
    title: 'Café bom',
    updatedAt: 1782993600000,
    usefulCount: 0,
    usefulByCurrentUser: false,
  };
}

// Every category in the shared fixture above resolves a hue; this only
// throws if that fixture regresses the invariant the other tests rely on.
function builtCatalog(): NoteCatalog {
  const catalog = buildNoteCatalog(catalogs());
  if (catalog === null) {
    throw new Error('test catalog fixture must resolve every active category hue');
  }
  return catalog;
}
