import type { ReactNode } from 'react';
import { View } from 'react-native';

import { splitIntoColumns } from './masonry';
import { styles } from './masonry-grid.styles';

export type MasonryGridProps<T> = {
  items: readonly T[];
  estimateHeight: (item: T) => number;
  renderItem: (item: T) => ReactNode;
  keyFor: (item: T) => string;
};

export function MasonryGrid<T>({
  items,
  estimateHeight,
  renderItem,
  keyFor,
}: MasonryGridProps<T>) {
  const columns = splitIntoColumns(items, estimateHeight, 2);
  return (
    <View style={styles.row}>
      {columns.map((column, columnIndex) => (
        <View key={columnIndex} style={styles.column}>
          {column.map((item) => (
            <View key={keyFor(item)}>{renderItem(item)}</View>
          ))}
        </View>
      ))}
    </View>
  );
}
