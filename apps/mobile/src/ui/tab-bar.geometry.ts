/**
 * The bottom tab bar lays out TAB_BAR_SLOT_COUNT equal-width slots in a row:
 * two tab buttons, the FAB slot, then two more tab buttons. Every slot shares
 * the same flex weight in tab-bar.styles.ts, so this constant documents (and
 * tab-bar.geometry.test.ts locks) the count that math assumes.
 */
export const TAB_BAR_SLOT_COUNT = 5;

/**
 * Computes where `itemCount` equal-width slots land inside a bar of
 * `barWidth`, once `padding` is removed from each horizontal edge. Mirrors
 * the geometry that CSS flexbox produces when every slot shares the same
 * `flex: 1` weight: each slot gets `(barWidth - 2 * padding) / itemCount`,
 * so every gap between adjacent slot centers is identical.
 */
export function tabSlotLayout(
  barWidth: number,
  itemCount: number,
  padding: number,
): { centers: number[]; slotWidth: number } {
  const usableWidth = barWidth - padding * 2;
  const slotWidth = usableWidth / itemCount;
  const centers = Array.from(
    { length: itemCount },
    (_, index) => padding + slotWidth * (index + 0.5),
  );
  return { centers, slotWidth };
}
