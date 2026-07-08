import { describe, expect, it } from 'vitest';

import { buildNoteCatalog } from './catalog';
import {
  categoryFilterChipAccessibility,
  resolveCategoryFilterSlug,
} from './category-filter';
import type { Catalogs } from '@/lib/api/catalogs';

describe('category filter controls', () => {
  it('marks selected category chips for accessibility', () => {
    expect(categoryFilterChipAccessibility('Tudo', true)).toEqual({
      accessibilityLabel: 'Tudo, selecionado',
      accessibilityState: { selected: true },
    });
  });

  it('keeps unselected category chips unmarked', () => {
    expect(categoryFilterChipAccessibility('Comida', false)).toEqual({
      accessibilityLabel: 'Comida',
      accessibilityState: { selected: false },
    });
  });

  it('keeps active selected categories after catalog refreshes', () => {
    expect(resolveCategoryFilterSlug(buildNoteCatalog(catalogs()), 'food')).toBe(
      'food',
    );
  });

  it('clears inactive selected categories after catalog refreshes', () => {
    expect(
      resolveCategoryFilterSlug(buildNoteCatalog(catalogs()), 'travel'),
    ).toBeNull();
  });
});

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
