import { StyleSheet } from 'react-native';

import { spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    paddingHorizontal: spacing.gutter,
    gap: spacing.masonryGap,
  },
  column: {
    flex: 1,
    gap: spacing.masonryGap,
  },
});
