import { describe, expect, it } from 'vitest';

import { spacing } from '@sdds/tokens';

import { resolveGridLayout } from './grid-layout';

describe('resolveGridLayout', () => {
  it('keeps columns and gaps summing to the content width across a swept range of widths', () => {
    for (let width = 200; width <= 1400; width += 1) {
      const { contentWidth, columnCount, columnWidth } = resolveGridLayout(width);
      const reconstructed =
        columnCount * columnWidth + (columnCount - 1) * spacing.masonryGap + 2 * spacing.gutter;
      expect(reconstructed).toBeCloseTo(contentWidth, 10);
    }
  });

  it('never exceeds spacing.maxAppWidth for the content width', () => {
    for (let width = 200; width <= 1400; width += 1) {
      expect(resolveGridLayout(width).contentWidth).toBeLessThanOrEqual(spacing.maxAppWidth);
    }
  });

  it('matches the current 390px phone math exactly', () => {
    expect(resolveGridLayout(390)).toEqual({
      contentWidth: 390,
      columnCount: 2,
      columnWidth: 173,
    });
  });

  it('clamps a wide viewport to maxAppWidth instead of stretching', () => {
    expect(resolveGridLayout(820)).toEqual({
      contentWidth: spacing.maxAppWidth,
      columnCount: 2,
      columnWidth: 193,
    });
  });

  it('clamps degenerate widths so columnWidth never goes negative', () => {
    expect(resolveGridLayout(0)).toEqual({ contentWidth: 0, columnCount: 2, columnWidth: 0 });
    expect(resolveGridLayout(-100)).toEqual({ contentWidth: 0, columnCount: 2, columnWidth: 0 });
    expect(resolveGridLayout(40).columnWidth).toBe(0);
  });
});
