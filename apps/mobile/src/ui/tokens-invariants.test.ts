import { describe, expect, it } from 'vitest';

import { categoryColors, semanticColors } from '@sdds/tokens';

describe('design tokens invariants', () => {
  it('covers every catalog slug with a category hue', () => {
    expect(Object.keys(categoryColors).sort()).toEqual(['beauty', 'finds', 'food', 'travel']);
  });

  it('locks the palette v2 accent and app background', () => {
    expect(semanticColors.accent).toBe('#0B8043');
    expect(semanticColors.appBackground).toBe('#FBF1DC');
  });
});
