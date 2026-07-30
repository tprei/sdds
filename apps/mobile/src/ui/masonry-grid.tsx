import type { ReactNode } from 'react';
import { View } from 'react-native';

import { splitIntoColumns } from './masonry';
import { styles } from './masonry-grid.styles';

export type MasonryGridProps<T> = {
  items: readonly T[];
  columnCount: number;
  estimateHeight: (item: T) => number;
  renderItem: (item: T) => ReactNode;
  keyFor: (item: T) => string;
};

export function MasonryGrid<T>({
  items,
  columnCount,
  estimateHeight,
  renderItem,
  keyFor,
}: MasonryGridProps<T>) {
  const columns = splitIntoColumns(items, estimateHeight, columnCount);
  return (
    <View style={styles.row} testID="masonry-grid">
      {columns.map((column, columnIndex) => (
        <View
          key={columnIndex}
          style={styles.column}
          testID={`masonry-column-${columnIndex}`}
        >
          {column.map((item) => (
            <View key={keyFor(item)}>{renderItem(item)}</View>
          ))}
        </View>
      ))}
    </View>
  );
}
