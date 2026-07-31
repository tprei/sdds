import { spacing } from '@sdds/tokens';

const columnCount = 2;

/**
 * Resolves the two-column masonry grid geometry for a given viewport width.
 *
 * The content area is capped at `spacing.maxAppWidth` so the grid stays at
 * the density #180 was drawn for instead of stretching two cards across a
 * wide viewport. Column count is fixed at 2 — adapting it to more columns
 * is a separate, unreviewed design decision.
 *
 * A viewport narrower than the combined gutters and gap has no room for
 * content; `columnWidth` clamps at 0 rather than going negative.
 */
export function resolveGridLayout(viewportWidth: number): {
  contentWidth: number;
  columnCount: number;
  columnWidth: number;
} {
  const contentWidth = Math.max(0, Math.min(viewportWidth, spacing.maxAppWidth));
  const columnWidth = Math.max(
    0,
    (contentWidth - 2 * spacing.gutter - spacing.masonryGap) / columnCount,
  );
  return { contentWidth, columnCount, columnWidth };
}
