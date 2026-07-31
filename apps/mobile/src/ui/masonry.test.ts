import { describe, expect, it } from 'vitest';

import { splitIntoColumns } from './masonry';

describe('splitIntoColumns', () => {
  it('preserves input order within each column', () => {
    const columns = splitIntoColumns([1, 2, 3, 4], () => 10);
    expect(columns[0]).toEqual([1, 3]);
    expect(columns[1]).toEqual([2, 4]);
  });

  it('fills the left column first when heights are equal', () => {
    const columns = splitIntoColumns(['a', 'b'], () => 5);
    expect(columns[0]).toEqual(['a']);
    expect(columns[1]).toEqual(['b']);
  });

  it('routes later items to the shorter column after a tall first item', () => {
    const columns = splitIntoColumns([1, 2, 3], (item) => (item === 1 ? 100 : 10));
    expect(columns[0]).toEqual([1]);
    expect(columns[1]).toEqual([2, 3]);
  });

  it('is deterministic across calls for the same input and estimator', () => {
    const estimator = (item: number) => item * 10;
    expect(splitIntoColumns([1, 2, 3, 4, 5], estimator)).toEqual(
      splitIntoColumns([1, 2, 3, 4, 5], estimator),
    );
  });

  it('returns a single column equal to the input when columnCount is 1', () => {
    expect(splitIntoColumns([1, 2, 3], () => 9, 1)).toEqual([[1, 2, 3]]);
  });
});
