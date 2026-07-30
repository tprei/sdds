import { describe, expect, it } from 'vitest';

import { categoryHueFor, semanticColors } from '@sdds/tokens';

describe('design tokens invariants', () => {
  it('resolves the hue for a category slug the catalog covers', () => {
    expect(categoryHueFor('food')).toEqual({
      ink: '#086B37',
      background: '#E1F5E8',
    });
  });

  it('fails explicitly for a catalog category slug with no configured hue', () => {
    const uncoveredCatalogCategory = {
      active: true,
      displayOrder: 5,
      label: 'Bem-estar',
      slug: 'wellness',
    };
    expect(categoryHueFor(uncoveredCatalogCategory.slug)).toBeNull();
  });

  it('locks the palette v2 accent and app background', () => {
    expect(semanticColors.accent).toBe('#0B8043');
    expect(semanticColors.appBackground).toBe('#FBF1DC');
  });
});
