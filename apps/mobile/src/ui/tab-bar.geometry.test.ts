import { describe, expect, it } from 'vitest';

import { spacing } from '@sdds/tokens';

import { TAB_BAR_SLOT_COUNT, tabSlotLayout } from './tab-bar.geometry';

// The bar's real horizontal padding (tab-bar.styles.ts's `bar.paddingHorizontal`).
const PADDING = spacing.sp2;

const BAR_WIDTHS = [360, 375, 390, 414, 430];

describe('tabSlotLayout', () => {
  it.each(BAR_WIDTHS)('splits a %dpx bar into %d equal slots with uniform gaps', (barWidth) => {
    const { centers, slotWidth } = tabSlotLayout(barWidth, TAB_BAR_SLOT_COUNT, PADDING);

    expect(centers.length).toBe(TAB_BAR_SLOT_COUNT);
    expect(slotWidth).toBeCloseTo((barWidth - 2 * PADDING) / TAB_BAR_SLOT_COUNT, 10);

    const gaps = centers.slice(1).map((center, index) => center - centers[index]);
    expect(gaps).toHaveLength(TAB_BAR_SLOT_COUNT - 1);
    for (const gap of gaps) {
      // Every gap must equal the slot width itself: that is what "equal
      // slots" means for a row of adjacent, identically sized items.
      expect(gap).toBeCloseTo(slotWidth, 10);
    }
    expect(Math.max(...gaps) - Math.min(...gaps)).toBeLessThan(0.5);
  });

  it('matches the hand-derived 390px worked example (usable 382, slot 76.4)', () => {
    const { centers, slotWidth } = tabSlotLayout(390, TAB_BAR_SLOT_COUNT, PADDING);
    expect(slotWidth).toBeCloseTo(76.4, 10);
    expect(centers.map((center) => Math.round(center * 100) / 100)).toEqual([
      42.2, 118.6, 195, 271.4, 347.8,
    ]);
  });

  it('matches the hand-derived 430px worked example (usable 422, slot 84.4)', () => {
    const { centers, slotWidth } = tabSlotLayout(430, TAB_BAR_SLOT_COUNT, PADDING);
    expect(slotWidth).toBeCloseTo(84.4, 10);
    expect(centers.map((center) => Math.round(center * 100) / 100)).toEqual([
      46.2, 130.6, 215, 299.4, 383.8,
    ]);
  });

  it('places the FAB slot (index 2) exactly at the bar center', () => {
    const { centers } = tabSlotLayout(390, TAB_BAR_SLOT_COUNT, PADDING);
    expect(centers[2]).toBeCloseTo(390 / 2, 10);
  });
});
