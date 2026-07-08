import { describe, expect, it } from 'vitest';

import { buildNoteCatalog } from './catalog';
import {
  categoryChipAccessibility,
  resolveExploreCategorySlug,
} from './explore-screen';
import type { Catalogs } from '@/lib/api/catalogs';

describe('explore screen helpers', () => {
  it('marks selected category chips for accessibility', () => {
    expect(categoryChipAccessibility('Tudo', true)).toEqual({
      accessibilityLabel: 'Tudo, selecionado',
      accessibilityState: { selected: true },
    });
  });

  it('keeps unselected category chips unmarked', () => {
    expect(categoryChipAccessibility('Comida', false)).toEqual({
      accessibilityLabel: 'Comida',
      accessibilityState: { selected: false },
    });
  });

  it('keeps active selected categories after catalog refreshes', () => {
    expect(
      resolveExploreCategorySlug(buildNoteCatalog(catalogs()), 'food'),
    ).toBe('food');
  });

  it('clears inactive selected categories after catalog refreshes', () => {
    expect(
      resolveExploreCategorySlug(buildNoteCatalog(catalogs()), 'travel'),
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
