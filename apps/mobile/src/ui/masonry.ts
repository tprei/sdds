export function splitIntoColumns<T>(
  items: readonly T[],
  estimateHeight: (item: T) => number,
  columnCount = 2,
): T[][] {
  const columns: T[][] = Array.from({ length: columnCount }, () => []);
  const heights = new Array<number>(columnCount).fill(0);
  for (const item of items) {
    let target = 0;
    for (let index = 1; index < columnCount; index += 1) {
      if (heights[index] < heights[target]) target = index;
    }
    columns[target].push(item);
    heights[target] += estimateHeight(item);
  }
  return columns;
}
